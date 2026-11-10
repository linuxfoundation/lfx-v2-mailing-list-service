// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// SyncApplyOutcome classifies how an individual add or remove in a sync-apply request was
// handled. ITX performs the add/remove itself and reports back only success or failure per
// entry — it has no "invited" concept (no LFX-account resolution) or "skipped_cross_list"
// concept of its own, so those outcomes never appear here.
type SyncApplyOutcome string

const (
	// SyncApplyOutcomeAdded means the member was added.
	SyncApplyOutcomeAdded SyncApplyOutcome = "added"
	// SyncApplyOutcomeRemoved means the member was removed from the mailing list.
	SyncApplyOutcomeRemoved SyncApplyOutcome = "removed"
	// SyncApplyOutcomeFailed means the add or remove could not be completed.
	SyncApplyOutcomeFailed SyncApplyOutcome = "failed"
)

// SyncApplyAddResult reports what happened when applying one approved SyncDiffAdd.
type SyncApplyAddResult struct {
	Email      string           `json:"email"`
	MemberID   string           `json:"member_id"`
	MemberType MemberType       `json:"member_type"`
	Outcome    SyncApplyOutcome `json:"outcome"`
	// NameHint is the member's display name, echoed from the source roster so an admin reviewing
	// the result can confirm who this is. It is not a fuzzy name match — that requires a
	// user-search-by-name capability this service's upstreams do not yet expose (see
	// linuxfoundation/lfx-self-serve#4364) — just the name already known from the source roster.
	NameHint string `json:"name_hint,omitempty"`
	Error    string `json:"error,omitempty"`
}

// SyncApplyRemoveResult reports what happened when applying one approved SyncDiffRemove.
type SyncApplyRemoveResult struct {
	Email      string           `json:"email"`
	MemberID   string           `json:"member_id"`
	MemberType MemberType       `json:"member_type"`
	Outcome    SyncApplyOutcome `json:"outcome"`
	Error      string           `json:"error,omitempty"`
}

// SyncApplyResult is the outcome of applying an admin-approved subset of a SyncDiff.
type SyncApplyResult struct {
	MailingListID string                  `json:"mailing_list_id"`
	CommitteeUID  string                  `json:"committee_uid"`
	Added         []SyncApplyAddResult    `json:"added"`
	Removed       []SyncApplyRemoveResult `json:"removed"`
}

// SyncJobResultEntry is one add/remove outcome from an async sync-apply job, as reported directly
// by ITX (flat email/action/status/error) — unlike SyncApplyAddResult/SyncApplyRemoveResult, it
// carries no MemberID/MemberType/NameHint enrichment, since GetSyncApplyStatus has no persisted
// state linking a result's email back to the request-time add/remove entry it came from (see
// StartSyncApply/GetSyncApplyStatus doc comments).
type SyncJobResultEntry struct {
	Email  string `json:"email"`
	Action string `json:"action"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// SyncApplyJobStatus is a point-in-time progress/result snapshot of an async sync-apply job
// started by StartSyncApply, as returned by GetSyncApplyStatus. AddedCount/RemovedCount/ErrorCount
// are ITX's own pre-aggregated tallies of Results by outcome. Results is populated once Status
// reaches a terminal state ("completed" or "failed") — ITX records partial results for entries
// processed before a mid-job failure, so Results and ErrorMessage can both be present.
type SyncApplyJobStatus struct {
	JobID          string               `json:"job_id"`
	Status         string               `json:"status"`
	ProcessedCount int                  `json:"processed_count"`
	TotalCount     int                  `json:"total_count"`
	AddedCount     int                  `json:"added_count"`
	RemovedCount   int                  `json:"removed_count"`
	ErrorCount     int                  `json:"error_count"`
	ErrorMessage   string               `json:"error_message,omitempty"`
	Results        []SyncJobResultEntry `json:"results,omitempty"`
}
