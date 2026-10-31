// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// stubSyncDiffComputer returns a configured diff/err, recording the mailingListID it was called
// with.
type stubSyncDiffComputer struct {
	diff  *model.SyncDiff
	err   error
	calls []string
}

func (c *stubSyncDiffComputer) ComputeSyncDiff(_ context.Context, mailingListID string) (*model.SyncDiff, error) {
	c.calls = append(c.calls, mailingListID)
	return c.diff, c.err
}

// stubSyncApplier records the adds/removes it was asked to apply and returns a configured
// result/err.
type stubSyncApplier struct {
	result *model.SyncApplyResult
	err    error

	called           bool
	gotMailingListID string
	gotAdds          []model.SyncDiffAdd
	gotRemoves       []model.SyncDiffRemove
}

func (a *stubSyncApplier) ApplySyncDiff(
	_ context.Context,
	mailingListID string,
	approvedAdds []model.SyncDiffAdd,
	approvedRemoves []model.SyncDiffRemove,
) (*model.SyncApplyResult, error) {
	a.called = true
	a.gotMailingListID = mailingListID
	a.gotAdds = approvedAdds
	a.gotRemoves = approvedRemoves
	return a.result, a.err
}

func newTestRealtimeAutoSyncOrchestrator(
	mlReader *stubMLReader,
	diffComputer *stubSyncDiffComputer,
	applier *stubSyncApplier,
) port.RealtimeAutoSyncer {
	return NewRealtimeAutoSyncOrchestrator(
		WithRealtimeAutoSyncMailingListReader(mlReader),
		WithRealtimeAutoSyncDiffComputer(diffComputer),
		WithRealtimeAutoSyncApplier(applier),
	)
}

func TestTriggerAutoSync_NoLinkedCommitteeOrManual_NoOp(t *testing.T) {
	mlReader := &stubMLReader{ml: mlWithCommittee("")}
	diffComputer := &stubSyncDiffComputer{err: errs.NewValidation("mailing list is not linked to a committee")}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.False(t, applier.called, "apply must not be attempted when there is nothing to auto-sync")
}

func TestTriggerAutoSync_ComputeDiffUnexpectedError_Propagates(t *testing.T) {
	mlReader := &stubMLReader{ml: mlWithCommittee("committee-1")}
	diffComputer := &stubSyncDiffComputer{err: errors.New("itx unavailable")}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.Error(t, err)
	assert.False(t, applier.called)
}

func TestTriggerAutoSync_EmptyDiff_NoOp(t *testing.T) {
	mlReader := &stubMLReader{ml: mlWithCommittee("committee-1")}
	diffComputer := &stubSyncDiffComputer{diff: &model.SyncDiff{MailingListID: "ml-1"}}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.False(t, applier.called)
}

func TestTriggerAutoSync_AddsAlwaysPropagate_RegardlessOfVisibility(t *testing.T) {
	for _, public := range []bool{true, false} {
		ml := mlWithCommittee("committee-1")
		ml.Public = public
		mlReader := &stubMLReader{ml: ml}
		diff := &model.SyncDiff{
			Adds: []model.SyncDiffAdd{{Email: "a@example.com", MemberType: model.MemberTypeMailingListMember}},
		}
		diffComputer := &stubSyncDiffComputer{diff: diff}
		applier := &stubSyncApplier{}

		o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
		err := o.TriggerAutoSync(context.Background(), "ml-1")

		require.NoError(t, err)
		require.True(t, applier.called)
		assert.Equal(t, diff.Adds, applier.gotAdds)
		assert.Empty(t, applier.gotRemoves)
	}
}

func TestTriggerAutoSync_PublicList_RemovesNeverPropagate(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	ml.Public = true
	mlReader := &stubMLReader{ml: ml}
	diff := &model.SyncDiff{
		Removes: []model.SyncDiffRemove{{Email: "gone@example.com", MemberType: model.MemberTypeCommitteeMember}},
	}
	diffComputer := &stubSyncDiffComputer{diff: diff}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.False(t, applier.called, "a public list with nothing but a gated remove has nothing left to apply")
}

func TestTriggerAutoSync_PrivateList_RemovesPropagate_RegardlessOfMemberType(t *testing.T) {
	for _, memberType := range []model.MemberType{model.MemberTypeMailingListMember, model.MemberTypeCommitteeMember} {
		ml := mlWithCommittee("committee-1")
		ml.Public = false
		mlReader := &stubMLReader{ml: ml}
		diff := &model.SyncDiff{
			Removes: []model.SyncDiffRemove{{Email: "gone@example.com", MemberType: memberType}},
		}
		diffComputer := &stubSyncDiffComputer{diff: diff}
		applier := &stubSyncApplier{}

		o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
		err := o.TriggerAutoSync(context.Background(), "ml-1")

		require.NoError(t, err)
		require.True(t, applier.called)
		assert.Equal(t, diff.Removes, applier.gotRemoves)
	}
}

func TestTriggerAutoSync_GetMailingListError_Propagates(t *testing.T) {
	mlReader := &stubMLReader{err: errors.New("not found")}
	diffComputer := &stubSyncDiffComputer{diff: &model.SyncDiff{
		Adds: []model.SyncDiffAdd{{Email: "a@example.com"}},
	}}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.Error(t, err)
	assert.False(t, applier.called)
}

func TestTriggerAutoSync_PerEntryFailure_LoggedNotDropped(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	mlReader := &stubMLReader{ml: ml}
	diffComputer := &stubSyncDiffComputer{diff: &model.SyncDiff{
		Adds: []model.SyncDiffAdd{{Email: "bounced@example.com", MemberType: model.MemberTypeMailingListMember}},
	}}
	applier := &stubSyncApplier{result: &model.SyncApplyResult{
		MailingListID: "ml-1",
		Added: []model.SyncApplyAddResult{
			{Email: "bounced@example.com", Outcome: model.SyncApplyOutcomeFailed, Error: "subscriber add rejected"},
		},
	}}

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	o := NewRealtimeAutoSyncOrchestrator(
		WithRealtimeAutoSyncMailingListReader(mlReader),
		WithRealtimeAutoSyncDiffComputer(diffComputer),
		WithRealtimeAutoSyncApplier(applier),
		WithRealtimeAutoSyncLogger(logger),
	)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err, "a per-entry failure inside an otherwise-successful job is not a call error")
	assert.Contains(t, logBuf.String(), "add failed")
	assert.Contains(t, logBuf.String(), "subscriber add rejected")
	assert.NotContains(t, logBuf.String(), "bounced@example.com", "the raw email must not be logged")
}

func TestTriggerAutoSync_ApplySyncDiffError_Propagates(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	mlReader := &stubMLReader{ml: ml}
	diffComputer := &stubSyncDiffComputer{diff: &model.SyncDiff{
		Adds: []model.SyncDiffAdd{{Email: "a@example.com"}},
	}}
	applier := &stubSyncApplier{err: errors.New("itx job failed")}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.Error(t, err)
}
