// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"sync"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// FakeCommitteeSyncSettingsStore is an in-memory test double for port.CommitteeSyncSettingsStore,
// used for local dev (REPOSITORY_SOURCE=mock) and tests.
type FakeCommitteeSyncSettingsStore struct {
	mu    sync.Mutex
	modes map[string]string
}

var _ port.CommitteeSyncSettingsStore = (*FakeCommitteeSyncSettingsStore)(nil)

// NewMockCommitteeSyncSettingsStore returns a FakeCommitteeSyncSettingsStore with an empty map.
func NewMockCommitteeSyncSettingsStore() *FakeCommitteeSyncSettingsStore {
	return &FakeCommitteeSyncSettingsStore{modes: make(map[string]string)}
}

func (f *FakeCommitteeSyncSettingsStore) key(mailingListUID, committeeUID string) string {
	return mailingListUID + "|" + committeeUID
}

// GetSyncMode returns the stored sync mode for the given pair, if any.
func (f *FakeCommitteeSyncSettingsStore) GetSyncMode(_ context.Context, mailingListUID, committeeUID string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	mode, ok := f.modes[f.key(mailingListUID, committeeUID)]
	return mode, ok, nil
}

// PutSyncMode stores the sync mode for the given pair.
func (f *FakeCommitteeSyncSettingsStore) PutSyncMode(_ context.Context, mailingListUID, committeeUID, syncMode string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.modes[f.key(mailingListUID, committeeUID)] = syncMode
	return nil
}

// DeleteSyncMode removes the stored sync mode for the given pair, if any.
func (f *FakeCommitteeSyncSettingsStore) DeleteSyncMode(_ context.Context, mailingListUID, committeeUID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.modes, f.key(mailingListUID, committeeUID))
	return nil
}
