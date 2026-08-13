package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ory/terraform-provider-ory/internal/testutil"
)

// recordingSleeper stands in for the real wait so a test runs instantly. It
// records every duration the transport asked for.
type recordingSleeper struct {
	mu     sync.Mutex
	waits  []time.Duration
	err    error
	errAt  int
	called int
}

func (s *recordingSleeper) sleep(_ context.Context, d time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.called++
	s.waits = append(s.waits, d)
	if s.err != nil && s.called >= s.errAt {
		return s.err
	}
	return nil
}

func (s *recordingSleeper) recorded() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Duration, len(s.waits))
	copy(out, s.waits)
	return out
}

// newTestTransport builds a transport with no jitter and no real wait, so a
// test can assert on exact durations.
func newTestTransport(base http.RoundTripper, maxRetries int, sleeper *recordingSleeper) *rateLimitTransport {
	t := newRateLimitTransport(base, maxRetries)
	t.sleep = sleeper.sleep
	t.randFloat = func() float64 { return 0 }
	return t
}

// rateLimitServer answers with 429 for the first rejectCount requests and then
// with 200. It records every request it saw.
type rateLimitServer struct {
	rejectCount int32
	seen        int32
	bodies      []string
	headers     http.Header
	mu          sync.Mutex
}

func (s *rateLimitServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.bodies = append(s.bodies, string(body))
		s.mu.Unlock()

		for key, values := range s.headers {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		if atomic.AddInt32(&s.seen, 1) <= atomic.LoadInt32(&s.rejectCount) {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}

func (s *rateLimitServer) sentBodies() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.bodies))
	copy(out, s.bodies)
	return out
}

func TestRateLimitTransport_RetriesUntilSuccess(t *testing.T) {
	backend := &rateLimitServer{rejectCount: 2}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 5, sleeper)}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&backend.seen), "two rejections plus the accepted attempt")
	assert.Len(t, sleeper.recorded(), 2, "one wait per rejection")
}

func TestRateLimitTransport_ReturnsLastResponseWhenRetriesRunOut(t *testing.T) {
	backend := &rateLimitServer{rejectCount: 100}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 2, sleeper)}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err, "an exhausted retry returns the API response, not a transport error")
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&backend.seen), "the first attempt plus two retries")
}

func TestRateLimitTransport_ZeroRetriesSendsOneRequest(t *testing.T) {
	backend := &rateLimitServer{rejectCount: 100}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 0, sleeper)}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&backend.seen))
	assert.Empty(t, sleeper.recorded())
}

func TestRateLimitTransport_ReplaysRequestBody(t *testing.T) {
	backend := &rateLimitServer{rejectCount: 2}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 5, sleeper)}

	const payload = `{"client_name":"tf-bulk"}`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, strings.NewReader(payload))
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []string{payload, payload, payload}, backend.sentBodies(),
		"every attempt must carry the full body, not an empty one")
}

func TestRateLimitTransport_DoesNotRetryWhenBodyCannotReplay(t *testing.T) {
	backend := &rateLimitServer{rejectCount: 100}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 5, sleeper)}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, nil)
	require.NoError(t, err)
	// A reader net/http cannot rewind leaves GetBody nil, so a retry would send
	// an empty body and change the meaning of the call.
	req.Body = io.NopCloser(strings.NewReader(`{"a":1}`))
	req.GetBody = nil

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&backend.seen), "the request is sent once")
	assert.Empty(t, sleeper.recorded())
}

func TestRateLimitTransport_LeavesOriginalRequestUnchanged(t *testing.T) {
	backend := &rateLimitServer{rejectCount: 1}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 5, sleeper)}

	const payload = `{"a":1}`
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("X-Test", "kept")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "kept", req.Header.Get("X-Test"))
	// Every attempt sends its own copy, so the caller's body stays unread and
	// the request can be sent again.
	body, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	assert.Equal(t, payload, string(body), "RoundTrip must not consume the caller's body")
}

func TestRateLimitTransport_HonoursResetHeader(t *testing.T) {
	backend := &rateLimitServer{
		rejectCount: 1,
		headers:     http.Header{headerRateLimitReset: []string{"4"}},
	}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 5, sleeper)}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, []time.Duration{4 * time.Second}, sleeper.recorded(),
		"a reported window longer than the exponential step wins")
}

