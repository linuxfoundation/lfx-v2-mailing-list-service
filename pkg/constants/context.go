// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package constants defines shared context key types used throughout the mailing list service.
package constants

// ContextKey is the unified type for all context keys to prevent type mismatches
type ContextKey string

// Context keys for various middleware and service contexts
const (
	// PrincipalContextID is the context key for the principal
	PrincipalContextID ContextKey = "principal"

	// AuthorizationContextID is the context key for the authorization
	AuthorizationContextID ContextKey = "authorization"

	// OnBehalfContextID is the context key for the on-behalf-of principal
	OnBehalfContextID ContextKey = "x-on-behalf-of"

	// RequestIDContextKey is the context key for request ID
	RequestIDContextKey ContextKey = "request-id"
)
