package unifier

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

type RetryableFunc func(ctx context.Context) error

var (
	randSource *rand.Rand
	randMu     sync.Mutex
)

func init() {
	// Initialize with a time-based seed for non-deterministic randomness
	randSource = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // Used only for jitter in retries
}

// PermanentError marks errors that shouldn't trigger retries (e.g., auth failures, not found)
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	return fmt.Sprintf("permanent error: %v", e.Err)
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

func IsPermanent(err error) bool {
	var permErr *PermanentError
	return errors.As(err, &permErr)
}

func Retry(ctx context.Context, policy RetryPolicy, fn RetryableFunc) error {
	var lastErr error

	for attempt := range policy.MaxAttempts {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := fn(ctx); err == nil {
			return nil
		} else {
			lastErr = err

			if IsPermanent(err) {
				return err
			}

			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
		}

		if attempt < policy.MaxAttempts-1 {
			backoff := calculateBackoff(attempt, policy)

			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded: %w", policy.MaxAttempts, lastErr)
}

func RetryWithResult[T any](ctx context.Context, policy RetryPolicy, fn func(ctx context.Context) (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := range policy.MaxAttempts {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		res, err := fn(ctx)
		if err == nil {
			return res, nil
		}

		lastErr = err

		if IsPermanent(err) {
			return result, err
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return result, err
