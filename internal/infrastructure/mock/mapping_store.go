// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"errors"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// FakeMappingStore is an in-memory MappingReaderWriter for unit tests.
type FakeMappingStore struct {
	values          map[string]string
	tombstones      map[string]bool
	getErrors       map[string]bool
	putErrors       map[string]error
	tombstoneErrors map[string]error
}

var _ port.MappingReaderWriter = (*FakeMappingStore)(nil)

// NewFakeMappingStore returns an empty FakeMappingStore.
func NewFakeMappingStore() *FakeMappingStore {
	return &FakeMappingStore{
		values:          make(map[string]string),
		tombstones:      make(map[string]bool),
		getErrors:       make(map[string]bool),
		putErrors:       make(map[string]error),
		tombstoneErrors: make(map[string]error),
	}
}

// Set pre-populates a key/value pair (helper for test setup).
func (f *FakeMappingStore) Set(key, value string) { f.values[key] = value }

// SimulateGetError causes GetMappingValue to return ("", false) for the given key,
// simulating a transient KV read error even when the key exists.
func (f *FakeMappingStore) SimulateGetError(key string) { f.getErrors[key] = true }

// SimulatePutError causes PutMapping to return the given error for the given key.
func (f *FakeMappingStore) SimulatePutError(key string, err error) { f.putErrors[key] = err }

// SimulateTombstoneError causes PutTombstone to return the given error for the key.
// Pass nil to clear a previously configured error.
func (f *FakeMappingStore) SimulateTombstoneError(key string, err error) {
	if err == nil {
		delete(f.tombstoneErrors, key)
		return
	}
	f.tombstoneErrors[key] = err
}

func (f *FakeMappingStore) ResolveAction(_ context.Context, key string) model.MessageAction {
	if _, ok := f.values[key]; ok {
		return model.ActionUpdated
	}
	return model.ActionCreated
}

func (f *FakeMappingStore) IsMappingPresent(_ context.Context, key string) bool {
	_, ok := f.values[key]
	return ok && !f.tombstones[key]
}

func (f *FakeMappingStore) IsTombstoned(_ context.Context, key string) bool {
	return f.tombstones[key]
}

func (f *FakeMappingStore) GetMappingValue(ctx context.Context, key string) (string, bool) {
	value, present, err := f.GetMappingValueWithError(ctx, key)
	if err != nil {
		return "", false
	}
	return value, present
}

func (f *FakeMappingStore) GetMappingValueWithError(_ context.Context, key string) (string, bool, error) {
	if f.tombstones[key] || f.getErrors[key] {
		if f.getErrors[key] {
			return "", false, errors.New("simulated mapping read error")
		}
		return "", false, nil
	}
	v, ok := f.values[key]
	return v, ok, nil
}

func (f *FakeMappingStore) PutMapping(_ context.Context, key, value string) error {
	if err, ok := f.putErrors[key]; ok {
		return err
	}
	f.values[key] = value
	return nil
}

func (f *FakeMappingStore) CreateMapping(_ context.Context, key, value string) error {
	if _, exists := f.values[key]; exists {
		return port.ErrMappingAlreadyExists
	}
	f.values[key] = value
	return nil
}

func (f *FakeMappingStore) PurgeMapping(_ context.Context, key string) error {
	delete(f.values, key)
	return nil
}

func (f *FakeMappingStore) PutTombstone(_ context.Context, key string) error {
	if err, ok := f.tombstoneErrors[key]; ok {
		return err
	}
	f.tombstones[key] = true
	delete(f.values, key)
	return nil
}
