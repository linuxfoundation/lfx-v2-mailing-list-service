// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testServiceLock struct{ mu sync.Mutex }

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

func (l *testServiceLock) WithService(ctx context.Context, _ string, fn func(context.Context) bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return fn(ctx)
}

func (l *testServiceLock) WithServices(ctx context.Context, _ []string, fn func(context.Context) bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return fn(ctx)
}

type testObjects struct {
	mu       sync.Mutex
	service  map[string]any
	subgroup map[string]any
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
	h := NewEventHandler(pub, &synchronizedMappings{MappingReaderWriter: mappings}, lookup, WithSubgroupObjectReader(objects), WithServiceDomainLock(&testServiceLock{}))
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
