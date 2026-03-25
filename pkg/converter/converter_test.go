// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringVal(t *testing.T) {
	tests := []struct {
		name     string
		input    *string
		expected string
	}{
		{
			name:     "nil pointer returns empty string",
			input:    nil,
			expected: "",
		},
		{
			name:     "non-empty string returns value",
			input:    strPtr("hello"),
			expected: "hello",
		},
		{
			name:     "empty string pointer returns empty string",
			input:    strPtr(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, StringVal(tt.input))
		})
	}
}

func TestNonEmptyString(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectNil bool
		expected  string
	}{
		{
			name:      "empty string returns nil",
			input:     "",
			expectNil: true,
		},
		{
			name:      "non-empty string returns pointer",
			input:     "hello",
			expectNil: false,
			expected:  "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NonEmptyString(tt.input)
			if tt.expectNil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected, *result)
			}
		})
	}
}

func TestInt64Val(t *testing.T) {
	tests := []struct {
		name     string
		input    *int64
		expected int64
	}{
		{
			name:     "nil pointer returns 0",
			input:    nil,
			expected: 0,
		},
		{
			name:     "positive value returns value",
			input:    int64Ptr(42),
			expected: 42,
		},
		{
			name:     "zero value returns 0",
			input:    int64Ptr(0),
			expected: 0,
		},
		{
			name:     "negative value returns value",
			input:    int64Ptr(-1),
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Int64Val(tt.input))
		})
	}
}

func strPtr(s string) *string  { return &s }
func int64Ptr(i int64) *int64  { return &i }