func TestRateLimitTransport_IgnoresResetShorterThanBackoff(t *testing.T) {
	// The Ory API answers a rejected request on a sub-second bucket with
	// x-ratelimit-reset: 0. A wait taken from that header alone would spend
	// every retry inside one window. See issue #327.
	backend := &rateLimitServer{
		rejectCount: 2,
		headers:     http.Header{headerRateLimitReset: []string{"0", "0"}},
	}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 5, sleeper)}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []time.Duration{1 * time.Second, 2 * time.Second}, sleeper.recorded(),
		"the exponential step is the floor, so a reset of 0 does not shorten the wait")
}

func TestRateLimitTransport_FallsBackToExponentialBackoff(t *testing.T) {
	backend := &rateLimitServer{rejectCount: 3}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 5, sleeper)}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}, sleeper.recorded())
}

func TestRateLimitTransport_StopsWaitingOnCancelledContext(t *testing.T) {
	backend := &rateLimitServer{rejectCount: 100}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{err: context.Canceled, errAt: 1}
	transport := newTestTransport(srv.Client().Transport, 5, sleeper)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req) //nolint:bodyclose // the canceled wait returns no response to close
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(1), atomic.LoadInt32(&backend.seen))
}

func TestRateLimitTransport_ReturnsTransportError(t *testing.T) {
	sleeper := &recordingSleeper{}
	wantErr := errors.New("dial failed")
	transport := newTestTransport(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, wantErr
	}), 5, sleeper)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.invalid", nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req) //nolint:bodyclose // the failed transport returns no response to close
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, wantErr)
	assert.Empty(t, sleeper.recorded(), "a network error is not a rate limit")
}

func TestRateLimitTransport_ThrottlesWhenBudgetIsSpent(t *testing.T) {
	backend := &rateLimitServer{
		headers: http.Header{
			headerRateLimitRemaining: []string{"0", "480"},
			headerRateLimitReset:     []string{"0", "0"},
		},
	}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 5, sleeper)}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []time.Duration{rateLimitBaseDelay}, sleeper.recorded(),
		"an empty bucket pauses the worker before it spends a request on a certain 429")
}

func TestRateLimitTransport_ThrottlePauseIsCapped(t *testing.T) {
	backend := &rateLimitServer{
		headers: http.Header{
			headerRateLimitRemaining: []string{"0"},
			headerRateLimitReset:     []string{"60"},
		},
	}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 5, sleeper)}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, []time.Duration{rateLimitThrottleMaxDelay}, sleeper.recorded(),
		"a pause protects the next request; it must not stall the apply")
}

func TestRateLimitTransport_DoesNotThrottleWithBudgetLeft(t *testing.T) {
	backend := &rateLimitServer{
		headers: http.Header{
			headerRateLimitRemaining: []string{"9", "480"},
			headerRateLimitReset:     []string{"1"},
		},
	}
	srv := httptest.NewServer(backend.handler())
	defer srv.Close()

	sleeper := &recordingSleeper{}
	client := &http.Client{Transport: newTestTransport(srv.Client().Transport, 5, sleeper)}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Empty(t, sleeper.recorded())
}

func TestRateLimitTransport_JitterStaysInsideItsShare(t *testing.T) {
	transport := newRateLimitTransport(nil, DefaultMaxRetries)
	header := http.Header{headerRateLimitReset: []string{"2"}}

	for i := 0; i < 200; i++ {
		wait := transport.waitFor(header, 0)
		assert.GreaterOrEqual(t, wait, 2*time.Second, "an additive jitter never shortens the reported window")
		assert.LessOrEqual(t, wait, 3*time.Second, "the jitter adds at most half the wait")
	}
}

func TestRateLimitTransport_JitterSpreadsParallelRetries(t *testing.T) {
	transport := newRateLimitTransport(nil, DefaultMaxRetries)
	header := http.Header{headerRateLimitReset: []string{"1"}}

	distinct := map[time.Duration]struct{}{}
	for i := 0; i < 50; i++ {
		distinct[transport.waitFor(header, 0)] = struct{}{}
	}
	assert.Greater(t, len(distinct), 1, "parallel workers must not wake at the same instant")
}

