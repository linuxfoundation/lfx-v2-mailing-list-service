// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
)

type natsMappingReaderWriter struct {
	kv           jetstream.KeyValue
	serviceIndex jetstream.KeyValue
}

// NewMappingReaderWriter wraps the shared mappings and service lookup KV buckets.
// All tombstone marker and key-not-found semantics are encapsulated here so the
// service layer remains free of storage-level concerns.
func NewMappingReaderWriter(kv, serviceIndex jetstream.KeyValue) port.MappingReaderWriter {
	return &natsMappingReaderWriter{kv: kv, serviceIndex: serviceIndex}
}

func (m *natsMappingReaderWriter) bucketFor(key string) jetstream.KeyValue {
	if strings.HasPrefix(key, constants.KVMappingPrefixSubgroupByService+".") ||
		strings.HasPrefix(key, constants.KVMappingPrefixSubgroupParent+".") ||
		strings.HasPrefix(key, constants.KVMappingPrefixSubgroupPreviousService+".") {
		return m.serviceIndex
	}
	return m.kv
}

func (m *natsMappingReaderWriter) ResolveAction(ctx context.Context, key string) model.MessageAction {
	entry, err := m.bucketFor(key).Get(ctx, key)
	if err != nil || entry == nil {
		return model.ActionCreated
	}
	if string(entry.Value()) == constants.KVTombstoneMarker {
		return model.ActionCreated
	}
	return model.ActionUpdated
}

func (m *natsMappingReaderWriter) IsMappingPresent(ctx context.Context, key string) bool {
	entry, err := m.bucketFor(key).Get(ctx, key)
	if err != nil || entry == nil {
		slog.WarnContext(ctx, "mapping key not found", "mapping_key", key)
		return false
	}
	return string(entry.Value()) != constants.KVTombstoneMarker
}

func (m *natsMappingReaderWriter) IsTombstoned(ctx context.Context, key string) bool {
	entry, err := m.bucketFor(key).Get(ctx, key)
	if err != nil || entry == nil {
		slog.WarnContext(ctx, "mapping key not found - treating as not tombstoned", "mapping_key", key)
		return false
	}
	return string(entry.Value()) == constants.KVTombstoneMarker
}

func (m *natsMappingReaderWriter) GetMappingValue(ctx context.Context, key string) (string, bool) {
	value, present, err := m.GetMappingValueWithError(ctx, key)
	if err != nil {
		return "", false
	}
	return value, present
}

func (m *natsMappingReaderWriter) GetMappingValueWithError(ctx context.Context, key string) (string, bool, error) {
	entry, err := m.bucketFor(key).Get(ctx, key)
	if errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrKeyDeleted) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if entry == nil {
		return "", false, nil
	}
	val := string(entry.Value())
	if val == constants.KVTombstoneMarker {
		return "", false, nil
	}
	return val, true, nil
}

func (m *natsMappingReaderWriter) PutMapping(ctx context.Context, key, value string) error {
	_, err := m.bucketFor(key).Put(ctx, key, []byte(value))
	return err
}

func (m *natsMappingReaderWriter) CreateMapping(ctx context.Context, key, value string) error {
	_, err := m.bucketFor(key).Create(ctx, key, []byte(value))
	if errors.Is(err, jetstream.ErrKeyExists) {
		return port.ErrMappingAlreadyExists
	}
	return err
}

func (m *natsMappingReaderWriter) PurgeMapping(ctx context.Context, key string) error {
	return m.bucketFor(key).Purge(ctx, key)
}

func (m *natsMappingReaderWriter) PutTombstone(ctx context.Context, key string) error {
	_, err := m.bucketFor(key).Put(ctx, key, []byte(constants.KVTombstoneMarker))
	return err
}

func (m *natsMappingReaderWriter) ListSubgroupsByService(ctx context.Context, serviceUID string) ([]string, error) {
	if !constants.ValidKVKeySegment(serviceUID) {
		return nil, fmt.Errorf("invalid service UID %q", serviceUID)
	}
	prefix := constants.KVMappingPrefixSubgroupByService + "." + serviceUID + "."
	listCtx, cancel := context.WithCancel(ctx)
	watcher, err := m.serviceIndex.WatchFiltered(listCtx, []string{prefix + ">"}, jetstream.IgnoreDeletes(), jetstream.MetaOnly())
	if errors.Is(err, jetstream.ErrNoKeysFound) {
		cancel()
		return nil, nil
	}
	if err != nil {
		cancel()
		return nil, err
	}
	defer func() {
		// Drain while the context is live: a callback blocked sending to the
		// updates channel must complete before the watcher can close it.
		_ = watcher.Stop()
		for range watcher.Updates() {
		}
		cancel()
	}()

	var uids []string
	seen := make(map[string]bool)
	complete := false
	for entry := range watcher.Updates() {
		if entry == nil { // initial-values replay completed
			complete = true
			break
		}
		key := entry.Key()
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		uid := strings.TrimPrefix(key, prefix)
		if uid == "" || seen[uid] {
			continue
		}
		seen[uid] = true
		// PutTombstone writes a value rather than deleting the key, so the
		// listing can still contain an entry for a moved or deleted subgroup.
		indexedUID, indexed, err := m.GetMappingValueWithError(ctx, key)
		if err != nil {
			return nil, err
		}
		if !indexed || indexedUID != uid {
			continue
		}
		// Listing can include duplicates and moves can leave old index keys
		// until cleanup succeeds. Confirm the parent and subgroup are current.
		parent, present, err := m.GetMappingValueWithError(ctx, constants.KVMappingPrefixSubgroupParent+"."+uid)
		if err != nil {
			return nil, err
		}
		_, live, err := m.GetMappingValueWithError(ctx, constants.KVMappingPrefixSubgroup+"."+uid)
		if err != nil {
			return nil, err
		}
		if present && parent == serviceUID && live {
			uids = append(uids, uid)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !complete {
		return nil, errors.New("subgroup service index watcher closed before initial values completed")
	}
	return uids, nil
}
