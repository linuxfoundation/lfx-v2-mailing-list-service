// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

// Service constants
const (
	// ServiceName is the name of this service
	ServiceName = "mailing-list"
)

// HTTP header constants
const (
	// RequestIDHeader is the HTTP header name for request ID
	RequestIDHeader = "X-Request-Id"
)

// Context key types
type contextKey string

// Context keys
const (
	// RequestIDContextKey is the context key for request ID
	RequestIDContextKey contextKey = "request-id"
)

// NATS messaging subjects
const (
	// ProjectGetSlugSubject is the NATS subject for getting project slug
	ProjectGetSlugSubject = "project.get.slug"
	// ProjectGetNameSubject is the NATS subject for getting project name
	ProjectGetNameSubject = "project.get.name"
)

// Environment variables
const (
	// EnvNATSURL is the environment variable for NATS server URL
	EnvNATSURL = "NATS_URL"
	// EnvNATSCredentials is the environment variable for NATS credentials
	EnvNATSCredentials = "NATS_CREDENTIALS"
)
