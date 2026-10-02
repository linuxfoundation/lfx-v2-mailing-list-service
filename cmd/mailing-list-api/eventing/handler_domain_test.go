// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	fgatypes "github.com/linuxfoundation/lfx-v2-fga-sync/pkg/types"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	svc "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type keyedTestLock struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func (l *keyedTestLock) WithService(ctx context.Context, uid string, fn func(context.Context) bool) bool {
	l.mu.Lock()
	if l.locks == nil {
		l.locks = make(map[string]*sync.Mutex)
	}
	lock := l.locks[uid]
	if lock == nil {
		lock = &sync.Mutex{}
		l.locks[uid] = lock
	}
	l.mu.Unlock()
	lock.Lock()
	defer lock.Unlock()
	return fn(ctx)
}

func (l *keyedTestLock) WithServices(ctx context.Context, uids []string, fn func(context.Context) bool) bool {
	unique := make(map[string]struct{}, len(uids))
	for _, uid := range uids {
		unique[uid] = struct{}{}
	}
	ordered := make([]string, 0, len(unique))
	for uid := range unique {
		ordered = append(ordered, uid)
	}
	slices.Sort(ordered)
	var acquire func(int, context.Context) bool
	acquire = func(i int, current context.Context) bool {
		if i == len(ordered) {
			return fn(current)
		}
		return l.WithService(current, ordered[i], func(next context.Context) bool { return acquire(i+1, next) })
	}
	return acquire(0, ctx)
}

type synchronizedMappings struct {
	port.MappingReaderWriter
	mu sync.Mutex
}

func (m *synchronizedMappings) GetMappingValueWithError(ctx context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MappingReaderWriter.GetMappingValueWithError(ctx, key)
}

func (m *synchronizedMappings) PutMapping(ctx context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MappingReaderWriter.PutMapping(ctx, key, value)
}

func (m *synchronizedMappings) PutTombstone(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MappingReaderWriter.PutTombstone(ctx, key)
}

func (m *synchronizedMappings) ResolveAction(ctx context.Context, key string) model.MessageAction {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MappingReaderWriter.ResolveAction(ctx, key)
}

func (m *synchronizedMappings) IsTombstoned(ctx context.Context, key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MappingReaderWriter.IsTombstoned(ctx, key)
}

func (m *synchronizedMappings) IsMappingPresent(ctx context.Context, key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MappingReaderWriter.IsMappingPresent(ctx, key)
}

func (m *synchronizedMappings) GetMappingValue(ctx context.Context, key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MappingReaderWriter.GetMappingValue(ctx, key)
}

func (m *synchronizedMappings) ListSubgroupsByService(ctx context.Context, uid string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MappingReaderWriter.ListSubgroupsByService(ctx, uid)
}

type testObjects struct {
	mu       sync.Mutex
	service  map[string]any
	subgroup map[string]any
}

type serviceRemovalObjects struct{ service map[string]any }

type unreadableEventObjects struct{ port.V1ObjectReader }

func (*unreadableEventObjects) GetSubgroup(context.Context, string) (map[string]any, bool, error) {
	return nil, false, port.ErrV1ObjectDecode
}

func (o *serviceRemovalObjects) GetService(context.Context, string) (map[string]any, bool, error) {
	if o.service == nil {
		return nil, false, nil
	}
	return cloneObject(o.service), true, nil
}

func (*serviceRemovalObjects) GetSubgroup(context.Context, string) (map[string]any, bool, error) {
	return nil, false, nil
}

func (o *testObjects) GetService(context.Context, string) (map[string]any, bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return cloneObject(o.service), true, nil
}

func (o *testObjects) GetSubgroup(context.Context, string) (map[string]any, bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return cloneObject(o.subgroup), true, nil
}

func cloneObject(data map[string]any) map[string]any {
	copy := make(map[string]any, len(data))
	for key, value := range data {
		copy[key] = value
	}
	return copy
}

type blockedPublisher struct {
	mu       sync.Mutex
	calls    []*model.IndexerMessage
	blocked  chan struct{}
	release  chan struct{}
	blockOne sync.Once
}

func (p *blockedPublisher) Indexer(_ context.Context, subject string, msg any) error {
	if subject == constants.IndexGroupsIOMailingListSubject {
		p.blockOne.Do(func() {
			close(p.blocked)
			<-p.release
		})
		p.mu.Lock()
		p.calls = append(p.calls, msg.(*model.IndexerMessage))
		p.mu.Unlock()
	}
	return nil
}

