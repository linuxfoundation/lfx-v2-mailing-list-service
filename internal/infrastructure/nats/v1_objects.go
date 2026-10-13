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

type v1ObjectReader struct{ kv jetstream.KeyValue }

// NewV1ObjectReader binds an existing v1-objects bucket for read-only lookups.
func NewV1ObjectReader(kv jetstream.KeyValue) port.V1ObjectReader {
	return &v1ObjectReader{kv: kv}
}

func (r *v1ObjectReader) GetSubgroup(ctx context.Context, uid string) (map[string]any, bool, error) {
	return r.getObject(ctx, constants.KVObjectPrefixSubgroup, uid)
}

func (r *v1ObjectReader) GetService(ctx context.Context, uid string) (map[string]any, bool, error) {
	return r.getObject(ctx, constants.KVObjectPrefixService, uid)
}

func (r *v1ObjectReader) getObject(ctx context.Context, prefix, uid string) (map[string]any, bool, error) {
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
		return nil, false, fmt.Errorf("%w %s%s: %v", port.ErrV1ObjectDecode, prefix, uid, err)
	}
	if data == nil {
		return nil, false, fmt.Errorf("%w %s%s: null object", port.ErrV1ObjectDecode, prefix, uid)
	}
	if _, deleted := data[constants.KVObjectSoftDeletedAt]; deleted {
		return nil, false, nil
	}
	return data, true, nil
}
