// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// RealtimeAutoSyncer reacts to a single committee-membership or Groups.io subscriber event by
// reconciling a mailing list's auto sync in real time (issue #4023), gated by the mailing list's
// visibility and the cross-list removal guard. See service.RealtimeAutoSyncOrchestrator for the
// implementation.
type RealtimeAutoSyncer interface {
	TriggerAutoSync(ctx context.Context, mailingListID string) error
}
