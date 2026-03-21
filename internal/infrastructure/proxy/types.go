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