func (*blockedPublisher) Access(context.Context, string, any) error   { return nil }
func (*blockedPublisher) Internal(context.Context, string, any) error { return nil }

func TestDomainPublicationsSerializeAcrossServiceAndSubgroupEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	mappings := mock.NewFakeMappingStore()
	mappings.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	mappings.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	mappings.Set(constants.KVMappingPrefixServiceDomain+".svc-1", "a.example.test")
	mappings.Set(constants.KVMappingPrefixServiceDomainIndexed+".svc-1", "a.example.test")
	mappings.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
	mappings.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-1")
	mappings.Set(constants.KVMappingPrefixSubgroupByService+".svc-1.ml-1", "ml-1")
	objects := &testObjects{
		service:  map[string]any{"project_id": "project-sfid", "domain": "b.example.test"},
		subgroup: map[string]any{"project_id": "project-sfid", "parent_id": "svc-1"},
	}
	pub := &blockedPublisher{blocked: make(chan struct{}), release: make(chan struct{})}
	lookup := mock.NewFakeProjectLookup()
	lookup.Slugs["project-uid"] = "project"
	h := NewEventHandler(pub, &synchronizedMappings{MappingReaderWriter: mappings}, lookup, objects, WithServiceDomainLock(&keyedTestLock{}))
	serviceDone := make(chan bool, 1)
	go func() {
		serviceDone <- h.HandleChange(ctx, kvPrefixService+"svc-1", map[string]any{"domain": "b.example.test"})
	}()
	select {
	case <-pub.blocked:
	case <-ctx.Done():
		t.Fatal("service fan-out did not reach the publisher")
	}
	subgroupDone := make(chan bool, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		subgroupDone <- h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", map[string]any{"project_id": "project-sfid", "parent_id": "svc-1"})
	}()
	<-started
	select {
	case <-subgroupDone:
		t.Fatal("subgroup publication completed while service fan-out was blocked")
	default:
	}
	close(pub.release)
	require.False(t, <-serviceDone)
	require.False(t, <-subgroupDone)
	pub.mu.Lock()
	require.Len(t, pub.calls, 2)
	for _, msg := range pub.calls {
		assert.Equal(t, "b.example.test", msg.Data.(map[string]any)["domain"])
	}
	pub.mu.Unlock()
}

func TestDelayedServiceRemovalsReconcileCurrentSource(t *testing.T) {
	for _, removal := range []string{"soft", "hard"} {
		t.Run(removal, func(t *testing.T) {
			ctx := context.Background()
			mappings := mock.NewFakeMappingStore()
			mappings.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
			mappings.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
			mappings.Set(constants.KVMappingPrefixServiceDomainIndexed+".svc-1", "old.example.test")
			objects := &serviceRemovalObjects{service: map[string]any{"project_id": "project-sfid", "domain": "new.example.test"}}
			pub := &mock.SpyMessagePublisher{}
			h := NewEventHandler(pub, mappings, mock.NewFakeProjectLookup(), objects, WithServiceDomainLock(&keyedTestLock{}))
			remove := func() bool {
				if removal == "soft" {
					return h.HandleChange(ctx, kvPrefixService+"svc-1", map[string]any{sdcDeletedAt: "older"})
				}
				return h.HandleRemoval(ctx, kvPrefixService+"svc-1")
			}
			require.False(t, remove())
			assert.True(t, mappings.IsMappingPresent(ctx, constants.KVMappingPrefixService+".svc-1"))
			assert.Equal(t, "new.example.test", mustPublishedParent(t, ctx, mappings, constants.KVMappingPrefixServiceDomainIndexed+".svc-1"))
			require.Len(t, pub.IndexerCalls, 1)
			assert.NotEqual(t, model.ActionDeleted, pub.IndexerCalls[0].Message.(*model.IndexerMessage).Action)

			objects.service = nil // the same removal must still delete an absent source
			require.False(t, remove())
			assert.True(t, mappings.IsTombstoned(ctx, constants.KVMappingPrefixService+".svc-1"))
			require.Len(t, pub.IndexerCalls, 2)
			assert.Equal(t, model.ActionDeleted, pub.IndexerCalls[1].Message.(*model.IndexerMessage).Action)
		})
	}
}

