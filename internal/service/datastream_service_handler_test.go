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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSubgroupObjects map[string]map[string]any

func (f fakeSubgroupObjects) GetService(_ context.Context, uid string) (map[string]any, bool, error) {
	data, ok := f["service:"+uid]
	if !ok {
		return nil, false, errors.New("service source unavailable")
	}
	return cloneData(data), true, nil
}

func cloneData(data map[string]any) map[string]any {
	copy := make(map[string]any, len(data))
	for key, value := range data {
		copy[key] = value
	}
	return copy
}

type failingSubgroupObjects struct{ err error }

func (f failingSubgroupObjects) GetSubgroup(context.Context, string) (map[string]any, bool, error) {
	return nil, false, f.err
}

func (f failingSubgroupObjects) GetService(context.Context, string) (map[string]any, bool, error) {
	return map[string]any{"project_id": "project-sfid", "domain": "new.example.test"}, true, nil
}

type failingListPublisher struct {
	*mock.SpyMessagePublisher
	fail bool
}

var _ port.MessagePublisher = (*failingListPublisher)(nil)

func (p *failingListPublisher) Indexer(ctx context.Context, subject string, message any) error {
	if subject == constants.IndexGroupsIOMailingListSubject && p.fail {
		return errors.New("connection unavailable")
	}
	return p.SpyMessagePublisher.Indexer(ctx, subject, message)
}

func (f fakeSubgroupObjects) GetSubgroup(_ context.Context, uid string) (map[string]any, bool, error) {
	data, ok := f[uid]
	if !ok {
		return nil, false, nil
	}
	return cloneData(data), true, nil
}

func TestServiceDomainChange_ReindexesAssociatedLists(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixServiceDomain+".svc-1", "old.example.test")
	m.Set(constants.KVMappingPrefixServiceDomainIndexed+".svc-1", "old.example.test")
	for _, uid := range []string{"ml-1", "ml-2"} {
		m.Set(constants.KVMappingPrefixSubgroup+"."+uid, uid)
		m.Set(constants.KVMappingPrefixSubgroupParent+"."+uid, "svc-1")
		m.Set(constants.KVMappingPrefixSubgroupByService+".svc-1."+uid, uid)
	}
	m.Set(constants.KVMappingPrefixSubgroup+".other-ml", "other-ml")
	m.Set(constants.KVMappingPrefixSubgroupParent+".other-ml", "svc-2")
	m.Set(constants.KVMappingPrefixSubgroupByService+".svc-2.other-ml", "other-ml")
	objects := fakeSubgroupObjects{
		"service:svc-1": {"project_id": "project-sfid", "domain": "new.example.test"},
		"ml-1":          {"project_id": "project-sfid", "parent_id": "svc-1", "title": "List One", "visibility": "public"},
		"ml-2":          {"project_id": "project-sfid", "parent_id": "svc-1", "title": "List Two"},
		"other-ml":      {"project_id": "project-sfid", "parent_id": "svc-2", "title": "Other Service"},
	}
	pub := &mock.SpyMessagePublisher{}
	data := func(domain string) map[string]any {
		return map[string]any{"project_id": "project-sfid", "domain": domain}
	}
	require.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", data("new.example.test"), pub, m, objects))
	var listMessages []*model.IndexerMessage
	for _, call := range pub.IndexerCalls {
		if call.Subject == constants.IndexGroupsIOMailingListSubject {
			listMessages = append(listMessages, call.Message.(*model.IndexerMessage))
		}
	}
	require.Len(t, listMessages, 2)
	for _, msg := range listMessages {
		assert.Equal(t, model.ActionUpdated, msg.Action)
		assert.Equal(t, "new.example.test", msg.Data.(map[string]any)["domain"])
		assert.Equal(t, "svc-1", msg.Data.(map[string]any)["service_uid"])
		require.NotNil(t, msg.IndexingConfig)
		assert.Contains(t, msg.IndexingConfig.ParentRefs, "groupsio_service:svc-1")
		assert.NotEqual(t, "other-ml", msg.Data.(map[string]any)["uid"])
	}
	checkpoint, _ := m.GetMappingValue(ctx, constants.KVMappingPrefixServiceDomainIndexed+".svc-1")
	assert.Equal(t, "new.example.test", checkpoint)
	pub.IndexerCalls = nil
	require.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", data("new.example.test"), pub, m, objects))
	assert.Len(t, pub.IndexerCalls, 1, "unchanged domain should not republish lists")
}

