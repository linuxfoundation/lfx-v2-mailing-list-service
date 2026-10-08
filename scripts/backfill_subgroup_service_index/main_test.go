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
	assert.Len(t, mappings.puts, 4)
	assert.Equal(t, constants.KVTombstoneMarker, string(mappings.entries[constants.KVMappingPrefixServiceDomainIndexed+".svc-1"]))
	assert.Equal(t, constants.KVTombstoneMarker, string(mappings.entries[constants.KVMappingPrefixServiceDomainIndexed+".svc-2"]))
	assert.Equal(t, "svc-1", string(index.entries[constants.KVMappingPrefixSubgroupParent+".sg-1"]))
	assert.Equal(t, "sg-1", string(index.entries[constants.KVMappingPrefixSubgroupByService+".svc-1.sg-1"]))
	assert.Equal(t, "svc-2", string(index.entries[constants.KVMappingPrefixSubgroupParent+".packed"]))

	again, err := backfill(ctx, objects, mappings, index, true)
	require.NoError(t, err)
	assert.Equal(t, results{listed: 9, skipped: 9}, again)
	assert.Len(t, index.puts, 8)
	assert.Len(t, mappings.puts, 4)
}

func TestBackfillSkipsCleanUnassociatedSubgroups(t *testing.T) {
	ctx := context.Background()
	objects := &fakeKV{entries: map[string][]byte{
		subgroupPrefix + "missing-parent": []byte(`{"title":"Unassociated"}`),
		subgroupPrefix + "empty-parent":   []byte(`{"parent_id":""}`),
		subgroupPrefix + "associated":     []byte(`{"parent_id":"svc-1"}`),
	}, keys: []string{subgroupPrefix + "missing-parent", subgroupPrefix + "empty-parent", subgroupPrefix + "associated"}}
	mappings := &fakeKV{entries: map[string][]byte{
		constants.KVMappingPrefixSubgroup + ".missing-parent": []byte("missing-parent"),
		constants.KVMappingPrefixSubgroup + ".empty-parent":   []byte("empty-parent"),
		constants.KVMappingPrefixSubgroup + ".associated":     []byte("associated"),
	}}
	index := &fakeKV{entries: map[string][]byte{
		constants.KVMappingPrefixSubgroupParent + ".missing-parent":          []byte(constants.KVTombstoneMarker),
		constants.KVMappingPrefixSubgroupPublishedParent + ".missing-parent": []byte(constants.KVTombstoneMarker),
		constants.KVMappingPrefixSubgroupPreviousService + ".missing-parent": []byte(constants.KVTombstoneMarker),
		constants.KVMappingPrefixSubgroupUnassociated + ".missing-parent":    []byte("missing-parent"),
		constants.KVMappingPrefixSubgroupUnassociated + ".empty-parent":      []byte("empty-parent"),
	}}
	for _, write := range []bool{false, true} {
		counts, err := backfill(ctx, objects, mappings, index, write)
		require.NoError(t, err)
		assert.Equal(t, results{listed: 3, processed: 1, skipped: 2}, counts)
		assert.NotContains(t, index.entries, constants.KVMappingPrefixSubgroupParent+".empty-parent")
	}
	assert.Equal(t, "associated", string(index.entries[constants.KVMappingPrefixSubgroupByService+".svc-1.associated"]))
	assert.NotContains(t, index.puts, constants.KVMappingPrefixSubgroupParent+".missing-parent")
	assert.NotContains(t, index.puts, constants.KVMappingPrefixSubgroupParent+".empty-parent")
}

func TestBackfillReportsIncompleteUnassociation(t *testing.T) {
	for _, prefix := range []string{
		constants.KVMappingPrefixSubgroupParent,
		constants.KVMappingPrefixSubgroupPublishedParent,
		constants.KVMappingPrefixSubgroupPreviousService,
	} {
		t.Run(prefix, func(t *testing.T) {
			uid := "sg-1"
			objects := &fakeKV{entries: map[string][]byte{subgroupPrefix + uid: []byte(`{"parent_id":""}`)}, keys: []string{subgroupPrefix + uid}}
			mappings := &fakeKV{entries: map[string][]byte{constants.KVMappingPrefixSubgroup + "." + uid: []byte(uid)}}
			index := &fakeKV{entries: map[string][]byte{prefix + "." + uid: []byte("svc-1")}}
			counts, err := backfill(context.Background(), objects, mappings, index, true)
			require.ErrorContains(t, err, "1 subgroup records failed")
			assert.Equal(t, results{listed: 1, failed: 1}, counts)
			assert.Empty(t, index.puts, "incomplete cleanup must not be silently backfilled")
		})
	}
}