func TestDelayedSubgroupRemovalsPreserveLiveParentlessMapping(t *testing.T) {
	for _, withPointers := range []bool{true, false} {
		name := "without parent pointers"
		if withPointers {
			name = "with parent pointers"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			mappings := mock.NewFakeMappingStore()
			mappings.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
			mappings.Set(constants.KVMappingPrefixService+".svc-b", "svc-b")
			mappings.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
			if withPointers {
				mappings.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-a")
				mappings.Set(constants.KVMappingPrefixSubgroupPublishedParent+".ml-1", "svc-a")
				mappings.Set(constants.KVMappingPrefixSubgroupByService+".svc-a.ml-1", "ml-1")
			}
			objects := &mutableLeaseObjects{subgroup: map[string]any{"project_id": "project-sfid"}}
			pub := &mock.SpyMessagePublisher{}
			lookup := mock.NewFakeProjectLookup()
			lookup.Slugs["project-uid"] = "project"
			h := NewEventHandler(pub, mappings, lookup, objects, WithServiceDomainLock(&keyedTestLock{}))
			if withPointers {
				require.False(t, h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", map[string]any{sdcDeletedAt: "older"}))
				assert.False(t, mappings.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroupByService+".svc-a.ml-1"))
			} else {
				require.False(t, h.HandleRemoval(ctx, kvPrefixSubgroup+"ml-1"))
			}
			require.Len(t, pub.IndexerCalls, 1, "legacy lists without pointers still need UID-based document deletion")
			assert.Equal(t, model.ActionDeleted, pub.IndexerCalls[0].Message.(*model.IndexerMessage).Action)
			assert.Len(t, pub.AccessCalls, 1, "unassociation must revoke inherited access")
			assert.Equal(t, "ml-1", mustPublishedParent(t, ctx, mappings, constants.KVMappingPrefixSubgroupUnassociated+".ml-1"))
			assert.True(t, mappings.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroup+".ml-1"))
			assert.False(t, mappings.IsTombstoned(ctx, constants.KVMappingPrefixSubgroup+".ml-1"))
			require.False(t, h.HandleRemoval(ctx, kvPrefixSubgroup+"ml-1"))
			assert.Len(t, pub.IndexerCalls, 1, "completion marker prevents duplicate cleanup")
			objects.setSubgroup(map[string]any{"project_id": "project-sfid", "parent_id": "svc-b"})
			require.False(t, h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", nil))
			assert.True(t, mappings.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroupByService+".svc-b.ml-1"))
			assert.True(t, mappings.IsTombstoned(ctx, constants.KVMappingPrefixSubgroupUnassociated+".ml-1"), "reparenting invalidates the prior completion marker")
		})
	}
}

func TestSubgroupUnassociationReconcilesCurrentVisibility(t *testing.T) {
	ctx := context.Background()
	mappings := mock.NewFakeMappingStore()
	mappings.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
	mappings.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-a")
	mappings.Set(constants.KVMappingPrefixSubgroupByService+".svc-a.ml-1", "ml-1")
	committeeKey := constants.KVMappingPrefixSubgroupCommittee + ".ml-1"
	mappings.Set(committeeKey, "committee-uid|true")
	mappings.Set(constants.KVMappingPrefixSubgroupByGroupID+".42", "ml-1")
	objects := &mutableLeaseObjects{subgroup: map[string]any{"visibility": "private"}}
	pub := &mock.SpyMessagePublisher{}
	h := NewEventHandler(pub, mappings, mock.NewFakeProjectLookup(), objects, WithServiceDomainLock(&keyedTestLock{}))
	change := func() bool {
		return h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", map[string]any{"parent_id": "svc-a", "visibility": "public"})
	}
	require.False(t, change(), "the latest private source must win over the stale public event")
	require.Len(t, pub.AccessCalls, 1)
	assert.Equal(t, "ml-1", mustPublishedParent(t, ctx, mappings, constants.KVMappingPrefixSubgroupUnassociated+".ml-1"))
	private := pub.AccessCalls[0].Message.(fgatypes.GenericFGAMessage).Data.(fgatypes.GenericAccessData)
	assert.False(t, private.Public)
	assert.NotContains(t, private.ExcludeRelations, constants.RelationViewer)
	assert.Equal(t, "committee-uid|false", mustPublishedParent(t, ctx, mappings, committeeKey))
	require.Len(t, pub.IndexerCalls, 1)

	objects.setSubgroup(map[string]any{"visibility": "Public"})
	require.False(t, change(), "a later parentless public change must restore the viewer grant")
	public := pub.AccessCalls[1].Message.(fgatypes.GenericFGAMessage).Data.(fgatypes.GenericAccessData)
	assert.True(t, public.Public)
	assert.NotContains(t, public.ExcludeRelations, constants.RelationViewer)
	assert.Equal(t, "committee-uid|true", mustPublishedParent(t, ctx, mappings, committeeKey))
	assert.Len(t, pub.IndexerCalls, 1, "visibility-only updates must not re-delete the document")

	objects.setSubgroup(map[string]any{"visibility": "private"})
	require.False(t, change(), "a later parentless private change must revoke the viewer grant")
	private = pub.AccessCalls[2].Message.(fgatypes.GenericFGAMessage).Data.(fgatypes.GenericAccessData)
	assert.False(t, private.Public)
	assert.Equal(t, "committee-uid|false", mustPublishedParent(t, ctx, mappings, committeeKey))
	assert.Len(t, pub.IndexerCalls, 1)
	require.False(t, svc.HandleDataStreamMessageUpdate(ctx, "msg-1", map[string]any{"group_id": float64(42)}, pub, mappings))
	message := pub.IndexerCalls[len(pub.IndexerCalls)-1].Message.(*model.IndexerMessage).Data.(map[string]any)
	assert.Equal(t, true, message["is_private"], "new messages must use the current parentless visibility")
	assert.Equal(t, "committee-uid", message["committee_uid"])
}

