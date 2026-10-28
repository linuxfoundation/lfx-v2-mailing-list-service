// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newReaderOrchestrator(reader *stubMLReader) *GroupsIOMailingListReaderOrchestrator {
	return &GroupsIOMailingListReaderOrchestrator{
		reader:     reader,
		translator: &passthroughTranslator{},
	}
}

// TestGetMailingList_SyncModePassedThroughFromWire pins that sync_mode now comes straight from
// the ITX-proxied wire response (via converters.go's fromWireSubgroup), not a local store.
func TestGetMailingList_SyncModePassedThroughFromWire(t *testing.T) {
	ml := mlWith("committee-1")
	ml.UID = "ml-1"
	ml.Committees[0].SyncMode = "auto_c_to_ml"
	reader := &stubMLReader{ml: ml}

	o := newReaderOrchestrator(reader)
	resp, err := o.GetMailingList(context.Background(), "ml-1")
	require.NoError(t, err)
	require.Len(t, resp.Committees, 1)
	assert.Equal(t, "auto_c_to_ml", resp.Committees[0].SyncMode)
}

func TestGetMailingList_EmptySyncMode_LeftEmpty(t *testing.T) {
	ml := mlWith("committee-1")
	ml.UID = "ml-1"
	reader := &stubMLReader{ml: ml}

	o := newReaderOrchestrator(reader)
	resp, err := o.GetMailingList(context.Background(), "ml-1")
	require.NoError(t, err)
	require.Len(t, resp.Committees, 1)
	assert.Empty(t, resp.Committees[0].SyncMode)
}

func TestGetMailingList_NoCommittee_NoPanic(t *testing.T) {
	ml := &model.GroupsIOMailingList{UID: "ml-1"}
	reader := &stubMLReader{ml: ml}

	o := newReaderOrchestrator(reader)
	resp, err := o.GetMailingList(context.Background(), "ml-1")
	require.NoError(t, err)
	assert.Empty(t, resp.Committees)
}

func TestListMailingLists_SyncModePassedThroughPerItem(t *testing.T) {
	ml1 := mlWith("committee-1")
	ml1.UID = "ml-1"
	ml1.Committees[0].SyncMode = "manual"
	ml2 := mlWith("committee-2")
	ml2.UID = "ml-2"
	ml2.Committees[0].SyncMode = "auto_ml_to_c"
	reader := &stubMLReader{listMLs: []*model.GroupsIOMailingList{ml1, ml2}}

	o := newReaderOrchestrator(reader)
	items, total, err := o.ListMailingLists(context.Background(), "", "")
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, "manual", items[0].Committees[0].SyncMode)
	assert.Equal(t, "auto_ml_to_c", items[1].Committees[0].SyncMode)
}
