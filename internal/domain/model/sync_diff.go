// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// MemberType identifies which roster a sync diff/apply entry's MemberID belongs to. The same
// entry types (SyncDiffAdd, SyncDiffRemove, ...) are used for both sync_mode directions, so the
// roster can't be inferred from the field alone.
type MemberType string

const (
	MemberTypeCommitteeMember   MemberType = "committee_member"
	MemberTypeMailingListMember MemberType = "mailing_list_member"
)

// SyncDiffAdd is a member present on one roster but missing from the other, who should be added
// to reconcile them. MemberType says which roster MemberID belongs to.
type SyncDiffAdd struct {
	Email      string     `json:"email"`
	MemberID   string     `json:"member_id"`
	MemberType MemberType `json:"member_type"`
	// Name is the member's display name, carried through for sync-apply so a failed email
	// resolution can still show an admin who this is, even without a fuzzy name match.
	Name string `json:"name,omitempty"`
}

// SyncDiffRemove is a member who should be removed from the roster MemberType identifies, because
// they no longer belong there.
type SyncDiffRemove struct {
	Email      string     `json:"email"`
	MemberID   string     `json:"member_id"`
	MemberType MemberType `json:"member_type"`
}

// SyncDiffConflict is an email collision between the committee roster and the mailing list
// roster that ITX could not confidently resolve into an add or remove.
type SyncDiffConflict struct {
	CommitteeEmail   string `json:"committee_email"`
	MailingListEmail string `json:"mailing_list_email"`
	Name             string `json:"name,omitempty"`
}

// SyncDiff is the result of reconciling a mailing list's membership against its linked
// committee's membership for the committee's configured sync_mode direction.
type SyncDiff struct {
	MailingListID string             `json:"mailing_list_id"`
	CommitteeUID  string             `json:"committee_uid"`
	SyncMode      string             `json:"sync_mode"`
	Adds          []SyncDiffAdd      `json:"adds"`
	Removes       []SyncDiffRemove   `json:"removes"`
	Conflicts     []SyncDiffConflict `json:"conflicts"`
}

// SyncDiffJobStatus is a point-in-time progress/result snapshot of an async sync-diff job started
// by StartSyncDiff, as returned by GetSyncDiffStatus. Diff is nil until Status reaches a terminal
// state ("completed"); a job that reaches "failed" never populates Diff, and ErrorMessage explains
// why instead.
type SyncDiffJobStatus struct {
	JobID          string `json:"job_id"`
	Status         string `json:"status"`
	ProcessedCount int    `json:"processed_count"`
	TotalCount     int    `json:"total_count"`
	ErrorMessage   string `json:"error_message,omitempty"`
	// Diff is populated only once Status is "completed".
	Diff *SyncDiff `json:"diff,omitempty"`
}
