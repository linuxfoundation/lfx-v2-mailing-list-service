// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package utils provides utility functions for the mailing list service.
package utils

import (
	"fmt"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// ValidateRFC3339 validates that a timestamp string is in RFC3339 format.
// Returns the parsed time.Time and nil error if valid, or zero time and error if invalid.
func ValidateRFC3339(timestamp string) (time.Time, error) {
	if timestamp == "" {
		return time.Time{}, fmt.Errorf(constants.ErrEmptyTimestamp)
	}

	t, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: %w", constants.ErrInvalidTimestampFormat, err)
	}

	return t, nil
}

// ValidateRFC3339Ptr validates a timestamp pointer string is in RFC3339 format.
// Returns nil error if the pointer is nil or contains a valid timestamp.
func ValidateRFC3339Ptr(timestamp *string) error {
	if timestamp == nil {
		return nil // nil is allowed
	}

	_, err := ValidateRFC3339(*timestamp)
	return err
}

// NowRFC3339Ptr returns the current time as an RFC3339 formatted string pointer.
func NowRFC3339Ptr() *string {
	now := time.Now().Format(constants.TimestampFormat)
	return &now
}

// ParseTimestampPtr safely parses a timestamp pointer into a time.Time pointer.
// Returns nil if the input is nil or empty, or a pointer to the parsed time.
// Returns an error if parsing fails.
func ParseTimestampPtr(timestamp *string) (*time.Time, error) {
	if timestamp == nil || *timestamp == "" {
		return nil, nil
	}

	t, err := ValidateRFC3339(*timestamp)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// FormatTimePtr formats a time.Time pointer as an RFC3339 string pointer.
// Returns nil if the input is nil.
func FormatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}

	formatted := t.Format(constants.TimestampFormat)
	return &formatted
}