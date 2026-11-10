// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// SyncApplier applies an admin-approved subset of a mailing list's sync-diff: adding and removing
// members on the sync-diff's target side (committee or mailing list, per the linked committee's
// sync_mode) to reconcile it against the source side.
type SyncApplier interface {
	// ApplySyncDiff applies approvedAdds and approvedRemoves — both expected to be drawn from the
	// SyncDiff previously computed for mailingListID with committee_uid expectedCommitteeUID — and
	// reports the outcome of each. Returns NotFound when the mailing list or its linked committee
	// does not exist. Returns Validation when the mailing list has no linked committee or its
	// sync_mode does not support apply. Returns Conflict when the mailing list's currently-linked
	// committee no longer matches expectedCommitteeUID. Returns Unauthorized when the caller lacks
	// writer access to the linked committee.
	//
	// ApplySyncDiff blocks until the underlying ITX job completes (or SyncJobPollTimeout
	// elapses) — for a non-blocking alternative, see StartSyncApply/GetSyncApplyStatus.
	ApplySyncDiff(
		ctx context.Context,
		mailingListID string,
		expectedCommitteeUID string,
		approvedAdds []model.SyncDiffAdd,
		approvedRemoves []model.SyncDiffRemove,
	) (*model.SyncApplyResult, error)

	// StartSyncApply performs the same validation, committee-writer access check, and
	// expectedCommitteeUID Conflict check as ApplySyncDiff, then starts the underlying ITX
	// sync-apply job and returns its job ID immediately, without waiting for it to complete.
	// Returns the same NotFound/Validation/Conflict/Unauthorized errors ApplySyncDiff does for the
	// same reasons.
	StartSyncApply(
		ctx context.Context,
		mailingListID string,
		expectedCommitteeUID string,
		approvedAdds []model.SyncDiffAdd,
		approvedRemoves []model.SyncDiffRemove,
	) (jobID string, err error)

	// GetSyncApplyStatus returns the current progress/result of a sync-apply job started by
	// StartSyncApply. It re-runs the committee-writer access check against the mailing list's
	// CURRENT linked committee on every call. Returns Conflict if the mailing list's linked
	// committee has changed since the job was started (the same relink-safety check ApplySyncDiff
	// performs after polling, applied statelessly). Unlike ApplySyncDiff's result, the per-entry
	// results returned here carry no MemberID/MemberType/NameHint enrichment — see
	// model.SyncJobResultEntry.
	GetSyncApplyStatus(ctx context.Context, mailingListID, jobID string) (*model.SyncApplyJobStatus, error)
}
