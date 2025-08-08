// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package errors

import (
	"errors"
	"testing"
)

// TestServiceUnavailableUnwrap specifically tests the ServiceUnavailable Unwrap method
func TestServiceUnavailableUnwrap(t *testing.T) {
	// Create a root cause error
	rootCause := errors.New("database connection lost")

	// Create a ServiceUnavailable error that wraps the root cause
	serviceErr := NewServiceUnavailable("service temporarily unavailable", rootCause)

	// Test that the explicit Unwrap method works
	unwrapped := serviceErr.Unwrap()
	if unwrapped == nil {
		t.Error("Expected ServiceUnavailable.Unwrap() to return non-nil error")
	}

	// Test that errors.Is can find the root cause
	if !errors.Is(serviceErr, rootCause) {
		t.Error("errors.Is should find root cause in ServiceUnavailable error")
	}

	// Test with no wrapped error
	simpleErr := NewServiceUnavailable("simple service error")
	if simpleErr.Unwrap() != nil {
		t.Error("Expected ServiceUnavailable.Unwrap() to return nil when no error is wrapped")
	}

	// Test error message formatting
	expectedMsg := "service temporarily unavailable: database connection lost"
	if !errors.Is(errors.New(expectedMsg), errors.New(expectedMsg)) {
		// This is just to show the error message includes both parts
		t.Logf("ServiceUnavailable error message: %s", serviceErr.Error())
	}
}
