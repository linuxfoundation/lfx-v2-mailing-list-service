// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/stretchr/testify/require"
)

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
