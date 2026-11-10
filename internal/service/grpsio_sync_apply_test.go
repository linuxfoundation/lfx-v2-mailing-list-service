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

// bindSyncApplyJob records the binding StartSyncApply would have written for a job started
// against mailingListID and approved against committeeUID, and returns the job_id
// GetSyncApplyStatus now expects — the raw ITX job ID, since the binding is authenticated
// server-side instead of decoded from the ID itself.
func bindSyncApplyJob(t *testing.T, store port.SyncApplyJobBindingStore, mailingListID, committeeUID, itxJobID string) string {
	t.Helper()
	err := store.Put(context.Background(), itxJobID, port.SyncApplyJobBinding{
		MailingListID: mailingListID,
		CommitteeUID:  committeeUID,
	})
	require.NoError(t, err)
	return itxJobID
}

func newApplyOrchestrator(mlReader port.GroupsIOMailingListReader, itxClient port.ITXSyncClient) port.SyncApplier {
	return NewSyncApplyOrchestrator(
		WithSyncApplyMailingListReader(mlReader),
		WithSyncApplyITXSyncClient(itxClient),
		WithSyncApplyAccessChecker(mock.NewMockAccessChecker()),
		WithSyncApplyJobBindingStore(mock.NewFakeSyncApplyJobBindingStore()),
	)
}

// newApplyOrchestratorWithStore is like newApplyOrchestrator but also returns the job binding
// store, so a test can pre-populate the binding GetSyncApplyStatus will look up.
func newApplyOrchestratorWithStore(mlReader port.GroupsIOMailingListReader, itxClient port.ITXSyncClient) (port.SyncApplier, port.SyncApplyJobBindingStore) {
	store := mock.NewFakeSyncApplyJobBindingStore()
	o := NewSyncApplyOrchestrator(
		WithSyncApplyMailingListReader(mlReader),
		WithSyncApplyITXSyncClient(itxClient),
		WithSyncApplyAccessChecker(mock.NewMockAccessChecker()),
		WithSyncApplyJobBindingStore(store),
	)
	return o, store
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
	assert.Equal(t, []string{"carol@example.com"}, stub.postApplyRemoves)
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

// TestApplySyncDiff_TrustedCallerDropsRemovesWhenListIsPublic pins that a trusted-system-caller
// (realtime auto-sync) apply re-checks the mailing list's visibility against the configuration it
// is about to submit against, and drops previously-approved removes if the list is public by then
// — closing the window where RealtimeAutoSyncOrchestrator approved the removes against a stale
// private snapshot but the list was made public before ApplySyncDiff actually ran (per
// @copilot-pull-request-reviewer).
func TestApplySyncDiff_TrustedCallerDropsRemovesWhenListIsPublic(t *testing.T) {
	ml := mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)
	ml.AudienceAccess = model.AudienceAccessPublic
	stub := &stubITXSyncClient{postApplyJobID: "apply-1", applyJob: &port.SyncApplyJob{ID: "apply-1", Status: port.SyncJobStatusCompleted}}
	o := newApplyOrchestrator(&stubMLReader{ml: ml}, stub)

	result, err := o.ApplySyncDiff(withTrustedSystemCaller(context.Background()), "ml-1", "committee-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Removed)
	assert.Empty(t, stub.postApplyRemoves, "the public list's removes must not be submitted to ITX")
}

// TestApplySyncDiff_TrustedCallerKeepsRemovesWhenListIsPrivate pins that the trusted-caller
// visibility recheck does not drop removes when the list is (still) private.
func TestApplySyncDiff_TrustedCallerKeepsRemovesWhenListIsPrivate(t *testing.T) {
	ml := mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)
	ml.AudienceAccess = model.AudienceAccessInviteOnly
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
	o := newApplyOrchestrator(&stubMLReader{ml: ml}, stub)

	result, err := o.ApplySyncDiff(withTrustedSystemCaller(context.Background()), "ml-1", "committee-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MemberID: "cm-1", MemberType: model.MemberTypeMailingListMember},
	})
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.Equal(t, []string{"carol@example.com"}, stub.postApplyRemoves)
}

