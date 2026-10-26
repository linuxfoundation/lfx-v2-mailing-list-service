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
	mu        sync.Mutex
	modes     map[string]string
	revisions map[string]uint64
	nextRev   uint64
}

var _ port.CommitteeSyncSettingsStore = (*FakeCommitteeSyncSettingsStore)(nil)

// NewMockCommitteeSyncSettingsStore returns a FakeCommitteeSyncSettingsStore with an empty map.
func NewMockCommitteeSyncSettingsStore() *FakeCommitteeSyncSettingsStore {
	return &FakeCommitteeSyncSettingsStore{
		modes:     make(map[string]string),
		revisions: make(map[string]uint64),
	}
}

func (f *FakeCommitteeSyncSettingsStore) key(mailingListUID, committeeUID string) string {
	return mailingListUID + "|" + committeeUID
}

// GetSyncMode returns the stored sync mode and its revision for the given pair, if any.
func (f *FakeCommitteeSyncSettingsStore) GetSyncMode(_ context.Context, mailingListUID, committeeUID string) (string, uint64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := f.key(mailingListUID, committeeUID)
	mode, ok := f.modes[key]
	return mode, f.revisions[key], ok, nil
}

// PutSyncMode stores the sync mode for the given pair, bumping its revision.
func (f *FakeCommitteeSyncSettingsStore) PutSyncMode(_ context.Context, mailingListUID, committeeUID, syncMode string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := f.key(mailingListUID, committeeUID)
	f.nextRev++
	f.modes[key] = syncMode
	f.revisions[key] = f.nextRev
	return nil
}

// DeleteSyncModeIfUnchanged removes the stored sync mode for the given pair only if it is
// still at expectedRevision. Mirrors the NATS implementation's CAS semantics.
func (f *FakeCommitteeSyncSettingsStore) DeleteSyncModeIfUnchanged(_ context.Context, mailingListUID, committeeUID string, expectedRevision uint64) (bool, error) {
	if expectedRevision == 0 {
		return true, nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	key := f.key(mailingListUID, committeeUID)
	if f.revisions[key] != expectedRevision {
		return false, nil
	}
	delete(f.modes, key)
	delete(f.revisions, key)
	return true, nil
}