func TestRateLimitTransport_WaitForCapsAndFloors(t *testing.T) {
	transport := newRateLimitTransport(nil, DefaultMaxRetries)
	transport.randFloat = func() float64 { return 0 }

	tests := []struct {
		name    string
		header  http.Header
		attempt int
		want    time.Duration
	}{
		{
			name:   "retry after in seconds wins over the reset window",
			header: http.Header{headerRetryAfter: []string{"7"}, headerRateLimitReset: []string{"2"}},
			want:   7 * time.Second,
		},
		{
			name:   "longest reset window wins across limit buckets",
			header: http.Header{headerRateLimitReset: []string{"1", "12", "3"}},
			want:   12 * time.Second,
		},
		{
			name:   "a reset window longer than the cap is trimmed to 30 s",
			header: http.Header{headerRateLimitReset: []string{"600"}},
			want:   rateLimitMaxDelay,
		},
		{
			name:   "a reset of zero leaves the exponential step in place",
			header: http.Header{headerRateLimitReset: []string{"0"}},
			want:   rateLimitBaseDelay,
		},
		{
			name:    "a reset shorter than the exponential step does not shorten it",
			header:  http.Header{headerRateLimitReset: []string{"1"}},
			attempt: 3,
			want:    8 * time.Second,
		},
		{
			name:   "an unparsable header falls back to the exponential step",
			header: http.Header{headerRateLimitReset: []string{"soon"}},
			want:   rateLimitBaseDelay,
		},
		{
			name:    "the exponential fallback stops at the cap",
			header:  http.Header{},
			attempt: 9,
			want:    rateLimitMaxDelay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, transport.waitFor(tt.header, tt.attempt))
		})
	}
}

func TestRetryAfterDelay(t *testing.T) {
	t.Run("http date", func(t *testing.T) {
		when := time.Now().Add(5 * time.Second).UTC()
		header := http.Header{headerRetryAfter: []string{when.Format(http.TimeFormat)}}

		delay, ok := retryAfterDelay(header)
		require.True(t, ok)
		assert.InDelta(t, (5 * time.Second).Seconds(), delay.Seconds(), 1.5)
	})

	t.Run("date in the past", func(t *testing.T) {
		when := time.Now().Add(-1 * time.Minute).UTC()
		header := http.Header{headerRetryAfter: []string{when.Format(http.TimeFormat)}}

		delay, ok := retryAfterDelay(header)
		require.True(t, ok)
		assert.Equal(t, time.Duration(0), delay)
	})

	t.Run("absent", func(t *testing.T) {
		_, ok := retryAfterDelay(http.Header{})
		assert.False(t, ok)
	})

	t.Run("negative seconds", func(t *testing.T) {
		_, ok := retryAfterDelay(http.Header{headerRetryAfter: []string{"-3"}})
		assert.False(t, ok)
	})
}

