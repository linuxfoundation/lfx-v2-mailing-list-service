// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	pkgerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/nats-io/nats.go/jetstream"
)

// syncApplyJobBindingValue is the JSON shape stored in the KV bucket, keyed by ITX job ID.
type syncApplyJobBindingValue struct {
	MailingListID string `json:"ml"`
	CommitteeUID  string `json:"c"`
}

type natsSyncApplyJobBindingStore struct {
	kv jetstream.KeyValue
}

// NewSyncApplyJobBindingStore wraps the sync-apply-job-bindings KV bucket.
func NewSyncApplyJobBindingStore(kv jetstream.KeyValue) port.SyncApplyJobBindingStore {
	return &natsSyncApplyJobBindingStore{kv: kv}
}

func (s *natsSyncApplyJobBindingStore) Put(ctx context.Context, itxJobID string, binding port.SyncApplyJobBinding) error {
	data, err := json.Marshal(syncApplyJobBindingValue{MailingListID: binding.MailingListID, CommitteeUID: binding.CommitteeUID})
	if err != nil {
		return pkgerrors.NewUnexpected("failed to encode sync-apply job binding", err)
	}
	if _, err := s.kv.Put(ctx, itxJobID, data); err != nil {
		return classifyKVError(fmt.Errorf("failed to record sync-apply job binding: %w", err))
	}
	return nil
}

func (s *natsSyncApplyJobBindingStore) Get(ctx context.Context, itxJobID string) (port.SyncApplyJobBinding, bool, error) {
	entry, err := s.kv.Get(ctx, itxJobID)
	if errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrKeyDeleted) {
		return port.SyncApplyJobBinding{}, false, nil
	}
	if err != nil {
		return port.SyncApplyJobBinding{}, false, classifyKVError(fmt.Errorf("failed to read sync-apply job binding: %w", err))
	}
	var v syncApplyJobBindingValue
	if err := json.Unmarshal(entry.Value(), &v); err != nil {
		return port.SyncApplyJobBinding{}, false, pkgerrors.NewUnexpected("failed to decode sync-apply job binding", err)
	}
	return port.SyncApplyJobBinding{MailingListID: v.MailingListID, CommitteeUID: v.CommitteeUID}, true, nil
}
