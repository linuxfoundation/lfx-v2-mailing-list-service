// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package errors provides custom error types for the mailing list service.
package errors

import "fmt"

// base is a struct that holds the common fields for error types
type base struct {
	message string
	err     error
}

// error is a method that returns the error message for the base struct
// any changes to the error message here will be reflected in all error types that embed base
func (b base) error() string {
	if b.err == nil {
		return b.message
	}
	return fmt.Sprintf("%s: %v", b.message, b.err)
}

// Unwrap exposes the underlying error to support errors.Is / errors.As.
func (b base) Unwrap() error {
	return b.err
}
