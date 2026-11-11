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
	// directionOverride, when non-empty, is used instead of the direction implied by the mailing
	// list's configured sync_mode — only valid when sync_mode is "manual" or unset (Validation
	// otherwise); see resolveSyncJobDirection for why. Pass "" to use sync_mode's own direction.
	// Returns Validation when the mailing list has no linked committee, directionOverride is
	// empty and sync_mode is "manual", or directionOverride is invalid for the current sync_mode.
	// Returns NotFound when the mailing list or its linked committee does not exist.
	//
	// ComputeSyncDiff blocks until the underlying ITX job completes (or SyncJobPollTimeout
	// elapses) — for a non-blocking alternative, see StartSyncDiff/GetSyncDiffStatus.
	ComputeSyncDiff(ctx context.Context, mailingListID string, directionOverride string) (*model.SyncDiff, error)

	// StartSyncDiff performs the same validation and committee-writer access check as
	// ComputeSyncDiff (including directionOverride's rules), then starts the underlying ITX
	// sync-diff job and returns its job ID immediately, without waiting for it to complete.
	// Returns the same Validation/NotFound/Unauthorized errors ComputeSyncDiff does for the same
	// reasons.
	StartSyncDiff(ctx context.Context, mailingListID string, directionOverride string) (jobID string, err error)

	// GetSyncDiffStatus returns the current progress/result of a sync-diff job started by
	// StartSyncDiff. directionOverride must match the value StartSyncDiff was called with for
	// this same job (including "" if none was given) — it is re-applied here because this method
	// re-derives the job's expected direction from the mailing list's CURRENT sync_mode rather
	// than persisting it, the same way it re-runs the committee-writer access check on every call
	// (StartSyncDiff's own check is not reusable across polls, since a caller's access can change
	// between calls). Returns Conflict if the mailing list's linked committee or effective
	// direction has changed since the job was started (the same relink-safety check ComputeSyncDiff
	// performs after polling, applied statelessly).
	GetSyncDiffStatus(ctx context.Context, mailingListID, jobID string, directionOverride string) (*model.SyncDiffJobStatus, error)
}
