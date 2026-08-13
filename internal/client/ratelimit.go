package client

import (
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Rate-limit handling.
//
// The Ory API rejects a request with HTTP 429 when the caller exceeds the
// request budget for a route. Terraform runs resource operations in parallel,
// ten at a time by default, so a bulk apply reaches that budget quickly. Before
// this transport existed, the provider surfaced the 429 as a hard error and
// aborted the apply with the resources half-written.
//
// rateLimitTransport absorbs the 429 inside the HTTP client, so every SDK call
// and every raw console call retries without a change at the call site. The
// wait follows Ory's guidance at
// https://www.ory.com/docs/guides/rate-limits-new#how-to-handle-429-responses:
//
//  1. Back off exponentially, capped at 30 s.
//  2. Wait longer when the x-ratelimit-reset header reports a longer window.
//  3. Add random jitter, so parallel workers do not retry in lockstep.
//  4. Slow down before the budget runs out, based on x-ratelimit-remaining.
//
// The exponential step is the floor, not the fallback, because the Ory API
// reports x-ratelimit-reset: 0 on the sub-second buckets it rejects most often.
// A wait taken from that header alone lasts a few hundred milliseconds, spends
// every retry inside one window, and fails the apply anyway.

const (
	// DefaultMaxRetries is how many more times the transport sends a request
	// that the API rejected with HTTP 429. Ory meters each route with a
	// one-second burst bucket and a sixty-second sustained bucket, so six
	// retries are the smallest number whose waits (1 s, 2 s, 4 s, 8 s, 16 s and
	// 30 s, 61 s in total) outlast a full sustained window.
	DefaultMaxRetries = 6

	// MaxRetriesUpperBound is the largest value the provider accepts for
	// max_retries. It stops a typo from stalling an apply for hours.
	MaxRetriesUpperBound = 20
)

const (
	// rateLimitBaseDelay is the first step of the exponential backoff. It
	// applies when the response carries no usable rate-limit header.
	rateLimitBaseDelay = 1 * time.Second
	// rateLimitMaxDelay caps a single wait, as Ory's 429 guidance requires.
	rateLimitMaxDelay = 30 * time.Second
	// rateLimitJitterShare is the fraction of the wait added as random jitter.
	// The jitter is additive, so a wait never ends before the window the server
	// reported.
	rateLimitJitterShare = 0.5
	// rateLimitThrottleAt is the x-ratelimit-remaining value at or below which
	// the transport pauses after a successful response. The pause costs the same
	// as a retry but does not spend a request, and Ory blocks a client that
	// keeps exceeding its limit.
	rateLimitThrottleAt = 1
	// rateLimitThrottleMaxDelay caps a proactive pause. A pause protects the
	// next request; it must not stall the apply the way a real 429 wait can.
	rateLimitThrottleMaxDelay = 2 * time.Second
	// rateLimitDrainLimit is how much of a rejected response body the transport
	// reads before it closes the body, so the connection returns to the pool.
	// Ory sends an empty body with a 429, so the limit only bounds an
	// unexpected large one.
	rateLimitDrainLimit = 4 << 10
	// backoffShiftLimit bounds the exponential shift, so the delay cannot
	// overflow before the cap applies.
	backoffShiftLimit = 20
)

// Rate-limit response headers. Ory documents all three at
// https://www.ory.com/docs/guides/rate-limits-new. A response carries one set
// per limit bucket, for example a per-route bucket and a global bucket, so the
// transport reads every value of a header and not only the first.
const (
	headerRateLimitRemaining = "X-Ratelimit-Remaining"
	headerRateLimitReset     = "X-Ratelimit-Reset"
	// headerRetryAfter is the standard fallback. Ory does not send it today, so
	// the transport prefers it only because it is unambiguous when present.
	headerRetryAfter = "Retry-After"
)

// rateLimitTransport retries a request that the Ory API rejects with HTTP 429.
// It wraps a base RoundTripper and adds no connection pool of its own, so many
// transports can share one pool.
type rateLimitTransport struct {
	// base sends the request. A nil base means http.DefaultTransport.
	base http.RoundTripper
	// maxRetries is how many more times a rejected request is sent. Zero
	// disables the retry and leaves the proactive throttle in place.
	maxRetries int
	// sleep waits for d or until ctx is done. Tests replace it so a case runs
	// without a real wait.
	sleep func(ctx context.Context, d time.Duration) error
	// randFloat returns a number in [0, 1) for the jitter. Tests replace it to
	// make a wait deterministic.
	randFloat func() float64
}

// newRateLimitTransport returns a transport that retries a 429 up to maxRetries
// times on top of base. A nil base means http.DefaultTransport, which keeps the
// shared connection pool the Ory SDK uses by default.
func newRateLimitTransport(base http.RoundTripper, maxRetries int) *rateLimitTransport {
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &rateLimitTransport{base: base, maxRetries: maxRetries}
}

// newOryHTTPClient returns the HTTP client the provider uses for every call to
// the Ory API. The client carries no overall timeout, because that budget would
// also cover the waits between retries and cut a legitimate backoff short. The
// per-attempt guard lives on the transport instead.
func newOryHTTPClient(maxRetries int) *http.Client {
	return &http.Client{Transport: newRateLimitTransport(oryBaseTransport(), maxRetries)}
}

// oryBaseTransport returns the transport that sends a request to the Ory API.
// It clones the standard transport, so the connection pool, the proxy settings,
// and the dial timeouts stay as the Go runtime configures them, and adds a
// per-attempt response-header timeout. That timeout is the stall guard: an
// overall http.Client.Timeout cannot serve that role here, because it also
// covers the waits between retries.
func oryBaseTransport() http.RoundTripper {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}
	cloned := transport.Clone()
	cloned.ResponseHeaderTimeout = 30 * time.Second
	return cloned
}

