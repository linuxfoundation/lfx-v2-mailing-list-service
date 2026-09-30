// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	msgpack "github.com/vmihailenco/msgpack/v5"
)

type fakeKV struct {
	jetstream.KeyValue
	entries map[string][]byte
	keys    []string
	puts    []string
	putErr  error
}

func (kv *fakeKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	value, ok := kv.entries[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return fakeEntry{data: value}, nil
}

func (kv *fakeKV) Put(_ context.Context, key string, value []byte) (uint64, error) {
	if kv.putErr != nil {
		return 0, kv.putErr
	}
	kv.puts = append(kv.puts, key)
	kv.entries[key] = value
	return 1, nil
}

func (kv *fakeKV) ListKeysFiltered(_ context.Context, _ ...string) (jetstream.KeyLister, error) {
	keys := make(chan string, len(kv.keys))
	for _, key := range kv.keys {
		keys <- key
	}
	close(keys)
	return fakeLister{keys: keys}, nil
}

type fakeLister struct{ keys <-chan string }

func (l fakeLister) Keys() <-chan string { return l.keys }
func (l fakeLister) Stop() error         { return nil }

type fakeEntry struct {
	jetstream.KeyValueEntry
	data []byte
}

func (e fakeEntry) Value() []byte { return e.data }

func TestBackfillExistingSubgroups(t *testing.T) {
	ctx := context.Background()
	objects := &fakeKV{entries: map[string][]byte{}, keys: []string{}}
	mappings := &fakeKV{entries: map[string][]byte{}}
	index := &fakeKV{entries: map[string][]byte{}}
	add := func(uid, payload, forward string) {
		key := subgroupPrefix + uid
		objects.keys = append(objects.keys, key)
		objects.entries[key] = []byte(payload)
		if forward != "" {
			mappings.entries[constants.KVMappingPrefixSubgroup+"."+uid] = []byte(forward)
		}
	}
	add("sg-1", `{"parent_id":"svc-1"}`, "sg-1")
	add("sg-2", `{"parent_id":"svc-1"}`, "sg-2")
	add("sg-3", `{"parent_id":"svc-2"}`, "sg-3")
	add("deleted", `{"parent_id":"svc-1","_sdc_deleted_at":"now"}`, "deleted")
	add("unprocessed", `{"parent_id":"svc-1"}`, "")
	add("unprocessed-no-parent", `{}`, "")
	add("unprocessed-malformed", `not-json-or-msgpack`, "")
	add("tombstoned", `{"parent_id":"svc-1"}`, constants.KVTombstoneMarker)
	packed, err := msgpack.Marshal(map[string]any{"parent_id": "svc-2"})
	require.NoError(t, err)
	add("packed", ``, "packed")
	objects.entries[subgroupPrefix+"packed"] = packed
	objects.keys = append(objects.keys, subgroupPrefix+"sg-1") // duplicate from a busy bucket

	dry, err := backfill(ctx, objects, mappings, index, false)
	require.NoError(t, err)
	assert.Equal(t, results{listed: 9, processed: 4, skipped: 5}, dry)
	assert.Empty(t, index.puts)

	written, err := backfill(ctx, objects, mappings, index, true)
	require.NoError(t, err)
	assert.Equal(t, dry, written)
	assert.Len(t, index.puts, 8)
	assert.Empty(t, mappings.puts)
	assert.Equal(t, "svc-1", string(index.entries[constants.KVMappingPrefixSubgroupParent+".sg-1"]))
	assert.Equal(t, "sg-1", string(index.entries[constants.KVMappingPrefixSubgroupByService+".svc-1.sg-1"]))
	assert.Equal(t, "svc-2", string(index.entries[constants.KVMappingPrefixSubgroupParent+".packed"]))

	again, err := backfill(ctx, objects, mappings, index, true)
	require.NoError(t, err)
	assert.Equal(t, results{listed: 9, skipped: 9}, again)
	assert.Len(t, index.puts, 8)
}

func TestBackfillRejectsConflictsAndSurfacesWriteErrors(t *testing.T) {
	ctx := context.Background()
	objects := &fakeKV{entries: map[string][]byte{subgroupPrefix + "sg-1": []byte(`{"parent_id":"svc-1"}`)}, keys: []string{subgroupPrefix + "sg-1"}}
	mappings := &fakeKV{entries: map[string][]byte{
		constants.KVMappingPrefixSubgroup + ".sg-1": []byte("sg-1"),
	}}
	index := &fakeKV{entries: map[string][]byte{
		constants.KVMappingPrefixSubgroupParent + ".sg-1": []byte("svc-2"),
	}}
	counts, err := backfill(ctx, objects, mappings, index, true)
	require.ErrorContains(t, err, "1 subgroup records failed")
	assert.Equal(t, 1, counts.failed)
	assert.Empty(t, index.puts)

	delete(index.entries, constants.KVMappingPrefixSubgroupParent+".sg-1")
	index.putErr = errors.New("NATS unavailable")
	counts, err = backfill(ctx, objects, mappings, index, true)
	require.Error(t, err)
	assert.Equal(t, 1, counts.failed)
	assert.Empty(t, index.puts)
}

func TestBackfillCompletesPartialMapping(t *testing.T) {
	ctx := context.Background()
	objects := &fakeKV{entries: map[string][]byte{subgroupPrefix + "sg-1": []byte(`{"parent_id":"svc-1"}`)}, keys: []string{subgroupPrefix + "sg-1"}}
	mappings := &fakeKV{entries: map[string][]byte{
		constants.KVMappingPrefixSubgroup + ".sg-1": []byte("sg-1"),
	}}
	index := &fakeKV{entries: map[string][]byte{
		constants.KVMappingPrefixSubgroupByService + ".svc-1.sg-1": []byte("sg-1"),
	}}
	counts, err := backfill(ctx, objects, mappings, index, true)
	require.NoError(t, err)
	assert.Equal(t, results{listed: 1, processed: 1}, counts)
	assert.Equal(t, []string{constants.KVMappingPrefixSubgroupParent + ".sg-1"}, index.puts)
}

func TestBackfillRepairsPendingMove(t *testing.T) {
	ctx := context.Background()
	uid := "sg-1"
	objects := &fakeKV{entries: map[string][]byte{subgroupPrefix + uid: []byte(`{"parent_id":"svc-2"}`)}, keys: []string{subgroupPrefix + uid}}
	mappings := &fakeKV{entries: map[string][]byte{constants.KVMappingPrefixSubgroup + "." + uid: []byte(uid)}}
	index := &fakeKV{entries: map[string][]byte{
		constants.KVMappingPrefixSubgroupParent + "." + uid:          []byte("svc-1"),
		constants.KVMappingPrefixSubgroupPreviousService + "." + uid: []byte("svc-1"),
		constants.KVMappingPrefixSubgroupByService + ".svc-1." + uid: []byte(uid),
		constants.KVMappingPrefixSubgroupByService + ".svc-2." + uid: []byte(uid),
	}}
	counts, err := backfill(ctx, objects, mappings, index, true)
	require.NoError(t, err)
	assert.Equal(t, results{listed: 1, processed: 1}, counts)
	assert.Equal(t, "svc-2", string(index.entries[constants.KVMappingPrefixSubgroupParent+"."+uid]))
	assert.Equal(t, constants.KVTombstoneMarker, string(index.entries[constants.KVMappingPrefixSubgroupByService+".svc-1."+uid]))
	assert.Equal(t, constants.KVTombstoneMarker, string(index.entries[constants.KVMappingPrefixSubgroupPreviousService+"."+uid]))
}
