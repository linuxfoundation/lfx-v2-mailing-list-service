// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	pkgerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type getKV struct {
	jetstream.KeyValue
	entry jetstream.KeyValueEntry
	err   error
}

func (kv *getKV) Get(context.Context, string) (jetstream.KeyValueEntry, error) {
	return kv.entry, kv.err
}

type valueEntry struct {
	jetstream.KeyValueEntry
	value string
}

func (e valueEntry) Value() []byte { return []byte(e.value) }

func TestLookup(t *testing.T) {
	tests := []struct {
		name      string
		kv        *getKV
		wantState port.MappingState
		wantValue string
		wantErr   bool
	}{
		{name: "key not found", kv: &getKV{err: jetstream.ErrKeyNotFound}, wantState: port.MappingAbsent},
		{name: "key deleted", kv: &getKV{err: jetstream.ErrKeyDeleted}, wantState: port.MappingAbsent},
		{name: "nil entry", kv: &getKV{}, wantState: port.MappingAbsent},
		{name: "live value", kv: &getKV{entry: valueEntry{value: "uid-1"}}, wantState: port.MappingPresent, wantValue: "uid-1"},
		{name: "tombstone", kv: &getKV{entry: valueEntry{value: constants.KVTombstoneMarker}}, wantState: port.MappingTombstoned},
		{name: "read failure is an error, not absent", kv: &getKV{err: errors.New("boom")}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewMappingReaderWriter(tt.kv, tt.kv)
			got, err := store.Lookup(context.Background(), "k")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantState, got.State)
			assert.Equal(t, tt.wantValue, got.Value)
		})
	}
}

func TestClassifyKVError(t *testing.T) {
	plain := errors.New("boom")

	tests := []struct {
		name          string
		err           error
		wantTransient bool
		wantPermanent bool
	}{
		{name: "nil"},
		{name: "unknown stays unclassified", err: plain},
		{name: "invalid key", err: jetstream.ErrInvalidKey, wantPermanent: true},
		{name: "api 400", err: &jetstream.APIError{Code: 400}, wantPermanent: true},
		{name: "api 404", err: &jetstream.APIError{Code: 404}, wantPermanent: true},
		{name: "api 408 timeout", err: &jetstream.APIError{Code: 408}, wantTransient: true},
		{name: "api 429 throttled", err: &jetstream.APIError{Code: 429}, wantTransient: true},
		{name: "api 503", err: &jetstream.APIError{Code: 503}, wantTransient: true},
		{name: "context deadline", err: context.DeadlineExceeded, wantTransient: true},
		{name: "nats timeout", err: nats.ErrTimeout, wantTransient: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyKVError(tt.err)
			assert.Equal(t, tt.wantPermanent, pkgerrors.IsPermanent(got))
			assert.Equal(t, tt.wantTransient, pkgerrors.IsTransient(got))
			if tt.err != nil {
				assert.ErrorIs(t, got, tt.err)
			}
		})
	}
}

func TestPutMappingClassifiesErrors(t *testing.T) {
	kv := &putKV{err: &jetstream.APIError{Code: 400}}
	err := NewMappingReaderWriter(kv, kv).PutMapping(context.Background(), "k", "v")
	require.Error(t, err)
	assert.True(t, pkgerrors.IsPermanent(err))
}

type putKV struct {
	jetstream.KeyValue
	err error
}

func (kv *putKV) Put(context.Context, string, []byte) (uint64, error) { return 0, kv.err }