func TestServiceDomainChange_NoListsAndClearedDomain(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixServiceDomain+".svc-1", "old.example.test")
	m.Set(constants.KVMappingPrefixServiceDomainIndexed+".svc-1", "old.example.test")
	pub := &mock.SpyMessagePublisher{}
	objects := fakeSubgroupObjects{"service:svc-1": {"project_id": "project-sfid"}}
	require.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", map[string]any{"project_id": "project-sfid"}, pub, m, objects))
	assert.Len(t, pub.IndexerCalls, 1)
	m.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
	m.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-1")
	m.Set(constants.KVMappingPrefixSubgroupByService+".svc-1.ml-1", "ml-1")
	m.Set(constants.KVMappingPrefixServiceDomainIndexed+".svc-1", "old.example.test")
	objects["ml-1"] = map[string]any{"project_id": "project-sfid", "parent_id": "svc-1"}
	require.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", map[string]any{"project_id": "project-sfid"}, pub, m, objects))
	listMsg := pub.IndexerCalls[len(pub.IndexerCalls)-1].Message.(*model.IndexerMessage)
	assert.NotContains(t, listMsg.Data.(map[string]any), "domain")
}

func TestServiceDomainChange_PublishFailureRetriesOnRedelivery(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixServiceDomain+".svc-1", "old.example.test")
	m.Set(constants.KVMappingPrefixServiceDomainIndexed+".svc-1", "old.example.test")
	m.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
	m.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-1")
	m.Set(constants.KVMappingPrefixSubgroupByService+".svc-1.ml-1", "ml-1")
	objects := fakeSubgroupObjects{"service:svc-1": {"project_id": "project-sfid", "domain": "new.example.test"}, "ml-1": {"project_id": "project-sfid", "parent_id": "svc-1"}}
	pub := &failingListPublisher{SpyMessagePublisher: &mock.SpyMessagePublisher{}, fail: true}
	data := func() map[string]any {
		return map[string]any{"project_id": "project-sfid", "domain": "new.example.test"}
	}
	assert.True(t, HandleDataStreamServiceUpdate(ctx, "svc-1", data(), pub, m, objects))
	_, present := m.GetMappingValue(ctx, constants.KVMappingPrefixServiceDomainIndexed+".svc-1")
	assert.False(t, present, "partial propagation must invalidate the completed checkpoint")
	pub.fail = false
	assert.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", data(), pub, m, objects))
	checkpoint, _ := m.GetMappingValue(ctx, constants.KVMappingPrefixServiceDomainIndexed+".svc-1")
	assert.Equal(t, "new.example.test", checkpoint)
}

func TestServiceDomainChange_LegacyServiceWithoutCheckpointRefreshesLists(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixServiceDomain+".svc-1", "old.example.test")
	m.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
	m.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-1")
	m.Set(constants.KVMappingPrefixSubgroupByService+".svc-1.ml-1", "ml-1")
	pub := &mock.SpyMessagePublisher{}
	objects := fakeSubgroupObjects{"service:svc-1": {"project_id": "project-sfid", "domain": "old.example.test"}, "ml-1": {"project_id": "project-sfid", "parent_id": "svc-1"}}
	require.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", map[string]any{"project_id": "project-sfid", "domain": "old.example.test"}, pub, m, objects))
	assert.Len(t, pub.IndexerCalls, 2, "existing service without a checkpoint must refresh lists once")
}

