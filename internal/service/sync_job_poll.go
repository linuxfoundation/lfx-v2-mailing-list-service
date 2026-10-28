// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// normalizeEmail returns email in the canonical form used to match an ITX sync job result back
// to the approvedAdds/approvedRemoves entry it came from: trimmed and lowercased.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Polling cadence for ITX's async sync-diff/sync-apply jobs. ITX processes these jobs quickly
// (a single Groups.io subgroup's membership), so a short interval and bound keep the synchronous
// ComputeSyncDiff/ApplySyncDiff calls responsive without hammering ITX.
const (
	syncJobPollInterval = 500 * time.Millisecond
	syncJobPollTimeout  = 60 * time.Second
)

// pollSyncDiffJob polls GetSyncDiffJob until it reaches a terminal status (completed or failed),
// or syncJobPollTimeout elapses.
func pollSyncDiffJob(ctx context.Context, client port.ITXSyncClient, mailingListID, jobID string) (*port.SyncDiffJob, error) {
	deadline := time.Now().Add(syncJobPollTimeout)
	for {
		job, err := client.GetSyncDiffJob(ctx, mailingListID, jobID)
		if err != nil {
			return nil, err
		}
		switch job.Status {
		case port.SyncJobStatusCompleted, port.SyncJobStatusFailed:
			return job, nil
		}
		if time.Now().After(deadline) {
			return nil, errs.NewServiceUnavailable("timed out waiting for ITX sync-diff job to complete")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(syncJobPollInterval):
		}
	}
}

// pollSyncApplyJob polls GetSyncApplyJob until it reaches a terminal status (completed or
// failed), or syncJobPollTimeout elapses.
func pollSyncApplyJob(ctx context.Context, client port.ITXSyncClient, mailingListID, jobID string) (*port.SyncApplyJob, error) {
	deadline := time.Now().Add(syncJobPollTimeout)
	for {
		job, err := client.GetSyncApplyJob(ctx, mailingListID, jobID)
		if err != nil {
			return nil, err
		}
		switch job.Status {
		case port.SyncJobStatusCompleted, port.SyncJobStatusFailed:
			return job, nil
		}
		if time.Now().After(deadline) {
			return nil, errs.NewServiceUnavailable("timed out waiting for ITX sync-apply job to complete")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(syncJobPollInterval):
		}
	}
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
