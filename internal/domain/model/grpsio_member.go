// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package model defines the domain models and entities for the mailing list service.
package model

import (
	"fmt"
	"strings"
	"time"
)

// GrpsIOMember represents a GroupsIO mailing list member
type GrpsIOMember struct {
	// Internal IDs (UUIDs)
	UID            string `json:"uid"`              // Primary key
	MailingListUID string `json:"mailing_list_uid"` // FK to mailing list

	MemberID *int64 `json:"member_id"` // Groups.io member ID
	GroupID  *int64 `json:"group_id"`  // Groups.io group ID

	// Member Information
	Username string `json:"username"` // Username
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Email        string `json:"email"`        // Required, RFC 5322
	Organization string `json:"organization"` // Optional
	JobTitle     string `json:"job_title"`    // Optional

	// Normalised search fields (lowercase, for filtering)
	GroupsEmail       string `json:"groups_email,omitempty"`        // Lowercase email from Groups.io
	GroupsFullName    string `json:"groups_full_name,omitempty"`    // Lowercase full name from Groups.io
	CommitteeEmail    string `json:"committee_email,omitempty"`     // Lowercase email from Committee Service
	CommitteeFullName string `json:"committee_full_name,omitempty"` // Lowercase full name from Committee Service

	// Committee association
	CommitteeID  string `json:"committee_id,omitempty"`  // Committee ID if member belongs to a committee
	Role         string `json:"role,omitempty"`          // Role of the member
	VotingStatus string `json:"voting_status,omitempty"` // Voting status of the member

	// Member Configuration
	MemberType       string `json:"member_type"`                  // "committee" or "direct"
	DeliveryMode     string `json:"delivery_mode"`                // Email delivery preference
	DeliveryModeList string `json:"delivery_mode_list,omitempty"` // Delivery mode list from Groups.io
	ModStatus        string `json:"mod_status"`                   // "none", "moderator", "owner"

	// Status
	Status string `json:"status"` // Groups.io status: normal, pending, etc.

	LastReviewedAt *string `json:"last_reviewed_at"` // Nullable timestamp
	LastReviewedBy *string `json:"last_reviewed_by"` // Nullable user ID

	// Project association (inherited from the parent mailing list)
	ProjectUID  string `json:"project_uid,omitempty"`
	ProjectSlug string `json:"project_slug,omitempty"`

	// Timestamps
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	SystemUpdatedAt time.Time `json:"system_updated_at,omitempty"` // Last modified by system (scripts/webhooks)
}

// Tags generates a consistent set of tags for the member.
func (m *GrpsIOMember) Tags() []string {
	var tags []string

	if m == nil {
		return nil
	}

	if m.UID != "" {
		tags = append(tags, m.UID)
		tag := fmt.Sprintf("member_uid:%s", m.UID)
		tags = append(tags, tag)
	}

	if m.MailingListUID != "" {
		tag := fmt.Sprintf("mailing_list_uid:%s", m.MailingListUID)
		tags = append(tags, tag)
	}

	if m.Username != "" {
		tag := fmt.Sprintf("username:%s", m.Username)
		tags = append(tags, tag)
	}

	if m.Email != "" {
		tag := fmt.Sprintf("email:%s", m.Email)
		tags = append(tags, tag)
	}

	if m.Status != "" {
		tag := fmt.Sprintf("status:%s", m.Status)
		tags = append(tags, tag)
	}

	if m.ProjectUID != "" {
		tag := fmt.Sprintf("project_uid:%s", m.ProjectUID)
		tags = append(tags, tag)
	}

	return tags
}

// ParentRefs returns the parent resource references for indexing.
func (m *GrpsIOMember) ParentRefs() []string {
	if m == nil {
		return nil
	}
	var refs []string
	if m.MailingListUID != "" {
		refs = append(refs, fmt.Sprintf("groupsio_mailing_list:%s", m.MailingListUID))
	}
	if m.ProjectUID != "" {
		refs = append(refs, fmt.Sprintf("project:%s", m.ProjectUID))
	}
	return refs
}

// NameAndAliases returns searchable names for the member.
func (m *GrpsIOMember) NameAndAliases() []string {
	if m == nil {
		return nil
	}
	var names []string
	if fullName := strings.TrimSpace(m.FirstName + " " + m.LastName); fullName != "" {
		names = append(names, fullName)
	}
	if m.Username != "" {
		names = append(names, m.Username)
	}
	if m.Email != "" {
		names = append(names, m.Email)
	}
	return names
}

// SortName returns the primary sort name for the member.
func (m *GrpsIOMember) SortName() string {
	if m == nil {
		return ""
	}
	if m.LastName != "" && m.FirstName != "" {
		return m.LastName + ", " + m.FirstName
	}
	if m.LastName != "" {
		return m.LastName
	}
	if m.FirstName != "" {
		return m.FirstName
	}
	return m.Username
}

// Fulltext returns a concatenated string for full-text search.
func (m *GrpsIOMember) Fulltext() string {
	if m == nil {
		return ""
	}
	var parts []string
	if m.FirstName != "" {
		parts = append(parts, m.FirstName)
	}
	if m.LastName != "" {
		parts = append(parts, m.LastName)
	}
	if m.Email != "" {
		parts = append(parts, m.Email)
	}
	if m.Organization != "" {
		parts = append(parts, m.Organization)
	}
	if m.JobTitle != "" {
		parts = append(parts, m.JobTitle)
	}
	return strings.Join(parts, " ")
}
