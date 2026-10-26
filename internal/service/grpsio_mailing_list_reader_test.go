// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSyncSettingsStore returns configured responses/errors for each method, independent of
// the mock package's in-memory implementation — used to simulate store failures.
type stubSyncSettingsStore struct {
	getMode     string
	getRevision uint64
	getFound    bool
	getErr      error
	putErr      error
	deleteOK    bool
	deleteErr   error
}

func (s *stubSyncSettingsStore) GetSyncMode(_ context.Context, _, _ string) (string, uint64, bool, error) {
	return s.getMode, s.getRevision, s.getFound, s.getErr
}
func (s *stubSyncSettingsStore) PutSyncMode(_ context.Context, _, _, _ string) error { return s.putErr }
func (s *stubSyncSettingsStore) DeleteSyncModeIfUnchanged(_ context.Context, _, _ string, _ uint64) (bool, error) {
	return s.deleteOK, s.deleteErr
}

var _ port.CommitteeSyncSettingsStore = (*stubSyncSettingsStore)(nil)

func newReaderOrchestrator(reader port.GroupsIOMailingListReader, store port.CommitteeSyncSettingsStore) *GroupsIOMailingListReaderOrchestrator {
	return &GroupsIOMailingListReaderOrchestrator{
		reader:            reader,
		translator:        &passthroughTranslator{},
		syncSettingsStore: store,
	}
}

func TestGetMailingList_HydratesSyncMode_WhenFound(t *testing.T) {
	ml := mlWith("committee-1")
	ml.UID = "ml-1"
	reader := &stubMLReader{ml: ml}
	store := mock.NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-1", "auto_c_to_ml"))

	o := newReaderOrchestrator(reader, store)
	resp, err := o.GetMailingList(context.Background(), "ml-1")
	require.NoError(t, err)
	require.Len(t, resp.Committees, 1)
	assert.Equal(t, "auto_c_to_ml", resp.Committees[0].SyncMode)
}

func TestGetMailingList_NoStoredSyncMode_LeavesEmpty(t *testing.T) {
	ml := mlWith("committee-1")
	ml.UID = "ml-1"
	reader := &stubMLReader{ml: ml}
	store := mock.NewMockCommitteeSyncSettingsStore()

	o := newReaderOrchestrator(reader, store)
	resp, err := o.GetMailingList(context.Background(), "ml-1")
	require.NoError(t, err)
	require.Len(t, resp.Committees, 1)
	assert.Empty(t, resp.Committees[0].SyncMode)
}

func TestGetMailingList_NoCommittee_SkipsHydration(t *testing.T) {
	ml := &model.GroupsIOMailingList{UID: "ml-1"}
	reader := &stubMLReader{ml: ml}
	store := &stubSyncSettingsStore{getFound: true, getMode: "auto_ml_to_c"}

	o := newReaderOrchestrator(reader, store)
	resp, err := o.GetMailingList(context.Background(), "ml-1")
	require.NoError(t, err)
	assert.Empty(t, resp.Committees)
}

func TestGetMailingList_NilStore_NoPanic_LeavesSyncModeEmpty(t *testing.T) {
	ml := mlWith("committee-1")
	ml.UID = "ml-1"
	reader := &stubMLReader{ml: ml}

	o := newReaderOrchestrator(reader, nil)
	var resp *model.GroupsIOMailingList
	var err error
	assert.NotPanics(t, func() {
		resp, err = o.GetMailingList(context.Background(), "ml-1")
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Committees[0].SyncMode)
}

func TestGetMailingList_StoreError_LeavesSyncModeEmpty(t *testing.T) {
	ml := mlWith("committee-1")
	ml.UID = "ml-1"
	reader := &stubMLReader{ml: ml}
	store := &stubSyncSettingsStore{getErr: errors.New("kv unavailable")}

	o := newReaderOrchestrator(reader, store)
	resp, err := o.GetMailingList(context.Background(), "ml-1")
	require.NoError(t, err, "store failure must not fail the overall read")
	assert.Empty(t, resp.Committees[0].SyncMode)
}

func TestListMailingLists_HydratesSyncModePerItem(t *testing.T) {
	ml1 := mlWith("committee-1")
	ml1.UID = "ml-1"
	ml2 := mlWith("committee-2")
	ml2.UID = "ml-2"
	reader := &stubMLReader{listMLs: []*model.GroupsIOMailingList{ml1, ml2}}
	store := mock.NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-1", "manual"))
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-2", "committee-2", "auto_ml_to_c"))

	o := newReaderOrchestrator(reader, store)
	items, total, err := o.ListMailingLists(context.Background(), "", "")
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, "manual", items[0].Committees[0].SyncMode)
	assert.Equal(t, "auto_ml_to_c", items[1].Committees[0].SyncMode)
}
