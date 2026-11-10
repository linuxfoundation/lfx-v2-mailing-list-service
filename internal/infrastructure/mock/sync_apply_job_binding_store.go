// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"sync"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// FakeSyncApplyJobBindingStore is an in-memory port.SyncApplyJobBindingStore for unit tests and
// REPOSITORY_SOURCE=mock local development — it never expires entries, since there is no bounded
// test/dev run long enough for that to matter. The real HTTP server shares a single instance of
// this store across concurrent requests when REPOSITORY_SOURCE=mock, so mu guards bindings against
// the resulting concurrent start/poll access (per @greptile-apps).
type FakeSyncApplyJobBindingStore struct {
	mu       sync.Mutex
	bindings map[string]port.SyncApplyJobBinding
}

var _ port.SyncApplyJobBindingStore = (*FakeSyncApplyJobBindingStore)(nil)

// NewFakeSyncApplyJobBindingStore returns an empty FakeSyncApplyJobBindingStore.
func NewFakeSyncApplyJobBindingStore() *FakeSyncApplyJobBindingStore {
	return &FakeSyncApplyJobBindingStore{bindings: make(map[string]port.SyncApplyJobBinding)}
}

func (f *FakeSyncApplyJobBindingStore) Put(_ context.Context, itxJobID string, binding port.SyncApplyJobBinding) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindings[itxJobID] = binding
	return nil
}

func (f *FakeSyncApplyJobBindingStore) Get(_ context.Context, itxJobID string) (port.SyncApplyJobBinding, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	binding, ok := f.bindings[itxJobID]
	return binding, ok, nil
}
