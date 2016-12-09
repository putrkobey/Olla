package unifier

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetry_Success(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:       3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	attempts := 0
	err := Retry(context.Background(), policy, func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRetry_MaxAttemptsExceeded(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:       2,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	attempts := 0
	err := Retry(context.Background(), policy, func(ctx context.Context) error {
		attempts++
		return errors.New("always fails")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retry attempts (2) exceeded")
	assert.Equal(t, 2, attempts)
}

func TestRetry_PermanentError(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:       3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	attempts := 0
	err := Retry(context.Background(), policy, func(ctx context.Context) error {
		attempts++
		return &PermanentError{Err: errors.New("permanent failure")}
	})

	assert.Error(t, err)
	assert.Equal(t, 1, attempts) // Should not retry permanent errors

	var permErr *PermanentError
	assert.ErrorAs(t, err, &permErr)
}

func TestRetry_ContextCancellation(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:       5,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	err := Retry(ctx, policy, func(ctx context.Context) error {
		attempts++
		return errors.New("temporary error")
	})

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
	assert.Less(t, attempts, 5) // Should stop before max attempts
}

func TestRetryWithResult_Success(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts:       3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}

	attempts := 0
	result, err := RetryWithResult(context.Background(), policy, func(ctx context.Context) (string, error) {
		attempts++
		if attempts < 2 {
			return "", errors.New("temporary error")
		}
		return "success", nil
	})

	assert.NoError(t, err)
