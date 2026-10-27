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

func TestHandleDataStreamMessageUpdate_PublishesWithMailingListTagAndNoCommitteeFields(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "ml-uid-123")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMessageUpdate(context.Background(), "msg-1",
		map[string]any{"group_id": float64(42), "subject": "Hello world"},
		pub, m)

	assert.False(t, nak, "no subgroup committee mapping is required to index a message")
	require.Len(t, pub.IndexerCalls, 1)

	msg, ok := pub.IndexerCalls[0].Message.(*model.IndexerMessage)
	require.True(t, ok)
	require.NotNil(t, msg.IndexingConfig)

	assert.Contains(t, msg.IndexingConfig.Tags, "mailing_list:ml-uid-123")
	assert.Contains(t, msg.IndexingConfig.ParentRefs, "groupsio_mailing_list:ml-uid-123")
	for _, tag := range msg.IndexingConfig.Tags {
		assert.NotContains(t, tag, "committee", "committee is resolved from the mailing list, not stored on messages")
	}
	for _, ref := range msg.IndexingConfig.ParentRefs {
		assert.NotContains(t, ref, "committee:", "messages must not reference a committee")
	}

	data, ok := msg.Data.(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, data, "committee_uid")
	assert.NotContains(t, data, "is_private")
	assert.Equal(t, "ml-uid-123", data["mailing_list_uid"])
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

func TestHandleDataStreamMessageUpdate_ReadError_NAK(t *testing.T) {
	for _, key := range []string{
		fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID),
		fmt.Sprintf("%s.msg-1", constants.KVMappingPrefixMessage),
	} {
		t.Run(key, func(t *testing.T) {
			m := mock.NewFakeMappingStore()
			m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "ml-uid-1")
			m.SimulateGetError(key)
			pub := &mock.SpyMessagePublisher{}

			nak := HandleDataStreamMessageUpdate(context.Background(), "msg-1",
				map[string]any{"group_id": float64(42)}, pub, m)

			assert.True(t, nak)
			assert.Empty(t, pub.IndexerCalls)
		})
	}
}

func TestHandleDataStreamMessageUpdate_PutMappingFailure_PermanentACKsUnknownNAKs(t *testing.T) {
	tests := []struct {
		name    string
		putErr  error
		wantNAK bool
	}{
		{name: "unclassified error retries", putErr: errors.New("boom"), wantNAK: true},
		{name: "permanent error is dropped", putErr: pkgerrors.MarkPermanent(errors.New("boom")), wantNAK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mock.NewFakeMappingStore()
			m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "ml-uid-1")
			m.SimulatePutError(fmt.Sprintf("%s.msg-1", constants.KVMappingPrefixMessage), tt.putErr)

			nak := HandleDataStreamMessageUpdate(context.Background(), "msg-1",
				map[string]any{"group_id": float64(42)}, &mock.SpyMessagePublisher{}, m)

			assert.Equal(t, tt.wantNAK, nak)
		})
	}
}

func TestHandleDataStreamMessageDelete_ReadError_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	mKey := fmt.Sprintf("%s.msg-1", constants.KVMappingPrefixMessage)
	m.Set(mKey, "msg-1")
	m.SimulateGetError(mKey)
	pub := &mock.SpyMessagePublisher{}

	nak := HandleDataStreamMessageDelete(context.Background(), "msg-1", pub, m)

	assert.True(t, nak)
	assert.Empty(t, pub.IndexerCalls, "no delete may be published when the prior state is unknown")
}
