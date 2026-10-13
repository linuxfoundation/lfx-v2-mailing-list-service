// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	infraNATS "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Both handlers share this KV, but each has its own real lease implementation.
type sharedLeaseKV struct {
	jetstream.KeyValue
	mu      sync.Mutex
	entries map[string]sharedLeaseEntry
	rev     uint64
}

type sharedLeaseEntry struct {
	jetstream.KeyValueEntry
	value []byte
	rev   uint64
}

func (e sharedLeaseEntry) Value() []byte    { return e.value }
func (e sharedLeaseEntry) Revision() uint64 { return e.rev }

func (kv *sharedLeaseKV) Create(_ context.Context, key string, value []byte, _ ...jetstream.KVCreateOpt) (uint64, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if _, ok := kv.entries[key]; ok {
		return 0, jetstream.ErrKeyExists
	}
	kv.rev++
	kv.entries[key] = sharedLeaseEntry{value: value, rev: kv.rev}
	return kv.rev, nil
}

func (kv *sharedLeaseKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	entry, ok := kv.entries[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return entry, nil
}

func (kv *sharedLeaseKV) Update(_ context.Context, key string, value []byte, revision uint64) (uint64, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if entry, ok := kv.entries[key]; !ok || entry.rev != revision {
		return 0, jetstream.ErrKeyRevisionMismatch
	}
	kv.rev++
	kv.entries[key] = sharedLeaseEntry{value: value, rev: kv.rev}
	return kv.rev, nil
}

func (kv *sharedLeaseKV) Delete(_ context.Context, key string, _ ...jetstream.KVDeleteOpt) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	delete(kv.entries, key)
	return nil
}

type mutableLeaseObjects struct {
	mu       sync.Mutex
	subgroup map[string]any
}

func (*mutableLeaseObjects) GetService(context.Context, string) (map[string]any, bool, error) {
	return map[string]any{"project_id": "project-sfid", "domain": "new.example.test"}, true, nil
}

func (o *mutableLeaseObjects) GetSubgroup(_ context.Context, _ string) (map[string]any, bool, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.subgroup == nil {
		return nil, false, nil
	}
	return cloneObject(o.subgroup), true, nil
}

func (o *mutableLeaseObjects) setSubgroup(data map[string]any) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.subgroup = data
}

func TestServiceDomainLeaseSerializesFanoutWithSubgroupMoveAndDelete(t *testing.T) {
	for _, operation := range []string{"move", "delete"} {
		t.Run(operation, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			mappings := mock.NewFakeMappingStore()
			mappings.Set(constants.KVMappingPrefixProjectBySFID+".project-sfid", "project-uid")
			for _, svc := range []string{"svc-a", "svc-b"} {
				mappings.Set(constants.KVMappingPrefixService+"."+svc, svc)
			}
			mappings.Set(constants.KVMappingPrefixServiceDomain+".svc-a", "old.example.test")
			mappings.Set(constants.KVMappingPrefixServiceDomainIndexed+".svc-a", "old.example.test")
			mappings.Set(constants.KVMappingPrefixServiceDomain+".svc-b", "other.example.test")
			mappings.Set(constants.KVMappingPrefixSubgroup+".ml-1", "ml-1")
			mappings.Set(constants.KVMappingPrefixSubgroupParent+".ml-1", "svc-a")
			mappings.Set(constants.KVMappingPrefixSubgroupByService+".svc-a.ml-1", "ml-1")
			objects := &mutableLeaseObjects{subgroup: map[string]any{"project_id": "project-sfid", "parent_id": "svc-a"}}
			pub := &blockedPublisher{blocked: make(chan struct{}), release: make(chan struct{})}
			defer func() {
				select {
				case <-pub.release:
				default:
					close(pub.release)
				}
			}()
			kv := &sharedLeaseKV{entries: make(map[string]sharedLeaseEntry)}
			lookup := mock.NewFakeProjectLookup()
			lookup.Slugs["project-uid"] = "project"
			store := &synchronizedMappings{MappingReaderWriter: mappings}
			serviceHandler := NewEventHandler(pub, store, lookup, objects, WithServiceDomainLock(infraNATS.NewServiceDomainLock(kv)))
			subgroupHandler := NewEventHandler(pub, store, lookup, objects, WithServiceDomainLock(infraNATS.NewServiceDomainLock(kv)))

			serviceDone := make(chan bool, 1)
			go func() { serviceDone <- serviceHandler.HandleChange(ctx, kvPrefixService+"svc-a", nil) }()
			select {
			case <-pub.blocked:
			case <-ctx.Done():
				t.Fatal("service fan-out did not reach publisher")
			}
			var change func() bool
			if operation == "move" {
				objects.setSubgroup(map[string]any{"project_id": "project-sfid", "parent_id": "svc-b"})
				change = func() bool { return subgroupHandler.HandleChange(ctx, kvPrefixSubgroup+"ml-1", nil) }
			} else {
				objects.setSubgroup(nil)
				change = func() bool { return subgroupHandler.HandleRemoval(ctx, kvPrefixSubgroup+"ml-1") }
			}
			subgroupDone := make(chan bool, 1)
			go func() { subgroupDone <- change() }()
			select {
			case <-subgroupDone:
				t.Fatal("subgroup change completed while service held its lease")
			case <-time.After(30 * time.Millisecond):
			}
			close(pub.release)
			require.False(t, <-serviceDone)
			require.False(t, <-subgroupDone)
			assert.Equal(t, "new.example.test", mustPublishedParent(t, ctx, mappings, constants.KVMappingPrefixServiceDomainIndexed+".svc-a"))
			assert.False(t, mappings.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroupByService+".svc-a.ml-1"))
			pub.mu.Lock()
			require.Len(t, pub.calls, 2)
			if operation == "move" {
				assert.Equal(t, "svc-b", mustPublishedParent(t, ctx, mappings, constants.KVMappingPrefixSubgroupParent+".ml-1"))
				assert.True(t, mappings.IsMappingPresent(ctx, constants.KVMappingPrefixSubgroupByService+".svc-b.ml-1"))
				assert.Equal(t, "other.example.test", pub.calls[1].Data.(map[string]any)["domain"])
			} else {
				assert.True(t, mappings.IsTombstoned(ctx, constants.KVMappingPrefixSubgroup+".ml-1"))
				assert.Equal(t, model.ActionDeleted, pub.calls[1].Action)
			}
			pub.mu.Unlock()
		})
	}
}
