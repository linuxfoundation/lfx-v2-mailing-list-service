// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// SyncApplyOutcome classifies how an individual add or remove in a sync-apply request was
// handled. Keeping this as a dedicated enum (rather than inferring success from an empty
// Reason) is what lets a caller tell "invited, no LFX account yet" apart from a hard failure —
// the PCC-1604 lesson: both look like "didn't add directly," but only one needs admin
// attention.
type SyncApplyOutcome string

const (
	// SyncApplyOutcomeAdded means the member was resolved to an LFX account and added directly.
	SyncApplyOutcomeAdded SyncApplyOutcome = "added"
	// SyncApplyOutcomeInvited means no LFX account was found for the email, so an invite was
	// sent instead of a direct add.
	SyncApplyOutcomeInvited SyncApplyOutcome = "invited"
	// SyncApplyOutcomeRemoved means the member was removed from the mailing list.
	SyncApplyOutcomeRemoved SyncApplyOutcome = "removed"
	// SyncApplyOutcomeSkippedCrossList means a requested removal was skipped because the member
	// is still subscribed via another mailing list linked to the same committee.
	SyncApplyOutcomeSkippedCrossList SyncApplyOutcome = "skipped_cross_list"
	// SyncApplyOutcomeFailed means the add or remove could not be completed.
	SyncApplyOutcomeFailed SyncApplyOutcome = "failed"
)

// SyncApplyAddResult reports what happened when applying one approved SyncDiffAdd.
type SyncApplyAddResult struct {
	Email              string           `json:"email"`
	CommitteeMemberUID string           `json:"committee_member_uid"`
	Outcome            SyncApplyOutcome `json:"outcome"`
	// NameHint is the committee member's display name, surfaced for SyncApplyOutcomeInvited so
	// an admin reviewing the result can tell who this is. It is not a fuzzy name match — that
	// requires a user-search-by-name capability this service's upstreams do not yet expose (see
	// linuxfoundation/lfx-self-serve#4339) — just the name already known from the committee
	// roster.
	NameHint string `json:"name_hint,omitempty"`
	Error    string `json:"error,omitempty"`
}

// SyncApplyRemoveResult reports what happened when applying one approved SyncDiffRemove.
type SyncApplyRemoveResult struct {
	Email               string           `json:"email"`
	MailingListMemberID string           `json:"mailing_list_member_id"`
	Outcome             SyncApplyOutcome `json:"outcome"`
	Error               string           `json:"error,omitempty"`
}

// SyncApplyResult is the outcome of applying an admin-approved subset of a SyncDiff.
type SyncApplyResult struct {
	MailingListID string                  `json:"mailing_list_id"`
	CommitteeUID  string                  `json:"committee_uid"`
	Added         []SyncApplyAddResult    `json:"added"`
	Removed       []SyncApplyRemoveResult `json:"removed"`
}