// TestApplySyncDiff_NonTrustedCallerKeepsRemovesEvenWhenPublic pins that an ordinary (HTTP, manual
// Sync Now) caller's approved removes are never filtered by visibility — a human already reviewed
// and approved them via the diff, regardless of the list's audience_access.
func TestApplySyncDiff_NonTrustedCallerKeepsRemovesEvenWhenPublic(t *testing.T) {
	ml := mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)
	ml.AudienceAccess = model.AudienceAccessPublic
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
	o := newApplyOrchestrator(&stubMLReader{ml: ml}, stub)

	result, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MemberID: "cm-1", MemberType: model.MemberTypeMailingListMember},
	})
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.Equal(t, []string{"carol@example.com"}, stub.postApplyRemoves)
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
// approved entries reports the results ITX did return, plus a SyncApplyOutcomeFailed for each
// approved entry ITX omitted, instead of discarding everything behind a single request-level error
// (per @andrest50 — the earlier behavior turned a job that partly succeeded into an opaque 503 and
// invited a caller retry that re-submitted entries that had already taken effect).
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
	result, err := o.ApplySyncDiff(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
		{Email: "bob@example.com", MemberID: "cm-2", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.NoError(t, err)
	require.Len(t, result.Added, 2)

	byEmail := make(map[string]model.SyncApplyAddResult, len(result.Added))
	for _, a := range result.Added {
		byEmail[a.Email] = a
	}
	assert.Equal(t, model.SyncApplyOutcomeAdded, byEmail["alice@example.com"].Outcome)
	assert.Equal(t, model.SyncApplyOutcomeFailed, byEmail["bob@example.com"].Outcome)
	assert.Equal(t, "cm-2", byEmail["bob@example.com"].MemberID)
	assert.NotEmpty(t, byEmail["bob@example.com"].Error)
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

func TestStartSyncApply_NoLinkedCommittee(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: &model.GroupsIOMailingList{UID: "ml-1"}},
		&stubITXSyncClient{},
	)
	_, err := o.StartSyncApply(ctxWithPrincipal(), "ml-1", "committee-1", nil, nil)
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestStartSyncApply_RelinkedBeforeStart(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-2", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{},
	)
	_, err := o.StartSyncApply(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.Error(t, err)
	var conflict errs.Conflict
	assert.True(t, errors.As(err, &conflict))
}

func TestStartSyncApply_AccessDenied(t *testing.T) {
	o := NewSyncApplyOrchestrator(
		WithSyncApplyMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncApplyITXSyncClient(&stubITXSyncClient{}),
		WithSyncApplyAccessChecker(&stubAccessChecker{allow: false}),
	)
	_, err := o.StartSyncApply(ctxWithPrincipal(), "ml-1", "committee-1", nil, nil)
	require.Error(t, err)
	var unauthorized errs.Unauthorized
	assert.True(t, errors.As(err, &unauthorized))
}

// TestStartSyncApply_ReturnsJobID pins the happy path: it returns ITX's job ID directly without
// polling for completion, unlike ApplySyncDiff, and submits the approved emails to ITX.
func TestStartSyncApply_ReturnsJobID(t *testing.T) {
	stub := &stubITXSyncClient{postApplyJobID: "apply-1"}
	o, store := newApplyOrchestratorWithStore(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		stub,
	)
	jobID, err := o.StartSyncApply(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "cm-1", MemberType: model.MemberTypeCommitteeMember},
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, "apply-1", jobID, "job_id returned to the caller must be the raw ITX job ID")

	binding, found, err := store.Get(context.Background(), jobID)
	require.NoError(t, err)
	require.True(t, found, "StartSyncApply must record the job's binding server-side")
	assert.Equal(t, "ml-1", binding.MailingListID)
	assert.Equal(t, "committee-1", binding.CommitteeUID)

	assert.Equal(t, []string{"alice@example.com"}, stub.postApplyAdds)
	assert.Equal(t, port.SyncJobDirectionCommitteeToMailingList, stub.postApplyDirection)
}

func TestStartSyncApply_MemberTypeMismatch(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{},
	)
	_, err := o.StartSyncApply(ctxWithPrincipal(), "ml-1", "committee-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", MemberID: "mlm-1", MemberType: model.MemberTypeMailingListMember},
	}, nil)
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestStartSyncApply_PostSyncApplyError(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{postApplyErr: errs.NewServiceUnavailable("itx down")},
	)
	_, err := o.StartSyncApply(ctxWithPrincipal(), "ml-1", "committee-1", nil, nil)
	require.Error(t, err)
	var unavailable errs.ServiceUnavailable
	assert.True(t, errors.As(err, &unavailable))
}

func TestGetSyncApplyStatus_NoLongerLinked(t *testing.T) {
	o, store := newApplyOrchestratorWithStore(
		&stubMLReader{ml: &model.GroupsIOMailingList{UID: "ml-1"}},
		&stubITXSyncClient{},
	)
	jobID := bindSyncApplyJob(t, store, "ml-1", "committee-1", "apply-1")
	_, err := o.GetSyncApplyStatus(ctxWithPrincipal(), "ml-1", jobID)
	require.Error(t, err)
	var conflict errs.Conflict
	assert.True(t, errors.As(err, &conflict))
}

