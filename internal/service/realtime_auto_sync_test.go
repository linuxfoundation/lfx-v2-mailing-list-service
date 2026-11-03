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
// with and whether ctx was marked by withTrustedSystemCaller.
type stubSyncDiffComputer struct {
	diff  *model.SyncDiff
	err   error
	calls []string

	sawTrusted bool
}

func (c *stubSyncDiffComputer) ComputeSyncDiff(ctx context.Context, mailingListID string) (*model.SyncDiff, error) {
	c.calls = append(c.calls, mailingListID)
	if isTrustedSystemCaller(ctx) {
		c.sawTrusted = true
	}
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
	sawTrusted           bool
}

func (a *stubSyncApplier) ApplySyncDiff(
	ctx context.Context,
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
	if isTrustedSystemCaller(ctx) {
		a.sawTrusted = true
	}
	return a.result, a.err
}

// stubMemberReader controls CheckSubscriber/ListMembers responses, keyed by mailingListID, for the
// cross-list removal guard. subscribed maps mailingListID -> set of emails considered subscribed
// there.
type stubMemberReader struct {
	port.GroupsIOMailingListMemberReader
	subscribed map[string]map[string]bool
	err        error

	calls            []struct{ mailingListID, email string }
	listMembersCalls []string
}

func (r *stubMemberReader) CheckSubscriber(_ context.Context, mailingListID string, email string) (bool, error) {
	r.calls = append(r.calls, struct{ mailingListID, email string }{mailingListID, email})
	if r.err != nil {
		return false, r.err
	}
	return r.subscribed[mailingListID][email], nil
}

func (r *stubMemberReader) ListMembers(_ context.Context, mailingListID string) ([]*model.GrpsIOMember, int, error) {
	r.listMembersCalls = append(r.listMembersCalls, mailingListID)
	if r.err != nil {
		return nil, 0, r.err
	}
	emails := r.subscribed[mailingListID]
	members := make([]*model.GrpsIOMember, 0, len(emails))
	for email := range emails {
		members = append(members, &model.GrpsIOMember{Email: email})
	}
	return members, len(members), nil
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
	diffComputer := &stubSyncDiffComputer{err: errs.NewValidation("mailing list is not linked to a committee", ErrNothingToReconcile)}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.False(t, applier.called, "apply must not be attempted when there is nothing to auto-sync")
}

