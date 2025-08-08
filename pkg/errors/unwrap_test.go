// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package errors

import (
	"errors"
	"testing"
)

func TestUnwrap(t *testing.T) {
	// Test with a root cause error
	rootCause := errors.New("root cause error")

	// Create a custom error that wraps the root cause
	validationErr := NewValidation("validation failed", rootCause)

	// Test that Unwrap returns the joined error (which wraps our root cause)
	unwrapped := validationErr.Unwrap()
	if unwrapped == nil {
		t.Error("Expected unwrapped error to not be nil")
	}

	// Test errors.Is functionality - this should work even with errors.Join
	if !errors.Is(validationErr, rootCause) {
		t.Error("errors.Is should find the root cause in the wrapped error")
	}

	// Test with no wrapped error
	simpleErr := NewValidation("simple error")
	if simpleErr.Unwrap() != nil {
		t.Error("Expected Unwrap to return nil for error with no wrapped cause")
	}
}

func TestUnwrapWithDifferentErrorTypes(t *testing.T) {
	rootCause := errors.New("database connection failed")

	// Test with different error types that embed base
	testCases := []struct {
		name string
		err  error
	}{
		{"Validation", NewValidation("validation error", rootCause)},
		{"NotFound", NewNotFound("not found error", rootCause)},
		{"Unexpected", NewUnexpected("unexpected error", rootCause)},
		{"ServiceUnavailable", NewServiceUnavailable("service unavailable", rootCause)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test errors.Is functionality - this should work thanks to Unwrap
			if !errors.Is(tc.err, rootCause) {
				t.Errorf("errors.Is should find root cause in %s error", tc.name)
			}

			// Test that we can unwrap to get some underlying error
			type unwrapper interface {
				Unwrap() error
			}

			if u, ok := tc.err.(unwrapper); ok {
				underlying := u.Unwrap()
				if underlying == nil {
					t.Errorf("Expected %s error to have an underlying error", tc.name)
				}
				// Verify that errors.Is can traverse the chain
				if !errors.Is(underlying, rootCause) {
					t.Errorf("errors.Is should find root cause in unwrapped %s error", tc.name)
				}
			} else {
				t.Errorf("%s error should implement Unwrap()", tc.name)
			}
		})
	}
}
