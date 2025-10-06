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
	Extra      string      `json:"extra,omitempty"`      // Subgroup suffix
	ExtraID    int         `json:"extra_id,omitempty"`   // Subgroup ID for deletion
	ReceivedAt time.Time   `json:"received_at,omitempty"`
}

// GroupInfo represents group information in webhook event
type GroupInfo struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	ParentGroupID int    `json:"parent_group_id"`
}

// MemberInfo represents member information in webhook event
type MemberInfo struct {
	ID        int    `json:"id"`
	UserID    int    `json:"user_id"`
	GroupID   uint64 `json:"group_id"`
	GroupName string `json:"group_name"`
	Email     string `json:"email"`
	Status    string `json:"status"`
}
