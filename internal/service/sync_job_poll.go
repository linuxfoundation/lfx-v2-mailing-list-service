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
func checkCommitteeWriterAccess(ctx context.Context, accessChecker port.AccessChecker, committeeUID string) error {
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
	syncJobPollTimeout  = 5 * time.Minute
)

// pollSyncDiffJob polls GetSyncDiffJob until it reaches a terminal status (completed or failed),
// or syncJobPollTimeout elapses. The deadline is bound into ctx itself (not just checked between
// calls) so a GetSyncDiffJob call that hangs — rather than returning quickly — is also cancelled
// at the deadline instead of blocking indefinitely.
func pollSyncDiffJob(ctx context.Context, client port.ITXSyncClient, mailingListID, jobID string) (*port.SyncDiffJob, error) {
	ctx, cancel := context.WithTimeout(ctx, syncJobPollTimeout)
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
// failed), or syncJobPollTimeout elapses. See pollSyncDiffJob for why the deadline is bound into
// ctx rather than only checked between calls.
func pollSyncApplyJob(ctx context.Context, client port.ITXSyncClient, mailingListID, jobID string) (*port.SyncApplyJob, error) {
	ctx, cancel := context.WithTimeout(ctx, syncJobPollTimeout)
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

// memberTypesForDirection returns which roster (committee_member or mailing_list_member) an add
// and a remove entry belong to for the given sync direction: for c_to_ml, adds come from the
// committee roster and removes come from the mailing-list roster; for ml_to_c it is the reverse.
func memberTypesForDirection(direction string) (addType, removeType model.MemberType) {
	if direction == port.SyncJobDirectionMailingListToCommittee {
		return model.MemberTypeMailingListMember, model.MemberTypeCommitteeMember
	}
	return model.MemberTypeCommitteeMember, model.MemberTypeMailingListMember
}

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
		return "", errs.NewValidation("mailing list sync_mode is manual; there is nothing to reconcile")
	default:
		return "", errs.NewValidation("mailing list has an unrecognized sync_mode: " + syncMode)
	}
}
