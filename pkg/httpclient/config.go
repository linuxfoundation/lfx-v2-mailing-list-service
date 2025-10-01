// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package httpclient

import (
	"time"
)

// Config holds the configuration for the HTTP client
type Config struct {
	// Timeout is the HTTP client timeout for requests
	Timeout time.Duration

	// MaxRetries is the maximum number of retry attempts for failed requests
	MaxRetries int

	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration

	// RetryBackoff enables exponential backoff for retries
	RetryBackoff bool

	// MaxDelay is the maximum delay for exponential backoff (default: 30s)
	MaxDelay time.Duration
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		Timeout:      30 * time.Second,
		MaxRetries:   2,
		RetryDelay:   1 * time.Second,
		RetryBackoff: true,
		MaxDelay:     30 * time.Second,
	}
}
