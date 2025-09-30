// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package utils provides utility functions for the mailing list service.
package utils

// Int64PtrToUint64 safely converts *int64 to uint64 for API calls.
// Returns 0 if the pointer is nil.
// This is commonly used when converting domain model IDs to API parameter types.
func Int64PtrToUint64(val *int64) uint64 {
	if val == nil {
		return 0
	}
	return uint64(*val)
}

// Int64PtrToUint64Ptr safely converts *int64 to *uint64.
// Returns nil if the pointer is nil.
// This is used when optional pointer types need to be converted.
func Int64PtrToUint64Ptr(val *int64) *uint64 {
	if val == nil {
		return nil
	}
	converted := uint64(*val)
	return &converted
}
