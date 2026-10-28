// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// ---- port.ITXSyncClient implementation ----
//
// These proxy to ITX's own sync-diff/sync-apply async job endpoints
// (/v2/groupsio_subgroup/:subgroup_id/sync-diff[/:job_id] and .../sync-apply[/:apply_job_id]),
// which compute and apply committee<->mailing-list membership reconciliation natively.

// PostSyncDiff starts a sync-diff job for mailingListID.
func (c *itx) PostSyncDiff(ctx context.Context, mailingListID, direction string) (string, error) {
	bodyBytes, err := json.Marshal(&postSyncDiffRequestWire{Direction: direction})
	if err != nil {
		return "", errs.NewUnexpected("failed to marshal request", err)
	}

	u, err := c.buildURL("v2", "groupsio_subgroup", mailingListID, "sync-diff")
	if err != nil {
		return "", errs.NewUnexpected("failed to build URL", err)
	}
	resp, err := c.httpClient.Request(ctx, http.MethodPost, u, bytes.NewReader(bodyBytes), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return "", c.handleRequestError(err)
	}

	var wire postSyncJobResponseWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return "", errs.NewUnexpected("failed to parse response", err)
	}
	return wire.JobID, nil
}

// GetSyncDiffJob returns the current state of a sync-diff job.
func (c *itx) GetSyncDiffJob(ctx context.Context, mailingListID, jobID string) (*port.SyncDiffJob, error) {
	u, err := c.buildURL("v2", "groupsio_subgroup", mailingListID, "sync-diff", jobID)
	if err != nil {
		return nil, errs.NewUnexpected("failed to build URL", err)
	}
	resp, err := c.httpClient.Request(ctx, http.MethodGet, u, nil, nil)
	if err != nil {
		return nil, c.handleRequestError(err)
	}

	var wire syncDiffJobWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return fromWireSyncDiffJob(&wire), nil
}

// PostSyncApply starts a sync-apply job for mailingListID.
func (c *itx) PostSyncApply(ctx context.Context, mailingListID, direction string, approvedAdds, approvedRemoves []string, diffJobID string) (string, error) {
	bodyBytes, err := json.Marshal(&postSyncApplyRequestWire{
		Direction:       direction,
		ApprovedAdds:    approvedAdds,
		ApprovedRemoves: approvedRemoves,
		JobID:           diffJobID,
	})
	if err != nil {
		return "", errs.NewUnexpected("failed to marshal request", err)
	}

	u, err := c.buildURL("v2", "groupsio_subgroup", mailingListID, "sync-apply")
	if err != nil {
		return "", errs.NewUnexpected("failed to build URL", err)
	}
	resp, err := c.httpClient.Request(ctx, http.MethodPost, u, bytes.NewReader(bodyBytes), map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return "", c.handleRequestError(err)
	}

	var wire postSyncJobResponseWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return "", errs.NewUnexpected("failed to parse response", err)
	}
	return wire.JobID, nil
}

// GetSyncApplyJob returns the current state of a sync-apply job.
func (c *itx) GetSyncApplyJob(ctx context.Context, mailingListID, jobID string) (*port.SyncApplyJob, error) {
	u, err := c.buildURL("v2", "groupsio_subgroup", mailingListID, "sync-apply", jobID)
	if err != nil {
		return nil, errs.NewUnexpected("failed to build URL", err)
	}
	resp, err := c.httpClient.Request(ctx, http.MethodGet, u, nil, nil)
	if err != nil {
		return nil, c.handleRequestError(err)
	}

	var wire syncApplyJobWire
	if err := json.Unmarshal(resp.Body, &wire); err != nil {
		return nil, errs.NewUnexpected("failed to parse response", err)
	}
	return fromWireSyncApplyJob(&wire), nil
}

func fromWireSyncDiffJob(w *syncDiffJobWire) *port.SyncDiffJob {
	if w == nil {
		return nil
	}
	job := &port.SyncDiffJob{
		ID:           w.ID,
		Status:       w.Status,
		ToAdd:        make([]port.SyncJobMember, len(w.ToAdd)),
		ToRemove:     make([]port.SyncJobMember, len(w.ToRemove)),
		ErrorMessage: w.ErrorMessage,
	}
	for i, m := range w.ToAdd {
		job.ToAdd[i] = port.SyncJobMember{MemberID: m.MemberID, Email: m.Email, FullName: m.FullName}
	}
	for i, m := range w.ToRemove {
		job.ToRemove[i] = port.SyncJobMember{MemberID: m.MemberID, Email: m.Email, FullName: m.FullName}
	}
	job.EmailConflicts = make([]port.SyncJobConflict, len(w.EmailConflicts))
	for i, conf := range w.EmailConflicts {
		job.EmailConflicts[i] = port.SyncJobConflict{
			MLEmail:        conf.MLEmail,
			CommitteeEmail: conf.CommitteeEmail,
			FullName:       conf.FullName,
			MemberID:       conf.MemberID,
		}
	}
	return job
}

func fromWireSyncApplyJob(w *syncApplyJobWire) *port.SyncApplyJob {
	if w == nil {
		return nil
	}
	job := &port.SyncApplyJob{
		ID:           w.ID,
		Status:       w.Status,
		Results:      make([]port.SyncApplyJobResult, len(w.Results)),
		ErrorMessage: w.ErrorMessage,
	}
	for i, r := range w.Results {
		job.Results[i] = port.SyncApplyJobResult{Email: r.Email, Action: r.Action, Status: r.Status, Error: r.Error}
	}
	return job
}
