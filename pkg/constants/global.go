// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package constants defines global constants used throughout the mailing list service.
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

// NATS messaging subjects
const (
	// ProjectGetSlugSubject is the NATS subject for getting project slug
	ProjectGetSlugSubject = "lfx.projects-api.get_slug"
	// ProjectGetNameSubject is the NATS subject for getting project name
	ProjectGetNameSubject = "lfx.projects-api.get_name"
	// ProjectGetParentUIDSubject is the NATS subject for getting project parent UID
	ProjectGetParentUIDSubject = "lfx.projects-api.get_parent_uid"

	// CommitteeGetNameSubject is the NATS subject for getting committee name
	CommitteeGetNameSubject = "lfx.committee-api.get_name"
)

// Environment variables
const (
	// EnvNATSURL is the environment variable for NATS server URL
	EnvNATSURL = "NATS_URL"
	// EnvNATSCredentials is the environment variable for NATS credentials
	EnvNATSCredentials = "NATS_CREDENTIALS"
)

// Resource type constants for domain resolution
const (
	// ResourceTypeService represents a GroupsIO service resource
	ResourceTypeService = "service"
	// ResourceTypeMember represents a GroupsIO member resource
	ResourceTypeMember = "member"
	// ResourceTypeMailingList represents a GroupsIO mailing list resource
	ResourceTypeMailingList = "mailing_list"
)