// RoundTrip sends the request and retries it while the API answers HTTP 429.
// It returns the last response when the retries run out, so the caller still
// sees the API's own error.
func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	// Every attempt sends a copy that carries its own body, so the body the
	// caller handed over is never read and never closed by the base transport.
	// Close it here, as the RoundTripper contract requires.
	if req.Body != nil && req.GetBody != nil {
		defer func() { _ = req.Body.Close() }()
	}

	for attempt := 0; ; attempt++ {
		// A RoundTripper must not change the request it is given, and a retry
		// needs an unread body, so every attempt sends its own copy.
		attemptReq, err := cloneRequest(req)
		if err != nil {
			return nil, err
		}

		resp, err := base.RoundTrip(attemptReq)
		if err != nil {
			return resp, err
		}

		if resp.StatusCode != http.StatusTooManyRequests ||
			attempt >= t.maxRetries ||
			!canReplayBody(req) {
			t.throttleBeforeNext(req.Context(), resp)
			return resp, nil
		}

		wait := t.waitFor(resp.Header, attempt)
		tflog.Debug(req.Context(), "Ory API rate limit reached; retrying after backoff", map[string]interface{}{
			"method":      req.Method,
			"request_url": req.URL.Redacted(),
			"attempt":     attempt + 1,
			"max_retries": t.maxRetries,
			"wait_ms":     wait.Milliseconds(),
		})
		drainAndClose(resp.Body)

		if err := t.wait(req.Context(), wait); err != nil {
			return nil, err
		}
	}
}

// cloneRequest copies req and gives the copy an unread body. A request whose
// body cannot be replayed keeps the original body, and canReplayBody stops the
// retry before that copy is sent twice.
func cloneRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	if req.Body == nil || req.GetBody == nil {
		return clone, nil
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	clone.Body = body
	return clone, nil
}

// canReplayBody reports whether a second attempt can send the same body. A
// request built by the Ory SDK or by doConsoleRawRequest always can, because
// net/http fills in GetBody for the buffer types both use. A request that
// cannot replay its body is returned to the caller with its 429 intact, because
// a retry would send an empty body and change the meaning of the call.
func canReplayBody(req *http.Request) bool {
	return req.Body == nil || req.GetBody != nil
}

// waitFor returns how long to wait before the next attempt. The exponential
// step sets the floor, the window the server reports raises it when that window
// is longer, the cap trims the result, and the jitter spreads parallel workers.
func (t *rateLimitTransport) waitFor(header http.Header, attempt int) time.Duration {
	delay := backoffDelay(attempt)
	if reported, ok := reportedDelay(header); ok && reported > delay {
		delay = reported
	}
	if delay > rateLimitMaxDelay {
		delay = rateLimitMaxDelay
	}
	return delay + t.jitter(delay)
}