func TestGetSyncApplyStatus_AccessDenied(t *testing.T) {
	store := mock.NewFakeSyncApplyJobBindingStore()
	o := NewSyncApplyOrchestrator(
		WithSyncApplyMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncApplyITXSyncClient(&stubITXSyncClient{}),
		WithSyncApplyAccessChecker(&stubAccessChecker{allow: false}),
		WithSyncApplyJobBindingStore(store),
	)
	jobID := bindSyncApplyJob(t, store, "ml-1", "committee-1", "apply-1")
	_, err := o.GetSyncApplyStatus(ctxWithPrincipal(), "ml-1", jobID)
	require.Error(t, err)
	var unauthorized errs.Unauthorized
	assert.True(t, errors.As(err, &unauthorized))
}

// TestGetSyncApplyStatus_Ongoing pins that an in-progress job returns progress counts without
// Results.
func TestGetSyncApplyStatus_Ongoing(t *testing.T) {
	o, store := newApplyOrchestratorWithStore(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			applyJob: &port.SyncApplyJob{ID: "apply-1", Status: port.SyncJobStatusOngoing, ProcessedCount: 1, TotalCount: 2},
		},
	)
	jobID := bindSyncApplyJob(t, store, "ml-1", "committee-1", "apply-1")
	status, err := o.GetSyncApplyStatus(ctxWithPrincipal(), "ml-1", jobID)
	require.NoError(t, err)
	assert.Equal(t, port.SyncJobStatusOngoing, status.Status)
	assert.Equal(t, 1, status.ProcessedCount)
	assert.Nil(t, status.Results)
}

// TestGetSyncApplyStatus_Completed pins that a completed job's flat results and pre-aggregated
// counts are copied through without per-entry MemberID/MemberType/NameHint enrichment.
func TestGetSyncApplyStatus_Completed(t *testing.T) {
	o, store := newApplyOrchestratorWithStore(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			applyJob: &port.SyncApplyJob{
				ID:           "apply-1",
				Status:       port.SyncJobStatusCompleted,
				AddedCount:   1,
				RemovedCount: 0,
				ErrorCount:   0,
				Results: []port.SyncApplyJobResult{
					{Email: "alice@example.com", Action: "add", Status: "success"},
				},
			},
		},
	)
	jobID := bindSyncApplyJob(t, store, "ml-1", "committee-1", "apply-1")
	status, err := o.GetSyncApplyStatus(ctxWithPrincipal(), "ml-1", jobID)
	require.NoError(t, err)
	assert.Equal(t, port.SyncJobStatusCompleted, status.Status)
	assert.Equal(t, 1, status.AddedCount)
	require.Len(t, status.Results, 1)
	assert.Equal(t, "alice@example.com", status.Results[0].Email)
	assert.Equal(t, "add", status.Results[0].Action)
	assert.Equal(t, "success", status.Results[0].Status)
}

// TestGetSyncApplyStatus_Failed pins that a failed job also surfaces its (possibly partial)
// results rather than only succeeding for completed jobs.
func TestGetSyncApplyStatus_Failed(t *testing.T) {
	o, store := newApplyOrchestratorWithStore(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{
			applyJob: &port.SyncApplyJob{
				ID:           "apply-1",
				Status:       port.SyncJobStatusFailed,
				ErrorMessage: "subgroup locked",
			},
		},
	)
	jobID := bindSyncApplyJob(t, store, "ml-1", "committee-1", "apply-1")
	status, err := o.GetSyncApplyStatus(ctxWithPrincipal(), "ml-1", jobID)
	require.NoError(t, err)
	assert.Equal(t, port.SyncJobStatusFailed, status.Status)
	assert.Equal(t, "subgroup locked", status.ErrorMessage)
}

