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

// stubMemberReader controls CheckSubscriber responses by mailingListID+email.
type stubMemberReader struct {
	// subscribed[mailingListID][email] = true means CheckSubscriber returns (true, nil).
	subscribed map[string]map[string]bool
	err        error
}

func (r *stubMemberReader) ListMembers(_ context.Context, _ string) ([]*model.GrpsIOMember, int, error) {
	return nil, 0, nil
}

func (r *stubMemberReader) GetMember(_ context.Context, _, _ string) (*model.GrpsIOMember, error) {
	return nil, nil
}

func (r *stubMemberReader) CheckSubscriber(_ context.Context, mailingListID, email string) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	return r.subscribed[mailingListID][email], nil
}

func newTestRealtimeAutoSyncOrchestrator(
	mlReader *stubMLReader,
	memberReader *stubMemberReader,
	diffComputer *stubSyncDiffComputer,
	applier *stubSyncApplier,
) port.RealtimeAutoSyncer {
	return NewRealtimeAutoSyncOrchestrator(
		WithRealtimeAutoSyncMailingListReader(mlReader),
		WithRealtimeAutoSyncMemberReader(memberReader),
		WithRealtimeAutoSyncDiffComputer(diffComputer),
		WithRealtimeAutoSyncApplier(applier),
	)
}

func TestTriggerAutoSync_NoLinkedCommitteeOrManual_NoOp(t *testing.T) {
	mlReader := &stubMLReader{ml: mlWithCommittee("")}
	diffComputer := &stubSyncDiffComputer{err: errs.NewValidation("mailing list is not linked to a committee")}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, &stubMemberReader{}, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.False(t, applier.called, "apply must not be attempted when there is nothing to auto-sync")
}

func TestTriggerAutoSync_ComputeDiffUnexpectedError_Propagates(t *testing.T) {
	mlReader := &stubMLReader{ml: mlWithCommittee("committee-1")}
	diffComputer := &stubSyncDiffComputer{err: errors.New("itx unavailable")}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, &stubMemberReader{}, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.Error(t, err)
	assert.False(t, applier.called)
}

func TestTriggerAutoSync_EmptyDiff_NoOp(t *testing.T) {
	mlReader := &stubMLReader{ml: mlWithCommittee("committee-1")}
	diffComputer := &stubSyncDiffComputer{diff: &model.SyncDiff{MailingListID: "ml-1"}}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, &stubMemberReader{}, diffComputer, applier)
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

		o := newTestRealtimeAutoSyncOrchestrator(mlReader, &stubMemberReader{}, diffComputer, applier)
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

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, &stubMemberReader{}, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.False(t, applier.called, "a public list with nothing but a gated remove has nothing left to apply")
}

func TestTriggerAutoSync_PrivateList_MailingListMemberRemove_PropagatesWithoutGuardCheck(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	ml.Public = false
	mlReader := &stubMLReader{ml: ml}
	diff := &model.SyncDiff{
		Removes: []model.SyncDiffRemove{{Email: "gone@example.com", MemberType: model.MemberTypeMailingListMember}},
	}
	diffComputer := &stubSyncDiffComputer{diff: diff}
	applier := &stubSyncApplier{}
	// No other lists configured; CheckSubscriber would fail the test if the guard incorrectly
	// consulted it for a c_to_ml-direction (mailing-list-member) removal.
	memberReader := &stubMemberReader{err: errors.New("should not be called")}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, memberReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	require.True(t, applier.called)
	assert.Equal(t, diff.Removes, applier.gotRemoves)
}

func TestTriggerAutoSync_PrivateList_CommitteeMemberRemove_SkippedIfStillSubscribedElsewhere(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	ml.UID = "ml-1"
	ml.Public = false
	otherML := mlWithCommittee("committee-1")
	otherML.UID = "ml-2"
	mlReader := &stubMLReader{ml: ml, listMLs: []*model.GroupsIOMailingList{ml, otherML}}
	diff := &model.SyncDiff{
		Removes: []model.SyncDiffRemove{{Email: "still-subscribed@example.com", MemberType: model.MemberTypeCommitteeMember}},
	}
	diffComputer := &stubSyncDiffComputer{diff: diff}
	applier := &stubSyncApplier{}
	memberReader := &stubMemberReader{
		subscribed: map[string]map[string]bool{
			"ml-2": {"still-subscribed@example.com": true},
		},
	}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, memberReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.False(t, applier.called, "removal must be skipped while the person is still subscribed via another linked mailing list")
}

func TestTriggerAutoSync_PrivateList_CommitteeMemberRemove_PropagatesIfNotSubscribedElsewhere(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	ml.UID = "ml-1"
	ml.Public = false
	otherML := mlWithCommittee("committee-1")
	otherML.UID = "ml-2"
	mlReader := &stubMLReader{ml: ml, listMLs: []*model.GroupsIOMailingList{ml, otherML}}
	diff := &model.SyncDiff{
		Removes: []model.SyncDiffRemove{{Email: "fully-gone@example.com", MemberType: model.MemberTypeCommitteeMember}},
	}
	diffComputer := &stubSyncDiffComputer{diff: diff}
	applier := &stubSyncApplier{}
	memberReader := &stubMemberReader{subscribed: map[string]map[string]bool{}}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, memberReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	require.True(t, applier.called)
	assert.Equal(t, diff.Removes, applier.gotRemoves)
}

func TestTriggerAutoSync_CrossListGuardError_Propagates(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	ml.UID = "ml-1"
	ml.Public = false
	otherML := mlWithCommittee("committee-1")
	otherML.UID = "ml-2"
	mlReader := &stubMLReader{ml: ml, listMLs: []*model.GroupsIOMailingList{ml, otherML}}
	diff := &model.SyncDiff{
		Removes: []model.SyncDiffRemove{{Email: "x@example.com", MemberType: model.MemberTypeCommitteeMember}},
	}
	diffComputer := &stubSyncDiffComputer{diff: diff}
	applier := &stubSyncApplier{}
	memberReader := &stubMemberReader{err: errors.New("member service unavailable")}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, memberReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.Error(t, err)
	assert.False(t, applier.called)
}

func TestTriggerAutoSync_GetMailingListError_Propagates(t *testing.T) {
	mlReader := &stubMLReader{err: errors.New("not found")}
	diffComputer := &stubSyncDiffComputer{diff: &model.SyncDiff{
		Adds: []model.SyncDiffAdd{{Email: "a@example.com"}},
	}}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, &stubMemberReader{}, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.Error(t, err)
	assert.False(t, applier.called)
}

func TestTriggerAutoSync_ApplySyncDiffError_Propagates(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	mlReader := &stubMLReader{ml: ml}
	diffComputer := &stubSyncDiffComputer{diff: &model.SyncDiff{
		Adds: []model.SyncDiffAdd{{Email: "a@example.com"}},
	}}
	applier := &stubSyncApplier{err: errors.New("itx job failed")}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, &stubMemberReader{}, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.Error(t, err)
}
