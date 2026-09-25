// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package constants defines validation constants and formats for the mailing list service.
package constants

const (
	// TimestampFormat defines the standard timestamp format for the system (RFC3339)
	TimestampFormat = "2006-01-02T15:04:05Z07:00"

	// TimestampFormatName is the human-readable name for the timestamp format
	TimestampFormatName = "RFC3339"
)

// Validation error messages
const (
	ErrInvalidTimestampFormat = "invalid timestamp format, expected RFC3339 (2006-01-02T15:04:05Z07:00)"
	ErrEmptyTimestamp         = "timestamp cannot be empty"
)
