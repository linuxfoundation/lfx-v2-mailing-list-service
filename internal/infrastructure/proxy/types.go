// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package proxy provides the ITX HTTP proxy client for GroupsIO operations.
package proxy

// serviceWire represents a GroupsIO service as returned by the ITX API.
type serviceWire struct {
	ID        string `json:"id,omitempty"`
	ProjectID string `json:"project_id,omitempty"` // v1 SFID
	Type      string `json:"type,omitempty"`
	GroupID   int64  `json:"group_id,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"last_modified_at,omitempty"`
}

// serviceRequestWire represents a create/update request for a GroupsIO service.
type serviceRequestWire struct {
	ProjectID string `json:"project_id,omitempty"` // v1 SFID
	Type      string `json:"type,omitempty"`
	GroupID   int64  `json:"group_id,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Status    string `json:"status,omitempty"`
}

// serviceListResponseWire represents a list response of GroupsIO services from the ITX API.
type serviceListResponseWire struct {
	Items []*serviceWire `json:"items"`
	Total int            `json:"total"`
}

// subgroupWire represents a GroupsIO subgroup (mailing list) as returned by the ITX API.
type subgroupWire struct {
	ID             string `json:"id,omitempty"`
	ProjectID      string `json:"project_id,omitempty"`  // v1 SFID
	CommitteeID    string `json:"committee,omitempty"`   // v1 UUID
	ParentID       string `json:"parent_id,omitempty"`   // v1 Service ID
	GroupID        int64  `json:"group_id,omitempty"`
	Name           string `json:"group_name,omitempty"`
	Description    string `json:"description,omitempty"`
	Type           string `json:"type,omitempty"`
	AudienceAccess string `json:"visibility,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"last_modified_at,omitempty"`
}

// subgroupRequestWire represents a create/update request for a GroupsIO subgroup.
type subgroupRequestWire struct {
	ProjectID      string `json:"project_id,omitempty"` // v1 SFID
	CommitteeID    string `json:"committee,omitempty"`  // v1 UUID
	ParentID       string `json:"parent_id,omitempty"`  // v1 Service ID
	Name           string `json:"group_name,omitempty"`
	Description    string `json:"description,omitempty"`
	Type           string `json:"type,omitempty"`
	AudienceAccess string `json:"visibility,omitempty"`
}

// subgroupListResponseWire represents a list response of GroupsIO subgroups from the ITX API.
type subgroupListResponseWire struct {
	Items []*subgroupWire `json:"items"`
	Total int             `json:"total"`
}

// countResponseWire represents a count response from the ITX API.
type countResponseWire struct {
	Count int `json:"count"`
}
