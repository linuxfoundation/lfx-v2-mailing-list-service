// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package converter provides helpers for safely dereferencing and constructing pointer values.
package converter

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
