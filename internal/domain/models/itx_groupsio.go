// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package models contains domain model types for ITX GroupsIO integration.
package models

// GroupsioService represents a GroupsIO service in the ITX API.
type GroupsioService struct {
	ID        string `json:"id,omitempty"`
	ProjectID string `json:"project_id,omitempty"` // v1 SFID in ITX, v2 UUID in our API
	Type      string `json:"type,omitempty"`
	GroupID   int64  `json:"group_id,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"last_modified_at,omitempty"`
}

// GroupsioServiceRequest represents a create/update request for a GroupsIO service.
type GroupsioServiceRequest struct {
	ProjectID string `json:"project_id,omitempty"` // v1 SFID
	Type      string `json:"type,omitempty"`
	GroupID   int64  `json:"group_id,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Status    string `json:"status,omitempty"`
}

// GroupsioServiceListResponse represents a list of GroupsIO services.
type GroupsioServiceListResponse struct {
	Items []*GroupsioService `json:"items,omitempty"`
	Total int                `json:"total,omitempty"`
}

// GroupsioServiceProjectsResponse represents a list of projects with services.
type GroupsioServiceProjectsResponse struct {
	Projects []string `json:"projects,omitempty"`
}

// GroupsioSubgroup represents a GroupsIO subgroup (mailing list) in the ITX API.
type GroupsioSubgroup struct {
	ID             string `json:"id,omitempty"`
	ProjectID      string `json:"project_id,omitempty"` // v1 SFID in ITX, v2 UUID in our API
	CommitteeID    string `json:"committee,omitempty"`  // v1 UUID in ITX, v2 UUID in our API
	ParentID       string `json:"parent_id,omitempty"`  // v1 Service ID
	GroupID        int64  `json:"group_id,omitempty"`
	Name           string `json:"group_name,omitempty"`
	Description    string `json:"description,omitempty"`
	Type           string `json:"type,omitempty"`
	AudienceAccess string `json:"visibility,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"last_modified_at,omitempty"`
}

// GroupsioSubgroupRequest represents a create/update request for a GroupsIO subgroup.
type GroupsioSubgroupRequest struct {
	ProjectID      string `json:"project_id,omitempty"` // v1 SFID
	CommitteeID    string `json:"committee,omitempty"`  // v1 UUID
	ParentID       string `json:"parent_id,omitempty"`  // v1 Service ID
	GroupID        int64  `json:"group_id,omitempty"`
	Name           string `json:"group_name,omitempty"`
	Description    string `json:"description,omitempty"`
	Type           string `json:"type,omitempty"`
	AudienceAccess string `json:"visibility,omitempty"`
}

// GroupsioSubgroupMeta represents pagination metadata in the ITX list response.
type GroupsioSubgroupMeta struct {
	PageToken    string `json:"page_token"`
	TotalPages   int    `json:"total_pages"`
	TotalResults int    `json:"total_results"`
	PerPage      int    `json:"per_page"`
}

// GroupsioSubgroupListResponse represents a list of GroupsIO subgroups.
type GroupsioSubgroupListResponse struct {
	Items []*GroupsioSubgroup  `json:"data"`
	Meta  GroupsioSubgroupMeta `json:"meta"`
}

// GroupsioSubgroupCountResponse represents the count of subgroups.
type GroupsioSubgroupCountResponse struct {
	Count int `json:"count"`
}

// GroupsioMemberCountResponse represents the count of members in a subgroup.
type GroupsioMemberCountResponse struct {
	Count int `json:"count"`
}

// GroupsioMember represents a member of a GroupsIO subgroup.
type GroupsioMember struct {
	ID           string `json:"id,omitempty"`
	SubgroupID   string `json:"subgroup_id,omitempty"`
	Email        string `json:"email,omitempty"`
	Name         string `json:"name,omitempty"`
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	ModStatus    string `json:"mod_status,omitempty"`
	DeliveryMode string `json:"delivery_mode,omitempty"`
	Status       string `json:"status,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

// GroupsioMemberRequest represents a create/update request for a GroupsIO member.
type GroupsioMemberRequest struct {
	Email        string `json:"email,omitempty"`
	Name         string `json:"name,omitempty"`
	ModStatus    string `json:"mod_status,omitempty"`
	DeliveryMode string `json:"delivery_mode,omitempty"`
}

// GroupsioMemberListResponse represents a list of GroupsIO members.
type GroupsioMemberListResponse struct {
	Items []*GroupsioMember `json:"items,omitempty"`
	Total int               `json:"total,omitempty"`
}

// GroupsioInviteMembersRequest represents an invite members request.
type GroupsioInviteMembersRequest struct {
	Emails []string `json:"emails,omitempty"`
}

// GroupsioCheckSubscriberRequest represents a check subscriber request.
type GroupsioCheckSubscriberRequest struct {
	Email      string `json:"email,omitempty"`
	SubgroupID string `json:"subgroup_id,omitempty"`
}

// GroupsioCheckSubscriberResponse represents a check subscriber response.
type GroupsioCheckSubscriberResponse struct {
	Subscribed bool `json:"subscribed"`
}

// GroupsioFindParentResponse represents the find parent service response.
type GroupsioFindParentResponse struct {
	Service *GroupsioService `json:"service,omitempty"`
}
