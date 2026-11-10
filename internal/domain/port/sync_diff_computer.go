// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// SyncDiffComputer computes the membership reconciliation diff between a mailing list and its
// linked committee, per the committee's configured sync_mode.
type SyncDiffComputer interface {
	// ComputeSyncDiff returns the adds/removes needed to reconcile mailingListID's membership
	// with its linked committee, for either sync direction ("auto_c_to_ml" or "auto_ml_to_c").
	// Returns Validation when the mailing list has no linked committee or its sync_mode is
	// "manual". Returns NotFound when the mailing list or its linked committee does not exist.
	//
	// ComputeSyncDiff blocks until the underlying ITX job completes (or SyncJobPollTimeout
	// elapses) — for a non-blocking alternative, see StartSyncDiff/GetSyncDiffStatus.
	ComputeSyncDiff(ctx context.Context, mailingListID string) (*model.SyncDiff, error)

	// StartSyncDiff performs the same validation and committee-writer access check as
	// ComputeSyncDiff, then starts the underlying ITX sync-diff job and returns its job ID
	// immediately, without waiting for it to complete. Returns the same Validation/NotFound/
	// Unauthorized errors ComputeSyncDiff does for the same reasons.
	StartSyncDiff(ctx context.Context, mailingListID string) (jobID string, err error)

	// GetSyncDiffStatus returns the current progress/result of a sync-diff job started by
	// StartSyncDiff. It re-runs the committee-writer access check against the mailing list's
	// CURRENT linked committee on every call (StartSyncDiff's own check is not reusable across
	// polls, since a caller's access can change between calls). Returns Conflict if the mailing
	// list's linked committee has changed since the job was started (the same relink-safety
	// check ComputeSyncDiff performs after polling, applied statelessly).
	GetSyncDiffStatus(ctx context.Context, mailingListID, jobID string) (*model.SyncDiffJobStatus, error)
}
