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

	// EmailContextID is the context key for the authenticated caller's email
	EmailContextID ContextKey = "email"

	// AuthorizationContextID is the context key for the authorization
	AuthorizationContextID ContextKey = "authorization"

	// OnBehalfContextID is the context key for the on-behalf-of principal
	OnBehalfContextID ContextKey = "x-on-behalf-of"

	// RequestIDContextKey is the context key for request ID
	RequestIDContextKey ContextKey = "request-id"

	// GrpsIOWebhookBodyContextKey is the context key for raw GroupsIO webhook body
	GrpsIOWebhookBodyContextKey ContextKey = "grpsio-webhook-body"

	// TrustedSystemCallerContextID marks a context as originating from an internal background
	// process (e.g. a NATS event subscriber) rather than an end-user HTTP request. Only code paths
	// that are themselves reacting to an already-trusted, committee-scoped system event may set
	// this — it must never be set from, or derived from, request-handling middleware.
	TrustedSystemCallerContextID ContextKey = "trusted-system-caller"
)
