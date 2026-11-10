// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bindingKV is a minimal stub jetstream.KeyValue for natsSyncApplyJobBindingStore, mirroring the
// subgroupIndexKV pattern in mapping_store_test.go. Embedding jetstream.KeyValue means any method
// this adapter doesn't call panics if exercised, instead of silently compiling to a no-op.
type bindingKV struct {
	jetstream.KeyValue
	values map[string][]byte
	getErr error
	putErr error
}

func (kv *bindingKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	if kv.getErr != nil {
		return nil, kv.getErr
	}
	value, ok := kv.values[key]
	if !ok {
		return nil, jetstream.ErrKeyNotFound
	}
	return bindingKVEntry{value: value}, nil
}

func (kv *bindingKV) Put(_ context.Context, key string, value []byte) (uint64, error) {
	if kv.putErr != nil {
		return 0, kv.putErr
	}
	if kv.values == nil {
		kv.values = make(map[string][]byte)
	}
	kv.values[key] = value
	return 1, nil
}

type bindingKVEntry struct {
	jetstream.KeyValueEntry
	value []byte
}

func (e bindingKVEntry) Value() []byte { return e.value }

func TestSyncApplyJobBindingStore_PutGetRoundTrip(t *testing.T) {
	kv := &bindingKV{}
	store := NewSyncApplyJobBindingStore(kv)
	ctx := context.Background()

	err := store.Put(ctx, "job-1", port.SyncApplyJobBinding{MailingListID: "ml-1", CommitteeUID: "committee-1"})
	require.NoError(t, err)

	binding, found, err := store.Get(ctx, "job-1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "ml-1", binding.MailingListID)
	assert.Equal(t, "committee-1", binding.CommitteeUID)
}

func TestSyncApplyJobBindingStore_Get_AbsentBindingCases(t *testing.T) {
	for name, getErr := range map[string]error{
		"key not found": jetstream.ErrKeyNotFound,
		"key deleted":   jetstream.ErrKeyDeleted,
		"invalid key":   jetstream.ErrInvalidKey,
	} {
		t.Run(name, func(t *testing.T) {
			store := NewSyncApplyJobBindingStore(&bindingKV{getErr: getErr})
			binding, found, err := store.Get(context.Background(), "job-1")
			require.NoError(t, err)
			assert.False(t, found)
			assert.Zero(t, binding)
		})
	}
}

func TestSyncApplyJobBindingStore_Get_MalformedJSON(t *testing.T) {
	kv := &bindingKV{values: map[string][]byte{"job-1": []byte("not json")}}
	store := NewSyncApplyJobBindingStore(kv)
	_, found, err := store.Get(context.Background(), "job-1")
	require.Error(t, err)
	assert.False(t, found)
}

func TestSyncApplyJobBindingStore_Get_StorageError(t *testing.T) {
	store := NewSyncApplyJobBindingStore(&bindingKV{getErr: errors.New("connection timeout")})
	_, found, err := store.Get(context.Background(), "job-1")
	require.Error(t, err)
	assert.False(t, found)
}

func TestSyncApplyJobBindingStore_Put_StorageError(t *testing.T) {
	store := NewSyncApplyJobBindingStore(&bindingKV{putErr: errors.New("connection timeout")})
	err := store.Put(context.Background(), "job-1", port.SyncApplyJobBinding{MailingListID: "ml-1", CommitteeUID: "committee-1"})
	require.Error(t, err)
}
