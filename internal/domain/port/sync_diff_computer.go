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
	ComputeSyncDiff(ctx context.Context, mailingListID string) (*model.SyncDiff, error)
}
