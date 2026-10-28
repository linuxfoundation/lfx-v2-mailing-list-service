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
	ID             string
	Status         string
	ToAdd          []SyncJobMember
	ToRemove       []SyncJobMember
	EmailConflicts []SyncJobConflict
	ErrorMessage   string
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
	ID           string
	Status       string
	Results      []SyncApplyJobResult
	ErrorMessage string
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
	// approvedAdds (emails) and approvedRemoves (mailing-list member IDs). diffJobID, when set,
	// links the apply job back to the sync-diff job it was approved from. Returns the new job's
	// ID.
	PostSyncApply(ctx context.Context, mailingListID, direction string, approvedAdds, approvedRemoves []string, diffJobID string) (jobID string, err error)

	// GetSyncApplyJob returns the current state of a sync-apply job started by PostSyncApply.
	GetSyncApplyJob(ctx context.Context, mailingListID, jobID string) (*SyncApplyJob, error)
}
