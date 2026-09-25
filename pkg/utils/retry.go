// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package utils provides utility functions for the mailing list service.
package utils

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// RetryConfig holds retry configuration for operations
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// NewRetryConfig creates a RetryConfig with specified parameters
func NewRetryConfig(maxAttempts int, baseDelay, maxDelay time.Duration) RetryConfig {
	return RetryConfig{
		MaxAttempts: maxAttempts,
		BaseDelay:   baseDelay,
		MaxDelay:    maxDelay,
	}
}

// RetryWithExponentialBackoff executes a function with exponential backoff retry logic
// The delay between retries follows the formula: baseDelay * 2^(attempt-1)
// The delay is capped at maxDelay to prevent excessively long waits
func RetryWithExponentialBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff: baseDelay * 2^(attempt-1)
			delay := time.Duration(1<<uint(attempt-1)) * config.BaseDelay
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}

			slog.WarnContext(ctx, "retrying operation",
				"attempt", attempt+1,
				"total_attempts", config.MaxAttempts,
				"retry_delay_ms", delay.Milliseconds(),
			)

			// Wait with context cancellation support
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled: %w", ctx.Err())
			}
		}

		err := fn()
		if err == nil {
			if attempt > 0 {
				slog.InfoContext(ctx, "retry succeeded",
					"attempt", attempt+1,
					"total_attempts", config.MaxAttempts,
				)
			}
			return nil
		}

		lastErr = err
		slog.ErrorContext(ctx, "operation attempt failed",
			"attempt", attempt+1,
			"total_attempts", config.MaxAttempts,
			"error", err,
		)
	}

	return fmt.Errorf("failed after %d attempts: %w", config.MaxAttempts, lastErr)
}
