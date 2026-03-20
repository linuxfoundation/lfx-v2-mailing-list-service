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
)

// --- HandleDataStreamMemberUpdate ---

func TestHandleDataStreamMemberUpdate_MissingGroupID_ACK(t *testing.T) {
	nak := HandleDataStreamMemberUpdate(context.Background(), "mem-1",
		map[string]any{},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.False(t, nak, "missing group_id should ACK (malformed data, no retry)")
}

func TestHandleDataStreamMemberUpdate_ParentSubgroupAbsent_NAK(t *testing.T) {
	// group_id present but no subgroup mapping written yet
	nak := HandleDataStreamMemberUpdate(context.Background(), "mem-1",
		map[string]any{"group_id": float64(42)},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.True(t, nak, "absent subgroup mapping should NAK for retry")
}

func TestHandleDataStreamMemberUpdate_Tombstoned_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")
	_ = m.PutTombstone(ctx, fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberUpdate(ctx, "mem-1",
		map[string]any{"group_id": float64(42)},
		pub, m)

	assert.False(t, nak)
	assert.Empty(t, pub.IndexerCalls, "tombstoned member should not publish")
}

func TestHandleDataStreamMemberUpdate_HappyPath_ACKAndPublishesAndWritesMapping(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberUpdate(context.Background(), "mem-1",
		map[string]any{
			"group_id":  float64(42),
			"member_id": float64(99),
			"email":     "alice@example.com",
			"full_name": "Alice Smith",
		},
		pub, m)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, constants.IndexGroupsIOMemberSubject, pub.IndexerCalls[0].Subject)
	assert.Empty(t, pub.AccessCalls, "member access is inherited — no access message expected")

	_, present := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember))
	assert.True(t, present, "forward mapping should be written after successful processing")
}

func TestHandleDataStreamMemberUpdate_CreateVsUpdate_Action(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID), "sg-1")

	data := func() map[string]any { return map[string]any{"group_id": float64(42)} }
	ctx := context.Background()
	mKey := fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)

	assert.Equal(t, model.ActionCreated, m.ResolveAction(ctx, mKey))
	HandleDataStreamMemberUpdate(ctx, "mem-1", data(), &mock.SpyMessagePublisher{}, m)
	assert.Equal(t, model.ActionUpdated, m.ResolveAction(ctx, mKey))
}

// --- HandleDataStreamMemberDelete ---

func TestHandleDataStreamMemberDelete_DuplicateDelete_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	_ = m.PutTombstone(ctx, fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberDelete(ctx, "mem-1", pub, m)

	assert.False(t, nak)
	assert.Empty(t, pub.IndexerCalls, "duplicate delete should not publish")
}

func TestHandleDataStreamMemberDelete_NeverIndexed_TombstonesWithoutPublishing(t *testing.T) {
	m := mock.NewFakeMappingStore()
	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberDelete(context.Background(), "mem-1", pub, m)

	assert.False(t, nak)
	assert.Empty(t, pub.IndexerCalls, "never-indexed member should not publish indexer message")
	assert.True(t, m.IsTombstoned(context.Background(),
		fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)),
		"should still tombstone to prevent future re-processing")
}

func TestHandleDataStreamMemberDelete_HappyPath_ACKAndTombstones(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	mKey := fmt.Sprintf("%s.mem-1", constants.KVMappingPrefixMember)
	_ = m.PutMapping(ctx, mKey, "mem-1")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamMemberDelete(ctx, "mem-1", pub, m)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, constants.IndexGroupsIOMemberSubject, pub.IndexerCalls[0].Subject)
	assert.Empty(t, pub.AccessCalls, "member delete should not publish access message")

	assert.True(t, m.IsTombstoned(ctx, mKey))
}
