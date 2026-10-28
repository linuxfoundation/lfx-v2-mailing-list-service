// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

func newApplyOrchestrator(mlReader port.GroupsIOMailingListReader, itxClient port.ITXSyncClient) port.SyncApplier {
	return NewSyncApplyOrchestrator(
		WithSyncApplyMailingListReader(mlReader),
		WithSyncApplyITXSyncClient(itxClient),
	)
}

func TestApplySyncDiff_NoLinkedCommittee(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: &model.GroupsIOMailingList{UID: "ml-1"}},
		&stubITXSyncClient{},
	)
	_, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, nil)
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestApplySyncDiff_ManualSyncMode(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeManual)},
		&stubITXSyncClient{},
	)
	_, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, nil)
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestApplySyncDiff_PostSyncApplyError(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{postApplyErr: errs.NewServiceUnavailable("itx down")},
	)
	_, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, nil)
	require.Error(t, err)
	var unavailable errs.ServiceUnavailable
	assert.True(t, errors.As(err, &unavailable))
}

func TestApplySyncDiff_JobFailed(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			postApplyJobID: "apply-1",
			applyJob:       &port.SyncApplyJob{ID: "apply-1", Status: port.SyncJobStatusFailed, ErrorMessage: "subgroup locked"},
		},
	)
	_, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, nil)
	require.Error(t, err)
	var unavailable errs.ServiceUnavailable
	assert.True(t, errors.As(err, &unavailable))
	assert.Contains(t, err.Error(), "subgroup locked")
}

func TestApplySyncDiff_AddSuccess(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			postApplyJobID: "apply-1",
			applyJob: &port.SyncApplyJob{
				ID:     "apply-1",
				Status: port.SyncJobStatusCompleted,
				Results: []port.SyncApplyJobResult{
					{Email: "alice@example.com", Action: "add", Status: "success"},
				},
			},
		},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember, Name: "Alice Example"},
	}, nil)
	require.NoError(t, err)
	require.Len(t, result.Added, 1)
	assert.Equal(t, model.SyncApplyOutcomeAdded, result.Added[0].Outcome)
	assert.Equal(t, "cm-1", result.Added[0].MemberID)
	assert.Equal(t, model.MemberTypeCommitteeMember, result.Added[0].MemberType)
	assert.Equal(t, "Alice Example", result.Added[0].NameHint)
	assert.Empty(t, result.Added[0].Error)
}

func TestApplySyncDiff_AddFailure_ReportedAsFailed(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			postApplyJobID: "apply-1",
			applyJob: &port.SyncApplyJob{
				ID:     "apply-1",
				Status: port.SyncJobStatusCompleted,
				Results: []port.SyncApplyJobResult{
					{Email: "alice@example.com", Action: "add", Status: "error", Error: "boom"},
				},
			},
		},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.NoError(t, err)
	require.Len(t, result.Added, 1)
	assert.Equal(t, model.SyncApplyOutcomeFailed, result.Added[0].Outcome)
	assert.Equal(t, "boom", result.Added[0].Error)
}

func TestApplySyncDiff_RemoveSuccess(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			postApplyJobID: "apply-1",
			applyJob: &port.SyncApplyJob{
				ID:     "apply-1",
				Status: port.SyncJobStatusCompleted,
				Results: []port.SyncApplyJobResult{
					{Email: "carol@example.com", Action: "remove", Status: "success"},
				},
			},
		},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MemberID: "mlm-1", MemberType: model.MemberTypeMailingListMember},
	})
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.Equal(t, model.SyncApplyOutcomeRemoved, result.Removed[0].Outcome)
	assert.Equal(t, "mlm-1", result.Removed[0].MemberID)
	assert.Equal(t, model.MemberTypeMailingListMember, result.Removed[0].MemberType)
	assert.Empty(t, result.Removed[0].Error)
}

// TestApplySyncDiff_MailingListToCommitteeDirection pins that auto_ml_to_c mailing lists are
// fully supported (not blocked) for apply, and that the reverse direction is sent to ITX.
func TestApplySyncDiff_MailingListToCommitteeDirection(t *testing.T) {
	stub := &stubITXSyncClient{
		postApplyJobID: "apply-1",
		applyJob: &port.SyncApplyJob{
			ID:     "apply-1",
			Status: port.SyncJobStatusCompleted,
			Results: []port.SyncApplyJobResult{
				{Email: "dana@example.com", Action: "add", Status: "success"},
			},
		},
	}
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoMailingListToCommittee)},
		stub,
	)

	result, err := o.ApplySyncDiff(context.Background(), "ml-1", []model.SyncDiffAdd{
		{Email: "dana@example.com", MemberID: "mlm-1", MemberType: model.MemberTypeMailingListMember},
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, port.SyncJobDirectionMailingListToCommittee, stub.postApplyDirection)
	require.Len(t, result.Added, 1)
	assert.Equal(t, model.SyncApplyOutcomeAdded, result.Added[0].Outcome)
}

func TestApplySyncDiff_RemoveFailure_ReportedAsFailed(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			postApplyJobID: "apply-1",
			applyJob: &port.SyncApplyJob{
				ID:     "apply-1",
				Status: port.SyncJobStatusCompleted,
				Results: []port.SyncApplyJobResult{
					{Email: "carol@example.com", Action: "remove", Status: "error", Error: "boom"},
				},
			},
		},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MemberID: "mlm-1", MemberType: model.MemberTypeMailingListMember},
	})
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.Equal(t, model.SyncApplyOutcomeFailed, result.Removed[0].Outcome)
	assert.Equal(t, "boom", result.Removed[0].Error)
}
