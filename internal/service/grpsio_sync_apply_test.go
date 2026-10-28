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

// stubMemberWriter records AddMember/DeleteMember/InviteMembers calls and returns configured
// responses, keyed by email/memberID so a test can fail individual operations.
type stubMemberWriter struct {
	addErrByEmail map[string]error
	deleteErrByID map[string]error
	inviteErr     error
	added         []*model.GrpsIOMember
	deletedIDs    []string
	invitedEmails []string
}

func (w *stubMemberWriter) AddMember(_ context.Context, _ string, member *model.GrpsIOMember) (*model.GrpsIOMember, error) {
	if err, ok := w.addErrByEmail[member.Email]; ok {
		return nil, err
	}
	w.added = append(w.added, member)
	return member, nil
}

func (w *stubMemberWriter) UpdateMember(_ context.Context, _, _ string, member *model.GrpsIOMember) (*model.GrpsIOMember, error) {
	return member, nil
}

func (w *stubMemberWriter) DeleteMember(_ context.Context, _, memberID string) error {
	if err, ok := w.deleteErrByID[memberID]; ok {
		return err
	}
	w.deletedIDs = append(w.deletedIDs, memberID)
	return nil
}

func (w *stubMemberWriter) InviteMembers(_ context.Context, _ string, emails []string) error {
	if w.inviteErr != nil {
		return w.inviteErr
	}
	w.invitedEmails = append(w.invitedEmails, emails...)
	return nil
}

var _ port.GroupsIOMailingListMemberWriter = (*stubMemberWriter)(nil)

// stubSyncApplyUserReader returns a configured username for a known set of emails and
// port.ErrUserNotFound for anything else, unless err is set (a hard failure). Unlike the
// single-value stubUserReader in datastream_member_invite_test.go, this is keyed by email since
// sync-apply tests need distinct resolved/unresolved emails in the same test.
type stubSyncApplyUserReader struct {
	usernameByEmail map[string]string
	err             error
}

func (r *stubSyncApplyUserReader) UsernameByEmail(_ context.Context, email string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if username, ok := r.usernameByEmail[email]; ok {
		return username, nil
	}
	return "", port.ErrUserNotFound
}

var _ port.UserReader = (*stubSyncApplyUserReader)(nil)

func newApplyOrchestrator(
	mlReader port.GroupsIOMailingListReader,
	memberReader port.GroupsIOMailingListMemberReader,
	memberWriter port.GroupsIOMailingListMemberWriter,
	store port.CommitteeSyncSettingsStore,
	userReader port.UserReader,
	committeeLookup port.CommitteeMembershipLookup,
) port.SyncApplier {
	return NewSyncApplyOrchestrator(
		WithSyncApplyMailingListReader(mlReader),
		WithSyncApplyMemberReader(memberReader),
		WithSyncApplyMemberWriter(memberWriter),
		WithSyncApplySyncSettingsStore(store),
		WithSyncApplyUserReader(userReader),
		WithSyncApplyCommitteeMembershipLookup(committeeLookup),
	)
}

func TestApplySyncDiff_NoLinkedCommittee(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: &model.GroupsIOMailingList{UID: "ml-1"}},
		&stubMemberReader{}, &stubMemberWriter{}, &stubSyncSettingsStore{}, &stubSyncApplyUserReader{},
		&stubCommitteeMembershipLookup{},
	)
	_, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, nil)
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestApplySyncDiff_ManualSyncMode(t *testing.T) {
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommittee("committee-1")},
		&stubMemberReader{}, &stubMemberWriter{},
		&stubSyncSettingsStore{getMode: model.SyncModeManual, getFound: true},
		&stubSyncApplyUserReader{},
		&stubCommitteeMembershipLookup{},
	)
	_, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, nil)
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestApplySyncDiff_AddResolvedUser_AddsDirectly(t *testing.T) {
	writer := &stubMemberWriter{}
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommittee("committee-1")},
		&stubMemberReader{}, writer,
		&stubSyncSettingsStore{getMode: model.SyncModeAutoCommitteeToMailingList, getFound: true},
		&stubSyncApplyUserReader{usernameByEmail: map[string]string{"alice@example.com": "alice"}},
		&stubCommitteeMembershipLookup{members: []port.CommitteeMember{{UID: "cm-1", Email: "alice@example.com", Name: "Alice Example"}}},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", CommitteeMemberUID: "cm-1", Name: "Alice Example"},
	}, nil)
	require.NoError(t, err)
	require.Len(t, result.Added, 1)
	assert.Equal(t, model.SyncApplyOutcomeAdded, result.Added[0].Outcome)
	assert.Empty(t, result.Added[0].Error)
	require.Len(t, writer.added, 1)
	assert.Equal(t, "alice", writer.added[0].Username)
	assert.Equal(t, committeeMemberType, writer.added[0].MemberType)
}

