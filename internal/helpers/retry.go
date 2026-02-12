package helpers

import (
	"context"
	"fmt"
	"time"
)

// Retry configuration for handling Ory API eventual consistency.
// After a successful PatchProject, GetProject may not immediately reflect the change.
// These values are shared across all resources to ensure consistent behavior.
const (
	// EventualConsistencyMaxAttempts is the number of polling attempts for
	// read-after-write checks in Create/Update operations.
	EventualConsistencyMaxAttempts = 10

	// EventualConsistencyDelay is the fixed delay between polling attempts.
	EventualConsistencyDelay = 500 * time.Millisecond

	// ReadRetryMaxAttempts is the number of retry attempts for Read operations
	// (used with exponential backoff during refresh/import).
	ReadRetryMaxAttempts = 5
)

// WaitForCondition polls a check function until it returns true or maxAttempts is exhausted.
// This handles eventual consistency where a write operation succeeds but subsequent reads
// don't immediately reflect the change.
//
// The check function should return (true, nil) when the condition is met,
// (false, nil) to keep retrying, or (false, err) to abort immediately.
func WaitForCondition(ctx context.Context, check func() (bool, error)) error {
	for i := 0; i < EventualConsistencyMaxAttempts; i++ {
		ok, err := check()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if i < EventualConsistencyMaxAttempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(EventualConsistencyDelay):
			}
		}
	}
	return fmt.Errorf("condition not met after %d attempts (%v total)",
		EventualConsistencyMaxAttempts,
		time.Duration(EventualConsistencyMaxAttempts)*EventualConsistencyDelay)
}