// TestTriggerAutoSync_BusyJobValidation_Propagates pins that a Validation error ITX's
// PostSyncDiff returns for a sync job already in progress for this mailing list is not mistaken
// for the "nothing to reconcile" Validation this orchestrator's own checks produce — it must
// propagate as a real error so the subscriber logs that the sync was missed, not silently treated
// as a successful no-op (per @greptile-apps/@copilot-pull-request-reviewer).
func TestTriggerAutoSync_BusyJobValidation_Propagates(t *testing.T) {
	mlReader := &stubMLReader{ml: mlWithCommittee("committee-1")}
	diffComputer := &stubSyncDiffComputer{err: errs.NewValidation("a sync job is already in progress for this mailing list")}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.Error(t, err)
	assert.False(t, applier.called)
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

func TestTriggerAutoSync_CrossListGuard_AllowsRemoveWhenSiblingIsAlsoAutoCToML(t *testing.T) {
	ml := mlWithAudienceAccess("committee-1", model.AudienceAccessInviteOnly)
	mlReader := &stubMLReader{
		ml: ml,
		listMLs: []*model.GroupsIOMailingList{
			ml,
			// ml-2 is also private and auto_c_to_ml for the same committee: it independently
			// removes this person on its own TriggerAutoSync run, so it is not an independent
			// membership ml-1's removal would conflict with. Without excluding it, ml-1 and ml-2
			// would each see the person "still subscribed" on the other and neither would ever
			// remove them.
			{
				UID:            "ml-2",
				AudienceAccess: model.AudienceAccessInviteOnly,
				Committees: []model.Committee{
					{UID: "committee-1", SyncMode: model.SyncModeAutoCommitteeToMailingList},
				},
			},
		},
	}
	diff := autoCToMLDiff(nil,
		[]model.SyncDiffRemove{{Email: "left-committee@example.com", MemberType: model.MemberTypeCommitteeMember}},
	)
	diffComputer := &stubSyncDiffComputer{diff: diff}
	applier := &stubSyncApplier{}
	memberReader := &stubMemberReader{
		subscribed: map[string]map[string]bool{
			"ml-2": {"left-committee@example.com": true},
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
	require.True(t, applier.called, "ml-2 is also auto_c_to_ml for the same committee and removes this person on its own; it must not block ml-1's removal")
	assert.Equal(t, diff.Removes, applier.gotRemoves)
	assert.Empty(t, memberReader.listMembersCalls, "auto_c_to_ml siblings for the same committee are excluded before any ListMembers call")
}

// TestTriggerAutoSync_CrossListGuard_ChecksPublicAutoCToMLSibling pins that a *public* auto_c_to_ml
// sibling is NOT excluded from the cross-list guard: TriggerAutoSync never auto-propagates removals
// on a public list, so a public auto_c_to_ml sibling's subscription is never cleaned up on its own.
// Excluding it would let a private sibling remove the member while they remain subscribed on the
// public list, bypassing the guard entirely (per @copilot-pull-request-reviewer).
func TestTriggerAutoSync_CrossListGuard_ChecksPublicAutoCToMLSibling(t *testing.T) {
	ml := mlWithAudienceAccess("committee-1", model.AudienceAccessInviteOnly)
	mlReader := &stubMLReader{
		ml: ml,
		listMLs: []*model.GroupsIOMailingList{
			ml,
			{
				UID:            "ml-2",
				AudienceAccess: model.AudienceAccessPublic,
				Committees: []model.Committee{
					{UID: "committee-1", SyncMode: model.SyncModeAutoCommitteeToMailingList},
				},
			},
		},
	}
	diff := autoCToMLDiff(nil,
		[]model.SyncDiffRemove{{Email: "left-committee@example.com", MemberType: model.MemberTypeCommitteeMember}},
	)
	diffComputer := &stubSyncDiffComputer{diff: diff}
	applier := &stubSyncApplier{}
	memberReader := &stubMemberReader{
		subscribed: map[string]map[string]bool{
			"ml-2": {"left-committee@example.com": true},
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
	assert.NotEmpty(t, memberReader.listMembersCalls, "public auto_c_to_ml siblings are not excluded from the ListMembers check")
	// Still subscribed to the public ml-2 sibling (which never auto-removes it on its own), so the
	// cross-list guard drops the remove; with no adds either, TriggerAutoSync never calls ApplySyncDiff.
	assert.False(t, applier.called, "ml-1 must not remove the member while they remain subscribed on the public sibling")
}

// TestTriggerAutoSync_CrossListGuard_FailsClosedOnListMembersError pins that a sibling roster we
// fail to fetch is treated as "the member might still be subscribed there" rather than silently
// allowing the removal to proceed — an unreadable roster could be hiding a still-active
// subscription (per @copilot-pull-request-reviewer).
func TestTriggerAutoSync_CrossListGuard_FailsClosedOnListMembersError(t *testing.T) {
	ml := mlWithAudienceAccess("committee-1", model.AudienceAccessInviteOnly)
	mlReader := &stubMLReader{
		ml: ml,
		listMLs: []*model.GroupsIOMailingList{
			ml,
			{UID: "ml-2", Committees: []model.Committee{{UID: "committee-1"}}},
		},
	}
	diff := autoCToMLDiff(nil,
		[]model.SyncDiffRemove{{Email: "unknown-status@example.com", MemberType: model.MemberTypeMailingListMember}},
	)
	diffComputer := &stubSyncDiffComputer{diff: diff}
	applier := &stubSyncApplier{}
	memberReader := &stubMemberReader{err: errors.New("itx unavailable")}

	o := NewRealtimeAutoSyncOrchestrator(
		WithRealtimeAutoSyncMailingListReader(mlReader),
		WithRealtimeAutoSyncMemberReader(memberReader),
		WithRealtimeAutoSyncDiffComputer(diffComputer),
		WithRealtimeAutoSyncApplier(applier),
	)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.NotEmpty(t, memberReader.listMembersCalls, "ListMembers must be attempted for the sibling")
	assert.False(t, applier.called, "a failed sibling roster fetch must fail closed and skip the remove rather than allow it")
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

func TestTriggerAutoSync_MarksContextAsTrustedSystemCaller(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	mlReader := &stubMLReader{ml: ml}
	diffComputer := &stubSyncDiffComputer{diff: autoCToMLDiff(
		[]model.SyncDiffAdd{{Email: "a@example.com"}}, nil,
	)}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	// Starting from context.Background() mirrors a NATS event handler's context, which carries no
	// end-user caller principal.
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.True(t, diffComputer.sawTrusted, "ComputeSyncDiff must see a trusted-system-caller-marked context")
	assert.True(t, applier.sawTrusted, "ApplySyncDiff must see a trusted-system-caller-marked context")
}

// TestTriggerAutoSync_DisabledByKillSwitch_NoOp pins issue #4878: AUTO_SYNC_ENABLED=false (wired
// through WithRealtimeAutoSyncEnabled(false)) must stop TriggerAutoSync before it touches
// ComputeSyncDiff/ApplySyncDiff at all, regardless of what the diff would have been — this is the
// single choke point CommitteeMemberSubscriber and the auto-sync-driven ApplySyncDiff path both go
// through, independent of the frontend LaunchDarkly flag.
func TestTriggerAutoSync_DisabledByKillSwitch_NoOp(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	mlReader := &stubMLReader{ml: ml}
	diffComputer := &stubSyncDiffComputer{diff: autoCToMLDiff(
		[]model.SyncDiffAdd{{Email: "a@example.com", MemberType: model.MemberTypeMailingListMember}}, nil,
	)}
	applier := &stubSyncApplier{}

	o := NewRealtimeAutoSyncOrchestrator(
		WithRealtimeAutoSyncMailingListReader(mlReader),
		WithRealtimeAutoSyncMemberReader(&stubMemberReader{}),
		WithRealtimeAutoSyncDiffComputer(diffComputer),
		WithRealtimeAutoSyncApplier(applier),
		WithRealtimeAutoSyncEnabled(false),
	)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	assert.Empty(t, diffComputer.calls, "a disabled kill switch must short-circuit before ComputeSyncDiff is even called")
	assert.False(t, applier.called)
}

// TestTriggerAutoSync_EnabledByDefault_WhenOptionOmitted pins that omitting
// WithRealtimeAutoSyncEnabled entirely (as every pre-#4878 test in this file does via
// newTestRealtimeAutoSyncOrchestrator) still runs auto-sync — the kill switch fails open.
func TestTriggerAutoSync_EnabledByDefault_WhenOptionOmitted(t *testing.T) {
	ml := mlWithCommittee("committee-1")
	mlReader := &stubMLReader{ml: ml}
	diffComputer := &stubSyncDiffComputer{diff: autoCToMLDiff(
		[]model.SyncDiffAdd{{Email: "a@example.com", MemberType: model.MemberTypeMailingListMember}}, nil,
	)}
	applier := &stubSyncApplier{}

	o := newTestRealtimeAutoSyncOrchestrator(mlReader, diffComputer, applier)
	err := o.TriggerAutoSync(context.Background(), "ml-1")

	require.NoError(t, err)
	require.True(t, applier.called)
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
