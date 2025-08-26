// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package model defines the domain models and entities for the mailing list service.
package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// GrpsIOMailingList represents a GroupsIO mailing list entity with committee support
type GrpsIOMailingList struct {
	UID              string   `json:"uid"`
	GroupName        string   `json:"group_name"`
	Public           bool     `json:"public"`            // Whether the mailing list is publicly accessible
	Type             string   `json:"type"`              // "announcement" | "discussion_moderated" | "discussion_open"
	CommitteeUID     string   `json:"committee_uid"`     // Committee UUID (optional)
	CommitteeName    string   `json:"committee_name"`    // Committee name (optional)
	CommitteeFilters []string `json:"committee_filters"` // Committee member filters (optional)
	Description      string   `json:"description"`       // Minimum 11 characters
	Title            string   `json:"title"`
	SubjectTag       string   `json:"subject_tag"` // Optional
	ParentUID        string   `json:"parent_uid"`  // Parent service UUID (required)
	ProjectUID       string   `json:"project_uid"` // Inherited from parent service

	// Audit trail fields (following GrpsIOService pattern)
	LastReviewedAt *string  `json:"last_reviewed_at"` // Nullable timestamp
	LastReviewedBy *string  `json:"last_reviewed_by"` // Nullable user ID
	Writers        []string `json:"writers"`          // Manager user IDs who can edit
	Auditors       []string `json:"auditors"`         // Auditor user IDs who can audit

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Removed visibility constants - now using Public bool field

// Valid mailing list types
const (
	TypeAnnouncement        = "announcement"
	TypeDiscussionModerated = "discussion_moderated"
	TypeDiscussionOpen      = "discussion_open"
)

// Valid committee filters
const (
	CommitteeFilterVotingRep    = "voting_rep"
	CommitteeFilterAltVotingRep = "alt_voting_rep"
	CommitteeFilterObserver     = "observer"
	CommitteeFilterEmeritus     = "emeritus"
)

// ValidCommitteeFilters returns all valid committee filter values
func ValidCommitteeFilters() []string {
	return []string{
		CommitteeFilterVotingRep,
		CommitteeFilterAltVotingRep,
		CommitteeFilterObserver,
		CommitteeFilterEmeritus,
	}
}

// ValidateBasicFields validates the basic required fields and formats
func (ml *GrpsIOMailingList) ValidateBasicFields() error {
	// Group name validation
	if ml.GroupName == "" {
		return errors.NewValidation("group_name is required")
	}
	if !isValidGroupName(ml.GroupName) {
		return errors.NewValidation("group_name must match pattern: ^[a-z][a-z0-9-]*[a-z0-9]$")
	}

	// Public field is boolean, no validation needed

	// Type validation
	if ml.Type == "" {
		return errors.NewValidation("type is required")
	}
	if !isValidMailingListType(ml.Type) {
		return errors.NewValidation("type must be 'announcement', 'discussion_moderated', or 'discussion_open'")
	}

	// Description validation (minimum 11 characters as per ITX service)
	if ml.Description == "" {
		return errors.NewValidation("description is required")
	}
	if len(ml.Description) < 11 {
		return errors.NewValidation("description must be at least 11 characters long")
	}

	// Title validation
	if ml.Title == "" {
		return errors.NewValidation("title is required")
	}

	// Parent ID validation
	if ml.ParentUID == "" {
		return errors.NewValidation("parent_id is required")
	}

	return nil
}

// ValidateCommitteeFields validates committee-related fields
func (ml *GrpsIOMailingList) ValidateCommitteeFields() error {
	// Committee filters validation
	if len(ml.CommitteeFilters) > 0 {
		// If filters are specified, committee must be provided
		if ml.CommitteeUID == "" {
			return errors.NewValidation("committee must not be empty if committee_filters is non-empty")
		}

		// Validate each filter value
		validFilters := ValidCommitteeFilters()
		for _, filter := range ml.CommitteeFilters {
			if !contains(validFilters, filter) {
				return errors.NewValidation(fmt.Sprintf("invalid committee_filter: %s. Valid values: %v", filter, validFilters))
			}
		}
	}

	// If committee is empty, filters must also be empty
	if ml.CommitteeUID == "" && len(ml.CommitteeFilters) > 0 {
		return errors.NewValidation("committee_filters must be empty if committee is not specified")
	}

	return nil
}

// ValidateGroupNamePrefix validates that group name starts with required prefix for non-primary services
func (ml *GrpsIOMailingList) ValidateGroupNamePrefix(parentServiceType, parentServicePrefix string) error {
	// For non-primary services, group name must start with parent service prefix
	if parentServiceType != "primary" {
		if parentServicePrefix == "" {
			return errors.NewValidation("parent service prefix is required for non-primary services")
		}
		if !strings.HasPrefix(ml.GroupName, parentServicePrefix+"-") {
			return errors.NewValidation(fmt.Sprintf("group_name must start with parent service prefix '%s-' for %s services", parentServicePrefix, parentServiceType))
		}
	}
	return nil
}

// IsCommitteeBased returns true if this mailing list is committee-based
func (ml *GrpsIOMailingList) IsCommitteeBased() bool {
	return ml.CommitteeUID != "" || len(ml.CommitteeFilters) > 0
}

// IsPublic returns true if the mailing list is publicly accessible
func (ml *GrpsIOMailingList) IsPublic() bool {
	return ml.Public
}

// GetAccessControlObjectType returns the OpenFGA object type for this mailing list
func (ml *GrpsIOMailingList) GetAccessControlObjectType() string {
	return "groupsio_mailing_list"
}

// BuildIndexKey generates a SHA-256 hash for use as a NATS KV key
func (ml *GrpsIOMailingList) BuildIndexKey(ctx context.Context) string {
	// Combine parent_id and group_name for uniqueness constraint
	data := fmt.Sprintf("%s|%s", ml.ParentUID, ml.GroupName)

	hash := sha256.Sum256([]byte(data))
	key := hex.EncodeToString(hash[:])

	slog.DebugContext(ctx, "mailing list index key built",
		"parent_uid", ml.ParentUID,
		"group_name", ml.GroupName,
		"key", key,
	)

	return key
}

// Tags generates a consistent set of tags for the mailing list
func (ml *GrpsIOMailingList) Tags() []string {
	var tags []string

	if ml == nil {
		return nil
	}

	if ml.ProjectUID != "" {
		tag := fmt.Sprintf("project_uid:%s", ml.ProjectUID)
		tags = append(tags, tag)
	}

	if ml.ParentUID != "" {
		tag := fmt.Sprintf("parent_uid:%s", ml.ParentUID)
		tags = append(tags, tag)
	}

	if ml.UID != "" {
		tag := fmt.Sprintf("groupsio_mailing_list_uid:%s", ml.UID)
		tags = append(tags, tag)
	}

	if ml.Type != "" {
		tag := fmt.Sprintf("mailing_list_type:%s", ml.Type)
		tags = append(tags, tag)
	}

	if ml.CommitteeUID != "" {
		tag := fmt.Sprintf("committee:%s", ml.CommitteeUID)
		tags = append(tags, tag)
	}

	return tags
}

// Helper functions for validation

func isValidGroupName(groupName string) bool {
	// Pattern: ^[a-z][a-z0-9-]*[a-z0-9]$
	if len(groupName) < 2 {
		return false
	}

	// Must start with lowercase letter
	if groupName[0] < 'a' || groupName[0] > 'z' {
		return false
	}

	// Must end with lowercase letter or digit
	last := groupName[len(groupName)-1]
	if (last < 'a' || last > 'z') && (last < '0' || last > '9') {
		return false
	}

	// Middle characters can be lowercase letters, digits, or hyphens
	for _, char := range groupName[1 : len(groupName)-1] {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}

	return true
}

// Removed isValidVisibility - now using Public bool field

func isValidMailingListType(mlType string) bool {
	return mlType == TypeAnnouncement || mlType == TypeDiscussionModerated || mlType == TypeDiscussionOpen
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
