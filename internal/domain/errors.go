// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package domain

import "errors"

// ErrorType represents the semantic category of an error
type ErrorType int

const (
	ErrorTypeValidation  ErrorType = iota // Input validation errors (400 Bad Request)
	ErrorTypeNotFound                     // Resource not found errors (404 Not Found)
	ErrorTypeConflict                     // Resource conflict errors (409 Conflict)
	ErrorTypeInternal                     // Internal server errors (500 Internal Server Error)
	ErrorTypeUnavailable                  // Service unavailable errors (503 Service Unavailable)
)

// DomainError represents an error with semantic type information
type DomainError struct {
	Type    ErrorType
	Message string
	Err     error // underlying error for wrapping
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// GetErrorType returns the semantic type of an error
func GetErrorType(err error) ErrorType {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Type
	}
	return ErrorTypeInternal // default fallback
}

// Error constructors for different types
func NewValidationError(message string, err ...error) *DomainError {
	return &DomainError{Type: ErrorTypeValidation, Message: message, Err: errors.Join(err...)}
}

func NewNotFoundError(message string, err ...error) *DomainError {
	return &DomainError{Type: ErrorTypeNotFound, Message: message, Err: errors.Join(err...)}
}

func NewConflictError(message string, err ...error) *DomainError {
	return &DomainError{Type: ErrorTypeConflict, Message: message, Err: errors.Join(err...)}
}

func NewInternalError(message string, err ...error) *DomainError {
	return &DomainError{Type: ErrorTypeInternal, Message: message, Err: errors.Join(err...)}
}

func NewUnavailableError(message string, err ...error) *DomainError {
	return &DomainError{Type: ErrorTypeUnavailable, Message: message, Err: errors.Join(err...)}
}
