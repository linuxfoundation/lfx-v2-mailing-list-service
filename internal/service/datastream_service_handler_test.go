// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	fgaconstants "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
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
	assert.Equal(t, fgaconstants.GenericUpdateAccessSubject, pub.AccessCalls[0].Subject)

	_, present := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService))
	assert.True(t, present, "mapping should be written after successful processing")

	domain, present := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain))
	assert.True(t, present, "service domain mapping should be written")
	assert.Equal(t, "example.com", domain)
}

func TestHandleDataStreamServiceUpdate_MissingDomain_StoresEmptyMapping(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")

	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{"project_id": "sfid-proj"}, &mock.SpyMessagePublisher{}, m)

	assert.False(t, nak)
	domain, present := m.GetMappingValue(context.Background(),
		fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain))
	assert.True(t, present)
	assert.Empty(t, domain, "empty service domain should remain empty in the mapping")
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

func TestHandleDataStreamServiceDelete_DuplicateRepairsDomainTombstone(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	serviceKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)
	domainKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
	_ = m.PutTombstone(ctx, serviceKey)
	m.Set(domainKey, "stale.example.com")
	m.SimulateTombstoneError(domainKey, errors.New("connection timeout"))

	pub := &mock.SpyMessagePublisher{}
	assert.True(t, HandleDataStreamServiceDelete(ctx, "svc-1", pub, m), "failed domain tombstone should NAK")
	assert.Empty(t, pub.IndexerCalls, "duplicate delete should not republish")

	m.SimulateTombstoneError(domainKey, nil)
	assert.False(t, HandleDataStreamServiceDelete(ctx, "svc-1", pub, m))
	assert.True(t, m.IsTombstoned(ctx, domainKey), "duplicate delivery should repair domain tombstone")
}

func TestHandleDataStreamServiceDelete_DuplicateTombstoneFailure_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	ctx := context.Background()
	serviceKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)
	domainKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
	_ = m.PutTombstone(ctx, serviceKey)
	m.SimulateTombstoneError(domainKey, errors.New("invalid mapping key"))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamServiceDelete(ctx, "svc-1", pub, m)

	assert.True(t, nak, "any duplicate domain tombstone failure should NAK")
	assert.Empty(t, pub.IndexerCalls)
}

func TestHandleDataStreamServiceDelete_TombstoneFailures_NAK(t *testing.T) {
	for _, keyType := range []string{"domain", "service"} {
		t.Run(keyType, func(t *testing.T) {
			m := mock.NewFakeMappingStore()
			key := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
			if keyType == "service" {
				key = fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)
			}
			m.SimulateTombstoneError(key, errors.New("connection timeout"))

			nak := HandleDataStreamServiceDelete(context.Background(), "svc-1", &mock.SpyMessagePublisher{}, m)

			assert.True(t, nak, "transient tombstone failures should NAK")
		})
	}
}

func TestHandleDataStreamServiceDelete_AttemptsBothTombstonesOnFailure(t *testing.T) {
	m := mock.NewFakeMappingStore()
	domainKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
	serviceKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)
	m.SimulateTombstoneError(domainKey, errors.New("invalid domain mapping key"))

	assert.True(t, HandleDataStreamServiceDelete(context.Background(), "svc-1", &mock.SpyMessagePublisher{}, m))
	assert.True(t, m.IsTombstoned(context.Background(), serviceKey), "service tombstone is attempted despite domain failure")
}

func TestHandleDataStreamServiceDelete_HappyPath_ACKAndTombstones(t *testing.T) {
	m := mock.NewFakeMappingStore()
	domainKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
	m.Set(domainKey, "example.com")
	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamServiceDelete(context.Background(), "svc-1", pub, m)

	assert.False(t, nak)
	assert.Len(t, pub.IndexerCalls, 1)
	assert.Equal(t, constants.IndexGroupsIOServiceSubject, pub.IndexerCalls[0].Subject)
	assert.Len(t, pub.AccessCalls, 1)
	assert.Equal(t, fgaconstants.GenericDeleteAccessSubject, pub.AccessCalls[0].Subject)

	assert.True(t, m.IsTombstoned(context.Background(),
		fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)))
	assert.True(t, m.IsTombstoned(context.Background(), domainKey))
}

