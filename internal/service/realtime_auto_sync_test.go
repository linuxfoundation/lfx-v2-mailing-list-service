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

	called               bool
	gotMailingListID     string
	gotExpectedCommittee string
	gotAdds              []model.SyncDiffAdd
	gotRemoves           []model.SyncDiffRemove
}

func (a *stubSyncApplier) ApplySyncDiff(
	_ context.Context,
	mailingListID string,
	expectedCommitteeUID string,
	approvedAdds []model.SyncDiffAdd,
	approvedRemoves []model.SyncDiffRemove,
) (*model.SyncApplyResult, error) {
	a.called = true
	a.gotMailingListID = mailingListID
	a.gotExpectedCommittee = expectedCommitteeUID
	a.gotAdds = approvedAdds
	a.gotRemoves = approvedRemoves
	return a.result, a.err
}

// stubMemberReader controls CheckSubscriber responses, keyed by mailingListID, for the cross-list
// removal guard. subscribed maps mailingListID -> set of emails considered subscribed there.
type stubMemberReader struct {
	port.GroupsIOMailingListMemberReader
	subscribed map[string]map[string]bool
	err        error

	calls []struct{ mailingListID, email string }
}

func (r *stubMemberReader) CheckSubscriber(_ context.Context, mailingListID string, email string) (bool, error) {
	r.calls = append(r.calls, struct{ mailingListID, email string }{mailingListID, email})
	if r.err != nil {
		return false, r.err
	}
	return r.subscribed[mailingListID][email], nil
}

func newTestRealtimeAutoSyncOrchestrator(
	mlReader *stubMLReader,
	diffComputer *stubSyncDiffComputer,
	applier *stubSyncApplier,
) port.RealtimeAutoSyncer {
	return NewRealtimeAutoSyncOrchestrator(
		WithRealtimeAutoSyncMailingListReader(mlReader),
		WithRealtimeAutoSyncMemberReader(&stubMemberReader{}),
		WithRealtimeAutoSyncDiffComputer(diffComputer),
		WithRealtimeAutoSyncApplier(applier),
	)
}

// mlWithAudienceAccess returns a mailing list linked to committeeUID with the given
// audience_access value, for visibility-gating tests.
func mlWithAudienceAccess(committeeUID, audienceAccess string) *model.GroupsIOMailingList {
	ml := mlWithCommittee(committeeUID)
	ml.AudienceAccess = audienceAccess
	return ml
}

func autoCToMLDiff(adds []model.SyncDiffAdd, removes []model.SyncDiffRemove) *model.SyncDiff {
	return &model.SyncDiff{
		MailingListID: "ml-1",
		CommitteeUID:  "committee-1",
		SyncMode:      model.SyncModeAutoCommitteeToMailingList,
		Adds:          adds,
		Removes:       removes,
	}
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
	diffComputer := &stubSyncDiffComputer{diff: autoCToMLDiff(nil, nil)}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.False(t, applier.called)
}

func TestTriggerAutoSync_SyncModeFlippedSinceSnapshot_NoOp(t *testing.T) {
	mlReader := &stubMLReader{ml: mlWithCommittee("committee-1")}
	diff := &model.SyncDiff{
		MailingListID: "ml-1",
		CommitteeUID:  "committee-1",
		SyncMode:      model.SyncModeAutoMailingListToCommittee,
		Adds:          []model.SyncDiffAdd{{Email: "a@example.com"}},
	}
	diffComputer := &stubSyncDiffComputer{diff: diff}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err, "sync_mode flipped away from auto_c_to_ml since the snapshot that triggered this event; nothing for this path to apply")
	assert.False(t, applier.called)
}

func TestTriggerAutoSync_AddsAlwaysPropagate_RegardlessOfVisibility(t *testing.T) {
	for _, audienceAccess := range []string{model.AudienceAccessPublic, model.AudienceAccessInviteOnly, ""} {
		ml := mlWithAudienceAccess("committee-1", audienceAccess)
		mlReader := &stubMLReader{ml: ml}
		diff := autoCToMLDiff(
			[]model.SyncDiffAdd{{Email: "a@example.com", MemberType: model.MemberTypeMailingListMember}},
			nil,
		)
		diffComputer := &stubSyncDiffComputer{diff: diff}
		applier := &stubSyncApplier{}

		o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
		err := o.TriggerAutoSync(context.Background(), "ml-1")

		require.NoError(t, err)
		require.True(t, applier.called)
		assert.Equal(t, diff.Adds, applier.gotAdds)
		assert.Empty(t, applier.gotRemoves)
		assert.Equal(t, "committee-1", applier.gotExpectedCommittee)
	}
}

func TestTriggerAutoSync_PublicOrUnknownVisibility_RemovesNeverPropagate(t *testing.T) {
	for _, audienceAccess := range []string{model.AudienceAccessPublic, "", "something_unrecognized"} {
		ml := mlWithAudienceAccess("committee-1", audienceAccess)
		mlReader := &stubMLReader{ml: ml}
		diff := autoCToMLDiff(nil,
			[]model.SyncDiffRemove{{Email: "gone@example.com", MemberType: model.MemberTypeCommitteeMember}},
		)
		diffComputer := &stubSyncDiffComputer{diff: diff}
		applier := &stubSyncApplier{}

		o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
		err := o.TriggerAutoSync(context.Background(), "ml-1")

		require.NoError(t, err)
		assert.False(t, applier.called, "audience_access %q is not a known-private value; a gated remove leaves nothing to apply", audienceAccess)
	}
}

