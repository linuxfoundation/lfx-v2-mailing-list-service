// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInt64PtrToUint64(t *testing.T) {
	tests := []struct {
		name     string
		input    *int64
		expected uint64
	}{
		{
			name:     "nil pointer returns 0",
			input:    nil,
			expected: 0,
		},
		{
			name:     "positive value converts correctly",
			input:    int64Ptr(123),
			expected: 123,
		},
		{
			name:     "zero value converts correctly",
			input:    int64Ptr(0),
			expected: 0,
		},
		{
			name:     "large value converts correctly",
			input:    int64Ptr(9223372036854775807), // max int64
			expected: 9223372036854775807,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Int64PtrToUint64(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInt64PtrToUint64Ptr(t *testing.T) {
	tests := []struct {
		name      string
		input     *int64
		expectNil bool
		expected  uint64
	}{
		{
			name:      "nil pointer returns nil",
			input:     nil,
			expectNil: true,
		},
		{
			name:      "positive value converts correctly",
			input:     int64Ptr(456),
			expectNil: false,
			expected:  456,
		},
		{
			name:      "zero value converts correctly",
			input:     int64Ptr(0),
			expectNil: false,
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Int64PtrToUint64Ptr(tt.input)

			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected, *result)
			}
		})
	}
}

// Helper function for tests
func int64Ptr(i int64) *int64 {
	return &i
}