func TestHandleDataStreamServiceUpdate_PutMappingFailure_Transient_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	mKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)
	m.SimulatePutError(mKey, errors.New("connection timeout"))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{"project_id": "sfid-proj"},
		pub, m)

	assert.True(t, nak, "transient PutMapping failure should NAK for retry")
	assert.Len(t, pub.IndexerCalls, 1, "indexer message was published before mapping write failed")
	assert.Len(t, pub.AccessCalls, 1, "access message was published before mapping write failed")
}

func TestHandleDataStreamServiceUpdate_PutMappingFailure_Permanent_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	mKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)
	m.SimulatePutError(mKey, errors.New("internal server error"))

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{"project_id": "sfid-proj"},
		pub, m)

	assert.False(t, nak, "permanent PutMapping failure should ACK (not retry)")
}

func TestHandleDataStreamServiceUpdate_DomainMappingFailure_Permanent_ACK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	domainKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
	m.SimulatePutError(domainKey, jetstream.ErrInvalidKey)

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{"project_id": "sfid-proj", "domain": "example.com"},
		pub, m)

	assert.False(t, nak, "a verified permanent error with no stale mapping should ACK")
	assert.True(t, m.IsMappingPresent(context.Background(), fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)))
}

func TestHandleDataStreamServiceUpdate_DomainMappingFailure_StaleDomainCleared(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	domainKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
	m.Set(domainKey, "old.example.org")
	m.SimulatePutError(domainKey, jetstream.ErrInvalidKey)

	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{"project_id": "sfid-proj", "domain": "new.example.org"},
		&mock.SpyMessagePublisher{}, m)

	assert.False(t, nak)
	_, present := m.GetMappingValue(context.Background(), domainKey)
	assert.False(t, present, "stale domain mapping should be tombstoned")
	assert.True(t, m.IsMappingPresent(context.Background(), fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)))
}

func TestHandleDataStreamServiceUpdate_DomainMappingFailure_UnverifiedDomain_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	domainKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
	m.Set(domainKey, "old.example.org")
	m.SimulatePutError(domainKey, jetstream.ErrInvalidKey)
	m.SimulateTombstoneError(domainKey, errors.New("connection timeout"))

	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{"project_id": "sfid-proj", "domain": "new.example.org"},
		&mock.SpyMessagePublisher{}, m)

	assert.True(t, nak, "failed stale-domain cleanup must retry")
	assert.False(t, m.IsMappingPresent(context.Background(), fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)))
}

func TestHandleDataStreamServiceUpdate_DomainMappingFailure_Unknown_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	domainKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
	m.SimulatePutError(domainKey, errors.New("unexpected write error"))

	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{"project_id": "sfid-proj", "domain": "new.example.org"},
		&mock.SpyMessagePublisher{}, m)

	assert.True(t, nak, "unclassified errors should not be ACKed as permanent")
}

func TestHandleDataStreamServiceUpdate_DomainMappingFailure_TransientClearsStaleDomain(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	domainKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
	m.Set(domainKey, "old.example.org")
	m.SimulatePutError(domainKey, errors.New("connection timeout"))

	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{"project_id": "sfid-proj", "domain": "new.example.org"},
		&mock.SpyMessagePublisher{}, m)

	assert.True(t, nak)
	_, present := m.GetMappingValue(context.Background(), domainKey)
	assert.False(t, present, "stale domain should be cleared before retrying the write")
	assert.False(t, m.IsMappingPresent(context.Background(), fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)))
}

func TestHandleDataStreamServiceUpdate_DomainMappingFailure_ReadErrorHidesService(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	serviceKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixService)
	domainKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
	m.Set(serviceKey, "svc-1")
	m.Set(domainKey, "old.example.org")
	m.SimulatePutError(domainKey, jetstream.ErrInvalidKey)
	m.SimulateGetError(domainKey)

	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{"project_id": "sfid-proj", "domain": "new.example.org"},
		&mock.SpyMessagePublisher{}, m)

	assert.True(t, nak)
	assert.True(t, m.IsTombstoned(context.Background(), serviceKey), "unverified domain must not remain visible to subgroups")
}
