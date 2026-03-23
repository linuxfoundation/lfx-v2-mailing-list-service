// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// FakeMappingStore is an in-memory MappingReaderWriter for unit tests.
type FakeMappingStore struct {
	values     map[string]string
	tombstones map[string]bool
}

var _ port.MappingReaderWriter = (*FakeMappingStore)(nil)

// NewFakeMappingStore returns an empty FakeMappingStore.
func NewFakeMappingStore() *FakeMappingStore {
	return &FakeMappingStore{values: make(map[string]string), tombstones: make(map[string]bool)}
}

// Set pre-populates a key/value pair (helper for test setup).
func (f *FakeMappingStore) Set(key, value string) { f.values[key] = value }

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

func (f *FakeMappingStore) GetMappingValue(_ context.Context, key string) (string, bool) {
	if f.tombstones[key] {
		return "", false
	}
	v, ok := f.values[key]
	return v, ok
}

func (f *FakeMappingStore) PutMapping(_ context.Context, key, value string) error {
	f.values[key] = value
	return nil
}

func (f *FakeMappingStore) PutTombstone(_ context.Context, key string) error {
	f.tombstones[key] = true
	delete(f.values, key)
	return nil
}
