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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

func newApplyOrchestrator(mlReader port.GroupsIOMailingListReader, itxClient port.ITXSyncClient) port.SyncApplier {
	return NewSyncApplyOrchestrator(
		WithSyncApplyMailingListReader(mlReader),
		WithSyncApplyITXSyncClient(itxClient),
		WithSyncApplyAccessChecker(mock.NewMockAccessChecker()),
	)
}

func TestApplySyncDiff_NoLinkedCommittee(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: &model.GroupsIOMailingList{UID: "ml-1"}},
		&stubITXSyncClient{},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", nil, nil)
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestApplySyncDiff_ManualSyncMode(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeManual)},
		&stubITXSyncClient{},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", nil, nil)
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestApplySyncDiff_PostSyncApplyError(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{postApplyErr: errs.NewServiceUnavailable("itx down")},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", nil, nil)
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
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", nil, nil)
	require.Error(t, err)
	var unavailable errs.ServiceUnavailable
	assert.True(t, errors.As(err, &unavailable))
	assert.Contains(t, err.Error(), "subgroup locked")
}

func TestApplySyncDiff_AddSuccess(t *testing.T) {
	stub := &stubITXSyncClient{
		postApplyJobID: "apply-1",
		applyJob: &port.SyncApplyJob{
			ID:     "apply-1",
			Status: port.SyncJobStatusCompleted,
			Results: []port.SyncApplyJobResult{
				{Email: "alice@example.com", Action: "add", Status: "success"},
			},
		},
	}
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		stub,
	)
	result, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember, Name: "Alice Example"},
	}, nil)
	require.NoError(t, err)
	require.Len(t, result.Added, 1)
	assert.Equal(t, model.SyncApplyOutcomeAdded, result.Added[0].Outcome)
	assert.Equal(t, "cm-1", result.Added[0].MemberID)
	assert.Equal(t, model.MemberTypeCommitteeMember, result.Added[0].MemberType)
	assert.Equal(t, "Alice Example", result.Added[0].NameHint)
	assert.Empty(t, result.Added[0].Error)
	assert.Equal(t, []string{"alice@example.com"}, stub.postApplyAdds)
	assert.Empty(t, stub.postApplyRemoves)
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
	result, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.NoError(t, err)
	require.Len(t, result.Added, 1)
	assert.Equal(t, model.SyncApplyOutcomeFailed, result.Added[0].Outcome)
	assert.Equal(t, "boom", result.Added[0].Error)
}

func TestApplySyncDiff_RemoveSuccess(t *testing.T) {
	stub := &stubITXSyncClient{
		postApplyJobID: "apply-1",
		applyJob: &port.SyncApplyJob{
			ID:     "apply-1",
			Status: port.SyncJobStatusCompleted,
			Results: []port.SyncApplyJobResult{
				{Email: "carol@example.com", Action: "remove", Status: "success"},
			},
		},
	}
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		stub,
	)
	result, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MemberID: "mlm-1", MemberType: model.MemberTypeMailingListMember},
	})
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.Equal(t, model.SyncApplyOutcomeRemoved, result.Removed[0].Outcome)
	assert.Equal(t, "mlm-1", result.Removed[0].MemberID)
	assert.Equal(t, model.MemberTypeMailingListMember, result.Removed[0].MemberType)
	assert.Empty(t, result.Removed[0].Error)
	assert.Equal(t, []string{"mlm-1"}, stub.postApplyRemoves)
	assert.Empty(t, stub.postApplyAdds)
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

	result, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
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
	result, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MemberID: "mlm-1", MemberType: model.MemberTypeMailingListMember},
	})
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.Equal(t, model.SyncApplyOutcomeFailed, result.Removed[0].Outcome)
	assert.Equal(t, "boom", result.Removed[0].Error)
}

// TestApplySyncDiff_AddMemberTypeMismatch pins that an approved add whose MemberType doesn't
// match the roster the mailing list's *current* sync_mode direction expects is rejected up front,
// rather than silently applied as if it were for the direction it was actually approved under.
func TestApplySyncDiff_AddMemberTypeMismatch(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "mlm-1", MemberType: model.MemberTypeMailingListMember},
	}, nil)
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestApplySyncDiff_RemoveMemberTypeMismatch(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoMailingListToCommittee)},
		&stubITXSyncClient{},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MemberID: "mlm-1", MemberType: model.MemberTypeMailingListMember},
	})
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

// TestApplySyncDiff_UnmatchedResultEmail pins that an ITX result whose email doesn't normalize-
// match any approved add/remove fails the whole request, since there's no valid MemberType to
// report for it and the response schema requires one of the enum values on every entry.
func TestApplySyncDiff_UnmatchedResultEmail(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			postApplyJobID: "apply-1",
			applyJob: &port.SyncApplyJob{
				ID:     "apply-1",
				Status: port.SyncJobStatusCompleted,
				Results: []port.SyncApplyJobResult{
					{Email: "unexpected@example.com", Action: "add", Status: "success"},
				},
			},
		},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected@example.com")
}