func TestBackfillRejectsLegacyUnassociatedListWithoutCompletionMarker(t *testing.T) {
	uid := "sg-1"
	objects := &fakeKV{entries: map[string][]byte{subgroupPrefix + uid: []byte(`{"parent_id":""}`)}, keys: []string{subgroupPrefix + uid}}
	mappings := &fakeKV{entries: map[string][]byte{constants.KVMappingPrefixSubgroup + "." + uid: []byte(uid)}}
	index := &fakeKV{entries: map[string][]byte{}}
	for _, write := range []bool{false, true} {
		counts, err := backfill(context.Background(), objects, mappings, index, write)
		require.ErrorContains(t, err, "1 subgroup records failed")
		assert.Equal(t, results{listed: 1, failed: 1}, counts)
	}
	assert.Empty(t, index.puts, "missing pointers without durable cleanup proof must not count as complete")
	index.entries[constants.KVMappingPrefixSubgroupUnassociated+"."+uid] = []byte("other-uid")
	counts, err := backfill(context.Background(), objects, mappings, index, true)
	require.ErrorContains(t, err, "1 subgroup records failed")
	assert.Equal(t, results{listed: 1, failed: 1}, counts)
}

func TestBackfillRequiresReassociationBeforeRestoringUnassociatedList(t *testing.T) {
	ctx := context.Background()
	uid := "sg-1"
	objects := &fakeKV{entries: map[string][]byte{subgroupPrefix + uid: []byte(`{"parent_id":"svc-1"}`)}, keys: []string{subgroupPrefix + uid}}
	mappings := &fakeKV{entries: map[string][]byte{constants.KVMappingPrefixSubgroup + "." + uid: []byte(uid)}}
	markerKey := constants.KVMappingPrefixSubgroupUnassociated + "." + uid
	index := &fakeKV{entries: map[string][]byte{
		constants.KVMappingPrefixSubgroupParent + "." + uid:          []byte("svc-1"),
		constants.KVMappingPrefixSubgroupByService + ".svc-1." + uid: []byte(uid),
		markerKey: []byte(uid),
	}}
	for _, write := range []bool{false, true} {
		counts, err := backfill(ctx, objects, mappings, index, write)
		require.ErrorContains(t, err, "1 subgroup records failed")
		assert.Equal(t, results{listed: 1, failed: 1}, counts)
		assert.Equal(t, uid, string(index.entries[markerKey]), "backfill must not clear unassociation proof")
	}
	assert.Empty(t, index.puts)
	assert.Empty(t, mappings.puts, "backfill must not checkpoint before subgroup access is restored")
	// The subgroup handler tombstones the marker only after re-publishing access.
	index.entries[markerKey] = []byte(constants.KVTombstoneMarker)
	again, err := backfill(ctx, objects, mappings, index, true)
	require.NoError(t, err)
	assert.Equal(t, results{listed: 1, skipped: 1}, again)
}

func TestBackfillCannotRestoreParentBeforeReassociationAccess(t *testing.T) {
	ctx := context.Background()
	uid := "sg-1"
	objects := &fakeKV{entries: map[string][]byte{subgroupPrefix + uid: []byte(`{"parent_id":"svc-1"}`)}, keys: []string{subgroupPrefix + uid}}
	mappings := &fakeKV{entries: map[string][]byte{constants.KVMappingPrefixSubgroup + "." + uid: []byte(uid)}}
	markerKey := constants.KVMappingPrefixSubgroupUnassociated + "." + uid
	index := &fakeKV{entries: map[string][]byte{markerKey: []byte(uid)}}
	for _, write := range []bool{false, true} {
		counts, err := backfill(ctx, objects, mappings, index, write)
		require.ErrorContains(t, err, "1 subgroup records failed")
		assert.Equal(t, results{listed: 1, failed: 1}, counts)
	}
	assert.Empty(t, mappings.puts)
	assert.Empty(t, index.puts, "a service-only replay must not expose this list without access")
	// After the subgroup handler restores access and clears the marker, the
	// remaining service-index mappings can safely be backfilled if needed.
	index.entries[markerKey] = []byte(constants.KVTombstoneMarker)
	counts, err := backfill(ctx, objects, mappings, index, true)
	require.NoError(t, err)
	assert.Equal(t, results{listed: 1, processed: 1}, counts)
	assert.Equal(t, uid, string(index.entries[constants.KVMappingPrefixSubgroupByService+".svc-1."+uid]))
}