func TestApplySyncDiff_AddUnresolvedUser_FallsBackToInvite(t *testing.T) {
	writer := &stubMemberWriter{}
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommittee("committee-1")},
		&stubMemberReader{}, writer,
		&stubSyncSettingsStore{getMode: model.SyncModeAutoCommitteeToMailingList, getFound: true},
		&stubSyncApplyUserReader{},
		&stubCommitteeMembershipLookup{members: []port.CommitteeMember{{UID: "cm-2", Email: "bob@example.com", Name: "Bob Example"}}},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", []model.SyncDiffAdd{
		{Email: "bob@example.com", CommitteeMemberUID: "cm-2", Name: "Bob Example"},
	}, nil)
	require.NoError(t, err)
	require.Len(t, result.Added, 1)
	assert.Equal(t, model.SyncApplyOutcomeInvited, result.Added[0].Outcome)
	assert.Equal(t, "Bob Example", result.Added[0].NameHint)
	assert.Equal(t, []string{"bob@example.com"}, writer.invitedEmails)
}

func TestApplySyncDiff_AddWriterFailure_ReportedAsFailed(t *testing.T) {
	writer := &stubMemberWriter{addErrByEmail: map[string]error{"alice@example.com": errors.New("boom")}}
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommittee("committee-1")},
		&stubMemberReader{}, writer,
		&stubSyncSettingsStore{getMode: model.SyncModeAutoCommitteeToMailingList, getFound: true},
		&stubSyncApplyUserReader{usernameByEmail: map[string]string{"alice@example.com": "alice"}},
		&stubCommitteeMembershipLookup{members: []port.CommitteeMember{{UID: "cm-1", Email: "alice@example.com"}}},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", CommitteeMemberUID: "cm-1"},
	}, nil)
	require.NoError(t, err)
	require.Len(t, result.Added, 1)
	assert.Equal(t, model.SyncApplyOutcomeFailed, result.Added[0].Outcome)
	assert.Equal(t, "failed to add member to mailing list", result.Added[0].Error)
}

func TestApplySyncDiff_AddNoLongerInCurrentDiff_ReportedAsFailed(t *testing.T) {
	writer := &stubMemberWriter{}
	o := newApplyOrchestrator(
		&stubMLReader{ml: mlWithCommittee("committee-1")},
		&stubMemberReader{}, writer,
		&stubSyncSettingsStore{getMode: model.SyncModeAutoCommitteeToMailingList, getFound: true},
		&stubSyncApplyUserReader{usernameByEmail: map[string]string{"alice@example.com": "alice"}},
		// alice is no longer a committee member by apply time — the current diff has no adds.
		&stubCommitteeMembershipLookup{members: nil},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", []model.SyncDiffAdd{
		{Email: "alice@example.com", CommitteeMemberUID: "cm-1"},
	}, nil)
	require.NoError(t, err)
	require.Len(t, result.Added, 1)
	assert.Equal(t, model.SyncApplyOutcomeFailed, result.Added[0].Outcome)
	assert.Contains(t, result.Added[0].Error, "no longer reflects the current committee roster")
	assert.Empty(t, writer.added)
}

