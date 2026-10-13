// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	msgpack "github.com/vmihailenco/msgpack/v5"
)

type subgroupObjectKV struct {
	jetstream.KeyValue
	value []byte
	err   error
	key   string
}

func (kv *subgroupObjectKV) Get(_ context.Context, key string) (jetstream.KeyValueEntry, error) {
	kv.key = key
	if kv.err != nil {
		return nil, kv.err
	}
	return subgroupIndexEntry{value: string(kv.value)}, nil
}

func TestV1ObjectReader(t *testing.T) {
	ctx := context.Background()
	kv := &subgroupObjectKV{value: []byte(`{"parent_id":"svc-1","title":"List"}`)}
	reader := NewV1ObjectReader(kv)
	data, present, err := reader.GetSubgroup(ctx, "ml-1")
	require.NoError(t, err)
	assert.True(t, present)
	assert.Equal(t, "List", data["title"])
	assert.Equal(t, constants.KVObjectPrefixSubgroup+"ml-1", kv.key)

	kv.value = []byte(`{"project_id":"project-sfid","domain":"current.example.test"}`)
	service, present, err := reader.GetService(ctx, "svc-1")
	require.NoError(t, err)
	assert.True(t, present)
	assert.Equal(t, "current.example.test", service["domain"])
	assert.Equal(t, constants.KVObjectPrefixService+"svc-1", kv.key)

	kv.value, err = msgpack.Marshal(map[string]any{"parent_id": "svc-1"})
	require.NoError(t, err)
	data, present, err = reader.GetSubgroup(ctx, "ml-1")
	require.NoError(t, err)
	assert.True(t, present)
	assert.Equal(t, "svc-1", data["parent_id"])

	kv.value = []byte(`{"_sdc_deleted_at":"2026-01-01"}`)
	_, present, err = reader.GetSubgroup(ctx, "ml-1")
	require.NoError(t, err)
	assert.False(t, present)

	kv.err = jetstream.ErrKeyNotFound
	_, present, err = reader.GetSubgroup(ctx, "ml-1")
	require.NoError(t, err)
	assert.False(t, present)
	kv.err = jetstream.ErrKeyDeleted
	_, present, err = reader.GetService(ctx, "svc-1")
	require.NoError(t, err)
	assert.False(t, present)
	kv.err = errors.New("connection unavailable")
	_, _, err = reader.GetSubgroup(ctx, "ml-1")
	require.Error(t, err)
	assert.NotErrorIs(t, err, port.ErrV1ObjectDecode)
	kv.err = nil
	kv.value = []byte("{bad")
	_, _, err = reader.GetSubgroup(ctx, "ml-1")
	require.ErrorIs(t, err, port.ErrV1ObjectDecode)
	for _, payload := range [][]byte{[]byte(`null`), []byte{0xc0}} { // JSON and msgpack nil
		kv.value = payload
		_, present, err = reader.GetSubgroup(ctx, "ml-1")
		require.ErrorIs(t, err, port.ErrV1ObjectDecode)
		assert.False(t, present)
	}
	_, _, err = reader.GetSubgroup(ctx, "bad.uid")
	require.Error(t, err)
	assert.NotErrorIs(t, err, port.ErrV1ObjectDecode)
}
