// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package log

import (
	"log/slog"
	"testing"
)

func TestLogOptionalInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    *int64
		expected slog.Value
	}{
		{
			name:     "nil pointer returns nil value",
			input:    nil,
			expected: slog.AnyValue(nil),
		},
		{
			name:     "zero value returns zero",
			input:    ptrInt64(0),
			expected: slog.Int64Value(0),
		},
		{
			name:     "positive value returns value",
			input:    ptrInt64(123),
			expected: slog.Int64Value(123),
		},
		{
			name:     "negative value returns value",
			input:    ptrInt64(-456),
			expected: slog.Int64Value(-456),
		},
		{
			name:     "max int64 value",
			input:    ptrInt64(9223372036854775807),
			expected: slog.Int64Value(9223372036854775807),
		},
		{
			name:     "min int64 value",
			input:    ptrInt64(-9223372036854775808),
			expected: slog.Int64Value(-9223372036854775808),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LogOptionalInt64(tt.input)

			// Compare using Equal method for slog.Value
			if !result.Equal(tt.expected) {
				t.Errorf("LogOptionalInt64(%v) = %v, want %v", tt.input, result, tt.expected)
			}

			// Additional validation for non-nil cases
			if tt.input != nil {
				if result.Kind() != slog.KindInt64 {
					t.Errorf("Expected Kind to be KindInt64, got %v", result.Kind())
				}
				if result.Int64() != *tt.input {
					t.Errorf("Expected Int64() to return %d, got %d", *tt.input, result.Int64())
				}
			}
		})
	}
}

// ptrInt64 is a helper function to create int64 pointers for test cases
func ptrInt64(v int64) *int64 {
	return &v
}
