// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package groupsio

import (
	"os"
	"strconv"
	"time"
)

// Config holds the configuration for the GroupsIO client
type Config struct {
	// BaseURL is the Groups.io API base URL
	BaseURL string

	// Email is the Groups.io account email for authentication
	Email string

	// Password is the Groups.io account password for authentication
	Password string

	// Timeout is the HTTP client timeout for requests
	Timeout time.Duration

	// MaxRetries is the maximum number of retry attempts for failed requests
	MaxRetries int

	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration

	// MockMode disables real Groups.io API calls (for testing)
	MockMode bool
}

// DefaultConfig returns a Config with sensible defaults
func DefaultConfig() Config {
	return Config{
		BaseURL:    "https://api.groups.io",
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
		MockMode:   false,
	}
}

// NewConfigFromEnv creates a Config from environment variables
func NewConfigFromEnv() Config {
	config := DefaultConfig()

	if baseURL := os.Getenv("GROUPSIO_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	}

	if email := os.Getenv("GROUPSIO_EMAIL"); email != "" {
		config.Email = email
	}

	if password := os.Getenv("GROUPSIO_PASSWORD"); password != "" {
		config.Password = password
	}

	if timeoutStr := os.Getenv("GROUPSIO_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			config.Timeout = timeout
		}
	}

	if retriesStr := os.Getenv("GROUPSIO_MAX_RETRIES"); retriesStr != "" {
		if retries, err := strconv.Atoi(retriesStr); err == nil {
			config.MaxRetries = retries
		}
	}

	if delayStr := os.Getenv("GROUPSIO_RETRY_DELAY"); delayStr != "" {
		if delay, err := time.ParseDuration(delayStr); err == nil {
			config.RetryDelay = delay
		}
	}

	// Check for mock mode
	if mockMode := os.Getenv("GROUPSIO_SOURCE"); mockMode == "mock" {
		config.MockMode = true
	}

	return config
}
