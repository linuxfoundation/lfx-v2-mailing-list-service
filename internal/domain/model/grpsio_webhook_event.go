// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import "time"

// GrpsIOWebhookEvent represents a parsed webhook event from Groups.io
type GrpsIOWebhookEvent struct {
	ID         int         `json:"id"`
	Action     string      `json:"action"`
	Group      *GroupInfo  `json:"group,omitempty"`
	MemberInfo *MemberInfo `json:"member_info,omitempty"`
	Extra      string      `json:"extra,omitempty"`    // Subgroup suffix
	ExtraID    int         `json:"extra_id,omitempty"` // Subgroup ID for deletion
	ReceivedAt time.Time   `json:"received_at,omitempty"`
}

// GroupInfo represents group information in webhook event
// Note: Minimal fields for internal processing. Full GroupCreated struct (100+ fields)
// available in production (itx-service-groupsio/pkg/models/models.go:56-211) if needed.
type GroupInfo struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	ParentGroupID int    `json:"parent_group_id"`
}

// MemberInfo represents member information in webhook event
// TODO: For NATS publishing PR - Add these fields from production (itx-service-groupsio/pkg/models/models.go:213-224):
//   - Object string `json:"object"` - Groups.io object type
//   - Created time.Time `json:"created"` - Member creation timestamp
//   - Updated time.Time `json:"updated"` - Last update timestamp
//
// These fields are required when publishing member events to NATS for consumption by:
//   - Zoom event handler
//   - Query service/indexer
//   - Other downstream services
type MemberInfo struct {
	ID        int    `json:"id"`
	UserID    int    `json:"user_id"`
	GroupID   uint64 `json:"group_id"`
	GroupName string `json:"group_name"`
	Email     string `json:"email"`
	Status    string `json:"status"`
}
