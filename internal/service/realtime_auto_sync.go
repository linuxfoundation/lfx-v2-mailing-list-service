// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"

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
	memberReader      port.GroupsIOMailingListMemberReader
	syncDiffComputer  port.SyncDiffComputer
	syncApplier       port.SyncApplier
	logger            *slog.Logger
	autoSyncEnabled   bool
}

// RealtimeAutoSyncOption configures a RealtimeAutoSyncOrchestrator.
type RealtimeAutoSyncOption func(*RealtimeAutoSyncOrchestrator)

// WithRealtimeAutoSyncMailingListReader sets the reader used to resolve a mailing list's linked
// committee and visibility (Public) for the gates described above.
func WithRealtimeAutoSyncMailingListReader(r port.GroupsIOMailingListReader) RealtimeAutoSyncOption {
	return func(o *RealtimeAutoSyncOrchestrator) { o.mailingListReader = r }
}

// WithRealtimeAutoSyncMemberReader sets the reader used to check whether a removed email is still
// subscribed to a sibling mailing list linked to the same committee, before approving its removal.
func WithRealtimeAutoSyncMemberReader(r port.GroupsIOMailingListMemberReader) RealtimeAutoSyncOption {
	return func(o *RealtimeAutoSyncOrchestrator) { o.memberReader = r }
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

// WithRealtimeAutoSyncEnabled sets the issue #4878 kill switch: when false, TriggerAutoSync is a
// deliberate no-op regardless of caller, independent of the frontend LaunchDarkly flag that only
// controls whether the UI offers the auto-sync setting. Defaults to true (enabled) if not set.
func WithRealtimeAutoSyncEnabled(enabled bool) RealtimeAutoSyncOption {
	return func(o *RealtimeAutoSyncOrchestrator) { o.autoSyncEnabled = enabled }
}

// NewRealtimeAutoSyncOrchestrator creates a RealtimeAutoSyncOrchestrator with the given options.
func NewRealtimeAutoSyncOrchestrator(opts ...RealtimeAutoSyncOption) port.RealtimeAutoSyncer {
	o := &RealtimeAutoSyncOrchestrator{autoSyncEnabled: true}
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
	// Issue #4878 kill switch: disabling AUTO_SYNC_ENABLED must stop both the subscriber-driven
	// trigger (this method is CommitteeMemberSubscriber's only path to auto-sync) and the
	// auto-sync-driven ApplySyncDiff call below, independent of the frontend LD flag — a single
	// guard here covers both without duplicating the check in committee_member_subscriber.go. A
	// manual "Sync Now" HTTP call never goes through TriggerAutoSync, so it is unaffected.
	if !o.autoSyncEnabled {
		o.logger.Info("realtime auto-sync: skipped, AUTO_SYNC_ENABLED is disabled", "mailing_list_id", mailingListID)
		return nil
	}

	// ctx here comes from a committee-api NATS event, not an end-user HTTP request, so it carries
	// no caller principal — mark it as a trusted system caller so ComputeSyncDiff/ApplySyncDiff's
	// checkCommitteeWriterAccess doesn't reject it for "missing caller identity". mailingListID
	// (and the committeeUID resolved from it) come from the triggering event, not end-user input.
	ctx = withTrustedSystemCaller(ctx)

	diff, err := o.syncDiffComputer.ComputeSyncDiff(ctx, mailingListID)
	if err != nil {
		var validationErr errs.Validation
		if errors.As(err, &validationErr) && isNothingToReconcileValidation(validationErr) {
			// Not linked to a committee, or sync_mode is manual/unrecognized: nothing to
			// auto-sync. Any other Validation — notably ITX's PostSyncDiff reporting a sync job
			// already in progress for this mailing list — is a real, if transient, failure to
			// process this event and must not be swallowed as a successful no-op (per
			// @greptile-apps/@copilot-pull-request-reviewer): it falls through to the
			// "return err" below so the subscriber logs why the sync was missed.
			return nil
		}
		return err
	}

	// The mailing list's committee link stays the same but sync_mode can flip between the snapshot
	// that decided whether to call TriggerAutoSync at all and the diff actually computed here; only
	// auto_c_to_ml diffs are relevant to this (committee -> mailing list) path.
	if diff.SyncMode != model.SyncModeAutoCommitteeToMailingList {
		return nil
	}

	ml, err := o.mailingListReader.GetMailingList(ctx, mailingListID)
	if err != nil {
		return err
	}

	// Removals only auto-propagate on a known-private mailing list; public, empty, or unrecognized
	// audience_access values fail closed (no auto-remove) rather than fail open. Adds always
	// propagate regardless of visibility.
	approvedRemoves := diff.Removes
	switch ml.AudienceAccess {
	case model.AudienceAccessApprovalRequired, model.AudienceAccessInviteOnly:
		// private: removes may propagate, subject to the cross-list guard below.
		approvedRemoves = o.filterCrossListRemoves(ctx, ml, approvedRemoves)
	default:
		approvedRemoves = nil
	}

	if len(diff.Adds) == 0 && len(approvedRemoves) == 0 {
		return nil
	}

	result, err := o.syncApplier.ApplySyncDiff(ctx, mailingListID, diff.CommitteeUID, diff.Adds, approvedRemoves)
	if err != nil {
		return err
	}
	o.logFailedOutcomes(mailingListID, result)
	return nil
}

// isNothingToReconcileValidation reports whether validationErr is one of the three
// Validation errors ComputeSyncDiff/syncJobDirectionForMode itself produce for a mailing list
// with nothing to reconcile (not linked to a committee, sync_mode is manual, or sync_mode is
// unrecognized) — as opposed to a Validation error ITX's PostSyncDiff returns for an unrelated
// reason, most notably a sync job already in progress for this mailing list. The message is the
// only distinguishing signal the port.ITXSyncClient/errs.Validation contract exposes.
func isNothingToReconcileValidation(validationErr errs.Validation) bool {
	msg := validationErr.Error()
	return msg == notLinkedToCommitteeValidationMsg ||
		msg == syncModeManualValidationMsg ||
		strings.HasPrefix(msg, syncModeUnrecognizedValidationMsgPrefix)
}

// filterCrossListRemoves drops any removal whose email is still subscribed to another mailing
// list linked to the same committee as ml — removing it here would otherwise immediately conflict
// with that sibling list's own membership, which this mailing list's removal has no say over.
//
// A sibling whose own sync_mode is also auto_c_to_ml for the same committee, AND is itself private,
// is excluded from this check: that sibling is driven by the same committee roster ml is, so when
// the person leaves the committee the sibling independently removes them too — it is not an
// independent membership this removal would conflict with. Without this exclusion, two private
// auto_c_to_ml lists sharing a committee would each see the person "still subscribed" on the other
// and neither would ever remove them (reported by @lfx-one). A public auto_c_to_ml sibling is NOT
// excluded: TriggerAutoSync never auto-propagates removals on a public list (see the visibility
// switch above), so that sibling's subscription never gets cleaned up on its own — excluding it
// here would let a private list remove the member while they remain subscribed on the public
// sibling, silently bypassing the cross-list guard (reported by @copilot-pull-request-reviewer).
func (o *RealtimeAutoSyncOrchestrator) filterCrossListRemoves(
	ctx context.Context,
	ml *model.GroupsIOMailingList,
	removes []model.SyncDiffRemove,
) []model.SyncDiffRemove {
	if len(removes) == 0 || len(ml.Committees) == 0 {
		return removes
	}

	siblingLists, _, err := o.mailingListReader.ListMailingLists(ctx, "", ml.Committees[0].UID)
	if err != nil {
		o.logger.Warn("realtime auto-sync: failed to list sibling mailing lists for cross-list removal guard; skipping all removes",
			"mailing_list_id", ml.UID,
			"committee_uid", ml.Committees[0].UID,
			"error", err,
		)
		return nil
	}

	var siblingIDs []string
	for _, sibling := range siblingLists {
		if sibling.UID == ml.UID {
			continue
		}
		if len(sibling.Committees) > 0 && sibling.Committees[0].SyncMode == model.SyncModeAutoCommitteeToMailingList &&
			(sibling.AudienceAccess == model.AudienceAccessApprovalRequired || sibling.AudienceAccess == model.AudienceAccessInviteOnly) {
			continue
		}
		siblingIDs = append(siblingIDs, sibling.UID)
	}
	if len(siblingIDs) == 0 {
		return removes
	}

	approved := make([]model.SyncDiffRemove, 0, len(removes))
	for _, remove := range removes {
		stillLinked := false
		for _, siblingID := range siblingIDs {
			subscribed, err := o.memberReader.CheckSubscriber(ctx, siblingID, remove.Email)
			if err != nil {
				o.logger.Warn("realtime auto-sync: failed to check sibling list subscription; skipping remove",
					"mailing_list_id", ml.UID,
					"sibling_mailing_list_id", siblingID,
					"email", redaction.RedactEmail(remove.Email),
					"error", err,
				)
				stillLinked = true
				break
			}
			if subscribed {
				stillLinked = true
				break
			}
		}
		if !stillLinked {
			approved = append(approved, remove)
		}
	}
	return approved
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
