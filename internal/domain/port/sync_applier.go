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
	ApplySyncDiff(
		ctx context.Context,
		mailingListID string,
		expectedCommitteeUID string,
		approvedAdds []model.SyncDiffAdd,
		approvedRemoves []model.SyncDiffRemove,
	) (*model.SyncApplyResult, error)
}
