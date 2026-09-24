package client

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestRetryOnConflictStopsOnNonRetryableError is the regression guard for a
// retry loop that treated "not retryable" as "try again immediately".
//
// The rejected branch fell through to the next iteration instead of returning,
// so an error isRetryable refused still consumed the whole attempt budget --
// with no sleep, because only the retryable path waits. Every call site passes
// 30 attempts, so a single rejected create fired 30 identical POSTs in
// microseconds. Against a create route that is exactly the shape that leaves
// duplicate infrastructure behind.
func TestRetryOnConflictStopsOnNonRetryableError(t *testing.T) {
	calls := 0
	want := errors.New("422 unprocessable")

	err := RetryOnConflict(context.Background(), 30, time.Hour, func() error {
		calls++
		return want
	}, func(error) bool { return false })

	if calls != 1 {
		t.Errorf("operation called %d times, want 1", calls)
	}
	if !errors.Is(err, want) {
		t.Errorf("got error %v, want %v", err, want)
	}
}

func TestRetryOnConflictRetriesUntilSuccess(t *testing.T) {
	calls := 0

	err := RetryOnConflict(context.Background(), 5, time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return errors.New("operation is already in progress")
		}
		return nil
	}, func(error) bool { return true })

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("operation called %d times, want 3", calls)
	}
}

// A retryable error that never clears must stop at the attempt budget and
// return the last error rather than looping forever.
func TestRetryOnConflictExhaustsAttempts(t *testing.T) {
	calls := 0
	want := errors.New("operation is already in progress")

	err := RetryOnConflict(context.Background(), 4, time.Millisecond, func() error {
		calls++
		return want
	}, func(error) bool { return true })

	if calls != 4 {
		t.Errorf("operation called %d times, want 4", calls)
	}
	if !errors.Is(err, want) {
		t.Errorf("got error %v, want %v", err, want)
	}
}

// The wait between attempts is where cancellation has to be observed: a
// destroy that is retrying a 20-minute cluster deletion should stop when the
// user interrupts it.
func TestRetryOnConflictHonoursContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0

	err := RetryOnConflict(ctx, 10, time.Hour, func() error {
		calls++
		cancel()
		return errors.New("operation is already in progress")
	}, func(error) bool { return true })

	if !errors.Is(err, context.Canceled) {
		t.Errorf("got error %v, want context.Canceled", err)
	}
	if calls != 1 {
		t.Errorf("operation called %d times, want 1", calls)
	}
}

func TestIsNotFoundUnwrapsWrappedErrors(t *testing.T) {
	wrapped := fmt.Errorf("deleting schema: %w", &APIError{StatusCode: 404})
	if !IsNotFound(wrapped) {
		t.Error("IsNotFound did not see a wrapped 404")
	}
	if IsNotFound(fmt.Errorf("deleting schema: %w", &APIError{StatusCode: 409})) {
		t.Error("IsNotFound matched a wrapped 409")
	}
	if IsNotFound(errors.New("plain error")) {
		t.Error("IsNotFound matched a non-API error")
	}
}
