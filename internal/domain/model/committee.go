// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// Committee represents a committee associated with a mailing list.
// Multiple committees can be associated with a single mailing list,
// and any committee grants access (OR logic for access control).
type Committee struct {
	// UID is the unique identifier of the committee (required).
	UID string `json:"uid"`

	// Name is the display name of the committee (read-only, populated by server).
	Name string `json:"name,omitempty"`

	// AllowedVotingStatuses are the committee member voting statuses that determine which members
	// are synced to the mailing list (e.g., "Voting Rep", "Alternate Voting Rep").
	AllowedVotingStatuses []string `json:"allowed_voting_statuses,omitempty"`

	// SyncMode controls the direction of automatic membership synchronization between this
	// committee and the mailing list it's attached to. One of the SyncMode* constants below;
	// empty is treated the same as SyncModeManual. This is v2-local state: ITX has no concept
	// of it, so it is persisted separately (see port.CommitteeSyncSettingsStore) and merged
	// into this struct on read.
	SyncMode string `json:"sync_mode,omitempty"`
}

const (
	// SyncModeManual disables automatic membership synchronization between the committee and
	// the mailing list; membership changes must be made by hand on both sides.
	SyncModeManual = "manual"
	// SyncModeAutoCommitteeToMailingList automatically propagates committee membership changes
	// to the mailing list (adds/removes mailing list members to match the committee roster).
	SyncModeAutoCommitteeToMailingList = "auto_c_to_ml"
	// SyncModeAutoMailingListToCommittee automatically propagates mailing list membership
	// changes to the committee roster.
	SyncModeAutoMailingListToCommittee = "auto_ml_to_c"
)

// ValidSyncModes lists every accepted value for Committee.SyncMode.
var ValidSyncModes = []string{SyncModeManual, SyncModeAutoCommitteeToMailingList, SyncModeAutoMailingListToCommittee}
