// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"errors"
	"strings"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// FakeMappingStore is an in-memory MappingReaderWriter for unit tests.
type FakeMappingStore struct {
	values          map[string]string
	tombstones      map[string]bool
	getErrors       map[string]bool
	getErrorValues  map[string]error
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
		getErrorValues:  make(map[string]error),
		putErrors:       make(map[string]error),
		tombstoneErrors: make(map[string]error),
	}
}

// Set pre-populates a key/value pair (helper for test setup).
func (f *FakeMappingStore) Set(key, value string) { f.values[key] = value }

// SimulateGetError causes Lookup to return an error for the given key, simulating
// a KV read failure even when the key exists.
func (f *FakeMappingStore) SimulateGetError(key string) { f.getErrors[key] = true }

// SimulateGetErrorWith is like SimulateGetError but Lookup returns err, so tests
// can inject a typed transient or permanent failure. Pass nil to clear it.
func (f *FakeMappingStore) SimulateGetErrorWith(key string, err error) {
	if err == nil {
		delete(f.getErrors, key)
		delete(f.getErrorValues, key)
		return
	}
	f.getErrors[key] = true
	f.getErrorValues[key] = err
}

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

// Lookup reports the state of key, or the error configured with SimulateGetError
// or SimulateGetErrorWith.
func (f *FakeMappingStore) Lookup(_ context.Context, key string) (port.MappingEntry, error) {
	if f.getErrors[key] {
		if err, ok := f.getErrorValues[key]; ok {
			return port.MappingEntry{}, err
		}
		return port.MappingEntry{}, errors.New("simulated mapping read error")
	}
	if f.tombstones[key] || f.values[key] == constants.KVTombstoneMarker {
		return port.MappingEntry{State: port.MappingTombstoned}, nil
	}
	if v, ok := f.values[key]; ok {
		return port.MappingEntry{State: port.MappingPresent, Value: v}, nil
	}
	return port.MappingEntry{State: port.MappingAbsent}, nil
}

// GetMappingValue returns the stored value and true when key holds a live value.
// It is a test inspection helper and is not part of port.MappingReader.
func (f *FakeMappingStore) GetMappingValue(ctx context.Context, key string) (string, bool) {
	entry, err := f.Lookup(ctx, key)
	if err != nil {
		return "", false
	}
	return entry.Value, entry.Present()
}

// IsTombstoned reports whether key holds the deletion marker. It is a test
// inspection helper and is not part of port.MappingReader.
func (f *FakeMappingStore) IsTombstoned(_ context.Context, key string) bool {
	return f.tombstones[key] || f.values[key] == constants.KVTombstoneMarker
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
		parent, err := f.Lookup(ctx, constants.KVMappingPrefixSubgroupParent+"."+uid)
		if err != nil {
			return nil, err
		}
		live, err := f.Lookup(ctx, constants.KVMappingPrefixSubgroup+"."+uid)
		if err != nil {
			return nil, err
		}
		if parent.Present() && parent.Value == serviceUID && live.Present() {
			uids = append(uids, uid)
		}
	}
	return uids, nil
}