// reportedDelay returns the wait the server asked for, from Retry-After when it
// is present and from x-ratelimit-reset otherwise.
func reportedDelay(header http.Header) (time.Duration, bool) {
	if delay, ok := retryAfterDelay(header); ok {
		return delay, true
	}
	return resetDelay(header)
}

// throttleBeforeNext pauses when the response shows the request budget is spent,
// so the next call from the same Terraform worker starts in a fresh window
// instead of being rejected. Ory recommends this over waiting for the 429,
// because a client that keeps exceeding its limit can lose API access.
func (t *rateLimitTransport) throttleBeforeNext(ctx context.Context, resp *http.Response) {
	if resp.StatusCode == http.StatusTooManyRequests {
		// The retry loop already waited for this response, or the retries ran
		// out. Either way, a further pause helps nobody.
		return
	}
	remaining, ok := lowestRemaining(resp.Header)
	if !ok || remaining > rateLimitThrottleAt {
		return
	}
	delay, ok := reportedDelay(resp.Header)
	if !ok || delay < rateLimitBaseDelay {
		// The Ory API reports a reset of 0 on a sub-second bucket, which asks
		// for a wait too short to let the window turn over.
		delay = rateLimitBaseDelay
	}
	if delay > rateLimitThrottleMaxDelay {
		delay = rateLimitThrottleMaxDelay
	}
	pause := delay + t.jitter(delay)
	tflog.Debug(ctx, "Ory API request budget nearly spent; pausing before the next request", map[string]interface{}{
		"remaining": remaining,
		"wait_ms":   pause.Milliseconds(),
	})
	// A canceled context ends the pause. The response is already in hand, so
	// the caller still gets it and decides what the cancellation means.
	_ = t.wait(ctx, pause)
}

// backoffDelay returns the exponential step for an attempt, starting at
// rateLimitBaseDelay and doubling each time.
func backoffDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt > backoffShiftLimit {
		return rateLimitMaxDelay
	}
	return rateLimitBaseDelay << uint(attempt)
}

// jitter returns a random share of d, up to rateLimitJitterShare of it.
func (t *rateLimitTransport) jitter(d time.Duration) time.Duration {
	random := t.randFloat
	if random == nil {
		// #nosec G404 -- the jitter only spreads retries apart in time and
		// carries no security meaning, so the fast pseudo-random source fits.
		random = rand.Float64
	}
	return time.Duration(random() * rateLimitJitterShare * float64(d))
}

// wait blocks for d, or returns the context error when the caller cancels first.
func (t *rateLimitTransport) wait(ctx context.Context, d time.Duration) error {
	if t.sleep != nil {
		return t.sleep(ctx, d)
	}
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// retryAfterDelay reads a Retry-After header, in seconds or as an HTTP date.
func retryAfterDelay(header http.Header) (time.Duration, bool) {
	value := header.Get(headerRetryAfter)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	delay := time.Until(when)
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

// resetDelay reads x-ratelimit-reset, the number of seconds until the limit
// window resets. A response carries one value per limit bucket, so the longest
// window wins: a shorter wait would run into the bucket that is still full.
func resetDelay(header http.Header) (time.Duration, bool) {
	longest := time.Duration(0)
	found := false
	for _, value := range header.Values(headerRateLimitReset) {
		seconds, err := strconv.Atoi(value)
		if err != nil || seconds < 0 {
			continue
		}
		found = true
		if delay := time.Duration(seconds) * time.Second; delay > longest {
			longest = delay
		}
	}
	return longest, found
}

// lowestRemaining reads x-ratelimit-remaining and returns the smallest value.
// A response carries one value per limit bucket, and the bucket closest to
// empty is the one that rejects the next request.
func lowestRemaining(header http.Header) (int, bool) {
	lowest := 0
	found := false
	for _, value := range header.Values(headerRateLimitRemaining) {
		remaining, err := strconv.Atoi(value)
		if err != nil || remaining < 0 {
			continue
		}
		if !found || remaining < lowest {
			lowest = remaining
		}
		found = true
	}
	return lowest, found
}

// drainAndClose reads and closes a rejected response body, so the underlying
// connection goes back to the pool instead of being dropped.
func drainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, rateLimitDrainLimit))
	_ = body.Close()
}