// TestApplySyncDiff_IncompleteResultSet pins that a job completing with fewer results than
// approved entries fails the whole request instead of silently reporting success for a subset
// (per @copilot-pull-request-reviewer).
func TestApplySyncDiff_IncompleteResultSet(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			postApplyJobID: "apply-1",
			applyJob: &port.SyncApplyJob{
				ID:     "apply-1",
				Status: port.SyncJobStatusCompleted,
				Results: []port.SyncApplyJobResult{
					{Email: "alice@example.com", Action: "add", Status: "success"},
					// bob@example.com was approved but ITX reported no result for it.
				},
			},
		},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
		{Email: "bob@example.com", MemberID: "cm-2", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "every approved add/remove")
}

// TestApplySyncDiff_DuplicateResultForSameApprovedEntry pins that ITX reporting two results for
// the same approved entry fails the whole request rather than silently keeping only one.
func TestApplySyncDiff_DuplicateResultForSameApprovedEntry(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			postApplyJobID: "apply-1",
			applyJob: &port.SyncApplyJob{
				ID:     "apply-1",
				Status: port.SyncJobStatusCompleted,
				Results: []port.SyncApplyJobResult{
					{Email: "alice@example.com", Action: "add", Status: "success"},
					{Email: "alice@example.com", Action: "add", Status: "error"},
				},
			},
		},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than one result")
}

// TestApplySyncDiff_UnrecognizedResultAction pins that an unrecognized action in ITX's result
// (neither "add" nor "remove") fails the request instead of being silently ignored.
func TestApplySyncDiff_UnrecognizedResultAction(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			postApplyJobID: "apply-1",
			applyJob: &port.SyncApplyJob{
				ID:     "apply-1",
				Status: port.SyncJobStatusCompleted,
				Results: []port.SyncApplyJobResult{
					{Email: "alice@example.com", Action: "update", Status: "success"},
				},
			},
		},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized action")
}

// TestApplySyncDiff_CommitteeChangedWhileJobRunning pins that if the mailing list's linked
// committee changes between the sync-apply job starting and completing, the request fails
// instead of attributing the (stale) job's results to the newly-linked committee.
func TestApplySyncDiff_CommitteeChangedWhileJobRunning(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{
			ml:             mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList),
			mlOnSecondCall: mlWithCommitteeSyncMode("committee-2", model.SyncModeAutoCommitteeToMailingList),
		},
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
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.Error(t, err)
	var conflict errs.Conflict
	assert.True(t, errors.As(err, &conflict))
}

// TestApplySyncDiff_RelinkedBeforeApply pins that if the mailing list was relinked to a
// different committee between the diff being fetched and this apply call (i.e. before
// PostSyncApply is even called), the request fails instead of attributing the stale diff's
// approved adds/removes to the newly-linked committee.
func TestApplySyncDiff_RelinkedBeforeApply(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-2", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.Error(t, err)
	var conflict errs.Conflict
	assert.True(t, errors.As(err, &conflict))
}

// TestApplySyncDiff_DuplicateAddEmail pins that two approved adds normalizing to the same email
// are rejected up front, rather than silently misattributing ITX's single per-email result to
// whichever of the two happened to win the map-key overwrite.
func TestApplySyncDiff_DuplicateAddEmail(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "Alice@Example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
		{Email: "alice@example.com", MemberID: "cm-2", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

// TestApplySyncDiff_DuplicateRemoveEmail is the remove-side counterpart of
// TestApplySyncDiff_DuplicateAddEmail.
func TestApplySyncDiff_DuplicateRemoveEmail(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{},
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", nil, []model.SyncDiffRemove{
		{Email: "Carol@Example.com", MemberID: "mlm-1", MemberType: model.MemberTypeMailingListMember},
		{Email: "carol@example.com", MemberID: "mlm-2", MemberType: model.MemberTypeMailingListMember},
	})
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

// TestApplySyncDiff_AccessDenied pins that a caller with mailing-list access but no writer
// access on the linked committee cannot apply changes on the committee's behalf.
func TestApplySyncDiff_AccessDenied(t *testing.T) {
	o := NewSyncApplyOrchestrator(
		WithSyncApplyMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncApplyITXSyncClient(&stubITXSyncClient{}),
		WithSyncApplyAccessChecker(&stubAccessChecker{allow: false}),
	)
	_, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", nil, nil)
	require.Error(t, err)
	var unauthorized errs.Unauthorized
	assert.True(t, errors.As(err, &unauthorized))
}

// TestApplySyncDiff_NoPrincipal pins that a caller with no principal in context is rejected
// before any access check or apply is attempted.
func TestApplySyncDiff_NoPrincipal(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{},
	)
	_, err := o.ApplySyncDiff(context.Background(), "ml-1", "committee-1", nil, nil)
	require.Error(t, err)
	var unauthorized errs.Unauthorized
	assert.True(t, errors.As(err, &unauthorized))
}
