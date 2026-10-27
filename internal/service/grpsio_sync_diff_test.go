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

// stubMemberReader returns configured responses for ListMembers.
type stubMemberReader struct {
	members []*model.GrpsIOMember
	err     error
}

func (r *stubMemberReader) ListMembers(_ context.Context, _ string) ([]*model.GrpsIOMember, int, error) {
	return r.members, len(r.members), r.err
}
func (r *stubMemberReader) GetMember(_ context.Context, _, _ string) (*model.GrpsIOMember, error) {
	return nil, nil
}
func (r *stubMemberReader) CheckSubscriber(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

var _ port.GroupsIOMailingListMemberReader = (*stubMemberReader)(nil)

// stubCommitteeMembershipLookup returns configured responses for GetCommitteeMembers.
type stubCommitteeMembershipLookup struct {
	members []port.CommitteeMember
	err     error
}

func (l *stubCommitteeMembershipLookup) GetCommitteeMembers(_ context.Context, _ string) ([]port.CommitteeMember, error) {
	return l.members, l.err
}

var _ port.CommitteeMembershipLookup = (*stubCommitteeMembershipLookup)(nil)

func mlWithSyncMode(committeeUID, syncMode string) *model.GroupsIOMailingList {
	return &model.GroupsIOMailingList{
		UID:        "ml-1",
		Committees: []model.Committee{{UID: committeeUID, SyncMode: syncMode}},
	}
}

func TestComputeSyncDiff_NoLinkedCommittee(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: &model.GroupsIOMailingList{UID: "ml-1"}}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestComputeSyncDiff_ManualSyncMode(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithSyncMode("committee-1", model.SyncModeManual)}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestComputeSyncDiff_EmptySyncModeTreatedAsManual(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithSyncMode("committee-1", "")}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	assert.True(t, errors.As(err, &validation))
}

func TestComputeSyncDiff_AutoMLToCommittee_NotYetSupported(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithSyncMode("committee-1", model.SyncModeAutoMailingListToCommittee)}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
	var validation errs.Validation
	require.True(t, errors.As(err, &validation))
	assert.Contains(t, validation.Error(), "auto_ml_to_c")
}

func TestComputeSyncDiff_MailingListReaderError(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{err: errors.New("not found")}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
}

func TestComputeSyncDiff_CommitteeLookupError(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffCommitteeMembershipLookup(&stubCommitteeMembershipLookup{err: errs.NewNotFound("committee not found")}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
	var notFound errs.NotFound
	assert.True(t, errors.As(err, &notFound))
}

func TestComputeSyncDiff_MemberReaderError(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffCommitteeMembershipLookup(&stubCommitteeMembershipLookup{}),
		WithSyncDiffMemberReader(&stubMemberReader{err: errors.New("itx down")}),
	)
	_, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.Error(t, err)
}

// TestComputeSyncDiff_Reconciliation pins the full happy-path reconciliation: a committee member
// missing from the list is an add, a committee-sourced list member no longer on the committee is
// a remove, a directly-added list member sharing a departed committee member's email is left
// alone, and matching is case/whitespace-insensitive by email.
func TestComputeSyncDiff_Reconciliation(t *testing.T) {
	committeeMembers := []port.CommitteeMember{
		{UID: "cm-1", Email: "Alice@Example.com"},  // already on the list (case-insensitive match) -> no add
		{UID: "cm-2", Email: " bob@example.com  "}, // missing from the list -> add
	}
	mlMembers := []*model.GrpsIOMember{
		{UID: "mlm-1", Email: "alice@example.com", MemberType: "committee"},
		{UID: "mlm-2", Email: "carol@example.com", MemberType: "committee"}, // no longer on committee -> remove
		{UID: "mlm-3", Email: "carol@example.com", MemberType: "direct"},    // direct member, same email -> left alone
	}

	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffCommitteeMembershipLookup(&stubCommitteeMembershipLookup{members: committeeMembers}),
		WithSyncDiffMemberReader(&stubMemberReader{members: mlMembers}),
	)

	diff, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.NoError(t, err)

	require.Len(t, diff.Adds, 1)
	assert.Equal(t, " bob@example.com  ", diff.Adds[0].Email)
	assert.Equal(t, "cm-2", diff.Adds[0].CommitteeMemberUID)

	require.Len(t, diff.Removes, 1)
	assert.Equal(t, "carol@example.com", diff.Removes[0].Email)
	assert.Equal(t, "mlm-2", diff.Removes[0].MailingListMemberID)

	assert.Equal(t, "ml-1", diff.MailingListID)
	assert.Equal(t, "committee-1", diff.CommitteeUID)
	assert.Equal(t, model.SyncModeAutoCommitteeToMailingList, diff.SyncMode)
}

func TestComputeSyncDiff_NoChangesNeeded(t *testing.T) {
	o := NewSyncDiffOrchestrator(
		WithSyncDiffMailingListReader(&stubMLReader{ml: mlWithSyncMode("committee-1", model.SyncModeAutoCommitteeToMailingList)}),
		WithSyncDiffCommitteeMembershipLookup(&stubCommitteeMembershipLookup{
			members: []port.CommitteeMember{{UID: "cm-1", Email: "alice@example.com"}},
		}),
		WithSyncDiffMemberReader(&stubMemberReader{
			members: []*model.GrpsIOMember{{UID: "mlm-1", Email: "alice@example.com", MemberType: "committee"}},
		}),
	)

	diff, err := o.ComputeSyncDiff(context.Background(), "ml-1")
	require.NoError(t, err)
	assert.Empty(t, diff.Adds)
	assert.Empty(t, diff.Removes)
}