func TestTriggerAutoSync_KnownPrivateList_RemovesPropagate_RegardlessOfMemberType(t *testing.T) {
	for _, audienceAccess := range []string{model.AudienceAccessApprovalRequired, model.AudienceAccessInviteOnly} {
		for _, memberType := range []model.MemberType{model.MemberTypeMailingListMember, model.MemberTypeCommitteeMember} {
			ml := mlWithAudienceAccess("committee-1", audienceAccess)
			mlReader := &stubMLReader{ml: ml}
			diff := autoCToMLDiff(nil,
				[]model.SyncDiffRemove{{Email: "gone@example.com", MemberType: memberType}},
			)
			diffComputer := &stubSyncDiffComputer{diff: diff}
			applier := &stubSyncApplier{}

			o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
			err := o.TriggerAutoSync(context.Background(), "ml-1")

			require.NoError(t, err)
			require.True(t, applier.called)
			assert.Equal(t, diff.Removes, applier.gotRemoves)
		}
	}
}

func TestTriggerAutoSync_CrossListGuard_SkipsRemoveStillSubscribedToSiblingList(t *testing.T) {
	ml := mlWithAudienceAccess("committee-1", model.AudienceAccessInviteOnly)
	mlReader := &stubMLReader{
		ml: ml,
		listMLs: []*model.GroupsIOMailingList{
			ml,
			{UID: "ml-2", Committees: []model.Committee{{UID: "committee-1"}}},
		},
	}
	diff := autoCToMLDiff(nil,
		[]model.SyncDiffRemove{{Email: "still-on-sibling@example.com", MemberType: model.MemberTypeMailingListMember}},
	)
	diffComputer := &stubSyncDiffComputer{diff: diff}
	applier := &stubSyncApplier{}
	memberReader := &stubMemberReader{
		subscribed: map[string]map[string]bool{
			"ml-2": {"still-on-sibling@example.com": true},
		},
	}

	o := NewRealtimeAutoSyncOrchestrator(
		WithRealtimeAutoSyncMailingListReader(mlReader),
		WithRealtimeAutoSyncMemberReader(memberReader),
		WithRealtimeAutoSyncDiffComputer(diffComputer),
		WithRealtimeAutoSyncApplier(applier),
	)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.False(t, applier.called, "the removed email is still subscribed to a sibling list linked to the same committee; removal must be skipped")
}

func TestTriggerAutoSync_CrossListGuard_AllowsRemoveNotOnAnySiblingList(t *testing.T) {
	ml := mlWithAudienceAccess("committee-1", model.AudienceAccessInviteOnly)
	mlReader := &stubMLReader{
		ml: ml,
		listMLs: []*model.GroupsIOMailingList{
			ml,
			{UID: "ml-2", Committees: []model.Committee{{UID: "committee-1"}}},
		},
	}
	diff := autoCToMLDiff(nil,
		[]model.SyncDiffRemove{{Email: "gone-everywhere@example.com", MemberType: model.MemberTypeMailingListMember}},
	)
	diffComputer := &stubSyncDiffComputer{diff: diff}
	applier := &stubSyncApplier{}
	memberReader := &stubMemberReader{
		subscribed: map[string]map[string]bool{
			"ml-2": {"still-on-sibling@example.com": true},
		},
	}

	o := NewRealtimeAutoSyncOrchestrator(
		WithRealtimeAutoSyncMailingListReader(mlReader),
		WithRealtimeAutoSyncMemberReader(memberReader),
		WithRealtimeAutoSyncDiffComputer(diffComputer),
		WithRealtimeAutoSyncApplier(applier),
	)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	require.True(t, applier.called)
	assert.Equal(t, diff.Removes, applier.gotRemoves)
}

func TestTriggerAutoSync_GetMailingListError_Propagates(t *testing.T) {
	mlReader := &stubMLReader{err: errors.New("not found")}
	diffComputer := &stubSyncDiffComputer{diff: autoCToMLDiff(
		[]model.SyncDiffAdd{{Email: "a@example.com"}}, nil,
	)}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.Error(t, err)
	assert.False(t, applier.called)
}

func TestTriggerAutoSync_PerEntryFailure_LoggedNotDropped(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	mlReader := &stubMLReader{ml: ml}
	diffComputer := &stubSyncDiffComputer{diff: autoCToMLDiff(
		[]model.SyncDiffAdd{{Email: "bounced@example.com", MemberType: model.MemberTypeMailingListMember}}, nil,
	)}
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
		WithRealtimeAutoSyncMemberReader(&stubMemberReader{}),
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
	diffComputer := &stubSyncDiffComputer{diff: autoCToMLDiff(
		[]model.SyncDiffAdd{{Email: "a@example.com"}}, nil,
	)}
	applier := &stubSyncApplier{err: errors.New("itx job failed")}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.Error(t, err)
}
