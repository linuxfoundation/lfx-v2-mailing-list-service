// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/stretchr/testify/require"
)

// fakeSyncDiffComputer is a test double for port.SyncDiffComputer that captures its call
// arguments and returns canned results.
type fakeSyncDiffComputer struct {
	startJobID      string
	startErr        error
	startCalledWith string

	status         *model.SyncDiffJobStatus
	statusErr      error
	statusJobIDArg string
}

func (f *fakeSyncDiffComputer) ComputeSyncDiff(_ context.Context, _ string) (*model.SyncDiff, error) {
	return nil, nil
}

func (f *fakeSyncDiffComputer) StartSyncDiff(_ context.Context, mailingListID string) (string, error) {
	f.startCalledWith = mailingListID
	return f.startJobID, f.startErr
}

func (f *fakeSyncDiffComputer) GetSyncDiffStatus(_ context.Context, _, jobID string) (*model.SyncDiffJobStatus, error) {
	f.statusJobIDArg = jobID
	return f.status, f.statusErr
}

// fakeSyncApplier is a test double for port.SyncApplier that captures its call arguments and
// returns canned results.
type fakeSyncApplier struct {
	startJobID      string
	startErr        error
	startAddsArg    []model.SyncDiffAdd
	startRemovesArg []model.SyncDiffRemove

	status         *model.SyncApplyJobStatus
	statusErr      error
	statusJobIDArg string
}

func (f *fakeSyncApplier) ApplySyncDiff(_ context.Context, _, _ string, _ []model.SyncDiffAdd, _ []model.SyncDiffRemove) (*model.SyncApplyResult, error) {
	return nil, nil
}

func (f *fakeSyncApplier) StartSyncApply(_ context.Context, _, _ string, approvedAdds []model.SyncDiffAdd, approvedRemoves []model.SyncDiffRemove) (string, error) {
	f.startAddsArg = approvedAdds
	f.startRemovesArg = approvedRemoves
	return f.startJobID, f.startErr
}

func (f *fakeSyncApplier) GetSyncApplyStatus(_ context.Context, _, jobID string) (*model.SyncApplyJobStatus, error) {
	f.statusJobIDArg = jobID
	return f.status, f.statusErr
}

func TestStartGroupsioMailingListSyncDiff(t *testing.T) {
	computer := &fakeSyncDiffComputer{startJobID: "job-1"}
	s := &mailingListAPI{syncDiffComputer: computer}

	got, err := s.StartGroupsioMailingListSyncDiff(context.Background(), &mailinglist.StartGroupsioMailingListSyncDiffPayload{SubgroupID: "ml-1"})
	require.NoError(t, err)
	require.Equal(t, "job-1", got.JobID)
	require.Equal(t, "ml-1", computer.startCalledWith)
}

func TestStartGroupsioMailingListSyncDiff_Error(t *testing.T) {
	computer := &fakeSyncDiffComputer{startErr: errs.NewServiceUnavailable("itx down")}
	s := &mailingListAPI{syncDiffComputer: computer}

	_, err := s.StartGroupsioMailingListSyncDiff(context.Background(), &mailinglist.StartGroupsioMailingListSyncDiffPayload{SubgroupID: "ml-1"})
	require.Error(t, err)
}

func TestGetGroupsioMailingListSyncDiffStatus(t *testing.T) {
	computer := &fakeSyncDiffComputer{status: &model.SyncDiffJobStatus{JobID: "job-1", Status: "ongoing"}}
	s := &mailingListAPI{syncDiffComputer: computer}

	got, err := s.GetGroupsioMailingListSyncDiffStatus(context.Background(), &mailinglist.GetGroupsioMailingListSyncDiffStatusPayload{SubgroupID: "ml-1", JobID: "job-1"})
	require.NoError(t, err)
	require.Equal(t, "job-1", got.JobID)
	require.Equal(t, "ongoing", got.Status)
	require.Equal(t, "job-1", computer.statusJobIDArg)
}

func TestGetGroupsioMailingListSyncDiffStatus_Error(t *testing.T) {
	computer := &fakeSyncDiffComputer{statusErr: errs.NewConflict("relinked")}
	s := &mailingListAPI{syncDiffComputer: computer}

	_, err := s.GetGroupsioMailingListSyncDiffStatus(context.Background(), &mailinglist.GetGroupsioMailingListSyncDiffStatusPayload{SubgroupID: "ml-1", JobID: "job-1"})
	require.Error(t, err)
}

