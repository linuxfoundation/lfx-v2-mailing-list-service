// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/redaction"
)

// RealtimeAutoSyncOrchestrator reacts to a single committee-member event by re-running the same
// reconciliation a manual "Sync Now" would (ComputeSyncDiff + ApplySyncDiff), rather than
// hand-resolving the one changed member: ITX already knows how to compute a correct,
// member-ID-resolved diff for a mailing list, and SyncDiffOrchestrator/SyncApplyOrchestrator
// (grpsio_sync_diff.go/grpsio_sync_apply.go) already implement that against ITX's job contract,
// including the stale-committee-link conflict check. Re-running it keeps automatic and manual sync
// identical by construction instead of maintaining a second, narrower code path that resolves a
// single add/remove by hand.
//
// Before approving the diff's removes, a visibility gate from issue #4023 is applied that a
// manual Sync Now does not apply automatically (a human approves removes there): removals only
// propagate automatically on a private mailing list. Adds are never gated: adding someone back
// into sync is never destructive.
type RealtimeAutoSyncOrchestrator struct {
	mailingListReader port.GroupsIOMailingListReader
	syncDiffComputer  port.SyncDiffComputer
	syncApplier       port.SyncApplier
	logger            *slog.Logger
}

// RealtimeAutoSyncOption configures a RealtimeAutoSyncOrchestrator.
type RealtimeAutoSyncOption func(*RealtimeAutoSyncOrchestrator)

// WithRealtimeAutoSyncMailingListReader sets the reader used to resolve a mailing list's linked
// committee and visibility (Public) for the gates described above.
func WithRealtimeAutoSyncMailingListReader(r port.GroupsIOMailingListReader) RealtimeAutoSyncOption {
	return func(o *RealtimeAutoSyncOrchestrator) { o.mailingListReader = r }
}

// WithRealtimeAutoSyncDiffComputer sets the computer used to recompute the mailing list's
// sync-diff in response to the triggering event.
func WithRealtimeAutoSyncDiffComputer(c port.SyncDiffComputer) RealtimeAutoSyncOption {
	return func(o *RealtimeAutoSyncOrchestrator) { o.syncDiffComputer = c }
}

// WithRealtimeAutoSyncApplier sets the applier used to apply the gated diff.
func WithRealtimeAutoSyncApplier(a port.SyncApplier) RealtimeAutoSyncOption {
	return func(o *RealtimeAutoSyncOrchestrator) { o.syncApplier = a }
}

// WithRealtimeAutoSyncLogger sets the logger used to surface per-entry add/remove failures that
// ApplySyncDiff reports inside an otherwise-successful job (see TriggerAutoSync). Defaults to
// slog.Default() if not set.
func WithRealtimeAutoSyncLogger(logger *slog.Logger) RealtimeAutoSyncOption {
	return func(o *RealtimeAutoSyncOrchestrator) { o.logger = logger }
}

// NewRealtimeAutoSyncOrchestrator creates a RealtimeAutoSyncOrchestrator with the given options.
func NewRealtimeAutoSyncOrchestrator(opts ...RealtimeAutoSyncOption) port.RealtimeAutoSyncer {
	o := &RealtimeAutoSyncOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	if o.logger == nil {
		o.logger = slog.Default()
	}
	return o
}

// TriggerAutoSync reacts to a membership-change event that may be relevant to mailingListID's
// auto sync by recomputing its sync-diff and applying the portion of it that passes the
// visibility gate. It is a deliberate no-op (nil error) when:
//   - the mailing list has no linked committee, or its sync_mode is "manual" (ComputeSyncDiff
//     reports this as a Validation error, which the caller — an event handler — should not treat
//     as a processing failure)
//   - the resulting gated diff has nothing to apply
//
// Any other error (ComputeSyncDiff/ApplySyncDiff failing for an unexpected reason, e.g. ITX being
// unavailable) is returned so the caller can log/alert on it; callers in this package treat auto
// sync as best-effort and do not retry a failed event.
func (o *RealtimeAutoSyncOrchestrator) TriggerAutoSync(ctx context.Context, mailingListID string) error {
	diff, err := o.syncDiffComputer.ComputeSyncDiff(ctx, mailingListID)
	if err != nil {
		var validationErr errs.Validation
		if errors.As(err, &validationErr) {
			// Not linked to a committee, or sync_mode is manual: nothing to auto-sync.
			return nil
		}
		return err
	}

	ml, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return err
	}

	// Public mailing lists never auto-propagate removals (issue #4023 visibility rule); adds
	// always propagate regardless of visibility.
	approvedRemoves := diff.Removes
	if ml.Public {
		approvedRemoves = nil
	}

	if len(diff.Adds) == 0 && len(approvedRemoves) == 0 {
		return nil
	}

	result, err := o.syncApplier.ApplySyncDiff(ctx, mailingListID, diff.Adds, approvedRemoves)
	if err != nil {
		return err
	}
	o.logFailedOutcomes(mailingListID, result)
	return nil
}

// logFailedOutcomes surfaces per-entry add/remove failures from an otherwise-successful
// ApplySyncDiff call. A manual Sync Now returns the full SyncApplyResult to the caller, who sees
// per-entry outcomes directly; TriggerAutoSync has no such caller, so a per-entry
// SyncApplyOutcomeFailed would otherwise be silently dropped even though the overall call
// reported no error.
func (o *RealtimeAutoSyncOrchestrator) logFailedOutcomes(mailingListID string, result *model.SyncApplyResult) {
	if result == nil {
		return
	}
	for _, add := range result.Added {
		if add.Outcome != model.SyncApplyOutcomeFailed {
			continue
		}
		o.logger.Warn("realtime auto-sync: add failed",
			"mailing_list_id", mailingListID,
			"email", redaction.RedactEmail(add.Email),
			"error", add.Error,
		)
	}
	for _, remove := range result.Removed {
		if remove.Outcome != model.SyncApplyOutcomeFailed {
			continue
		}
		o.logger.Warn("realtime auto-sync: remove failed",
			"mailing_list_id", mailingListID,
			"email", redaction.RedactEmail(remove.Email),
			"error", remove.Error,
		)
	}
}
