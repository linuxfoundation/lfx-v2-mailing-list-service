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
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/utils"
)

// DefaultGroupsIODomain is the default domain for Groups.io API calls
const DefaultGroupsIODomain = "groups.io"

// GrpsIOService represents a GroupsIO service entity
type GrpsIOService struct {
	Type             string    `json:"type"`
	UID              string    `json:"uid"`
	Domain           string    `json:"domain"`
	GroupID          *int64    `json:"-"` // Groups.io group ID - internal use only, nullable for async
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
	LastReviewedAt   *string   `json:"last_reviewed_at,omitempty"`
	LastReviewedBy   *string   `json:"last_reviewed_by,omitempty"`
	Writers          []string  `json:"writers"`
	Auditors         []string  `json:"auditors"`
}

// BuildIndexKey generates a SHA-256 hash for use as a NATS KV key
// This is necessary because the original input may contain special characters,
// exceed length limits, or have inconsistent formatting, and we do not control its content.
// Using a hash ensures a safe, fixed-length, and deterministic key.
func (s *GrpsIOService) BuildIndexKey(ctx context.Context) string {
	// Combine project_uid and service type/identifier with a delimiter
	var data string
	switch s.Type {
	case constants.ServiceTypePrimary:
		// Primary service: unique by project only
		data = fmt.Sprintf("%s|%s", s.ProjectUID, s.Type)
	case constants.ServiceTypeFormation:
		// Formation service: unique by project + prefix
		data = fmt.Sprintf("%s|%s|%s", s.ProjectUID, s.Type, s.Prefix)
	case constants.ServiceTypeShared:
		// Shared service: unique by project + group_name (decoupled from GroupID)
		data = fmt.Sprintf("%s|%s|%s", s.ProjectUID, s.Type, s.GroupName)
	default:
		// Fallback for unknown types
		data = fmt.Sprintf("%s|%s|%s", s.ProjectUID, s.Type, s.UID)
	}

	hash := sha256.Sum256([]byte(data))
	key := hex.EncodeToString(hash[:])

	slog.DebugContext(ctx, "index key built",
		"project_uid", s.ProjectUID,
		"service_type", s.Type,
		"service_prefix", s.Prefix,
		"service_group_name", s.GroupName,
		"key", key,
	)

	return key
}

// Tags generates a consistent set of tags for the GrpsIO service
func (s *GrpsIOService) Tags() []string {
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

// ValidateLastReviewedAt validates the LastReviewedAt timestamp format.
// Returns nil if the field is nil (allowed) or contains a valid RFC3339 timestamp.
func (s *GrpsIOService) ValidateLastReviewedAt() error {
	return utils.ValidateRFC3339Ptr(s.LastReviewedAt)
}

// GetLastReviewedAtTime safely parses LastReviewedAt into a time.Time pointer.
// Returns nil if the field is nil or empty, or the parsed time if valid.
func (s *GrpsIOService) GetLastReviewedAtTime() (*time.Time, error) {
	return utils.ParseTimestampPtr(s.LastReviewedAt)
}

// GetDomain returns the appropriate domain for Groups.io API calls
func (s *GrpsIOService) GetDomain() string {
	if s.Domain != "" {
		return s.Domain // Use custom domain if set
	}
	return DefaultGroupsIODomain // Default to groups.io
}

// GetGroupName returns the appropriate group name for Groups.io API calls with comprehensive fallback logic
func (s *GrpsIOService) GetGroupName() string {
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
