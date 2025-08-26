// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package constants defines OpenFGA relation constants used throughout the mailing list service.
package constants

// OpenFGA relation constants for authorization and access control
const (
	// RelationProject defines the project relation used for inheritance
	RelationProject = "project"

	// RelationCommittee defines the committee relation used for committee-based authorization
	RelationCommittee = "committee"

	// RelationViewer defines the viewer permission level
	RelationViewer = "viewer"

	// RelationWriter defines the writer permission level
	RelationWriter = "writer"

	// RelationOwner defines the owner permission level
	RelationOwner = "owner"
)
