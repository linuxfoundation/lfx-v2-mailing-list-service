// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// RealtimeAutoSyncOrchestrator reacts to a single committee-member or Groups.io subscriber event
// by re-running the same reconciliation a manual "Sync Now" would (ComputeSyncDiff + ApplySyncDiff),
// rather than hand-resolving the one changed member: ITX already knows how to compute a correct,
// member-ID-resolved diff for a mailing list, and SyncDiffOrchestrator/SyncApplyOrchestrator
// (grpsio_sync_diff.go/grpsio_sync_apply.go) already implement that against ITX's job contract,
// including the stale-committee-link conflict check. Re-running it keeps automatic and manual sync
// identical by construction instead of maintaining a second, narrower code path that resolves a
// single add/remove by hand.
//
// Before approving the diff's removes, two gates from issue #4023 are applied that a manual
// Sync Now does not apply automatically (a human approves removes there): a visibility gate
// (removals only propagate automatically on a private mailing list) and a cross-list guard
// (a committee removal is skipped if the person is still subscribed via another mailing list
// linked to the same committee). Adds are never gated: adding someone back into sync is never
// destructive.
type RealtimeAutoSyncOrchestrator struct {
	mailingListReader port.GroupsIOMailingListReader
	memberReader      port.GroupsIOMailingListMemberReader
	syncDiffComputer  port.SyncDiffComputer
	syncApplier       port.SyncApplier
}

// RealtimeAutoSyncOption configures a RealtimeAutoSyncOrchestrator.
type RealtimeAutoSyncOption func(*RealtimeAutoSyncOrchestrator)

// WithRealtimeAutoSyncMailingListReader sets the reader used to resolve a mailing list's linked
// committee and visibility (Public) for the gates described above.
func WithRealtimeAutoSyncMailingListReader(r port.GroupsIOMailingListReader) RealtimeAutoSyncOption {
	return func(o *RealtimeAutoSyncOrchestrator) { o.mailingListReader = r }
}

// WithRealtimeAutoSyncMemberReader sets the reader used by the cross-list removal guard to check
// whether a person is still subscribed to another mailing list linked to the same committee.
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

// NewRealtimeAutoSyncOrchestrator creates a RealtimeAutoSyncOrchestrator with the given options.
func NewRealtimeAutoSyncOrchestrator(opts ...RealtimeAutoSyncOption) port.RealtimeAutoSyncer {
	o := &RealtimeAutoSyncOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// TriggerAutoSync reacts to a membership-change event that may be relevant to mailingListID's
// auto sync by recomputing its sync-diff and applying the portion of it that passes the
// visibility and cross-list gates. It is a deliberate no-op (nil error) when:
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

	approvedRemoves := make([]model.SyncDiffRemove, 0, len(diff.Removes))
	if !ml.Public {
		for _, remove := range diff.Removes {
			allowed, err := o.crossListGuardAllows(ctx, ml, remove)
			if err != nil {
				return err
			}
			if allowed {
				approvedRemoves = append(approvedRemoves, remove)
			}
		}
	}
	// Public mailing lists never auto-propagate removals (issue #4023 visibility rule); adds
	// always propagate regardless of visibility.

	if len(diff.Adds) == 0 && len(approvedRemoves) == 0 {
		return nil
	}

	_, err = o.syncApplier.ApplySyncDiff(ctx, mailingListID, diff.Adds, approvedRemoves)
	return err
}

// crossListGuardAllows reports whether remove may be auto-applied, or must be skipped because the
// person it would remove is still subscribed via another mailing list linked to the same
// committee. The guard only applies to a committee-member removal (ml_to_c direction): a mailing
// list is linked to at most one committee for sync purposes (Committees[0]), so a mailing-list
// member removal (c_to_ml direction) has no "other committee" to check against.
func (o *RealtimeAutoSyncOrchestrator) crossListGuardAllows(
	ctx context.Context,
	ml *model.GroupsIOMailingList,
	remove model.SyncDiffRemove,
) (bool, error) {
	if remove.MemberType != model.MemberTypeCommitteeMember {
		return true, nil
	}
	if len(ml.Committees) == 0 || ml.Committees[0].UID == "" {
		return true, nil
	}
	committeeUID := ml.Committees[0].UID

	linkedLists, _, err := o.mailingListReader.ListMailingLists(ctx, "", committeeUID)
	if err != nil {
		return false, err
	}

	for _, other := range linkedLists {
		if other.UID == ml.UID {
			continue
		}
		subscribed, err := o.memberReader.CheckSubscriber(ctx, other.UID, remove.Email)
		if err != nil {
			return false, err
		}
		if subscribed {
			return false, nil
		}
	}
	return true, nil
}
