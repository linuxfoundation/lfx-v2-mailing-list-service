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

func TestHandleDataStreamServiceUpdate_MissingProjectID_ACK(t *testing.T) {
	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.False(t, nak, "missing project_id should ACK (not retry)")
}

func TestHandleDataStreamServiceUpdate_ProjectMappingAbsent_NAK(t *testing.T) {
	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{"project_id": "sfid-proj"},
		&mock.SpyMessagePublisher{}, mock.NewFakeMappingStore())
	assert.True(t, nak, "unknown project mapping should NAK for retry")
}

func TestHandleDataStreamServiceUpdate_HappyPath_ACKAndPublishes(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{
			"project_id":         "sfid-proj",
			"group_service_type": "mailing-list",
			"domain":             "example.com",
		},
		pub, m)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, constants.IndexGroupsIOServiceSubject, pub.IndexerCalls[0].Subject)
	assert.Len(t, pub.AccessCalls, 1)
	assert.Equal(t, constants.UpdateAccessGroupsIOServiceSubject, pub.AccessCalls[0].Subject)

	_, present := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService))
	assert.True(t, present, "mapping should be written after successful processing")
}

func TestHandleDataStreamServiceUpdate_CreateVsUpdate_Action(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")

	data := func() map[string]any { return map[string]any{"project_id": "sfid-proj"} }
	ctx := context.Background()
	mKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)

	assert.Equal(t, model.ActionCreated, m.ResolveAction(ctx, mKey))
	HandleDataStreamServiceUpdate(ctx, "svc-1", data(), &mock.SpyMessagePublisher{}, m)
	assert.Equal(t, model.ActionUpdated, m.ResolveAction(ctx, mKey))
}

func TestHandleDataStreamServiceDelete_DuplicateDelete_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	_ = m.PutTombstone(ctx, fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamServiceDelete(ctx, "svc-1", pub, m)

	assert.False(t, nak)
	assert.Empty(t, pub.IndexerCalls, "duplicate delete should not publish")
}

func TestHandleDataStreamServiceDelete_HappyPath_ACKAndTombstones(t *testing.T) {
	m := mock.NewFakeMappingStore()
	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamServiceDelete(context.Background(), "svc-1", pub, m)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, constants.IndexGroupsIOServiceSubject, pub.IndexerCalls[0].Subject)
	assert.Len(t, pub.AccessCalls, 1)
	assert.Equal(t, constants.DeleteAllAccessGroupsIOServiceSubject, pub.AccessCalls[0].Subject)

	assert.True(t, m.IsTombstoned(context.Background(),
		fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)))
}
