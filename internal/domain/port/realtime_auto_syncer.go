// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// RealtimeAutoSyncer reacts to a committee-membership event by reconciling a mailing list's
// auto_c_to_ml sync in real time (issue #4023), gated by the mailing list's visibility and the
// cross-list removal guard. The reverse direction (auto_ml_to_c) is not this service's concern:
// ITX already propagates Groups.io subscribe/unsubscribe events straight to the LFX committee API.
// See service.RealtimeAutoSyncOrchestrator for the implementation.
type RealtimeAutoSyncer interface {
	TriggerAutoSync(ctx context.Context, mailingListID string) error
}
