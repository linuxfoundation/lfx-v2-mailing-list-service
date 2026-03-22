// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package model defines the domain models and entities for the mailing list service.
package model

import (
	"fmt"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/utils"
)

// DefaultGroupsIODomain is the default domain for Groups.io API calls
const DefaultGroupsIODomain = "groups.io"

// UserInfo represents user information including profile details.
type UserInfo struct {
	Name     *string `json:"name,omitempty"`
	Email    *string `json:"email,omitempty"`
	Username *string `json:"username,omitempty"`
	Avatar   *string `json:"avatar,omitempty"`
}

type GrpsIOServiceFull struct {
	Base     *GroupsIOService       `json:"base"`
	Settings *GrpsIOServiceSettings `json:"settings"`
}

// GrpsIOServiceSettings represents the settings for a GroupsIO service (user management).
type GrpsIOServiceSettings struct {
	UID             string     `json:"uid"`
	Writers         []UserInfo `json:"writers"`
	Auditors        []UserInfo `json:"auditors"`
	LastReviewedAt  *string    `json:"last_reviewed_at,omitempty"`
	LastReviewedBy  *string    `json:"last_reviewed_by,omitempty"`
	LastAuditedBy   *string    `json:"last_audited_by,omitempty"`
	LastAuditedTime *string    `json:"last_audited_time,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Tags generates a consistent set of tags for the GroupsIOServices ettings
func (s *GrpsIOServiceSettings) Tags() []string {
	var tags []string

	if s == nil {
		return nil
	}

	if s.UID != "" {
		tags = append(tags, s.UID)
		tag := fmt.Sprintf("service_uid:%s", s.UID)
		tags = append(tags, tag)
	}

	return tags
}

// ValidateLastReviewedAt validates the LastReviewedAt timestamp format.
// Returns nil if the field is nil (allowed) or contains a valid RFC3339 timestamp.
func (s *GrpsIOServiceSettings) ValidateLastReviewedAt() error {
	return utils.ValidateRFC3339Ptr(s.LastReviewedAt)
}

// GetLastReviewedAtTime safely parses LastReviewedAt into a time.Time pointer.
// Returns nil if the field is nil or empty, or the parsed time if valid.
func (s *GrpsIOServiceSettings) GetLastReviewedAtTime() (*time.Time, error) {
	return utils.ParseTimestampPtr(s.LastReviewedAt)
}

// GroupsIOService represents a GroupsIO service entity
type GroupsIOService struct {
	Type             string    `json:"type"`
	UID              string    `json:"uid"`
	Domain           string    `json:"domain"`
	GroupID          *int64    `json:"group_id"` // Groups.io group ID
	Status           string    `json:"status"`
	Source           string    `json:"source"` // "api", "webhook", or "mock" - tracks origin for business logic
	GlobalOwners     []string  `json:"global_owners"`
	Prefix           string    `json:"prefix"`
	ParentServiceUID string    `json:"parent_service_uid"` // Parent primary service UID for shared type
	ProjectSlug      string    `json:"project_slug"`
	ProjectName      string    `json:"project_name"`
	ProjectUID       string    `json:"project_uid"`
	URL              string    `json:"url"`
	GroupName        string    `json:"group_name"`
	Public           bool      `json:"public"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	SystemUpdatedAt  time.Time `json:"system_updated_at,omitempty"` // Last modified by system (scripts/webhooks)
}

// Tags generates a consistent set of tags for the GroupsIOService
func (s *GroupsIOService) Tags() []string {
	var tags []string

	if s == nil {
		return nil
	}

	if s.ProjectUID != "" {
		tag := fmt.Sprintf("project_uid:%s", s.ProjectUID)
		tags = append(tags, tag)
	}

	if s.ProjectSlug != "" {
		tag := fmt.Sprintf("project_slug:%s", s.ProjectSlug)
		tags = append(tags, tag)
	}

	if s.UID != "" {
		tags = append(tags, s.UID)
		tag := fmt.Sprintf("service_uid:%s", s.UID)
		tags = append(tags, tag)
	}

	if s.Type != "" {
		tag := fmt.Sprintf("service_type:%s", s.Type)
		tags = append(tags, tag)
	}

	return tags
}

// GetDomain returns the appropriate domain for Groups.io API calls
func (s *GroupsIOService) GetDomain() string {
	if s.Domain != "" {
		return s.Domain // Use custom domain if set
	}
	return DefaultGroupsIODomain // Default to groups.io
}

// GetGroupName returns the appropriate group name for Groups.io API calls with comprehensive fallback logic
func (s *GroupsIOService) GetGroupName() string {
	if s.GroupName != "" {
		return s.GroupName // Use explicit group name if set
	}

	switch s.Type {
	case constants.ServiceTypePrimary:
		return s.ProjectSlug
	case constants.ServiceTypeFormation:
		return fmt.Sprintf("%s-formation", s.ProjectSlug)
	case constants.ServiceTypeShared:
		return s.ProjectSlug // fallback for shared services
	default:
		return s.ProjectUID // fallback for unknown types
	}
}