// TestGetSyncApplyStatus_FailedWithNoResultsSkipsCommitteeVerification pins that a failed job
// reporting neither a CommitteeID nor any Results (i.e. it never got far enough to process
// anything committee-scoped) still returns its terminal Failed status successfully, even with a
// translator configured — verifyJobCommitteeMatch would otherwise reject it as ServiceUnavailable
// for the empty CommitteeID, turning a legitimate failure into an opaque polling error (per
// @copilot-pull-request-reviewer).
func TestGetSyncApplyStatus_FailedWithNoResultsSkipsCommitteeVerification(t *testing.T) {
	store := mock.NewFakeSyncApplyJobBindingStore()
	o := NewSyncApplyOrchestrator(
		WithSyncApplyMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncApplyITXSyncClient(&stubITXSyncClient{
			applyJob: &port.SyncApplyJob{
				ID:           "apply-1",
				Status:       port.SyncJobStatusFailed,
				ErrorMessage: "subgroup locked",
			},
		}),
		WithSyncApplyAccessChecker(mock.NewMockAccessChecker()),
		WithSyncApplyTranslator(&stubTranslator{v2ByV1: map[string]string{"v1-committee-1": "committee-1"}}),
		WithSyncApplyJobBindingStore(store),
	)
	jobID := bindSyncApplyJob(t, store, "ml-1", "committee-1", "apply-1")
	status, err := o.GetSyncApplyStatus(ctxWithPrincipal(), "ml-1", jobID)
	require.NoError(t, err)
	assert.Equal(t, port.SyncJobStatusFailed, status.Status)
	assert.Equal(t, "subgroup locked", status.ErrorMessage)
	assert.Empty(t, status.Results)
}

// TestGetSyncApplyStatus_RejectsPathTraversalJobID pins that jobID is rejected as a path-traversal
// payload before it is ever used as a binding-store lookup key or ITX URL path segment.
func TestGetSyncApplyStatus_RejectsPathTraversalJobID(t *testing.T) {
	store := mock.NewFakeSyncApplyJobBindingStore()
	o := NewSyncApplyOrchestrator(
		WithSyncApplyMailingListReader(&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncApplyITXSyncClient(&stubITXSyncClient{applyJob: &port.SyncApplyJob{ID: "apply-1", Status: port.SyncJobStatusCompleted}}),
		WithSyncApplyAccessChecker(mock.NewMockAccessChecker()),
		WithSyncApplyTranslator(&stubTranslator{v2ByV1: map[string]string{"v1-committee-1": "committee-1"}}),
		WithSyncApplyJobBindingStore(store),
	)
	for _, itxJobID := range []string{
		"../../other-list/sync-apply/known-job",
		"..",
		"foo/bar",
		"foo%2f..%2fbar",
		"",
	} {
		// Bind the malicious ID anyway, so a test failure here would prove the path-traversal
		// guard itself rejects the payload, not merely that the binding happened to be missing.
		if itxJobID != "" {
			require.NoError(t, store.Put(context.Background(), itxJobID, port.SyncApplyJobBinding{MailingListID: "ml-1", CommitteeUID: "committee-1"}))
		}
		_, err := o.GetSyncApplyStatus(ctxWithPrincipal(), "ml-1", itxJobID)
		require.Error(t, err, "itxJobID=%q", itxJobID)
		var validation errs.Validation
		assert.True(t, errors.As(err, &validation), "itxJobID=%q", itxJobID)
	}
}

// TestGetSyncApplyStatus_UnknownOrExpiredJobID pins that a jobID which passes the path-traversal
// guard but has no recorded binding (unknown to this service, or its KV entry expired) is rejected
// as invalid rather than falling through to query ITX with no authenticated committee to check.
func TestGetSyncApplyStatus_UnknownOrExpiredJobID(t *testing.T) {
	o, _ := newApplyOrchestratorWithStore(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{applyJob: &port.SyncApplyJob{ID: "apply-1", Status: port.SyncJobStatusCompleted}},
	)
	_, err := o.GetSyncApplyStatus(ctxWithPrincipal(), "ml-1", "apply-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

// TestGetSyncApplyStatus_JobIDBoundToDifferentMailingList pins that a jobID bound to a different
// mailing list than the one in the request is rejected, even though it is a syntactically safe
// path segment and a real binding exists for it.
func TestGetSyncApplyStatus_JobIDBoundToDifferentMailingList(t *testing.T) {
	o, store := newApplyOrchestratorWithStore(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{applyJob: &port.SyncApplyJob{ID: "apply-1", Status: port.SyncJobStatusCompleted}},
	)
	jobID := bindSyncApplyJob(t, store, "ml-other", "committee-1", "apply-1")
	_, err := o.GetSyncApplyStatus(ctxWithPrincipal(), "ml-1", jobID)
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestGetSyncApplyStatus_GetSyncApplyJobError(t *testing.T) {
	o, store := newApplyOrchestratorWithStore(
		&stubMLReader{ml: mlWithCommitteeSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)},
		&stubITXSyncClient{applyJobErr: errors.New("itx unreachable")},
	)
	jobID := bindSyncApplyJob(t, store, "ml-1", "committee-1", "apply-1")
	_, err := o.GetSyncApplyStatus(ctxWithPrincipal(), "ml-1", jobID)
	require.Error(t, err)
}
