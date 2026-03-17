// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/stretchr/testify/assert"
)

func TestHandleDataStreamSubgroupUpdate_MissingProjectID_ACK(t *testing.T) {
	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.False(t, nak, "missing project_id should ACK")
}

func TestHandleDataStreamSubgroupUpdate_ProjectMappingAbsent_NAK(t *testing.T) {
	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{"project_id": "sfid-proj"},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.True(t, nak, "unknown project mapping should NAK")
}

func TestHandleDataStreamSubgroupUpdate_CommitteeMappingAbsent_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")

	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
			"committee":  "sfid-committee", // mapping absent
		},
		&mock.SpyMessagePublisher{}, m)
	assert.True(t, nak, "unknown committee mapping should NAK")
}

func TestHandleDataStreamSubgroupUpdate_ParentServiceAbsent_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	// service mapping deliberately absent

	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
		},
		&mock.SpyMessagePublisher{}, m)
	assert.True(t, nak, "absent parent service should NAK")
}

func TestHandleDataStreamSubgroupUpdate_HappyPath_ACKAndPublishesAndWritesMappings(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
			"group_id":   float64(42),
			"group_name": "dev",
		},
		pub, m)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, constants.IndexGroupsIOMailingListSubject, pub.IndexerCalls[0].Subject)
	assert.Len(t, pub.AccessCalls, 1)
	assert.Equal(t, constants.UpdateAccessGroupsIOMailingListSubject, pub.AccessCalls[0].Subject)

	_, present := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.sg-1", constants.KVMappingPrefixSubgroup))
	assert.True(t, present, "forward mapping should be written")

	rev, ok := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.42", constants.KVMappingPrefixSubgroupByGroupID))
	assert.True(t, ok, "reverse group_id index should be written")
	assert.Equal(t, "sg-1", rev)
}

func TestHandleDataStreamSubgroupUpdate_WithCommittee_ResolvesAndPublishes(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.sfid-committee", constants.KVMappingPrefixCommitteeBySFID), "committee-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{
			"project_id": "sfid-proj",
			"parent_id":  "svc-1",
			"committee":  "sfid-committee",
		},
		pub, m)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
}

func TestHandleDataStreamSubgroupUpdate_NoGroupID_NoReverseIndex(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	m.Set(fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService), "svc-1")

	HandleDataStreamSubgroupUpdate(context.Background(), "sg-1",
		map[string]any{"project_id": "sfid-proj", "parent_id": "svc-1"},
		&mock.SpyMessagePublisher{}, m)

	_, ok := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.0", constants.KVMappingPrefixSubgroupByGroupID))
	assert.False(t, ok, "should not write reverse index when group_id is absent")
}

func TestHandleDataStreamSubgroupDelete_DuplicateDelete_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	_ = m.PutTombstone(ctx, fmt.Sprintf("%s.sg-1", constants.KVMappingPrefixSubgroup))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamSubgroupDelete(ctx, "sg-1", pub, m)

	assert.False(t, nak)
	assert.Empty(t, pub.IndexerCalls, "duplicate delete should not publish")
}

func TestHandleDataStreamSubgroupDelete_HappyPath_ACKAndTombstones(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sg-1", constants.KVMappingPrefixSubgroup), "sg-1")
	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamSubgroupDelete(context.Background(), "sg-1", pub, m)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, constants.IndexGroupsIOMailingListSubject, pub.IndexerCalls[0].Subject)
	assert.Len(t, pub.AccessCalls, 1)
	assert.Equal(t, constants.DeleteAllAccessGroupsIOMailingListSubject, pub.AccessCalls[0].Subject)

	assert.True(t, m.IsTombstoned(context.Background(),
		fmt.Sprintf("%s.sg-1", constants.KVMappingPrefixSubgroup)))
}
