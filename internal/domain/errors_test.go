// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainError_ErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      *DomainError
		expected string
	}{
		{
			name:     "message only",
			err:      &DomainError{Type: ErrorTypeValidation, Message: "bad input"},
			expected: "bad input",
		},
		{
			name:     "message with cause",
			err:      &DomainError{Type: ErrorTypeInternal, Message: "failed", Err: errors.New("underlying")},
			expected: "failed: underlying",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestDomainError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := &DomainError{Type: ErrorTypeInternal, Message: "wrapped", Err: cause}

	assert.Equal(t, cause, err.Unwrap())
	assert.True(t, errors.Is(err, cause))
}

func TestDomainError_ErrorsAs(t *testing.T) {
	orig := NewNotFoundError("not found")
	wrapped := fmt.Errorf("outer: %w", orig)

	var domErr *DomainError
	require.True(t, errors.As(wrapped, &domErr))
	assert.Equal(t, ErrorTypeNotFound, domErr.Type)
	assert.Equal(t, "not found", domErr.Message)
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("invalid input")
	assert.Equal(t, ErrorTypeValidation, err.Type)
	assert.Equal(t, "invalid input", err.Message)
	assert.Nil(t, err.Err)
}

func TestNewValidationError_WithCause(t *testing.T) {
	cause := errors.New("cause")
	err := NewValidationError("invalid input", cause)
	assert.Equal(t, "invalid input: cause", err.Error())
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("resource not found")
	assert.Equal(t, ErrorTypeNotFound, err.Type)
	assert.Equal(t, "resource not found", err.Message)
}

func TestNewConflictError(t *testing.T) {
	err := NewConflictError("already exists")
	assert.Equal(t, ErrorTypeConflict, err.Type)
	assert.Equal(t, "already exists", err.Message)
}

func TestNewInternalError(t *testing.T) {
	err := NewInternalError("internal failure")
	assert.Equal(t, ErrorTypeInternal, err.Type)
	assert.Equal(t, "internal failure", err.Message)
}

func TestNewUnavailableError(t *testing.T) {
	err := NewUnavailableError("service down")
	assert.Equal(t, ErrorTypeUnavailable, err.Type)
	assert.Equal(t, "service down", err.Message)
}

func TestGetErrorType(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorType
	}{
		{"validation", NewValidationError("v"), ErrorTypeValidation},
		{"not found", NewNotFoundError("nf"), ErrorTypeNotFound},
		{"conflict", NewConflictError("c"), ErrorTypeConflict},
		{"internal", NewInternalError("i"), ErrorTypeInternal},
		{"unavailable", NewUnavailableError("u"), ErrorTypeUnavailable},
		{"non-domain error defaults to internal", errors.New("plain"), ErrorTypeInternal},
		{"wrapped domain error", fmt.Errorf("wrap: %w", NewNotFoundError("x")), ErrorTypeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetErrorType(tt.err))
		})
	}
}
