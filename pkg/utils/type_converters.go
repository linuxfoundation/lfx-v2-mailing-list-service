// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package utils provides utility functions for the mailing list service.
package utils

// Int64PtrToUint64 safely converts *int64 to uint64 for API calls.
// Returns 0 if the pointer is nil.
// This is commonly used when converting domain model IDs to API parameter types.
//
// WARNING: Negative values will wrap around to large uint64 values.
// This function assumes the input represents a valid ID (non-negative).
// If you need validation, check for negative values before calling this function.
func Int64PtrToUint64(val *int64) uint64 {
	if val == nil {
		return 0
	}
	// Note: This conversion will wrap negative values. For example:
	// -1 becomes 18446744073709551615 (max uint64)
	// Callers should ensure IDs from external APIs are non-negative.
	return uint64(*val)
}

// Int64PtrToUint64Ptr safely converts *int64 to *uint64.
// Returns nil if the pointer is nil.
// This is used when optional pointer types need to be converted.
//
// WARNING: Negative values will wrap around to large uint64 values.
// This function assumes the input represents a valid ID (non-negative).
// If you need validation, check for negative values before calling this function.
func Int64PtrToUint64Ptr(val *int64) *uint64 {
	if val == nil {
		return nil
	}
	// Note: This conversion will wrap negative values. For example:
	// -1 becomes 18446744073709551615 (max uint64)
	// Callers should ensure IDs from external APIs are non-negative.
	converted := uint64(*val)
	return &converted
}
