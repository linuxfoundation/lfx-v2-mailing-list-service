// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRetryConfig(t *testing.T) {
	config := NewRetryConfig(5, 100*time.Millisecond, 5*time.Second)

	assert.Equal(t, 5, config.MaxAttempts)
	assert.Equal(t, 100*time.Millisecond, config.BaseDelay)
	assert.Equal(t, 5*time.Second, config.MaxDelay)
}

func TestRetryWithExponentialBackoff_Success(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return nil
	}

	err := RetryWithExponentialBackoff(ctx, config, fn)

	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "function should be called once when it succeeds")
}

func TestRetryWithExponentialBackoff_SuccessAfterRetries(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	start := time.Now()
	err := RetryWithExponentialBackoff(ctx, config, fn)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, 3, callCount, "function should be called three times")
	// First retry: 10ms, second retry: 20ms = 30ms minimum
	assert.GreaterOrEqual(t, elapsed, 30*time.Millisecond, "should have waited for retries")
}

func TestRetryWithExponentialBackoff_AllAttemptsFail(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	expectedErr := errors.New("persistent error")
	callCount := 0
	fn := func() error {
		callCount++
		return expectedErr
	}

	err := RetryWithExponentialBackoff(ctx, config, fn)

	require.Error(t, err)
	assert.Equal(t, 3, callCount, "function should be called MaxAttempts times")
	assert.Contains(t, err.Error(), "failed after 3 attempts")
	assert.ErrorIs(t, err, expectedErr)
}

func TestRetryWithExponentialBackoff_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    1 * time.Second,
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 2 {
			// Cancel context after second attempt
			cancel()
		}
		return errors.New("error requiring retry")
	}

	err := RetryWithExponentialBackoff(ctx, config, fn)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry cancelled")
	assert.ErrorIs(t, err, context.Canceled)
	// Should be called twice (once initially, once before cancellation)
	assert.Equal(t, 2, callCount)
}

func TestRetryWithExponentialBackoff_ExponentialDelay(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts: 4,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    1 * time.Second,
	}

	attempts := []time.Time{}
	fn := func() error {
		attempts = append(attempts, time.Now())
		return errors.New("error")
	}

	_ = RetryWithExponentialBackoff(ctx, config, fn)

	require.Len(t, attempts, 4)

	// Verify exponential backoff
	// Attempt 1: immediate
	// Attempt 2: ~10ms after attempt 1 (baseDelay * 2^0)
	// Attempt 3: ~20ms after attempt 2 (baseDelay * 2^1)
	// Attempt 4: ~40ms after attempt 3 (baseDelay * 2^2)

	delay1 := attempts[1].Sub(attempts[0])
	delay2 := attempts[2].Sub(attempts[1])
	delay3 := attempts[3].Sub(attempts[2])

	assert.GreaterOrEqual(t, delay1, 10*time.Millisecond)
	assert.GreaterOrEqual(t, delay2, 20*time.Millisecond)
	assert.GreaterOrEqual(t, delay3, 40*time.Millisecond)

	// Verify each delay is approximately double the previous
	// Allow for some timing variance (within 2x range)
	assert.LessOrEqual(t, delay1, 2*delay1)
	assert.GreaterOrEqual(t, delay2, delay1)
	assert.LessOrEqual(t, delay2, 4*delay1)
	assert.GreaterOrEqual(t, delay3, delay2)
}

func TestRetryWithExponentialBackoff_MaxDelayRespected(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts: 10,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
	}

	attempts := []time.Time{}
	fn := func() error {
		attempts = append(attempts, time.Now())
		return errors.New("error")
	}

	_ = RetryWithExponentialBackoff(ctx, config, fn)

	require.Len(t, attempts, 10)

	// After several attempts, delay should be capped at MaxDelay
	// Attempt 6: baseDelay * 2^5 = 10 * 32 = 320ms, but capped at 50ms
	for i := 6; i < len(attempts); i++ {
		delay := attempts[i].Sub(attempts[i-1])
		assert.LessOrEqual(t, delay, 60*time.Millisecond, "delay should not exceed MaxDelay by much (allowing for timing variance)")
		assert.GreaterOrEqual(t, delay, 50*time.Millisecond, "delay should be at least MaxDelay")
	}
}

func TestRetryWithExponentialBackoff_SingleAttempt(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts: 1,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("error")
	}

	err := RetryWithExponentialBackoff(ctx, config, fn)

	require.Error(t, err)
	assert.Equal(t, 1, callCount, "should only attempt once")
	assert.Contains(t, err.Error(), "failed after 1 attempts")
}