func TestStartGroupsioMailingListSyncApply(t *testing.T) {
	applier := &fakeSyncApplier{startJobID: "apply-1"}
	s := &mailingListAPI{syncApplier: applier}

	payload := &mailinglist.StartGroupsioMailingListSyncApplyPayload{
		SubgroupID:   "ml-1",
		CommitteeUID: "c-1",
		Adds: []*mailinglist.GroupsioSyncDiffAdd{
			{Email: "alice@example.com", MemberID: "cm-1", MemberType: "committee_member"},
		},
	}
	got, err := s.StartGroupsioMailingListSyncApply(context.Background(), payload)
	require.NoError(t, err)
	require.Equal(t, "apply-1", got.JobID)
	require.Len(t, applier.startAddsArg, 1)
	require.Equal(t, "alice@example.com", applier.startAddsArg[0].Email)
}

func TestStartGroupsioMailingListSyncApply_Error(t *testing.T) {
	applier := &fakeSyncApplier{startErr: errs.NewValidation("mismatch")}
	s := &mailingListAPI{syncApplier: applier}

	_, err := s.StartGroupsioMailingListSyncApply(context.Background(), &mailinglist.StartGroupsioMailingListSyncApplyPayload{SubgroupID: "ml-1", CommitteeUID: "c-1"})
	require.Error(t, err)
}

func TestGetGroupsioMailingListSyncApplyStatus(t *testing.T) {
	applier := &fakeSyncApplier{status: &model.SyncApplyJobStatus{JobID: "apply-1", Status: "completed", AddedCount: 1}}
	s := &mailingListAPI{syncApplier: applier}

	got, err := s.GetGroupsioMailingListSyncApplyStatus(context.Background(), &mailinglist.GetGroupsioMailingListSyncApplyStatusPayload{SubgroupID: "ml-1", JobID: "apply-1"})
	require.NoError(t, err)
	require.Equal(t, "apply-1", got.JobID)
	require.Equal(t, 1, got.AddedCount)
	require.Equal(t, "apply-1", applier.statusJobIDArg)
}

func TestGetGroupsioMailingListSyncApplyStatus_Error(t *testing.T) {
	applier := &fakeSyncApplier{statusErr: errs.NewConflict("relinked")}
	s := &mailingListAPI{syncApplier: applier}

	_, err := s.GetGroupsioMailingListSyncApplyStatus(context.Background(), &mailinglist.GetGroupsioMailingListSyncApplyStatusPayload{SubgroupID: "ml-1", JobID: "apply-1"})
	require.Error(t, err)
}

// fakeMailingListWriter is a test double for port.GroupsIOMailingListWriter that
// captures the domain model it was called with.
type fakeMailingListWriter struct {
	captured *model.GroupsIOMailingList
}

func (f *fakeMailingListWriter) CreateMailingList(_ context.Context, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	f.captured = ml
	return ml, nil
}

func (f *fakeMailingListWriter) UpdateMailingList(_ context.Context, _ string, ml *model.GroupsIOMailingList) (*model.GroupsIOMailingList, error) {
	f.captured = ml
	return ml, nil
}

func (f *fakeMailingListWriter) DeleteMailingList(_ context.Context, _ string) error {
	return nil
}

func TestCreateGroupsioMailingListCommitteeFilters(t *testing.T) {
	writer := &fakeMailingListWriter{}
	s := &mailingListAPI{mailingListWriter: writer}

	payload := &mailinglist.CreateGroupsioMailingListPayload{
		CommitteeUID:         strp("c-1"),
		SyncMode:             strp("auto_c_to_ml"),
		CommitteeFilters:     []string{"Voting Rep", "Alternate Voting Rep"},
		CommitteeRoleFilters: []string{"chair"},
	}

	got, err := s.CreateGroupsioMailingList(context.Background(), payload)
	require.NoError(t, err)
	require.NotNil(t, writer.captured)

	require.Len(t, writer.captured.Committees, 1)
	require.Equal(t, "c-1", writer.captured.Committees[0].UID)
	require.Equal(t, "auto_c_to_ml", writer.captured.Committees[0].SyncMode)
	require.Equal(t, []string{"Voting Rep", "Alternate Voting Rep"}, writer.captured.Committees[0].AllowedVotingStatuses)
	require.Equal(t, []string{"chair"}, writer.captured.Committees[0].CommitteeRoleFilters)

	require.Equal(t, []string{"Voting Rep", "Alternate Voting Rep"}, got.CommitteeFilters)
	require.Equal(t, []string{"chair"}, got.CommitteeRoleFilters)
}

