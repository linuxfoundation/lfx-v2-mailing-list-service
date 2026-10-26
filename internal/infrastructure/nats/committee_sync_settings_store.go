// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"errors"
	"fmt"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go/jetstream"
)

type natsCommitteeSyncSettingsStore struct {
	kv jetstream.KeyValue
}

// NewCommitteeSyncSettingsStore wraps the dedicated groupsio-committee-sync-settings KV bucket.
func NewCommitteeSyncSettingsStore(kv jetstream.KeyValue) port.CommitteeSyncSettingsStore {
	return &natsCommitteeSyncSettingsStore{kv: kv}
}

func syncModeKey(mailingListUID, committeeUID string) (string, error) {
	if !constants.ValidKVKeySegment(mailingListUID) || !constants.ValidKVKeySegment(committeeUID) {
		return "", fmt.Errorf("invalid mailing list UID or committee UID for sync settings key")
	}
	return constants.KVSyncSettingsPrefixSyncMode + "." + mailingListUID + "." + committeeUID, nil
}

func (s *natsCommitteeSyncSettingsStore) GetSyncMode(ctx context.Context, mailingListUID, committeeUID string) (string, bool, error) {
	key, err := syncModeKey(mailingListUID, committeeUID)
	if err != nil {
		return "", false, err
	}
	entry, err := s.kv.Get(ctx, key)
	if errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrKeyDeleted) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if entry == nil {
		return "", false, nil
	}
	return string(entry.Value()), true, nil
}

func (s *natsCommitteeSyncSettingsStore) PutSyncMode(ctx context.Context, mailingListUID, committeeUID, syncMode string) error {
	key, err := syncModeKey(mailingListUID, committeeUID)
	if err != nil {
		return err
	}
	_, err = s.kv.Put(ctx, key, []byte(syncMode))
	return err
}

func (s *natsCommitteeSyncSettingsStore) DeleteSyncMode(ctx context.Context, mailingListUID, committeeUID string) error {
	key, err := syncModeKey(mailingListUID, committeeUID)
	if err != nil {
		return err
	}
	err = s.kv.Purge(ctx, key)
	if errors.Is(err, jetstream.ErrKeyNotFound) || errors.Is(err, jetstream.ErrKeyDeleted) {
		return nil
	}
	return err
}