func TestParentlessVisibilityAccessFailureRetries(t *testing.T) {
	ctx := context.Background()
	mappings := mock.NewFakeMappingStore()
	mappings.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
	mappings.Set(constants.KVMappingPrefixSubgroupUnassociated+".ml-1", "ml-1")
	objects := &mutableLeaseObjects{subgroup: map[string]any{"visibility": "public"}}
	pub := &mock.SpyMessagePublisher{AccessError: errors.New("connection unavailable")}
	h := NewEventHandler(pub, mappings, mock.NewFakeProjectLookup(), objects, WithServiceDomainLock(&keyedTestLock{}))
	require.True(t, h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", nil))
	assert.Equal(t, "ml-1", mustPublishedParent(t, ctx, mappings, constants.KVMappingPrefixSubgroupUnassociated+".ml-1"))
	pub.AccessError = nil
	require.False(t, h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", nil))
	assert.Empty(t, pub.IndexerCalls)
	assert.Len(t, pub.AccessCalls, 2)
}

func TestSubgroupUnassociationFinishesPendingMarkerAfterPartialCleanup(t *testing.T) {
	ctx := context.Background()
	mappings := mock.NewFakeMappingStore()
	mappings.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
	pendingKey := constants.KVMappingPrefixSubgroupUnassociationPending + ".ml-1"
	completedKey := constants.KVMappingPrefixSubgroupUnassociated + ".ml-1"
	mappings.SimulateTombstoneError(pendingKey, errors.New("connection unavailable"))
	objects := &mutableLeaseObjects{subgroup: map[string]any{"project_id": "project-sfid"}}
	pub := &mock.SpyMessagePublisher{}
	h := NewEventHandler(pub, mappings, mock.NewFakeProjectLookup(), objects, WithServiceDomainLock(&keyedTestLock{}))
	require.True(t, h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", nil))
	assert.Equal(t, "ml-1", mustPublishedParent(t, ctx, mappings, completedKey))
	assert.Equal(t, "ml-1", mustPublishedParent(t, ctx, mappings, pendingKey))
	require.Len(t, pub.IndexerCalls, 1)
	mappings.SimulateTombstoneError(pendingKey, nil)
	require.False(t, h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", nil))
	assert.True(t, mappings.IsTombstoned(ctx, pendingKey))
	assert.Len(t, pub.IndexerCalls, 2, "pending cleanup must be retried before ACK")
	require.False(t, h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", nil))
	assert.Len(t, pub.IndexerCalls, 2, "finalized cleanup should not publish again")
}

func TestUnreadableSubgroupDoesNotTriggerUnassociation(t *testing.T) {
	ctx := context.Background()
	mappings := mock.NewFakeMappingStore()
	mappings.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
	mappings.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-a")
	mappings.Set(constants.KVMappingPrefixSubgroupByService+".svc-a.ml-1", "ml-1")
	pub := &mock.SpyMessagePublisher{}
	h := NewEventHandler(pub, mappings, mock.NewFakeProjectLookup(), &unreadableEventObjects{}, WithServiceDomainLock(&keyedTestLock{}))
	require.True(t, h.HandleRemoval(ctx, kvPrefixSubgroup+"ml-1"))
	assert.Empty(t, pub.IndexerCalls)
	assert.Empty(t, pub.AccessCalls)
	assert.True(t, mappings.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroupByService+".svc-a.ml-1"))
	assert.False(t, mappings.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroupUnassociated+".ml-1"))
}

func TestSubgroupParentWriteFailureThenUnassociationCleansPublishedDocument(t *testing.T) {
	ctx := context.Background()
	mappings := mock.NewFakeMappingStore()
	mappings.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	mappings.Set(constants.KVMappingPrefixService+".svc-1", "svc-1")
	parentKey := constants.KVMappingPrefixSubgroupParent + ".ml-1"
	indexKey := constants.KVMappingPrefixSubgroupByService + ".svc-1.ml-1"
	publishedKey := constants.KVMappingPrefixSubgroupPublishedParent + ".ml-1"
	mappings.SimulatePutError(parentKey, errors.New("connection timeout"))
	objects := &testObjects{subgroup: map[string]any{"project_id": "project-sfid", "parent_id": "svc-1"}}
	lookup := mock.NewFakeProjectLookup()
	lookup.Slugs["project-uid"] = "project"
	pub := &mock.SpyMessagePublisher{}
	h := NewEventHandler(pub, mappings, lookup, objects, WithServiceDomainLock(&keyedTestLock{}))

	require.True(t, h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", objects.subgroup))
	require.Len(t, pub.IndexerCalls, 1, "creation must have published before the parent write failed")
	assert.Equal(t, "svc-1", mustPublishedParent(t, ctx, mappings, publishedKey))
	assert.False(t, mappings.IsMappingPresent(ctx, parentKey))
	assert.True(t, mappings.IsMappingPresent(ctx, indexKey))

	objects.subgroup = map[string]any{"project_id": "project-sfid"}
	require.False(t, h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", objects.subgroup))
	require.Len(t, pub.IndexerCalls, 2)
	deleted := pub.IndexerCalls[1].Message.(*model.IndexerMessage)
	assert.Equal(t, model.ActionDeleted, deleted.Action)
	require.NotNil(t, deleted.IndexingConfig)
	require.Len(t, pub.AccessCalls, 2, "unassociation must revoke inherited service access")
	assert.True(t, mappings.IsTombstoned(ctx, publishedKey))
	assert.True(t, mappings.IsTombstoned(ctx, parentKey))
	assert.False(t, mappings.IsMappingPresent(ctx, indexKey))
	assert.True(t, mappings.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroup+".ml-1"), "reparenting must remain possible")

	require.False(t, h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", objects.subgroup))
	assert.Len(t, pub.IndexerCalls, 2, "duplicate unassociation must not republish deletion")
}

func TestSubgroupUnassociatedAfterPartialMoveCleansBothServiceIndexes(t *testing.T) {
	ctx := context.Background()
	mappings := mock.NewFakeMappingStore()
	mappings.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	for _, svc := range []string{"svc-a", "svc-b"} {
		mappings.Set(constants.KVMappingPrefixService+"."+svc, svc)
	}
	parentKey := constants.KVMappingPrefixSubgroupParent + ".ml-1"
	publishedKey := constants.KVMappingPrefixSubgroupPublishedParent + ".ml-1"
	pendingKey := constants.KVMappingPrefixSubgroupPreviousService + ".ml-1"
	oldIndex := constants.KVMappingPrefixSubgroupByService + ".svc-a.ml-1"
	newIndex := constants.KVMappingPrefixSubgroupByService + ".svc-b.ml-1"
	objects := &testObjects{subgroup: map[string]any{"project_id": "project-sfid", "parent_id": "svc-a"}}
	lookup := mock.NewFakeProjectLookup()
	lookup.Slugs["project-uid"] = "project"
	pub := &mock.SpyMessagePublisher{}
	h := NewEventHandler(pub, mappings, lookup, objects, WithServiceDomainLock(&keyedTestLock{}))
	change := func() bool { return h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", objects.subgroup) }

	require.False(t, change())
	objects.subgroup = map[string]any{"project_id": "project-sfid"}
	mappings.SimulateTombstoneError(parentKey, errors.New("connection timeout"))
	require.True(t, change(), "the old index was purged but clearing parent A failed")
	assert.False(t, mappings.IsMappingPresent(ctx, oldIndex))
	assert.Equal(t, "svc-a", mustPublishedParent(t, ctx, mappings, parentKey))

	objects.subgroup["parent_id"] = "svc-b"
	mappings.SimulateTombstoneError(parentKey, nil)
	mappings.SimulatePutError(parentKey, errors.New("connection timeout"))
	require.True(t, change(), "the new document was published but updating the parent failed")
	assert.Equal(t, "svc-b", mustPublishedParent(t, ctx, mappings, publishedKey))
	assert.True(t, mappings.IsMappingPresent(ctx, newIndex))
	assert.Equal(t, "svc-a", mustPublishedParent(t, ctx, mappings, parentKey))
	assert.Equal(t, "svc-a", mustPublishedParent(t, ctx, mappings, pendingKey))

	delete(objects.subgroup, "parent_id")
	mappings.SimulatePutError(parentKey, nil)
	require.False(t, change())
	require.Len(t, pub.IndexerCalls, 4, "unassociation must delete the B document even though A's index is gone")
	assert.Equal(t, model.ActionDeleted, pub.IndexerCalls[3].Message.(*model.IndexerMessage).Action)
	assert.Len(t, pub.AccessCalls, 4, "unassociation must revoke access inherited from B")
	assert.False(t, mappings.IsMappingPresent(ctx, newIndex))
	assert.True(t, mappings.IsTombstoned(ctx, publishedKey))
	assert.True(t, mappings.IsTombstoned(ctx, pendingKey))
	assert.True(t, mappings.IsTombstoned(ctx, parentKey))
	assert.True(t, mappings.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroup+".ml-1"))
	require.False(t, change())
	assert.Len(t, pub.IndexerCalls, 4, "duplicate unassociation must not re-delete")
	assert.Len(t, pub.AccessCalls, 5, "a parentless delivery rechecks current public visibility")
}

func TestSubgroupUnassociatedRetriesWhenPublishedIndexPurgeFails(t *testing.T) {
	ctx := context.Background()
	mappings := mock.NewFakeMappingStore()
	mappings.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
	for _, svc := range []string{"svc-a", "svc-b"} {
		mappings.Set(constants.KVMappingPrefixService+"."+svc, svc)
	}
	parentKey := constants.KVMappingPrefixSubgroupParent + ".ml-1"
	publishedKey := constants.KVMappingPrefixSubgroupPublishedParent + ".ml-1"
	newIndex := constants.KVMappingPrefixSubgroupByService + ".svc-b.ml-1"
	objects := &testObjects{subgroup: map[string]any{"project_id": "project-sfid", "parent_id": "svc-a"}}
	lookup := mock.NewFakeProjectLookup()
	lookup.Slugs["project-uid"] = "project"
	pub := &mock.SpyMessagePublisher{}
	h := NewEventHandler(pub, mappings, lookup, objects, WithServiceDomainLock(&keyedTestLock{}))
	change := func() bool { return h.HandleChange(ctx, kvPrefixSubgroup+"ml-1", objects.subgroup) }

	require.False(t, change())
	objects.subgroup["parent_id"] = "svc-b"
	mappings.SimulatePutError(parentKey, errors.New("connection timeout"))
	require.True(t, change())
	delete(objects.subgroup, "parent_id")
	mappings.SimulatePutError(parentKey, nil)
	mappings.SimulateTombstoneError(publishedKey, errors.New("connection timeout"))
	require.True(t, change())
	assert.True(t, mappings.IsMappingPresent(ctx, publishedKey))
	assert.True(t, mappings.IsTombstoned(ctx, constants.KVMappingPrefixSubgroupPreviousService+".ml-1"))
	mappings.SimulateTombstoneError(publishedKey, nil)
	require.False(t, change())
	assert.True(t, mappings.IsTombstoned(ctx, publishedKey))
	assert.False(t, mappings.IsMappingPresent(ctx, newIndex))
	assert.True(t, mappings.IsTombstoned(ctx, parentKey))
}

func mustPublishedParent(t *testing.T, ctx context.Context, mappings *mock.FakeMappingStore, key string) string {
	t.Helper()
	value, present := mappings.GetMappingValue(ctx, key)
	require.True(t, present)
	return value
}
