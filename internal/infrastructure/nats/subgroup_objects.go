// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"fmt"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/mapconv"
	"github.com/nats-io/nats.go/jetstream"
)

type subgroupObjectReader struct{ kv jetstream.KeyValue }

// NewSubgroupObjectReader binds an existing v1-objects bucket for read-only lookups.
func NewSubgroupObjectReader(kv jetstream.KeyValue) port.SubgroupObjectReader {
	return &subgroupObjectReader{kv: kv}
}

func (r *subgroupObjectReader) GetSubgroup(ctx context.Context, uid string) (map[string]any, bool, error) {
	return r.getObject(ctx, constants.KVObjectPrefixSubgroup, uid)
}

func (r *subgroupObjectReader) GetService(ctx context.Context, uid string) (map[string]any, bool, error) {
	return r.getObject(ctx, constants.KVObjectPrefixService, uid)
}

func (r *subgroupObjectReader) getObject(ctx context.Context, prefix, uid string) (map[string]any, bool, error) {
	if !constants.ValidKVKeySegment(uid) {
		return nil, false, fmt.Errorf("invalid v1 object UID %q", uid)
	}
	entry, err := r.kv.Get(ctx, prefix+uid)
	if errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrKeyDeleted) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	data, err := mapconv.DecodeMapData(entry.Value())
	if err != nil {
		return nil, false, fmt.Errorf("decode v1 object %s%s: %w", prefix, uid, err)
	}
	if _, deleted := data[constants.KVObjectSoftDeletedAt]; deleted {
		return nil, false, nil
	}
	return data, true, nil
}
