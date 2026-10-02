// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type subgroupIndexKV struct {
	jetstream.KeyValue
	keys        []string
	values      map[string]string
	filter      string
	err         error
	puts        []string
	asyncLister bool
	finished    chan struct{}
	getErr      error
	ready       chan struct{}
	watcherDone chan struct{}
	truncated   bool
}

func (kv *subgroupIndexKV) WatchFiltered(ctx context.Context, filters []string, _ ...jetstream.WatchOpt) (jetstream.KeyWatcher, error) {
	kv.filter = filters[0]
	if kv.err != nil {
		return nil, kv.err
	}
	ch := make(chan jetstream.KeyValueEntry, 256)
	if kv.asyncLister {
		stopped := make(chan struct{})
		go func() {
			defer close(kv.finished)
			defer close(ch)
			for i, key := range kv.keys {
				if i == 257 {
					close(kv.ready)
				}
				select {
				case <-stopped:
					return
				default:
				}
				ch <- subgroupIndexEntry{key: key}
			}
			ch <- nil
		}()
		return subgroupIndexWatcher{ch: ch, stop: func() { close(stopped) }}, nil
	}
	for _, key := range kv.keys {
		ch <- subgroupIndexEntry{key: key}
	}
	if !kv.truncated {
		ch <- nil
	}
	close(ch)
	return subgroupIndexWatcher{ch: ch}, nil
}

type subgroupIndexWatcher struct {
	ch   <-chan jetstream.KeyValueEntry
	stop func()
}

func (w subgroupIndexWatcher) Updates() <-chan jetstream.KeyValueEntry { return w.ch }
func (w subgroupIndexWatcher) Stop() error {
	if w.stop != nil {
		w.stop()
	}
	return nil
}

func (kv *subgroupIndexKV) ListKeysFiltered(ctx context.Context, filters ...string) (jetstream.KeyLister, error) {
	kv.filter = filters[0]
	if kv.err != nil {
		return nil, kv.err
	}
	if kv.asyncLister {
		ch := make(chan string, 256)
		if kv.watcherDone != nil {
			updates := make(chan string, 256)
			stopped := make(chan struct{})
			go func() {
				defer close(kv.watcherDone)
				defer close(updates)
				for i, key := range kv.keys {
					if i == 400 {
						close(kv.ready) // both buffers have substantial pending keys
					}
					select {
					case <-stopped:
						return
					default:
					}
					updates <- key // like the KV watcher's callback, not cancellable
				}
			}()
			go func() {
				defer close(kv.finished)
				defer close(ch)
				for {
					// Prefer cancellation when both it and an update are ready.
					// This models the branch Go's select is allowed to take.
					select {
					case <-ctx.Done():
						return
					default:
					}
					select {
					case key, ok := <-updates:
						if !ok {
							return
						}
						ch <- key // like nats.go's unguarded key send
					case <-ctx.Done():
						return
					}
				}
			}()
			return subgroupIndexLister{ch: ch, stop: func() { close(stopped) }}, nil
		}
		go func() {
			defer close(kv.finished)
			defer close(ch)
			for i, key := range kv.keys {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if i == 257 && kv.ready != nil {
					close(kv.ready) // the 256-entry buffer is full after the first key is read
				}
				// Match nats.go v1.53.1: this send does not select on ctx.Done.
				ch <- key
			}
		}()
		return subgroupIndexLister{ch: ch}, nil
	}
	ch := make(chan string, len(kv.keys))
	for _, key := range kv.keys {
		ch <- key
	}
	close(ch)
	return subgroupIndexLister{ch: ch}, nil
}

