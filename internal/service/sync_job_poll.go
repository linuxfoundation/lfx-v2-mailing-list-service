// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// checkCommitteeWriterAccess verifies that the caller in ctx holds writer access on
// committee:committeeUID. Both sync-diff (reads committee roster emails/names into its response)
// and sync-apply (for auto_ml_to_c, mutates committee membership) only ever get here after the
// gateway has already confirmed the caller is a writer on the mailing list — but the ITX call
// that actually reads/mutates the committee uses service-level M2M credentials, not the caller's
// own, so a caller with mailing-list writer access but no committee access would otherwise be
// able to read committee roster PII or mutate committee membership through this path alone.
//
// ctx carrying constants.TrustedSystemCallerContextID (set only by withTrustedSystemCaller, from
// code reacting to an already-trusted committee-scoped system event — see realtime auto-sync)
// bypasses the principal/relation check entirely: that caller has no end-user principal to check,
// and is itself the authority for the committeeUID it passes in.
func checkCommitteeWriterAccess(ctx context.Context, accessChecker port.AccessChecker, committeeUID string) error {
	if isTrustedSystemCaller(ctx) {
		return nil
	}

	principal, _ := ctx.Value(constants.PrincipalContextID).(string)
	if principal == "" {
		return errs.NewUnauthorized("missing caller identity")
	}

	object := fmt.Sprintf("%s:%s", constants.ObjectTypeCommittee, committeeUID)
	user := fmt.Sprintf("%s:%s", constants.ObjectTypeUser, principal)
	isWriter, err := accessChecker.HasRelation(ctx, user, constants.RelationWriter, object)
	if err != nil {
		slog.ErrorContext(ctx, "access check failed for committee sync access", "error", err, "object", object)
		return errs.NewServiceUnavailable("unable to verify committee writer access", err)
	}
	if !isWriter {
		return errs.NewUnauthorized("caller does not have writer access to the mailing list's linked committee")
	}
	return nil
}

// withTrustedSystemCaller marks ctx as originating from an internal background process — not an
// end-user HTTP request — for checkCommitteeWriterAccess. Only call this immediately before
// invoking ComputeSyncDiff/ApplySyncDiff from code that is itself reacting to a trusted,
// committee-scoped system event (e.g. RealtimeAutoSyncOrchestrator.TriggerAutoSync, driven by
// committee-api's own committee_member.* NATS events): the committeeUID such code passes down
// always comes from that event's own payload, never from end-user input, so there is no separate
// caller identity left to check.
func withTrustedSystemCaller(ctx context.Context) context.Context {
	return context.WithValue(ctx, constants.TrustedSystemCallerContextID, true)
}

// isTrustedSystemCaller reports whether ctx was marked by withTrustedSystemCaller.
func isTrustedSystemCaller(ctx context.Context) bool {
	trusted, _ := ctx.Value(constants.TrustedSystemCallerContextID).(bool)
	return trusted
}

// normalizeEmail returns email in the canonical form used to match an ITX sync job result back
// to the approvedAdds/approvedRemoves entry it came from: trimmed and lowercased.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Polling cadence for ITX's async sync-diff/sync-apply jobs. PCC's own ITX client (which exercises
// this same job API) polls indefinitely at a 2s interval rather than enforcing a hard deadline, to
// tolerate large committees/mailing lists; since ComputeSyncDiff/ApplySyncDiff are synchronous
// calls here, a bound is still needed to avoid blocking a request forever, so it is set generously
// rather than removed.
const (
	syncJobPollInterval = 500 * time.Millisecond
	SyncJobPollTimeout  = 5 * time.Minute
)

// pollSyncDiffJob polls GetSyncDiffJob until it reaches a terminal status (completed or failed),
// or SyncJobPollTimeout elapses. The deadline is bound into ctx itself (not just checked between
// calls) so a GetSyncDiffJob call that hangs — rather than returning quickly — is also cancelled
// at the deadline instead of blocking indefinitely.
func pollSyncDiffJob(ctx context.Context, client port.ITXSyncClient, mailingListID, jobID string) (*port.SyncDiffJob, error) {
	ctx, cancel := context.WithTimeout(ctx, SyncJobPollTimeout)
	defer cancel()
	for {
		job, err := client.GetSyncDiffJob(ctx, mailingListID, jobID)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, errs.NewServiceUnavailable("timed out waiting for ITX sync-diff job to complete")
			}
			return nil, err
		}
		switch job.Status {
		case port.SyncJobStatusCompleted, port.SyncJobStatusFailed:
			return job, nil
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, errs.NewServiceUnavailable("timed out waiting for ITX sync-diff job to complete")
			}
			return nil, ctx.Err()
		case <-time.After(syncJobPollInterval):
		}
	}
}

