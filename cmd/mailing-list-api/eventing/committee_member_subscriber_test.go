// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// stubAutoSyncer records TriggerAutoSync calls and returns a configured error, optionally keyed
// per mailing list ID.
type stubAutoSyncer struct {
	calls     []string
	err       error
	errByList map[string]error
}

func (s *stubAutoSyncer) TriggerAutoSync(_ context.Context, mailingListID string) error {
	s.calls = append(s.calls, mailingListID)
	if s.errByList != nil {
		if err, ok := s.errByList[mailingListID]; ok {
			return err
		}
	}
	return s.err
}

// stubCommitteeMailingListReader is a minimal port.GroupsIOMailingListReader for the committee
// subscriber tests: only ListMailingLists is exercised.
type stubCommitteeMailingListReader struct {
	lists []*model.GroupsIOMailingList
	err   error
}

func (r *stubCommitteeMailingListReader) ListMailingLists(_ context.Context, _, _ string) ([]*model.GroupsIOMailingList, int, error) {
	return r.lists, len(r.lists), r.err
}
func (r *stubCommitteeMailingListReader) GetMailingList(_ context.Context, _ string) (*model.GroupsIOMailingList, error) {
	return nil, nil
}
func (r *stubCommitteeMailingListReader) GetMailingListCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (r *stubCommitteeMailingListReader) GetMailingListMemberCount(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func autoC2MList(uid string) *model.GroupsIOMailingList {
	return &model.GroupsIOMailingList{
		UID:        uid,
		Committees: []model.Committee{{UID: "committee-1", SyncMode: model.SyncModeAutoCommitteeToMailingList}},
	}
}

func TestCommitteeUIDFromEvent(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{name: "created event", data: `{"member_uid":"m1","committee_uid":"c1","project_uid":"p1"}`, want: "c1"},
		{name: "deleted event", data: `{"member_uid":"m1","committee_uid":"c2","email":"a@example.com"}`, want: "c2"},
		{name: "updated event", data: `{"member_uid":"m1","committee_uid":"c3","old_member":{},"new_member":{}}`, want: "c3"},
		{name: "missing committee_uid", data: `{"member_uid":"m1"}`, want: ""},
		{name: "malformed json", data: `not json`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := committeeUIDFromEvent([]byte(tt.data))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProcessCommitteeMemberEvent_TriggersAutoSyncForAutoC2MLinkedLists(t *testing.T) {
	reader := &stubCommitteeMailingListReader{lists: []*model.GroupsIOMailingList{autoC2MList("ml-1")}}
	syncer := &stubAutoSyncer{}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.NoError(t, err)
	assert.Equal(t, []string{"ml-1"}, syncer.calls)
}

func TestProcessCommitteeMemberEvent_SkipsListsNotConfiguredForAutoC2ML(t *testing.T) {
	manualList := &model.GroupsIOMailingList{
		UID:        "ml-manual",
		Committees: []model.Committee{{UID: "committee-1", SyncMode: model.SyncModeManual}},
	}
	ml2cList := &model.GroupsIOMailingList{
		UID:        "ml-ml2c",
		Committees: []model.Committee{{UID: "committee-1", SyncMode: model.SyncModeAutoMailingListToCommittee}},
	}
	noCommitteeList := &model.GroupsIOMailingList{UID: "ml-none"}
	reader := &stubCommitteeMailingListReader{lists: []*model.GroupsIOMailingList{manualList, ml2cList, noCommitteeList}}
	syncer := &stubAutoSyncer{}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.NoError(t, err)
	assert.Empty(t, syncer.calls, "only auto_c_to_ml lists should trigger auto-sync from a committee event")
}

func TestProcessCommitteeMemberEvent_MultipleLinkedLists_AllTriggered(t *testing.T) {
	reader := &stubCommitteeMailingListReader{lists: []*model.GroupsIOMailingList{
		autoC2MList("ml-1"),
		autoC2MList("ml-2"),
	}}
	syncer := &stubAutoSyncer{}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"ml-1", "ml-2"}, syncer.calls)
}

func TestProcessCommitteeMemberEvent_ListMailingListsError_Propagates(t *testing.T) {
	reader := &stubCommitteeMailingListReader{err: errors.New("itx unavailable")}
	syncer := &stubAutoSyncer{}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.Error(t, err)
	assert.Empty(t, syncer.calls)
}

func TestProcessCommitteeMemberEvent_OneListFails_OthersStillAttempted(t *testing.T) {
	reader := &stubCommitteeMailingListReader{lists: []*model.GroupsIOMailingList{
		autoC2MList("ml-1"),
		autoC2MList("ml-2"),
	}}
	syncer := &stubAutoSyncer{errByList: map[string]error{"ml-1": errors.New("itx job failed")}}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.Error(t, err, "a failure should be reported so the caller can log it")
	assert.ElementsMatch(t, []string{"ml-1", "ml-2"}, syncer.calls, "a failure on one list must not block attempting the others")
}

func TestProcessCommitteeMemberEvent_NoLinkedLists_NoOp(t *testing.T) {
	reader := &stubCommitteeMailingListReader{}
	syncer := &stubAutoSyncer{}

	err := processCommitteeMemberEvent(context.Background(), "committee-1", reader, syncer, silentLog())

	require.NoError(t, err)
	assert.Empty(t, syncer.calls)
}
