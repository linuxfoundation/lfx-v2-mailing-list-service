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

func TestHandleDataStreamMessageUpdate_MissingGroupID_ACK(t *testing.T) {
	nak := HandleDataStreamMessageUpdate(context.Background(), "msg-1",
		map[string]any{},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.False(t, nak, "missing group_id should ACK (malformed data, no retry)")
}

func TestHandleDataStreamMessageUpdate_ParentSubgroupAbsent_NAK(t *testing.T) {
	nak := HandleDataStreamMessageUpdate(context.Background(), "msg-1",
		map[string]any{"group_id": float64(42)},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.True(t, nak, "absent subgroup mapping should NAK for retry")
}

func TestHandleDataStreamMessageUpdate_NoCommittee_PublishesWithoutCommitteeTag(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "ml-uid-123")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMessageUpdate(context.Background(), "msg-1",
		map[string]any{"group_id": float64(42), "subject": "Hello world"},
		pub, m)

	assert.False(t, nak)
	require.Len(t, pub.IndexerCalls, 1)

	msg, ok := pub.IndexerCalls[0].Message.(*model.IndexerMessage)
	require.True(t, ok)
	require.NotNil(t, msg.IndexingConfig)

	assert.Equal(t, "groupsio_mailing_list:ml-uid-123", msg.IndexingConfig.AccessCheckObject)
	assert.Equal(t, "groupsio_mailing_list:ml-uid-123", msg.IndexingConfig.HistoryCheckObject)

	// No committee tag when list has no committee association.
	for _, tag := range msg.IndexingConfig.Tags {
		assert.NotContains(t, tag, "committee:", "should not have committee tag when no committee mapping")
	}
}

func TestHandleDataStreamMessageUpdate_WithCommittee_IncludesCommitteeTag(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "ml-uid-123")
	m.Set(fmt.Sprintf("%s.ml-uid-123", constants.KVMappingPrefixSubgroupCommittee), "committee-uid-456|false")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMessageUpdate(context.Background(), "msg-1",
		map[string]any{"group_id": float64(42), "subject": "Meeting notes"},
		pub, m)

	assert.False(t, nak)
	require.Len(t, pub.IndexerCalls, 1)

	msg, ok := pub.IndexerCalls[0].Message.(*model.IndexerMessage)
	require.True(t, ok)
	require.NotNil(t, msg.IndexingConfig)

	assert.Contains(t, msg.IndexingConfig.Tags, "committee:committee-uid-456")
	assert.Contains(t, msg.IndexingConfig.Tags, fmt.Sprintf("mailing_list:%s", "ml-uid-123"))
}

func TestHandleDataStreamMessageUpdate_AccessCheckUsesMailingList(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.99", constants.KVMappingPrefixSubgroupByGroupID), "ml-uid-999")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMessageUpdate(context.Background(), "msg-99",
		map[string]any{"group_id": float64(99)},
		pub, m)

	assert.False(t, nak)
	require.Len(t, pub.IndexerCalls, 1)

	msg, ok := pub.IndexerCalls[0].Message.(*model.IndexerMessage)
	require.True(t, ok)
	require.NotNil(t, msg.IndexingConfig)

	assert.Equal(t, "groupsio_mailing_list:ml-uid-999", msg.IndexingConfig.AccessCheckObject,
		"access check must reference the parent mailing list")
	assert.Equal(t, "groupsio_mailing_list:ml-uid-999", msg.IndexingConfig.HistoryCheckObject,
		"history check must reference the parent mailing list")
}

func TestHandleDataStreamMessageDelete_SendsIndexingConfig(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.msg-1", constants.KVMappingPrefixMessage), "msg-1")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMessageDelete(context.Background(), "msg-1", pub, m)

	assert.False(t, nak)
	require.Len(t, pub.IndexerCalls, 1)

	msg, ok := pub.IndexerCalls[0].Message.(*model.IndexerMessage)
	require.True(t, ok)
	require.NotNil(t, msg.IndexingConfig, "delete must include IndexingConfig")
	assert.Equal(t, "msg-1", msg.IndexingConfig.ObjectID)
	assert.NotEmpty(t, msg.IndexingConfig.AccessCheckObject)
	assert.NotEmpty(t, msg.IndexingConfig.AccessCheckRelation)
}

func TestHandleDataStreamMessageDelete_NeverIndexed_ACK(t *testing.T) {
	nak := HandleDataStreamMessageDelete(context.Background(), "msg-missing",
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.False(t, nak, "message never indexed should ACK without publishing")
}

func TestHandleDataStreamMessageDelete_AlreadyTombstoned_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	_ = m.PutTombstone(context.Background(), fmt.Sprintf("%s.msg-1", constants.KVMappingPrefixMessage))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMessageDelete(context.Background(), "msg-1", pub, m)

	assert.False(t, nak)
	assert.Empty(t, pub.IndexerCalls, "duplicate delete should ACK without publishing")
}