func TestCreateGroupsioMailingListFiltersWithoutCommitteeUIDOrSyncMode(t *testing.T) {
	writer := &fakeMailingListWriter{}
	s := &mailingListAPI{mailingListWriter: writer}

	// Filters present but no committee_uid/sync_mode still triggers Committees population;
	// validation of "requires committee_uid" happens in the orchestrator, not here.
	payload := &mailinglist.CreateGroupsioMailingListPayload{
		CommitteeFilters: []string{"Voting Rep"},
	}

	_, err := s.CreateGroupsioMailingList(context.Background(), payload)
	require.NoError(t, err)
	require.Len(t, writer.captured.Committees, 1)
	require.Equal(t, []string{"Voting Rep"}, writer.captured.Committees[0].AllowedVotingStatuses)
}

func TestCreateGroupsioMailingListNoCommitteeFieldsLeavesCommitteesEmpty(t *testing.T) {
	writer := &fakeMailingListWriter{}
	s := &mailingListAPI{mailingListWriter: writer}

	payload := &mailinglist.CreateGroupsioMailingListPayload{}

	_, err := s.CreateGroupsioMailingList(context.Background(), payload)
	require.NoError(t, err)
	require.Empty(t, writer.captured.Committees)
}

// An explicit, non-nil empty committee_filters array (distinct from an omitted field) must still
// populate Committees so the orchestrator's "requires committee_uid" guard can reject it —
// len(...) > 0 would treat this identically to an omitted field and silently drop it (PR #110).
func TestCreateGroupsioMailingListEmptyCommitteeFiltersWithoutCommitteeUID(t *testing.T) {
	writer := &fakeMailingListWriter{}
	s := &mailingListAPI{mailingListWriter: writer}

	payload := &mailinglist.CreateGroupsioMailingListPayload{
		CommitteeFilters: []string{},
	}

	_, err := s.CreateGroupsioMailingList(context.Background(), payload)
	require.NoError(t, err)
	require.Len(t, writer.captured.Committees, 1)
	require.NotNil(t, writer.captured.Committees[0].AllowedVotingStatuses)
}

func TestUpdateGroupsioMailingListEmptyCommitteeFiltersWithoutCommitteeUID(t *testing.T) {
	writer := &fakeMailingListWriter{}
	s := &mailingListAPI{mailingListWriter: writer}

	payload := &mailinglist.UpdateGroupsioMailingListPayload{
		SubgroupID:           "ml-1",
		CommitteeRoleFilters: []string{},
	}

	_, err := s.UpdateGroupsioMailingList(context.Background(), payload)
	require.NoError(t, err)
	require.Len(t, writer.captured.Committees, 1)
	require.NotNil(t, writer.captured.Committees[0].CommitteeRoleFilters)
}

func TestUpdateGroupsioMailingListCommitteeFilters(t *testing.T) {
	writer := &fakeMailingListWriter{}
	s := &mailingListAPI{mailingListWriter: writer}

	payload := &mailinglist.UpdateGroupsioMailingListPayload{
		SubgroupID:           "ml-1",
		CommitteeUID:         strp("c-2"),
		CommitteeFilters:     []string{"Voting Rep"},
		CommitteeRoleFilters: []string{"secretary"},
	}

	_, err := s.UpdateGroupsioMailingList(context.Background(), payload)
	require.NoError(t, err)
	require.Len(t, writer.captured.Committees, 1)
	require.Equal(t, "c-2", writer.captured.Committees[0].UID)
	require.Equal(t, []string{"Voting Rep"}, writer.captured.Committees[0].AllowedVotingStatuses)
	require.Equal(t, []string{"secretary"}, writer.captured.Committees[0].CommitteeRoleFilters)
}
