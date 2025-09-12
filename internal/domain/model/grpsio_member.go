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
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/redaction"
)

// GrpsIOMember represents a GroupsIO mailing list member
type GrpsIOMember struct {
	// Internal IDs (UUIDs)
	UID            string `json:"uid"`              // Primary key
	MailingListUID string `json:"mailing_list_uid"` // FK to mailing list

	// External Groups.io IDs
	GroupsIOMemberID int64 `json:"groupsio_member_id"` // From Groups.io
	GroupsIOGroupID  int64 `json:"groupsio_group_id"`  // From Groups.io

	// Member Information
	Username     string `json:"username"` // Username
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Email        string `json:"email"`        // Required, RFC 5322
	Organization string `json:"organization"` // Optional
	JobTitle     string `json:"job_title"`    // Optional

	// Member Configuration
	MemberType   string `json:"member_type"`   // "committee" or "direct"
	DeliveryMode string `json:"delivery_mode"` // Email delivery preference
	ModStatus    string `json:"mod_status"`    // "none", "moderator", "owner"

	// Status
	Status string `json:"status"` // Groups.io status: normal, pending, etc.

	LastReviewedAt *string `json:"last_reviewed_at"` // Nullable timestamp
	LastReviewedBy *string `json:"last_reviewed_by"` // Nullable user ID

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BuildIndexKey generates a SHA-256 hash for use as a NATS KV key.
// This enforces uniqueness for members within a mailing list.
func (m *GrpsIOMember) BuildIndexKey(ctx context.Context) string {
	mailingList := strings.TrimSpace(strings.ToLower(m.MailingListUID))
	email := strings.TrimSpace(strings.ToLower(m.Email))

	// Combine normalized values with a delimiter
	data := fmt.Sprintf("%s|%s", mailingList, email)

	hash := sha256.Sum256([]byte(data))
	key := hex.EncodeToString(hash[:])

	slog.DebugContext(ctx, "member index key built",
		"mailing_list_uid", m.MailingListUID,
		"email", redaction.RedactEmail(m.Email),
		"key", key,
	)

	return key
}

// Tags generates a consistent set of tags for the member.
func (m *GrpsIOMember) Tags() []string {
	var tags []string

	if m == nil {
		return nil
	}

	if m.UID != "" {
		tags = append(tags, m.UID)
		tag := fmt.Sprintf("member_uid:%s", m.UID)
		tags = append(tags, tag)
	}

	if m.MailingListUID != "" {
		tag := fmt.Sprintf("mailing_list_uid:%s", m.MailingListUID)
		tags = append(tags, tag)
	}

	if m.Username != "" {
		tag := fmt.Sprintf("username:%s", m.Username)
		tags = append(tags, tag)
	}

	if m.Email != "" {
		tag := fmt.Sprintf("email:%s", m.Email)
		tags = append(tags, tag)
	}

	if m.Status != "" {
		tag := fmt.Sprintf("status:%s", m.Status)
		tags = append(tags, tag)
	}

	return tags
}