func TestServiceDomainChange_SourceReadFailureRetainsCheckpoint(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixServiceDomain+".svc-1", "old.example.test")
	m.Set(constants.KVMappingPrefixServiceDomainIndexed+".svc-1", "old.example.test")
	m.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
	m.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-1")
	m.Set(constants.KVMappingPrefixSubgroupByService+".svc-1.ml-1", "ml-1")
	data := func() map[string]any {
		return map[string]any{"project_id": "project-sfid", "domain": "new.example.test"}
	}
	pub := &mock.SpyMessagePublisher{}
	assert.True(t, HandleDataStreamServiceUpdate(ctx, "svc-1", data(), pub, m, failingSubgroupObjects{err: errors.New("connection unavailable")}))
	_, present := m.GetMappingValue(ctx, constants.KVMappingPrefixServiceDomainIndexed+".svc-1")
	assert.False(t, present, "source read failure must leave propagation incomplete")
	objects := fakeSubgroupObjects{"service:svc-1": data(), "ml-1": {"project_id": "project-sfid", "parent_id": "svc-1"}}
	assert.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", data(), pub, m, objects))
	checkpoint, _ := m.GetMappingValue(ctx, constants.KVMappingPrefixServiceDomainIndexed+".svc-1")
	assert.Equal(t, "new.example.test", checkpoint)
}

func TestServiceDomainChange_MissingCheckpointRefreshesEvenWithCreatedAction(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	m.Set(constants.KVMappingPrefixServiceDomain+".svc-1", "old.example.test")
	m.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
	m.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-1")
	m.Set(constants.KVMappingPrefixSubgroupByService+".svc-1.ml-1", "ml-1")
	objects := fakeSubgroupObjects{
		"service:svc-1": {"project_id": "project-sfid", "domain": "new.example.test"},
		"ml-1":          {"project_id": "project-sfid", "parent_id": "svc-1"},
	}
	pub := &mock.SpyMessagePublisher{}
	assert.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", map[string]any{"project_id": "project-sfid"}, pub, m, objects))
	assert.Len(t, pub.IndexerCalls, 2)
	assert.Equal(t, constants.IndexGroupsIOMailingListSubject, pub.IndexerCalls[1].Subject)
}

func TestServiceDomainChange_PartialUpdateThenRevert(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixServiceDomain+".svc-1", "a.example.test")
	m.Set(constants.KVMappingPrefixServiceDomainIndexed+".svc-1", "a.example.test")
	for _, uid := range []string{"ml-1", "ml-2"} {
		m.Set(constants.KVMappingPrefixSubgroup+"."+uid, uid)
		m.Set(constants.KVMappingPrefixSubgroupParent+"."+uid, "svc-1")
		m.Set(constants.KVMappingPrefixSubgroupByService+".svc-1."+uid, uid)
	}
	objects := fakeSubgroupObjects{
		"service:svc-1": {"project_id": "project-sfid", "domain": "b.example.test"},
		"ml-1":          {"project_id": "project-sfid", "parent_id": "svc-1"},
		"ml-2":          {"project_id": "project-sfid", "parent_id": "svc-1"},
	}
	pub := &failNthListPublisher{SpyMessagePublisher: &mock.SpyMessagePublisher{}, failAt: 2}
	assert.True(t, HandleDataStreamServiceUpdate(ctx, "svc-1", nil, pub, m, objects))
	_, complete := m.GetMappingValue(ctx, constants.KVMappingPrefixServiceDomainIndexed+".svc-1")
	assert.False(t, complete)
	objects["service:svc-1"]["domain"] = "a.example.test"
	pub.failAt = 0
	pub.listCalls = 0
	pub.IndexerCalls = nil
	assert.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", nil, pub, m, objects))
	var count int
	for _, call := range pub.IndexerCalls {
		if call.Subject == constants.IndexGroupsIOMailingListSubject {
			count++
			assert.Equal(t, "a.example.test", call.Message.(*model.IndexerMessage).Data.(map[string]any)["domain"])
		}
	}
	assert.Equal(t, 2, count)
}

type failNthListPublisher struct {
	*mock.SpyMessagePublisher
	listCalls int
	failAt    int
}

func (p *failNthListPublisher) Indexer(ctx context.Context, subject string, message any) error {
	if subject == constants.IndexGroupsIOMailingListSubject {
		p.listCalls++
		if p.listCalls == p.failAt {
			return errors.New("connection unavailable")
		}
	}
	return p.SpyMessagePublisher.Indexer(ctx, subject, message)
}

