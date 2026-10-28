// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// pollCountingITXSyncClient returns SyncJobStatusPending for the first (calls-1) GetSyncDiffJob/
// GetSyncApplyJob calls, then the configured terminal job. Used to exercise the non-terminal
// branch of pollSyncDiffJob/pollSyncApplyJob, which the orchestrator-level tests never reach
// since their stub always returns a terminal status on the first call.
type pollCountingITXSyncClient struct {
	stubITXSyncClient
	pendingCalls int
	diffCalls    int
	applyCalls   int
}

func (c *pollCountingITXSyncClient) GetSyncDiffJob(ctx context.Context, mailingListID, jobID string) (*port.SyncDiffJob, error) {
	c.diffCalls++
	if c.diffCalls <= c.pendingCalls {
		return &port.SyncDiffJob{ID: jobID, Status: port.SyncJobStatusPending}, nil
	}
	return c.stubITXSyncClient.GetSyncDiffJob(ctx, mailingListID, jobID)
}

func (c *pollCountingITXSyncClient) GetSyncApplyJob(ctx context.Context, mailingListID, jobID string) (*port.SyncApplyJob, error) {
	c.applyCalls++
	if c.applyCalls <= c.pendingCalls {
		return &port.SyncApplyJob{ID: jobID, Status: port.SyncJobStatusOngoing}, nil
	}
	return c.stubITXSyncClient.GetSyncApplyJob(ctx, mailingListID, jobID)
}

func TestPollSyncDiffJob_ReturnsAfterNonTerminalStatuses(t *testing.T) {
	client := &pollCountingITXSyncClient{
		pendingCalls: 1,
		stubITXSyncClient: stubITXSyncClient{
			diffJob: &port.SyncDiffJob{ID: "diff-1", Status: port.SyncJobStatusCompleted},
		},
	}
	job, err := pollSyncDiffJob(context.Background(), client, "ml-1", "diff-1")
	require.NoError(t, err)
	assert.Equal(t, port.SyncJobStatusCompleted, job.Status)
	assert.Equal(t, 2, client.diffCalls)
}

func TestPollSyncApplyJob_ReturnsAfterNonTerminalStatuses(t *testing.T) {
	client := &pollCountingITXSyncClient{
		pendingCalls: 1,
		stubITXSyncClient: stubITXSyncClient{
			applyJob: &port.SyncApplyJob{ID: "apply-1", Status: port.SyncJobStatusCompleted},
		},
	}
	job, err := pollSyncApplyJob(context.Background(), client, "ml-1", "apply-1")
	require.NoError(t, err)
	assert.Equal(t, port.SyncJobStatusCompleted, job.Status)
	assert.Equal(t, 2, client.applyCalls)
}
