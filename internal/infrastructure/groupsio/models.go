// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package groupsio

import "time"

// GroupObject represents a Groups.io group (simplified from go-groupsio)
type GroupObject struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
	Domain      string `json:"domain"`
	CreatedAt   string `json:"created"`
	UpdatedAt   string `json:"updated"`
}

// SubgroupObject represents a Groups.io subgroup (mailing list)
type SubgroupObject struct {
	ID          uint64 `json:"id"`
	GroupID     uint64 `json:"group_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
	Type        string `json:"type"` // announcement, discussion_moderated, discussion_open
	CreatedAt   string `json:"created"`
	UpdatedAt   string `json:"updated"`
}

// MemberObject represents a Groups.io member (simplified from go-groupsio)
type MemberObject struct {
	ID           uint64 `json:"id"`
	GroupID      uint64 `json:"group_id"`
	SubgroupID   uint64 `json:"subgroup_id,omitempty"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	Status       string `json:"status"`     // normal, pending, bouncing, etc.
	ModStatus    string `json:"mod_status"` // none, moderator, owner
	DeliveryMode string `json:"delivery"`   // individual, digest, no_email
	JoinedAt     string `json:"joined"`
	UpdatedAt    string `json:"updated"`
}

// LoginObject represents the Groups.io login response
type LoginObject struct {
	Token     string `json:"token"`
	User      string `json:"user"`
	UserID    uint64 `json:"user_id"`
	Email     string `json:"email"`
	ExpiresAt string `json:"expires_at"`
}

// ErrorObject represents a Groups.io API error response
type ErrorObject struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

// GroupCreateOptions represents options for creating a group
type GroupCreateOptions struct {
	GroupName   string `url:"group_name"`
	Description string `url:"description"`
	Public      bool   `url:"public"`
	Domain      string `url:"domain,omitempty"`
}

// SubgroupCreateOptions represents options for creating a subgroup
type SubgroupCreateOptions struct {
	SubgroupName string `url:"subgroup_name"`
	Description  string `url:"description"`
	Public       bool   `url:"public"`
	Type         string `url:"type"` // announcement, discussion_moderated, discussion_open
	SubjectTag   string `url:"subject_tag,omitempty"`
}

// MemberUpdateOptions represents options for updating a member
type MemberUpdateOptions struct {
	ModStatus    string `url:"mod_status,omitempty"` // none, moderator, owner
	DeliveryMode string `url:"delivery,omitempty"`   // individual, digest, no_email
	Status       string `url:"status,omitempty"`     // normal, pending, bouncing
	FirstName    string `url:"first_name,omitempty"`
	LastName     string `url:"last_name,omitempty"`
}

// TokenCache represents a cached authentication token
type TokenCache struct {
	Token     string
	ExpiresAt time.Time
}
