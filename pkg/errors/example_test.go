// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package errors

import (
	"database/sql"
	"errors"
	"testing"
)

// customErr is a test error type to demonstrate errors.As functionality
type customErr struct {
	code int
	msg  string
}

func (c customErr) Error() string {
	return c.msg
}

// TestErrorsIsAndAs demonstrates how the Unwrap method enables
// better error handling with errors.Is and errors.As
func TestErrorsIsAndAs(t *testing.T) {
	// Simulate a database connection error
	dbErr := sql.ErrNoRows

	// Wrap it in our custom error
	serviceErr := NewNotFound("user not found", dbErr)

	// Test errors.Is - can identify the specific database error
	if !errors.Is(serviceErr, sql.ErrNoRows) {
		t.Error("Should be able to identify sql.ErrNoRows using errors.Is")
	}

	// Test with a custom error type
	originalErr := customErr{code: 404, msg: "resource not found"}
	wrappedErr := NewValidation("validation failed", originalErr)

	// Test that we can extract the custom error using errors.As
	var extracted customErr
	if !errors.As(wrappedErr, &extracted) {
		t.Error("Should be able to extract customErr using errors.As")
	} else {
		if extracted.code != 404 {
			t.Errorf("Expected code 404, got %d", extracted.code)
		}
	}
}
