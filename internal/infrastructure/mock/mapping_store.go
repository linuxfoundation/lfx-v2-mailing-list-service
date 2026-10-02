// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"errors"
	"strings"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
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
func (f *FakeMappingStore) SimulatePutError(key string, err error) {
	if err == nil {
		delete(f.putErrors, key)
		return
	}
	f.putErrors[key] = err
}

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
	if value, ok := f.values[key]; ok && value != constants.KVTombstoneMarker {
		return model.ActionUpdated
	}
	return model.ActionCreated
}

func (f *FakeMappingStore) IsMappingPresent(_ context.Context, key string) bool {
	value, ok := f.values[key]
	return ok && value != constants.KVTombstoneMarker
}

func (f *FakeMappingStore) IsTombstoned(_ context.Context, key string) bool {
	return f.tombstones[key] || f.values[key] == constants.KVTombstoneMarker
}

func (f *FakeMappingStore) GetMappingValue(ctx context.Context, key string) (string, bool) {
	value, present, err := f.GetMappingValueWithError(ctx, key)
	if err != nil {
		return "", false
	}
	return value, present
}

func (f *FakeMappingStore) GetMappingValueWithError(_ context.Context, key string) (string, bool, error) {
	if f.tombstones[key] || f.getErrors[key] || f.values[key] == constants.KVTombstoneMarker {
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
	delete(f.tombstones, key)
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
	f.values[key] = constants.KVTombstoneMarker
	return nil
}

func (f *FakeMappingStore) ListSubgroupsByService(ctx context.Context, serviceUID string) ([]string, error) {
	prefix := constants.KVMappingPrefixSubgroupByService + "." + serviceUID + "."
	var uids []string
	for key, indexedUID := range f.values {
		if !strings.HasPrefix(key, prefix) || f.tombstones[key] {
			continue
		}
		uid := strings.TrimPrefix(key, prefix)
		if uid == "" || indexedUID != uid {
			continue
		}
		parent, present, err := f.GetMappingValueWithError(ctx, constants.KVMappingPrefixSubgroupParent+"."+uid)
		if err != nil {
			return nil, err
		}
		_, live, err := f.GetMappingValueWithError(ctx, constants.KVMappingPrefixSubgroup+"."+uid)
		if err != nil {
			return nil, err
		}
		if present && parent == serviceUID && live {
			uids = append(uids, uid)
		}
	}
	return uids, nil
}
