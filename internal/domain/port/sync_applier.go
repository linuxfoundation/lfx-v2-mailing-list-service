// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// SyncApplier applies an admin-approved subset of a mailing list's sync-diff: adding resolved
// committee members and removing members who have dropped off the linked committee.
type SyncApplier interface {
	// ApplySyncDiff applies approvedAdds and approvedRemoves — both expected to be drawn from a
	// SyncDiff previously computed for mailingListID — and reports the outcome of each. Returns
	// NotFound when the mailing list or its linked committee does not exist. Returns Validation
	// when the mailing list has no linked committee or its sync_mode does not support apply.
	ApplySyncDiff(
		ctx context.Context,
		mailingListID string,
		approvedAdds []model.SyncDiffAdd,
		approvedRemoves []model.SyncDiffRemove,
	) (*model.SyncApplyResult, error)
}
