// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package model defines the domain models and entities for the mailing list service.
package model

import (
	"fmt"
	"strings"
	"time"
)

// Mailing list type constants.
const (
	TypeAnnouncement        = "announcement"
	TypeDiscussionModerated = "discussion_moderated"
	TypeDiscussionOpen      = "discussion_open"
)

// GroupsIOMailingList represents a GroupsIO mailing list entity with committee support
type GroupsIOMailingList struct {
	UID             string `json:"uid"`
	GroupID         *int64 `json:"group_id"` // Groups.io group ID
	GroupName       string `json:"group_name"`
	Public          bool   `json:"public"`           // Whether the mailing list is publicly accessible
	AudienceAccess  string `json:"audience_access"`  // "public" | "approval_required" | "invite_only"
	Source          string `json:"source"`           // "api", "webhook", or "mock" - tracks origin for business logic
	Type            string `json:"type"`             // "announcement" | "discussion_moderated" | "discussion_open"
	SubscriberCount int    `json:"subscriber_count"` // Number of members in this mailing list

	// Committee association - supports multiple committees with OR logic for access control
	Committees []Committee `json:"committees,omitempty"`

	Description string `json:"description"` // Minimum 11 characters
	Title       string `json:"title"`
	SubjectTag  string `json:"subject_tag"`  // Optional
	ServiceUID  string `json:"service_uid"`  // Service UUID (required)
	ProjectUID  string `json:"project_uid"`  // Inherited from parent service
	ProjectName string `json:"project_name"` // Inherited from parent service
	ProjectSlug string `json:"project_slug"` // Inherited from parent service

	URL   string   `json:"url,omitempty"`   // The groups.io URL for the subgroup
	Flags []string `json:"flags,omitempty"` // Warning messages about unusual settings

	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	SystemUpdatedAt time.Time `json:"system_updated_at,omitempty"` // Last modified by system (scripts/webhooks)
}

// GroupsIOMailingListSettings represents the settings for a GroupsIO mailing list (user management).
type GroupsIOMailingListSettings struct {
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

// Tags generates a consistent set of tags for the GrpsIO mailing list settings
func (s *GroupsIOMailingListSettings) Tags() []string {
	var tags []string

	if s == nil {
		return nil
	}

	if s.UID != "" {
		tags = append(tags, s.UID)
		tag := fmt.Sprintf("mailing_list_uid:%s", s.UID)
		tags = append(tags, tag)
	}

	return tags
}

// ParentRefs returns the parent resource references for indexing.
func (ml *GroupsIOMailingList) ParentRefs() []string {
	if ml == nil {
		return nil
	}
	var refs []string
	if ml.ServiceUID != "" {
		refs = append(refs, fmt.Sprintf("groupsio_service:%s", ml.ServiceUID))
	}
	if ml.ProjectUID != "" {
		refs = append(refs, fmt.Sprintf("project:%s", ml.ProjectUID))
	}
	for _, c := range ml.Committees {
		if c.UID != "" {
			refs = append(refs, fmt.Sprintf("committee:%s", c.UID))
		}
	}
	return refs
}

// NameAndAliases returns searchable names for the mailing list.
func (ml *GroupsIOMailingList) NameAndAliases() []string {
	if ml == nil {
		return nil
	}
	var names []string
	if ml.Title != "" {
		names = append(names, ml.Title)
	}
	if ml.GroupName != "" {
		names = append(names, ml.GroupName)
	}
	return names
}

// SortName returns the primary sort name for the mailing list.
func (ml *GroupsIOMailingList) SortName() string {
	if ml == nil {
		return ""
	}
	if ml.Title != "" {
		return ml.Title
	}
	return ml.GroupName
}

// Fulltext returns a concatenated string for full-text search.
func (ml *GroupsIOMailingList) Fulltext() string {
	if ml == nil {
		return ""
	}
	var parts []string
	if ml.Title != "" {
		parts = append(parts, ml.Title)
	}
	if ml.GroupName != "" {
		parts = append(parts, ml.GroupName)
	}
	if ml.Description != "" {
		parts = append(parts, ml.Description)
	}
	return strings.Join(parts, " ")
}

// ParentRefs returns the parent mailing list reference for settings indexing.
func (s *GroupsIOMailingListSettings) ParentRefs() []string {
	if s == nil {
		return nil
	}
	if s.UID != "" {
		return []string{fmt.Sprintf("groupsio_mailing_list:%s", s.UID)}
	}
	return nil
}

// Tags generates a consistent set of tags for the mailing list
func (ml *GroupsIOMailingList) Tags() []string {
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

	// Add audience_access tag
	if ml.AudienceAccess != "" {
		tags = append(tags, fmt.Sprintf("audience_access:%s", ml.AudienceAccess))
	}

	// Add committee tags for all associated committees
	for _, committee := range ml.Committees {
		if committee.UID != "" {
			tags = append(tags, fmt.Sprintf("committee_uid:%s", committee.UID))
		}
		// Add voting status tags for each committee
		for _, status := range committee.AllowedVotingStatuses {
			tags = append(tags, fmt.Sprintf("committee_voting_status:%s", status))
		}
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
