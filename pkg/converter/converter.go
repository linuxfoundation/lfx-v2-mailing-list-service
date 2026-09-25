// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package converter provides helpers for safely dereferencing and constructing pointer values.
package converter

import (
	"time"
)

// StringVal safely dereferences a *string, returning "" if nil.
func StringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// NonEmptyString returns a pointer to s, or nil if s is empty.
func NonEmptyString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Int64Val safely dereferences a *int64, returning 0 if nil.
func Int64Val(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// NonZeroInt64 returns a pointer to v, or nil if v is zero.
func NonZeroInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

// ParseRFC3339 parses an RFC3339 timestamp string into a time.Time.
// Returns a zero time.Time and nil error if s is empty.
func ParseRFC3339(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, s)
}
