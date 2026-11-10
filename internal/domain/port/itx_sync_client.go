// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// ITX's sync-diff/sync-apply direction values. Distinct from a committee's sync_mode
// (SyncModeAutoCommitteeToMailingList etc.): direction names the one-shot reconciliation this job
// performs, not the mailing list's persistent configuration.
const (
	SyncJobDirectionCommitteeToMailingList = "c_to_ml"
	SyncJobDirectionMailingListToCommittee = "ml_to_c"
)

// ITX sync-diff/sync-apply job status values, shared by both job types.
const (
	SyncJobStatusPending   = "pending"
	SyncJobStatusOngoing   = "ongoing"
	SyncJobStatusCompleted = "completed"
	SyncJobStatusFailed    = "failed"
)

// SyncJobMember is a committee or mailing-list member referenced by a sync-diff job's to_add or
// to_remove list.
type SyncJobMember struct {
	MemberID string
	Email    string
	FullName string
}

// SyncJobConflict is an email collision between the committee roster and the mailing list roster,
// reported by a sync-diff job instead of a to_add/to_remove entry.
type SyncJobConflict struct {
	MLEmail        string
	CommitteeEmail string
	FullName       string
	MemberID       string
}

// SyncDiffJob is the state of an ITX sync-diff job, as returned by its GET endpoint.
type SyncDiffJob struct {
	ID     string
	Status string
	// CommitteeID is the v1 committee ID ITX actually computed this diff against — not
	// necessarily the mailing list's currently-linked committee, since the list may have been
	// relinked while the job ran. Empty if ITX's response omitted it.
	CommitteeID string
	// Direction is the sync direction ("c_to_ml" or "ml_to_c") ITX actually ran this job with —
	// carried through so a caller re-deriving the result after the fact (GetSyncDiffStatus) does
	// not need to re-resolve it from the mailing list's current sync_mode, which could have
	// changed since the job started.
	Direction      string
	ToAdd          []SyncJobMember
	ToRemove       []SyncJobMember
	EmailConflicts []SyncJobConflict
	ErrorMessage   string
	// ProcessedCount and TotalCount report ITX's own progress accounting for this job. Both are 0
	// until ITX starts reporting progress (e.g. while the job is still Pending).
	ProcessedCount int
	TotalCount     int
}

// SyncApplyJobResult is the outcome of one add or remove within a sync-apply job.
type SyncApplyJobResult struct {
	Email  string
	Action string // "add" or "remove"
	Status string // "success" or "error"
	Error  string
}

// SyncApplyJob is the state of an ITX sync-apply job, as returned by its GET endpoint.
type SyncApplyJob struct {
	ID     string
	Status string
	// CommitteeID is the v1 committee ID ITX actually applied this job against — not necessarily
	// the mailing list's currently-linked committee, since the list may have been relinked while
	// the job ran. Empty if ITX's response omitted it.
	CommitteeID  string
	Results      []SyncApplyJobResult
	ErrorMessage string
	// ProcessedCount and TotalCount report ITX's own progress accounting for this job.
	// AddedCount, RemovedCount, and ErrorCount are ITX's own pre-aggregated tallies of Results by
	// outcome, so a progress/status caller does not need to recompute them from Results itself.
	ProcessedCount int
	TotalCount     int
	AddedCount     int
	RemovedCount   int
	ErrorCount     int
}

// ITXSyncClient proxies committee<->mailing-list membership reconciliation to ITX's own
// sync-diff/sync-apply async job endpoints (itx-service-groupsio), which already compute the
// diff and apply changes natively — including voting-status filtering and sync-direction
// validation this service does not need to reimplement.
type ITXSyncClient interface {
	// PostSyncDiff starts a sync-diff job for mailingListID in the given direction and returns
	// its job ID. Returns Validation if a job is already in progress for this mailing list.
	PostSyncDiff(ctx context.Context, mailingListID, direction string) (jobID string, err error)

	// GetSyncDiffJob returns the current state of a sync-diff job started by PostSyncDiff.
	GetSyncDiffJob(ctx context.Context, mailingListID, jobID string) (*SyncDiffJob, error)

	// PostSyncApply starts a sync-apply job for mailingListID in the given direction, applying
	// approvedAdds and approvedRemoves (both emails — ITX's apply workers match each
	// approved_removes entry by email against the target roster, not by member ID). diffJobID,
	// when set, links the apply job back to the sync-diff job it was approved from. Returns the
	// new job's ID.
	PostSyncApply(ctx context.Context, mailingListID, direction string, approvedAdds, approvedRemoves []string, diffJobID string) (jobID string, err error)

	// GetSyncApplyJob returns the current state of a sync-apply job started by PostSyncApply.
	GetSyncApplyJob(ctx context.Context, mailingListID, jobID string) (*SyncApplyJob, error)
}
