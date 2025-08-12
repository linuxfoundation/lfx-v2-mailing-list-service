// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package model defines the domain models and entities for the mailing list service.
package model

import (
	"time"
)

// GrpsIOService represents a GroupsIO service entity
type GrpsIOService struct {
	Type         string    `json:"type"`
	ID           string    `json:"id"`
	Domain       string    `json:"domain"`
	GroupID      int64     `json:"group_id"`
	Status       string    `json:"status"`
	GlobalOwners []string  `json:"global_owners"`
	Prefix       string    `json:"prefix"`
	ProjectSlug  string    `json:"project_slug"`
	ProjectID    string    `json:"project_id"`
	URL          string    `json:"url"`
	GroupName    string    `json:"group_name"`
	Public       bool      `json:"public"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
