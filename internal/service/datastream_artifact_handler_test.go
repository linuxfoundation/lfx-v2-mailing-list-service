// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	pkgerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleDataStreamArtifactUpdate_MissingGroupID_ACK(t *testing.T) {
	nak := HandleDataStreamArtifactUpdate(context.Background(), "art-1",
		map[string]any{},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.False(t, nak, "missing group_id should ACK (malformed data, no retry)")
}

func TestHandleDataStreamArtifactUpdate_ParentSubgroupAbsent_NAK(t *testing.T) {
	nak := HandleDataStreamArtifactUpdate(context.Background(), "art-1",
		map[string]any{"group_id": float64(42)},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.True(t, nak, "absent subgroup mapping should NAK for retry")
}

func TestHandleDataStreamArtifactUpdate_AccessCheckUsesMailingList(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "ml-uid-123")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamArtifactUpdate(context.Background(), "art-1",
		map[string]any{"group_id": float64(42)},
		pub, m)

	assert.False(t, nak)
	require.Len(t, pub.IndexerCalls, 1)

	msg, ok := pub.IndexerCalls[0].Message.(*model.IndexerMessage)
	require.True(t, ok, "published message should be *model.IndexerMessage")
	require.NotNil(t, msg.IndexingConfig)

	assert.Equal(t, "groupsio_mailing_list:ml-uid-123", msg.IndexingConfig.AccessCheckObject,
		"access_check_object must reference the parent mailing list, not groupsio_artifact")
	assert.Equal(t, "groupsio_mailing_list:ml-uid-123", msg.IndexingConfig.HistoryCheckObject,
		"history_check_object must reference the parent mailing list, not groupsio_artifact")
}

func TestHandleDataStreamArtifactDelete_SendsIndexingConfig(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.art-1", constants.KVMappingPrefixArtifact), "art-1")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamArtifactDelete(context.Background(), "art-1", pub, m)

	assert.False(t, nak)
	require.Len(t, pub.IndexerCalls, 1)

	msg, ok := pub.IndexerCalls[0].Message.(*model.IndexerMessage)
	require.True(t, ok)
	require.NotNil(t, msg.IndexingConfig, "delete must include IndexingConfig so the indexer skips ValidateObjectType")
	assert.Equal(t, "art-1", msg.IndexingConfig.ObjectID)
	assert.NotEmpty(t, msg.IndexingConfig.AccessCheckObject)
	assert.NotEmpty(t, msg.IndexingConfig.AccessCheckRelation)
	assert.NotEmpty(t, msg.IndexingConfig.HistoryCheckObject)
	assert.NotEmpty(t, msg.IndexingConfig.HistoryCheckRelation)
}

func TestHandleDataStreamArtifactDelete_NeverIndexed_ACK(t *testing.T) {
	nak := HandleDataStreamArtifactDelete(context.Background(), "art-missing",
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.False(t, nak, "artifact never indexed should ACK without publishing")
}

func TestHandleDataStreamArtifactDelete_AlreadyTombstoned_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	_ = m.PutTombstone(context.Background(), fmt.Sprintf("%s.art-1", constants.KVMappingPrefixArtifact))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamArtifactDelete(context.Background(), "art-1", pub, m)

	assert.False(t, nak)
	assert.Empty(t, pub.IndexerCalls, "duplicate delete should ACK without publishing")
}

func TestHandleDataStreamArtifactUpdate_PutMappingFailure_Transient_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "ml-uid-1")
	mKey := fmt.Sprintf("%s.art-1", constants.KVMappingPrefixArtifact)
	m.SimulatePutError(mKey, errors.New("connection timeout"))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamArtifactUpdate(context.Background(), "art-1",
		map[string]any{"group_id": float64(42)},
		pub, m)

	assert.True(t, nak, "transient PutMapping failure should NAK for retry")
	assert.Len(t, pub.IndexerCalls, 1, "indexer message was published before mapping write failed")
}

func TestHandleDataStreamArtifactUpdate_PutMappingFailure_Permanent_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "ml-uid-1")
	mKey := fmt.Sprintf("%s.art-1", constants.KVMappingPrefixArtifact)
	m.SimulatePutError(mKey, pkgerrors.MarkPermanent(errors.New("internal server error")))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamArtifactUpdate(context.Background(), "art-1",
		map[string]any{"group_id": float64(42)},
		pub, m)

	assert.False(t, nak, "permanent PutMapping failure should ACK (not retry)")
}

func TestHandleDataStreamArtifactUpdate_PutMappingFailure_Unclassified_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "ml-uid-1")
	m.SimulatePutError(fmt.Sprintf("%s.art-1", constants.KVMappingPrefixArtifact), errors.New("internal server error"))

	nak := HandleDataStreamArtifactUpdate(context.Background(), "art-1",
		map[string]any{"group_id": float64(42)}, &mock.SpyMessagePublisher{}, m)

	assert.True(t, nak, "an unclassified PutMapping failure should NAK rather than drop the event")
}

func TestHandleDataStreamArtifactUpdate_ReadError_NAK(t *testing.T) {
	for _, key := range []string{
		fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID),
		fmt.Sprintf("%s.art-1", constants.KVMappingPrefixArtifact),
	} {
		t.Run(key, func(t *testing.T) {
			m := mock.NewFakeMappingStore()
			m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "ml-uid-1")
			m.SimulateGetError(key)
			pub := &mock.SpyMessagePublisher{}

			nak := HandleDataStreamArtifactUpdate(context.Background(), "art-1",
				map[string]any{"group_id": float64(42)}, pub, m)

			assert.True(t, nak)
			assert.Empty(t, pub.IndexerCalls)
		})
	}
}

func TestHandleDataStreamArtifactDelete_ReadError_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	mKey := fmt.Sprintf("%s.art-1", constants.KVMappingPrefixArtifact)
	m.Set(mKey, "art-1")
	m.SimulateGetError(mKey)
	pub := &mock.SpyMessagePublisher{}

	nak := HandleDataStreamArtifactDelete(context.Background(), "art-1", pub, m)

	assert.True(t, nak)
	assert.Empty(t, pub.IndexerCalls, "no delete may be published when the prior state is unknown")
}

func TestHandleDataStreamArtifactDelete_TombstoneFailure_NAK(t *testing.T) {
	mKey := fmt.Sprintf("%s.art-1", constants.KVMappingPrefixArtifact)
	tests := []struct {
		name  string
		setup func(m *mock.FakeMappingStore)
	}{
		{name: "after a published delete", setup: func(m *mock.FakeMappingStore) { m.Set(mKey, "art-1") }},
		{name: "never indexed", setup: func(*mock.FakeMappingStore) {}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mock.NewFakeMappingStore()
			tt.setup(m)
			m.SimulateTombstoneError(mKey, errors.New("boom"))

			nak := HandleDataStreamArtifactDelete(context.Background(), "art-1", &mock.SpyMessagePublisher{}, m)

			assert.True(t, nak, "an unwritten tombstone must be retried so a replayed PUT cannot re-index the artifact")
		})
	}
}
