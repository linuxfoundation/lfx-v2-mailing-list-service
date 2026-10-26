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

	// RelationGroupsIOService defines the parent service relation used for service-level authorization inheritance
	RelationGroupsIOService = "groupsio_service"

	// RelationMailingList defines the parent mailing list relation used for member-level authorization inheritance
	RelationMailingList = "groupsio_mailing_list"

	// RelationViewer defines the viewer permission level
	RelationViewer = "viewer"

	// RelationWriter defines the writer permission level
	RelationWriter = "writer"

	// RelationOwner defines the owner permission level
	RelationOwner = "owner"

	// RelationMember defines the member permission level
	RelationMember = "member"

	// RelationAuditor defines the auditor permission level
	RelationAuditor = "auditor"

	// RelationPublic defines the public relation, kept separate from viewer so
	// callers can check "is this list public" in isolation from a user's other
	// relations to the list (e.g. to gate self-subscription on public lists
	// without granting it to writers/auditors of private lists).
	RelationPublic = "public"
)

// UserWildcard is the OpenFGA wildcard principal, granting a relation to all users.
const UserWildcard = "*"

// OpenFGA object type constants
const (
	// ObjectTypeGroupsIOService defines the object type for GroupsIO services
	ObjectTypeGroupsIOService = "groupsio_service"

	// ObjectTypeGroupsIOMailingList defines the object type for GroupsIO mailing lists
	ObjectTypeGroupsIOMailingList = "groupsio_mailing_list"

	// ObjectTypeUser defines the object type for users
	ObjectTypeUser = "user"
)

// Member moderation status constants
const (
	// ModStatusNone indicates a regular member with no special privileges
	ModStatusNone = "none"

	// ModStatusModerator indicates a member with moderation privileges (writer permissions)
	ModStatusModerator = "moderator"

	// ModStatusOwner indicates a member with owner privileges (owner permissions)
	ModStatusOwner = "owner"
)