func TestServiceDomainChange_DelayedOlderDeliveryUsesCurrentSource(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixServiceDomain+".svc-1", "a.example.test")
	m.Set(constants.KVMappingPrefixServiceDomainIndexed+".svc-1", "a.example.test")
	m.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
	m.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-1")
	m.Set(constants.KVMappingPrefixSubgroupByService+".svc-1.ml-1", "ml-1")
	objects := fakeSubgroupObjects{"service:svc-1": {"project_id": "project-sfid", "domain": "b.example.test"}, "ml-1": {"project_id": "project-sfid", "parent_id": "svc-1"}}
	pub := &failingListPublisher{SpyMessagePublisher: &mock.SpyMessagePublisher{}, fail: true}
	assert.True(t, HandleDataStreamServiceUpdate(ctx, "svc-1", nil, pub, m, objects))
	objects["service:svc-1"]["domain"] = "c.example.test"
	pub.fail = false
	assert.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", nil, pub, m, objects))
	pub.IndexerCalls = nil
	assert.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", map[string]any{"project_id": "project-sfid", "domain": "b.example.test"}, pub, m, objects))
	assert.Len(t, pub.IndexerCalls, 1, "older retry must not republish an obsolete list domain")
	checkpoint, _ := m.GetMappingValue(ctx, constants.KVMappingPrefixServiceDomainIndexed+".svc-1")
	assert.Equal(t, "c.example.test", checkpoint)
}

func TestServiceDomainChange_MalformedListDoesNotBlockLaterLists(t *testing.T) {
	ctx := context.Background()
	m := mock.NewFakeMappingStore()
	m.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	m.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	m.Set(constants.KVMappingPrefixServiceDomainIndexed+".svc-1", "old.example.test")
	for _, uid := range []string{"ml-bad", "ml-good"} {
		m.Set(constants.KVMappingPrefixSubgroup+"."+uid, uid)
		m.Set(constants.KVMappingPrefixSubgroupParent+"."+uid, "svc-1")
		m.Set(constants.KVMappingPrefixSubgroupByService+".svc-1."+uid, uid)
	}
	objects := fakeSubgroupObjects{
		"service:svc-1": {"project_id": "project-sfid", "domain": "new.example.test"},
		"ml-bad":        {"parent_id": "svc-1"},
		"ml-good":       {"project_id": "project-sfid", "parent_id": "svc-1"},
	}
	pub := &mock.SpyMessagePublisher{}
	require.False(t, HandleDataStreamServiceUpdate(ctx, "svc-1", nil, pub, m, objects))
	var listIDs []string
	for _, call := range pub.IndexerCalls {
		if call.Subject == constants.IndexGroupsIOMailingListSubject {
			listIDs = append(listIDs, call.Message.(*model.IndexerMessage).Data.(map[string]any)["uid"].(string))
		}
	}
	assert.Equal(t, []string{"ml-good"}, listIDs)
}

func TestPublishMailingListIndex_CancelledContextDoesNotPublish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pub := &mock.SpyMessagePublisher{}
	err := publishMailingListIndex(ctx, &model.GroupsIOMailingList{UID: "ml-1", ServiceUID: "svc-1"}, model.ActionUpdated, pub)
	require.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, pub.IndexerCalls)
}

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
		pub, m, fakeSubgroupObjects{"service:svc-1": {"project_id": "sfid-proj", "group_service_type": "mailing-list", "domain": "example.com"}})

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
		map[string]any{"project_id": "sfid-proj"}, &mock.SpyMessagePublisher{}, m, fakeSubgroupObjects{"service:svc-1": {"project_id": "sfid-proj"}})

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

func TestHandleDataStreamServiceUpdate_DomainMappingFailure_Permanent_NAK(t *testing.T) {
	m := mock.NewFakeMappingStore()
	m.Set(fmt.Sprintf("%s.sfid-proj", constants.KVMappingPrefixProjectBySFID), "proj-uid")
	domainKey := fmt.Sprintf("%s.svc-1", constants.KVMappingPrefixServiceDomain)
	m.SimulatePutError(domainKey, jetstream.ErrInvalidKey)

	pub := &mock.SpyMessagePublisher{}
	nak := HandleDataStreamServiceUpdate(context.Background(), "svc-1",
		map[string]any{"project_id": "sfid-proj", "domain": "example.com"},
		pub, m)

	assert.True(t, nak, "a domain mapping failure must leave propagation pending for repair")
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

	assert.True(t, nak, "a failed domain mapping must not ACK before refreshing existing lists")
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
