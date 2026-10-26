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
	mode, _, found, err := store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, mode)
}

func TestFakeCommitteeSyncSettingsStore_PutThenGet_ReturnsStoredValue(t *testing.T) {
	store := NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-1", "auto_c_to_ml"))

	mode, _, found, err := store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "auto_c_to_ml", mode)
}

func TestFakeCommitteeSyncSettingsStore_DifferentPairsAreIndependent(t *testing.T) {
	store := NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-1", "auto_c_to_ml"))
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-2", "committee-1", "auto_ml_to_c"))

	mode1, _, _, _ := store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	mode2, _, _, _ := store.GetSyncMode(context.Background(), "ml-2", "committee-1")
	assert.Equal(t, "auto_c_to_ml", mode1)
	assert.Equal(t, "auto_ml_to_c", mode2)
}

func TestFakeCommitteeSyncSettingsStore_Delete_RemovesEntry(t *testing.T) {
	store := NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-1", "manual"))
	_, revision, found, err := store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	require.NoError(t, err)
	require.True(t, found)
	ok, err := store.DeleteSyncModeIfUnchanged(context.Background(), "ml-1", "committee-1", revision)
	require.NoError(t, err)
	assert.True(t, ok)

	_, _, found, err = store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestFakeCommitteeSyncSettingsStore_DeleteMissing_NoError(t *testing.T) {
	store := NewMockCommitteeSyncSettingsStore()
	ok, err := store.DeleteSyncModeIfUnchanged(context.Background(), "ml-1", "committee-1", 0)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestFakeCommitteeSyncSettingsStore_DeleteIfUnchanged_RevisionMismatch_Skips(t *testing.T) {
	store := NewMockCommitteeSyncSettingsStore()
	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-1", "manual"))
	_, staleRevision, _, err := store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	require.NoError(t, err)

	require.NoError(t, store.PutSyncMode(context.Background(), "ml-1", "committee-1", "auto_c_to_ml"))

	ok, err := store.DeleteSyncModeIfUnchanged(context.Background(), "ml-1", "committee-1", staleRevision)
	require.NoError(t, err)
	assert.False(t, ok, "delete must be skipped when a newer value was written concurrently")

	mode, _, found, err := store.GetSyncMode(context.Background(), "ml-1", "committee-1")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "auto_c_to_ml", mode, "the newer value must not be clobbered")
}