func TestLowestRemaining(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   int
		found  bool
	}{
		{name: "single bucket", values: []string{"12"}, want: 12, found: true},
		{name: "smallest bucket wins", values: []string{"480", "0", "9"}, want: 0, found: true},
		{name: "unparsable values are skipped", values: []string{"many", "3"}, want: 3, found: true},
		{name: "no header", values: nil, found: false},
		{name: "only unparsable values", values: []string{"many"}, found: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			for _, value := range tt.values {
				header.Add(headerRateLimitRemaining, value)
			}
			got, ok := lowestRemaining(header)
			assert.Equal(t, tt.found, ok)
			if tt.found {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestOryClientConfig_MaxRetries(t *testing.T) {
	zero, five := 0, 5

	assert.Equal(t, DefaultMaxRetries, OryClientConfig{}.maxRetries(), "an unset value uses the default")
	assert.Equal(t, 0, OryClientConfig{MaxRetries: &zero}.maxRetries(), "an explicit zero turns the retry off")
	assert.Equal(t, 5, OryClientConfig{MaxRetries: &five}.maxRetries())
}

func TestOryClientConfig_EqualComparesMaxRetries(t *testing.T) {
	zero, five, unset := 0, 5, (*int)(nil)

	assert.True(t, OryClientConfig{MaxRetries: &five}.Equal(OryClientConfig{MaxRetries: &five}))
	assert.False(t, OryClientConfig{MaxRetries: &five}.Equal(OryClientConfig{MaxRetries: &zero}))
	assert.True(t, OryClientConfig{MaxRetries: unset}.Equal(OryClientConfig{}),
		"an unset value and the default are the same client")
}

func TestNewOryClient_UsesRateLimitTransport(t *testing.T) {
	two := 2
	client, err := NewOryClient(OryClientConfig{
		WorkspaceAPIKey: testutil.TestWorkspaceAPIKey,
		ConsoleAPIURL:   DefaultConsoleAPIURL,
		ProjectAPIKey:   testutil.TestProjectAPIKey,
		ProjectSlug:     testutil.TestProjectSlug,
		MaxRetries:      &two,
	})
	require.NoError(t, err)

	transport, ok := client.httpClient.Transport.(*rateLimitTransport)
	require.True(t, ok, "the provider client must retry a rate limit")
	assert.Equal(t, 2, transport.maxRetries)

	assert.Same(t, client.httpClient, client.consoleClient.GetConfig().HTTPClient,
		"console SDK calls go through the retrying client")
	assert.Same(t, client.httpClient, client.projectClient.GetConfig().HTTPClient,
		"project SDK calls go through the retrying client")
	assert.Same(t, client.httpClient, client.rawHTTPClient(),
		"raw console calls go through the retrying client")
}

func TestOryClient_RawHTTPClientFallsBackToPackageDefault(t *testing.T) {
	assert.Same(t, consoleHTTPClient, (&OryClient{}).rawHTTPClient())
}

// tokenBucket models the burst limit the Ory API enforces: a window holds a
// fixed number of requests, and the window refills when time passes.
type tokenBucket struct {
	mu       sync.Mutex
	tokens   int
	size     int
	rejected int
}

// take spends a token and reports whether one was left.
func (b *tokenBucket) take() (remaining int, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tokens == 0 {
		b.rejected++
		return 0, false
	}
	b.tokens--
	return b.tokens, true
}

// refill starts a new window.
func (b *tokenBucket) refill() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tokens = b.size
}

func (b *tokenBucket) rejectedCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.rejected
}

// TestRateLimitTransport_ParallelWorkersAllSucceed reproduces the shape of the
// bug in issue #327: many Terraform workers write at once, the API rejects the
// requests over its budget, and every worker must still finish.
func TestRateLimitTransport_ParallelWorkersAllSucceed(t *testing.T) {
	const workers = 20

	bucket := &tokenBucket{tokens: 3, size: 3}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remaining, ok := bucket.take()
		w.Header().Set(headerRateLimitReset, "1")
		w.Header().Set(headerRateLimitRemaining, strconv.Itoa(remaining))
		if !ok {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	sleeper := &recordingSleeper{}
	transport := newTestTransport(srv.Client().Transport, 10, sleeper)
	// A wait means the limit window passed, so the bucket starts over. The
	// stand-in keeps the test instant.
	transport.sleep = func(ctx context.Context, d time.Duration) error {
		bucket.refill()
		return sleeper.sleep(ctx, d)
	}
	client := &http.Client{Transport: transport}

	var wg sync.WaitGroup
	statuses := make([]int, workers)
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL,
				strings.NewReader(`{"client_name":"tf-bulk"}`))
			if err != nil {
				errs[index] = err
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				errs[index] = err
				return
			}
			statuses[index] = resp.StatusCode
			_ = resp.Body.Close()
		}(i)
	}
	wg.Wait()

	for i := 0; i < workers; i++ {
		require.NoError(t, errs[i], "worker %d", i)
		assert.Equal(t, http.StatusOK, statuses[i], "worker %d must finish despite the rate limit", i)
	}
	assert.Positive(t, bucket.rejectedCount(), "the test only proves something if the API rejected some requests")
}

// roundTripperFunc adapts a function to http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
