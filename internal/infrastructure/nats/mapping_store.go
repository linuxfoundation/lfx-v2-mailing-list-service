// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
)

type natsMappingReaderWriter struct {
	kv jetstream.KeyValue
}

// NewMappingReaderWriter wraps a JetStream KeyValue bucket as a port.MappingReaderWriter.
// All tombstone marker and key-not-found semantics are encapsulated here so the
// service layer remains free of storage-level concerns.
func NewMappingReaderWriter(kv jetstream.KeyValue) port.MappingReaderWriter {
	return &natsMappingReaderWriter{kv: kv}
}

func (m *natsMappingReaderWriter) ResolveAction(ctx context.Context, key string) model.MessageAction {
	entry, err := m.kv.Get(ctx, key)
	if err != nil || entry == nil {
		return model.ActionCreated
	}
	if string(entry.Value()) == constants.KVTombstoneMarker {
		return model.ActionCreated
	}
	return model.ActionUpdated
}

func (m *natsMappingReaderWriter) IsMappingPresent(ctx context.Context, key string) bool {
	entry, err := m.kv.Get(ctx, key)
	if err != nil || entry == nil {
		slog.WarnContext(ctx, "mapping key not found in v1-mappings", "mapping_key", key)
		return false
	}
	return string(entry.Value()) != constants.KVTombstoneMarker
}

func (m *natsMappingReaderWriter) IsTombstoned(ctx context.Context, key string) bool {
	entry, err := m.kv.Get(ctx, key)
	if err != nil || entry == nil {
		slog.WarnContext(ctx, "mapping key not found in v1-mappings - treating as not tombstoned", "mapping_key", key)
		return false
	}
	return string(entry.Value()) == constants.KVTombstoneMarker
}

func (m *natsMappingReaderWriter) GetMappingValue(ctx context.Context, key string) (string, bool) {
	entry, err := m.kv.Get(ctx, key)
	if err != nil || entry == nil {
		return "", false
	}
	val := string(entry.Value())
	if val == constants.KVTombstoneMarker {
		return "", false
	}
	return val, true
}

func (m *natsMappingReaderWriter) PutMapping(ctx context.Context, key, value string) error {
	_, err := m.kv.Put(ctx, key, []byte(value))
	return err
}

func (m *natsMappingReaderWriter) PutTombstone(ctx context.Context, key string) error {
	_, err := m.kv.Put(ctx, key, []byte(constants.KVTombstoneMarker))
	return err
}