// pollSyncApplyJob polls GetSyncApplyJob until it reaches a terminal status (completed or
// failed), or SyncJobPollTimeout elapses. See pollSyncDiffJob for why the deadline is bound into
// ctx rather than only checked between calls.
func pollSyncApplyJob(ctx context.Context, client port.ITXSyncClient, mailingListID, jobID string) (*port.SyncApplyJob, error) {
	ctx, cancel := context.WithTimeout(ctx, SyncJobPollTimeout)
	defer cancel()
	for {
		job, err := client.GetSyncApplyJob(ctx, mailingListID, jobID)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, errs.NewServiceUnavailable("timed out waiting for ITX sync-apply job to complete")
			}
			return nil, err
		}
		switch job.Status {
		case port.SyncJobStatusCompleted, port.SyncJobStatusFailed:
			return job, nil
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, errs.NewServiceUnavailable("timed out waiting for ITX sync-apply job to complete")
			}
			return nil, ctx.Err()
		case <-time.After(syncJobPollInterval):
		}
	}
}

// verifyJobCommitteeMatch confirms that jobCommitteeID (v1, as reported by ITX on a completed
// sync-diff/sync-apply job) is the same committee as committeeUID (v2, the one access was checked
// against before the job was started). jobCommitteeID is "" when ITX's response omitted it, or
// translator is nil (not wired) — in either case this is a no-op, relying solely on the
// mailing-list-relink recheck the caller also performs.
//
// This catches a relink that ITX's job itself observed mid-run (e.g. A -> B -> A, which a
// post-job GetMailingList call alone cannot distinguish from "never changed", since both show A)
// — without it, a job that ITX actually ran against committee B's roster could be attributed to
// committee A merely because the mailing list happens to be linked back to A by the time the
// result is read.
func verifyJobCommitteeMatch(ctx context.Context, translator port.Translator, jobCommitteeID, committeeUID string) error {
	if jobCommitteeID == "" || translator == nil {
		return nil
	}
	v2JobCommitteeUID, err := translator.MapID(ctx, constants.TranslationSubjectCommittee, constants.TranslationDirectionV1ToV2, jobCommitteeID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to translate job-reported committee ID", "error", err, "v1_committee_id", jobCommitteeID)
		return errs.NewServiceUnavailable("unable to verify the committee the sync job actually ran against", err)
	}
	if v2JobCommitteeUID != committeeUID {
		return errs.NewConflict("mailing list's linked committee changed while the sync job was running; request a fresh diff and re-apply")
	}
	return nil
}

// memberTypesForDirection returns which roster (committee_member or mailing_list_member) an add
// and a remove entry belong to for the given sync direction: for c_to_ml, adds come from the
// committee roster and removes come from the mailing-list roster; for ml_to_c it is the reverse.
func memberTypesForDirection(direction string) (addType, removeType model.MemberType) {
	if direction == port.SyncJobDirectionMailingListToCommittee {
		return model.MemberTypeMailingListMember, model.MemberTypeCommitteeMember
	}
	return model.MemberTypeCommitteeMember, model.MemberTypeMailingListMember
}

// syncModeManualValidationMsg and syncModeUnrecognizedValidationMsgPrefix identify, by exact
// message (or prefix), the two Validation errors syncJobDirectionForMode itself produces for a
// mailing list with nothing to reconcile. RealtimeAutoSyncOrchestrator.TriggerAutoSync matches
// against these (and notLinkedToCommitteeValidationMsg below) to tell "nothing to reconcile" apart
// from a Validation that ITX's PostSyncDiff/PostSyncApply returns for an unrelated reason — most
// notably a sync job already in progress for this mailing list (see port.ITXSyncClient.PostSyncDiff)
// — which must not be swallowed as a successful no-op (per @greptile-apps/@copilot-pull-request-reviewer).
const (
	notLinkedToCommitteeValidationMsg       = "mailing list is not linked to a committee"
	syncModeManualValidationMsg             = "mailing list sync_mode is manual; there is nothing to reconcile"
	syncModeUnrecognizedValidationMsgPrefix = "mailing list has an unrecognized sync_mode: "
)

// syncJobDirectionForMode maps a committee's configured sync_mode to the direction ITX's
// sync-diff/sync-apply jobs expect. Only auto_c_to_ml and auto_ml_to_c have a direction; manual
// (or unset) means there is nothing to reconcile.
func syncJobDirectionForMode(syncMode string) (string, error) {
	switch syncMode {
	case model.SyncModeAutoCommitteeToMailingList:
		return port.SyncJobDirectionCommitteeToMailingList, nil
	case model.SyncModeAutoMailingListToCommittee:
		return port.SyncJobDirectionMailingListToCommittee, nil
	case "", model.SyncModeManual:
		return "", errs.NewValidation(syncModeManualValidationMsg)
	default:
		return "", errs.NewValidation(syncModeUnrecognizedValidationMsgPrefix + syncMode)
	}
}