func TestApplySyncDiff_Remove_NotOnSiblingList_RemovesDirectly(t *testing.T) {
	writer := &stubMemberWriter{}
	mlReader := &stubMLReader{
		ml:      mlWithCommittee("committee-1"),
		listMLs: []*model.GroupsIOMailingList{{UID: "ml-1"}, {UID: "ml-2"}},
	}
	// carol is a current committee-sourced member of the target list (ml-1); the sibling list
	// (ml-2) has an unrelated member, so carol is not reachable cross-list.
	memberReader := &stubMemberReader{membersByID: map[string][]*model.GrpsIOMember{
		"ml-1": {{Email: "carol@example.com", UID: "mlm-1", MemberType: committeeMemberType}},
		"ml-2": {{Email: "other@example.com"}},
	}}
	o := newApplyOrchestrator(
		mlReader, memberReader, writer,
		&stubSyncSettingsStore{getMode: model.SyncModeAutoCommitteeToMailingList, getFound: true},
		&stubSyncApplyUserReader{},
		&stubCommitteeMembershipLookup{},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MailingListMemberID: "mlm-1"},
	})
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.Equal(t, model.SyncApplyOutcomeRemoved, result.Removed[0].Outcome)
	assert.Equal(t, []string{"mlm-1"}, writer.deletedIDs)
}

func TestApplySyncDiff_Remove_StillOnSiblingList_SkipsCrossList(t *testing.T) {
	writer := &stubMemberWriter{}
	mlReader := &stubMLReader{
		ml:      mlWithCommittee("committee-1"),
		listMLs: []*model.GroupsIOMailingList{{UID: "ml-1"}, {UID: "ml-2"}},
	}
	// carol is a current committee-sourced member of the target list (ml-1), so her remove is
	// still valid against the current diff; ml-2 also reports her, making her cross-list reachable.
	memberReader := &stubMemberReader{membersByID: map[string][]*model.GrpsIOMember{
		"ml-1": {{Email: "carol@example.com", UID: "mlm-1", MemberType: committeeMemberType}},
		"ml-2": {{Email: "Carol@Example.com"}},
	}}
	o := newApplyOrchestrator(
		mlReader, memberReader, writer,
		&stubSyncSettingsStore{getMode: model.SyncModeAutoCommitteeToMailingList, getFound: true},
		&stubSyncApplyUserReader{},
		&stubCommitteeMembershipLookup{},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MailingListMemberID: "mlm-1"},
	})
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.Equal(t, model.SyncApplyOutcomeSkippedCrossList, result.Removed[0].Outcome)
	assert.Empty(t, writer.deletedIDs)
}

func TestApplySyncDiff_Remove_WriterFailure_ReportedAsFailed(t *testing.T) {
	writer := &stubMemberWriter{deleteErrByID: map[string]error{"mlm-1": errors.New("boom")}}
	mlReader := &stubMLReader{ml: mlWithCommittee("committee-1")}
	memberReader := &stubMemberReader{members: []*model.GrpsIOMember{
		{Email: "carol@example.com", UID: "mlm-1", MemberType: committeeMemberType},
	}}
	o := newApplyOrchestrator(
		mlReader, memberReader, writer,
		&stubSyncSettingsStore{getMode: model.SyncModeAutoCommitteeToMailingList, getFound: true},
		&stubSyncApplyUserReader{},
		&stubCommitteeMembershipLookup{},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MailingListMemberID: "mlm-1"},
	})
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.Equal(t, model.SyncApplyOutcomeFailed, result.Removed[0].Outcome)
	assert.Equal(t, "failed to remove member from mailing list", result.Removed[0].Error)
}

func TestApplySyncDiff_RemoveNoLongerInCurrentDiff_ReportedAsFailed(t *testing.T) {
	writer := &stubMemberWriter{}
	mlReader := &stubMLReader{ml: mlWithCommittee("committee-1")}
	// carol is no longer a committee-sourced member of the target list by apply time (e.g.
	// already removed by a prior apply) — the current diff has no removes for mlm-1.
	memberReader := &stubMemberReader{}
	o := newApplyOrchestrator(
		mlReader, memberReader, writer,
		&stubSyncSettingsStore{getMode: model.SyncModeAutoCommitteeToMailingList, getFound: true},
		&stubSyncApplyUserReader{},
		&stubCommitteeMembershipLookup{},
	)
	result, err := o.ApplySyncDiff(context.Background(), "ml-1", nil, []model.SyncDiffRemove{
		{Email: "carol@example.com", MailingListMemberID: "mlm-1"},
	})
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.Equal(t, model.SyncApplyOutcomeFailed, result.Removed[0].Outcome)
	assert.Contains(t, result.Removed[0].Error, "no longer reflects the current committee roster")
	assert.Empty(t, writer.deletedIDs)
}
