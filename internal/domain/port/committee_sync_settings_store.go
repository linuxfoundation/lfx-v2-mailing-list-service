// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// CommitteeSyncSettingsStore persists the per-committee automatic membership sync mode for a
// mailing list. This is v2-local state: ITX (the legacy GroupsIO backend) has no concept of
// sync_mode, so it cannot be proxied or stored alongside the rest of the mailing list record.
type CommitteeSyncSettingsStore interface {
	// GetSyncMode returns the configured sync mode for the given mailing list + committee pair.
	// Returns ("", false, nil) when no mode has been set; callers should treat that the same
	// as model.SyncModeManual.
	GetSyncMode(ctx context.Context, mailingListUID, committeeUID string) (string, bool, error)

	// PutSyncMode sets the sync mode for the given mailing list + committee pair.
	PutSyncMode(ctx context.Context, mailingListUID, committeeUID, syncMode string) error

	// DeleteSyncMode removes any stored sync mode for the given mailing list + committee pair.
	// No-op (not an error) when no mode was stored.
	DeleteSyncMode(ctx context.Context, mailingListUID, committeeUID string) error
}
