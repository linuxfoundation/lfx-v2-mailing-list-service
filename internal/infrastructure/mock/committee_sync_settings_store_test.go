// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeCommitteeSyncSettingsStore_GetMissing_ReturnsNotFound(t *testing.T) {
	store := NewMockCommitteeSyncSettingsStore()
	mode, found, err := store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, mode)
}

func TestFakeCommitteeSyncSettingsStore_PutThenGet_ReturnsStoredValue(t *testing.T) {
	store := NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-1", "auto_c_to_ml"))

	mode, found, err := store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "auto_c_to_ml", mode)
}

func TestFakeCommitteeSyncSettingsStore_DifferentPairsAreIndependent(t *testing.T) {
	store := NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-1", "auto_c_to_ml"))
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-2", "committee-1", "auto_ml_to_c"))

	mode1, _, _ := store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	mode2, _, _ := store.GetSyncMode(context.Background(), "ml-2", "committee-1")
	assert.Equal(t, "auto_c_to_ml", mode1)
	assert.Equal(t, "auto_ml_to_c", mode2)
}

func TestFakeCommitteeSyncSettingsStore_Delete_RemovesEntry(t *testing.T) {
	store := NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-1", "manual"))
	require.NoError(t, store.DeleteSyncMode(context.Background(), "ml-1", "committee-1"))

	_, found, err := store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestFakeCommitteeSyncSettingsStore_DeleteMissing_NoError(t *testing.T) {
	store := NewMockCommitteeSyncSettingsStore()
	assert.NoError(t, store.DeleteSyncMode(context.Background(), "ml-1", "committee-1"))
}