func TestBackfillRejectsUnfinishedUnassociation(t *testing.T) {
	for _, tt := range []struct {
		name      string
		payload   string
		completed bool
		forward   bool
	}{
		{"associated after interrupted cleanup", `{"parent_id":"svc-1"}`, false, true},
		{"parentless during interrupted cleanup", `{"parent_id":""}`, false, true},
		{"completion recorded but progress not cleared", `{"parent_id":""}`, true, true},
		{"partial publication without forward mapping", `{"parent_id":"svc-1"}`, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			uid := "sg-1"
			objects := &fakeKV{entries: map[string][]byte{subgroupPrefix + uid: []byte(tt.payload)}, keys: []string{subgroupPrefix + uid}}
			mappings := &fakeKV{entries: map[string][]byte{}}
			if tt.forward {
				mappings.entries[constants.KVMappingPrefixSubgroup+"."+uid] = []byte(uid)
			}
			pendingKey := constants.KVMappingPrefixSubgroupUnassociationPending + "." + uid
			index := &fakeKV{entries: map[string][]byte{pendingKey: []byte(uid)}}
			if tt.completed {
				index.entries[constants.KVMappingPrefixSubgroupUnassociated+"."+uid] = []byte(uid)
			}
			for _, write := range []bool{false, true} {
				counts, err := backfill(ctx, objects, mappings, index, write)
				require.ErrorContains(t, err, "1 subgroup records failed")
				assert.Equal(t, results{listed: 1, failed: 1}, counts)
			}
			assert.Empty(t, mappings.puts)
			assert.Empty(t, index.puts)
		})
	}
}

func TestBackfillRejectsInvalidParentType(t *testing.T) {
	uid := "sg-1"
	objects := &fakeKV{entries: map[string][]byte{subgroupPrefix + uid: []byte(`{"parent_id":123}`)}, keys: []string{subgroupPrefix + uid}}
	mappings := &fakeKV{entries: map[string][]byte{constants.KVMappingPrefixSubgroup + "." + uid: []byte(uid)}}
	index := &fakeKV{entries: map[string][]byte{}}
	counts, err := backfill(context.Background(), objects, mappings, index, true)
	require.ErrorContains(t, err, "1 subgroup records failed")
	assert.Equal(t, results{listed: 1, failed: 1}, counts)
	assert.Empty(t, index.puts)
}

func TestBackfillRejectsNullSubgroupObject(t *testing.T) {
	uid := "sg-1"
	objects := &fakeKV{entries: map[string][]byte{subgroupPrefix + uid: []byte(`null`)}, keys: []string{subgroupPrefix + uid}}
	mappings := &fakeKV{entries: map[string][]byte{constants.KVMappingPrefixSubgroup + "." + uid: []byte(uid)}}
	index := &fakeKV{entries: map[string][]byte{}}
	counts, err := backfill(context.Background(), objects, mappings, index, true)
	require.ErrorContains(t, err, "1 subgroup records failed")
	assert.Equal(t, results{listed: 1, failed: 1}, counts)
	assert.Empty(t, index.puts)
}

func TestBackfillInvalidatesEmptyLookupCheckpointBeforeExposingIndex(t *testing.T) {
	ctx := context.Background()
	uid := "sg-1"
	serviceUID := "svc-1"
	checkpointKey := constants.KVMappingPrefixServiceDomainIndexed + "." + serviceUID
	objects := &fakeKV{entries: map[string][]byte{subgroupPrefix + uid: []byte(`{"parent_id":"svc-1"}`)}, keys: []string{subgroupPrefix + uid}}
	mappings := &fakeKV{entries: map[string][]byte{
		constants.KVMappingPrefixSubgroup + "." + uid: []byte(uid),
		checkpointKey: []byte("new.example.test"), // earlier service event saw no lists
	}}
	index := &fakeKV{entries: map[string][]byte{}}
	counts, err := backfill(ctx, objects, mappings, index, false)
	require.NoError(t, err)
	assert.Equal(t, results{listed: 1, processed: 1}, counts)
	assert.Equal(t, "new.example.test", string(mappings.entries[checkpointKey]), "dry run must not invalidate checkpoints")

	mappings.putErr = errors.New("NATS unavailable")
	counts, err = backfill(ctx, objects, mappings, index, true)
	require.Error(t, err)
	assert.Equal(t, 1, counts.failed)
	assert.Empty(t, index.puts, "failed checkpoint invalidation must not expose the list")
	mappings.putErr = nil
	counts, err = backfill(ctx, objects, mappings, index, true)
	require.NoError(t, err)
	assert.Equal(t, results{listed: 1, processed: 1}, counts)
	assert.Equal(t, constants.KVTombstoneMarker, string(mappings.entries[checkpointKey]))
	assert.Equal(t, []string{checkpointKey}, mappings.puts)
	assert.Equal(t, []string{constants.KVMappingPrefixSubgroupByService + ".svc-1." + uid, constants.KVMappingPrefixSubgroupParent + "." + uid}, index.puts)
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
