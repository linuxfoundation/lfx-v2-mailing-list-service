// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateRFC3339(t *testing.T) {
	tests := []struct {
		name        string
		timestamp   string
		expectError bool
	}{
		{
			name:        "valid RFC3339 timestamp",
			timestamp:   "2023-06-15T10:30:45Z",
			expectError: false,
		},
		{
			name:        "valid RFC3339 with timezone",
			timestamp:   "2023-06-15T10:30:45+02:00",
			expectError: false,
		},
		{
			name:        "valid RFC3339 with microseconds",
			timestamp:   "2023-06-15T10:30:45.123456Z",
			expectError: false,
		},
		{
			name:        "empty string",
			timestamp:   "",
			expectError: true,
		},
		{
			name:        "invalid format - missing timezone",
			timestamp:   "2023-06-15T10:30:45",
			expectError: true,
		},
		{
			name:        "invalid format - wrong delimiter",
			timestamp:   "2023-06-15 10:30:45Z",
			expectError: true,
		},
		{
			name:        "invalid date",
			timestamp:   "2023-13-45T10:30:45Z",
			expectError: true,
		},
		{
			name:        "not a timestamp",
			timestamp:   "not-a-timestamp",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateRFC3339(tt.timestamp)

			if tt.expectError {
				assert.Error(t, err)
				assert.Zero(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotZero(t, result)
				// Verify we can format it back to the same string (or compatible)
				formatted := result.Format("2006-01-02T15:04:05Z07:00")
				_, parseErr := time.Parse("2006-01-02T15:04:05Z07:00", formatted)
				assert.NoError(t, parseErr)
			}
		})
	}
}

func TestValidateRFC3339Ptr(t *testing.T) {
	tests := []struct {
		name        string
		timestamp   *string
		expectError bool
	}{
		{
			name:        "nil pointer is valid",
			timestamp:   nil,
			expectError: false,
		},
		{
			name:        "valid timestamp pointer",
			timestamp:   stringPtr("2023-06-15T10:30:45Z"),
			expectError: false,
		},
		{
			name:        "invalid timestamp pointer",
			timestamp:   stringPtr("invalid-timestamp"),
			expectError: true,
		},
		{
			name:        "empty string pointer",
			timestamp:   stringPtr(""),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRFC3339Ptr(tt.timestamp)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNowRFC3339Ptr(t *testing.T) {
	result := NowRFC3339Ptr()

	assert.NotNil(t, result)
	assert.NotEmpty(t, *result)

	// Verify it's a valid RFC3339 timestamp
	_, err := ValidateRFC3339(*result)
	assert.NoError(t, err)

	// Verify it's recent (within last minute)
	parsed, err := time.Parse("2006-01-02T15:04:05Z07:00", *result)
	assert.NoError(t, err)
	assert.WithinDuration(t, time.Now(), parsed, time.Minute)
}

func TestParseTimestampPtr(t *testing.T) {
	tests := []struct {
		name        string
		timestamp   *string
		expectError bool
		expectNil   bool
	}{
		{
			name:        "nil pointer returns nil",
			timestamp:   nil,
			expectError: false,
			expectNil:   true,
		},
		{
			name:        "empty string returns nil",
			timestamp:   stringPtr(""),
			expectError: false,
			expectNil:   true,
		},
		{
			name:        "valid timestamp",
			timestamp:   stringPtr("2023-06-15T10:30:45Z"),
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "invalid timestamp",
			timestamp:   stringPtr("invalid-timestamp"),
			expectError: true,
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTimestampPtr(tt.timestamp)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectNil {
				assert.Nil(t, result)
			} else if !tt.expectError {
				assert.NotNil(t, result)
				// Verify the parsed time is correct
				expected, _ := time.Parse("2006-01-02T15:04:05Z07:00", *tt.timestamp)
				assert.Equal(t, expected, *result)
			}
		})
	}
}

func TestFormatTimePtr(t *testing.T) {
	tests := []struct {
		name      string
		input     *time.Time
		expectNil bool
	}{
		{
			name:      "nil time returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name:      "valid time returns formatted string",
			input:     timePtr(time.Date(2023, 6, 15, 10, 30, 45, 0, time.UTC)),
			expectNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimePtr(tt.input)

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.NotEmpty(t, *result)

				// Verify it's a valid RFC3339 format
				_, err := ValidateRFC3339(*result)
				assert.NoError(t, err)

				// Verify round-trip conversion
				parsed, err := time.Parse("2006-01-02T15:04:05Z07:00", *result)
				assert.NoError(t, err)
				assert.Equal(t, tt.input.UTC(), parsed.UTC())
			}
		})
	}
}

// Helper functions for tests
func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}