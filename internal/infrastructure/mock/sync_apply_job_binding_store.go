// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// FakeSyncApplyJobBindingStore is an in-memory port.SyncApplyJobBindingStore for unit tests and
// REPOSITORY_SOURCE=mock local development — it never expires entries, since there is no bounded
// test/dev run long enough for that to matter.
type FakeSyncApplyJobBindingStore struct {
	bindings map[string]port.SyncApplyJobBinding
}

var _ port.SyncApplyJobBindingStore = (*FakeSyncApplyJobBindingStore)(nil)

// NewFakeSyncApplyJobBindingStore returns an empty FakeSyncApplyJobBindingStore.
func NewFakeSyncApplyJobBindingStore() *FakeSyncApplyJobBindingStore {
	return &FakeSyncApplyJobBindingStore{bindings: make(map[string]port.SyncApplyJobBinding)}
}

func (f *FakeSyncApplyJobBindingStore) Put(_ context.Context, itxJobID string, binding port.SyncApplyJobBinding) error {
	f.bindings[itxJobID] = binding
	return nil
}

func (f *FakeSyncApplyJobBindingStore) Get(_ context.Context, itxJobID string) (port.SyncApplyJobBinding, bool, error) {
	binding, ok := f.bindings[itxJobID]
	return binding, ok, nil
}