func (kv *subgroupIndexKV) Get(ctx context.Context, key string) (jetstream.KeyValueEntry, error) {
	if kv.getErr != nil {
		if kv.ready != nil {
			select {
			case <-kv.ready:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return nil, kv.getErr
	}
	value, ok := kv.values[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return subgroupIndexEntry{value: value}, nil
}

func (kv *subgroupIndexKV) Put(_ context.Context, key string, value []byte) (uint64, error) {
	kv.puts = append(kv.puts, key)
	if kv.values == nil {
		kv.values = make(map[string]string)
	}
	kv.values[key] = string(value)
	return 1, nil
}

type subgroupIndexLister struct {
	ch   <-chan string
	stop func()
}

func (l subgroupIndexLister) Keys() <-chan string { return l.ch }
func (l subgroupIndexLister) Stop() error {
	if l.stop != nil {
		l.stop()
	}
	return nil
}

type subgroupIndexEntry struct {
	jetstream.KeyValueEntry
	value string
	key   string
}

func (e subgroupIndexEntry) Value() []byte { return []byte(e.value) }
func (e subgroupIndexEntry) Key() string   { return e.key }

// The test KV embeds the remaining JetStream methods, which this lookup never calls.
func TestListSubgroupsByService(t *testing.T) {
	const serviceUID = "svc-1"
	prefix := constants.KVMappingPrefixSubgroupByService + "." + serviceUID + "."
	parentPrefix := constants.KVMappingPrefixSubgroupParent + "."
	subgroupPrefix := constants.KVMappingPrefixSubgroup + "."
	index := &subgroupIndexKV{
		keys: []string{prefix + "sg-1", prefix + "sg-2", prefix + "sg-1", prefix + "sg-3", prefix + "sg-4", prefix + "sg-5"},
		values: map[string]string{
			prefix + "sg-1":       "sg-1",
			prefix + "sg-2":       "sg-2",
			prefix + "sg-3":       "sg-3",
			prefix + "sg-4":       constants.KVTombstoneMarker,
			prefix + "sg-5":       "different-uid",
			parentPrefix + "sg-1": serviceUID,
			parentPrefix + "sg-2": "svc-2", // moved to another service
			parentPrefix + "sg-3": serviceUID,
			parentPrefix + "sg-4": serviceUID, // tombstoned index, parent still live
			parentPrefix + "sg-5": serviceUID, // mismatched index value
		},
	}
	mappings := &subgroupIndexKV{values: map[string]string{
		subgroupPrefix + "sg-1": "sg-1",
		subgroupPrefix + "sg-2": "sg-2",
		subgroupPrefix + "sg-3": constants.KVTombstoneMarker,
		subgroupPrefix + "sg-4": "sg-4",
		subgroupPrefix + "sg-5": "sg-5",
	}}
	uids, err := NewMappingReaderWriter(mappings, index).ListSubgroupsByService(context.Background(), serviceUID)
	require.NoError(t, err)
	assert.Equal(t, []string{"sg-1"}, uids)
	assert.Equal(t, prefix+">", index.filter)
}

func TestListSubgroupsByService_NoMatchesOrFailure(t *testing.T) {
	kv := &subgroupIndexKV{err: jetstream.ErrNoKeysFound}
	uids, err := NewMappingReaderWriter(&subgroupIndexKV{}, kv).ListSubgroupsByService(context.Background(), "svc-1")
	require.NoError(t, err)
	assert.Empty(t, uids)

	kv.err = errors.New("connection timeout")
	_, err = NewMappingReaderWriter(&subgroupIndexKV{}, kv).ListSubgroupsByService(context.Background(), "svc-1")
	require.Error(t, err)
}

func TestListSubgroupsByService_RejectsInvalidServiceUID(t *testing.T) {
	for _, uid := range []string{"", "*", ">", "a.b", "a*b", "a>b", "a/b"} {
		t.Run(uid, func(t *testing.T) {
			index := &subgroupIndexKV{}
			_, err := NewMappingReaderWriter(&subgroupIndexKV{}, index).ListSubgroupsByService(context.Background(), uid)
			require.ErrorContains(t, err, "invalid service UID")
			assert.Empty(t, index.filter, "must reject before issuing a NATS wildcard query")
		})
	}
}

func TestListSubgroupsByService_RejectsIncompleteWatch(t *testing.T) {
	index := &subgroupIndexKV{
		keys:      []string{constants.KVMappingPrefixSubgroupByService + ".svc-1.sg-1"},
		truncated: true,
	}
	_, err := NewMappingReaderWriter(&subgroupIndexKV{}, index).ListSubgroupsByService(context.Background(), "svc-1")
	require.ErrorContains(t, err, "before initial values completed")
}

func TestListSubgroupsByService_DrainsListerOnReadFailure(t *testing.T) {
	const serviceUID = "svc-1"
	prefix := constants.KVMappingPrefixSubgroupByService + "." + serviceUID + "."
	index := &subgroupIndexKV{
		asyncLister: true,
		finished:    make(chan struct{}),
		ready:       make(chan struct{}),
		keys:        make([]string, 600),
	}
	for i := range index.keys {
		index.keys[i] = prefix + fmt.Sprint(i)
	}
	index.keys[0] = prefix + "first"
	readErr := errors.New("mapping read failed")
	index.getErr = readErr
	store := NewMappingReaderWriter(&subgroupIndexKV{}, index)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := store.ListSubgroupsByService(ctx, serviceUID)
	require.ErrorIs(t, err, readErr)
	select {
	case <-index.finished:
	case <-ctx.Done():
		t.Fatal("key lister producer remained blocked after lookup failed")
	}
}

func TestListSubgroupsByService_DrainsWatcherBeforeCancelling(t *testing.T) {
	const serviceUID = "svc-1"
	prefix := constants.KVMappingPrefixSubgroupByService + "." + serviceUID + "."
	index := &subgroupIndexKV{
		asyncLister: true,
		ready:       make(chan struct{}),
		keys:        make([]string, 900),
		getErr:      errors.New("mapping read failed"),
	}
	index.finished = make(chan struct{})
	for i := range index.keys {
		index.keys[i] = prefix + fmt.Sprint(i)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := NewMappingReaderWriter(&subgroupIndexKV{}, index).ListSubgroupsByService(ctx, serviceUID)
	require.ErrorIs(t, err, index.getErr)
	select {
	case <-index.finished:
	case <-ctx.Done():
		t.Fatal("KV watcher callback remained blocked after lookup failed")
	}
}

func TestMappingStoreRoutesServiceIndexToDedicatedBucket(t *testing.T) {
	ctx := context.Background()
	mappings := &subgroupIndexKV{}
	index := &subgroupIndexKV{}
	store := NewMappingReaderWriter(mappings, index)
	require.NoError(t, store.PutMapping(ctx, constants.KVMappingPrefixSubgroup+".sg-1", "sg-1"))
	require.NoError(t, store.PutMapping(ctx, constants.KVMappingPrefixSubgroupParent+".sg-1", "svc-1"))
	require.NoError(t, store.PutMapping(ctx, constants.KVMappingPrefixSubgroupByService+".svc-1.sg-1", "sg-1"))
	assert.Equal(t, []string{constants.KVMappingPrefixSubgroup + ".sg-1"}, mappings.puts)
	assert.Equal(t, []string{constants.KVMappingPrefixSubgroupParent + ".sg-1", constants.KVMappingPrefixSubgroupByService + ".svc-1.sg-1"}, index.puts)
	parent, present, err := store.GetMappingValueWithError(ctx, constants.KVMappingPrefixSubgroupParent+".sg-1")
	require.NoError(t, err)
	assert.True(t, present)
	assert.Equal(t, "svc-1", parent)
}
