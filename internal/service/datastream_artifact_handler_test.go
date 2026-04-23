// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
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
