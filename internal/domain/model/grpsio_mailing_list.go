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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/utils"
)

// GrpsIOMailingList represents a GroupsIO mailing list entity with committee support
type GrpsIOMailingList struct {
	UID              string   `json:"uid"`
	SubgroupID       *int64   `json:"-"` // Groups.io subgroup ID - internal use only, nullable for async
	GroupName        string   `json:"group_name"`
	Public           bool     `json:"public"`                // Whether the mailing list is publicly accessible
	SyncStatus       string   `json:"sync_status,omitempty"` // "pending", "synced", "failed"
	Type             string   `json:"type"`                  // "announcement" | "discussion_moderated" | "discussion_open"
	CommitteeUID     string   `json:"committee_uid"`         // Committee UUID (optional)
	CommitteeName    string   `json:"committee_name"`        // Committee name (optional)
	CommitteeFilters []string `json:"committee_filters"`     // Committee member filters (optional)
	Description      string   `json:"description"`           // Minimum 11 characters
	Title            string   `json:"title"`
	SubjectTag       string   `json:"subject_tag"`  // Optional
	ServiceUID       string   `json:"service_uid"`  // Service UUID (required)
	ProjectUID       string   `json:"project_uid"`  // Inherited from parent service
	ProjectName      string   `json:"project_name"` // Inherited from parent service
	ProjectSlug      string   `json:"project_slug"` // Inherited from parent service

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
	TypeCustom              = "custom" // TODO: Verify if Groups.io actually supports custom type
)

// Valid committee filters
const (
	CommitteeFilterVotingRep    = "Voting Rep"
	CommitteeFilterAltVotingRep = "Alternate Voting Rep"
	CommitteeFilterObserver     = "Observer"
	CommitteeFilterEmeritus     = "Emeritus"
	CommitteeFilterNone         = "None"
)

// ValidCommitteeFilters returns all valid committee filter values
func ValidCommitteeFilters() []string {
	return []string{
		CommitteeFilterVotingRep,
		CommitteeFilterAltVotingRep,
		CommitteeFilterObserver,
		CommitteeFilterEmeritus,
		CommitteeFilterNone,
	}
}

// ValidMailingListTypes returns all valid mailing list type values
func ValidMailingListTypes() []string {
	return []string{
		TypeAnnouncement,
		TypeDiscussionModerated,
		TypeDiscussionOpen,
		TypeCustom,
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
	if ml.ServiceUID == "" {
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

// BuildIndexKey generates a SHA-256 hash for use as a NATS KV key
func (ml *GrpsIOMailingList) BuildIndexKey(ctx context.Context) string {
	// Combine parent_id and group_name for uniqueness constraint
	data := fmt.Sprintf("%s|%s", ml.ServiceUID, ml.GroupName)

	hash := sha256.Sum256([]byte(data))
	key := hex.EncodeToString(hash[:])

	slog.DebugContext(ctx, "mailing list index key built",
		"parent_uid", ml.ServiceUID,
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

	if ml.ServiceUID != "" {
		tag := fmt.Sprintf("service_uid:%s", ml.ServiceUID)
		tags = append(tags, tag)
	}

	if ml.Type != "" {
		tag := fmt.Sprintf("type:%s", ml.Type)
		tags = append(tags, tag)
	}

	// Add public tag
	tag := fmt.Sprintf("public:%t", ml.Public)
	tags = append(tags, tag)

	// Add committee tag if committee-based
	if ml.CommitteeUID != "" {
		tags = append(tags, fmt.Sprintf("committee_uid:%s", ml.CommitteeUID))
	}

	// Add committee filter tags
	for _, filter := range ml.CommitteeFilters {
		tags = append(tags, fmt.Sprintf("committee_filter:%s", filter))
	}

	if ml.UID != "" {
		tag := fmt.Sprintf("groupsio_mailing_list_uid:%s", ml.UID)
		tags = append(tags, tag)
	}

	if ml.GroupName != "" {
		tag := fmt.Sprintf("group_name:%s", ml.GroupName)
		tags = append(tags, tag)
	}

	return tags
}

// ValidateLastReviewedAt validates the LastReviewedAt timestamp format.
// Returns nil if the field is nil (allowed) or contains a valid RFC3339 timestamp.
func (ml *GrpsIOMailingList) ValidateLastReviewedAt() error {
	return utils.ValidateRFC3339Ptr(ml.LastReviewedAt)
}

// GetLastReviewedAtTime safely parses LastReviewedAt into a time.Time pointer.
// Returns nil if the field is nil or empty, or the parsed time if valid.
func (ml *GrpsIOMailingList) GetLastReviewedAtTime() (*time.Time, error) {
	return utils.ParseTimestampPtr(ml.LastReviewedAt)
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
	return contains(ValidMailingListTypes(), mlType)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// IsMainGroup determines if a mailing list is the main group for a service
// Main groups have the same group_name as their parent service's group_name
func (ml *GrpsIOMailingList) IsMainGroup(parentService *GrpsIOService) bool {
	return ml.GroupName == parentService.GroupName
}
