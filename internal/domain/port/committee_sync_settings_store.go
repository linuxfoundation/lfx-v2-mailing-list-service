// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// CommitteeSyncSettingsStore persists the per-committee automatic membership sync mode for a
// mailing list. This is v2-local state: ITX (the legacy GroupsIO backend) has no concept of
// sync_mode, so it cannot be proxied or stored alongside the rest of the mailing list record.
type CommitteeSyncSettingsStore interface {
	// GetSyncMode returns the configured sync mode for the given mailing list + committee pair,
	// along with an opaque revision token identifying the current value. Returns ("", 0, false,
	// nil) when no mode has been set; callers should treat that the same as model.SyncModeManual.
	GetSyncMode(ctx context.Context, mailingListUID, committeeUID string) (mode string, revision uint64, ok bool, err error)

	// PutSyncMode sets the sync mode for the given mailing list + committee pair.
	PutSyncMode(ctx context.Context, mailingListUID, committeeUID, syncMode string) error

	// DeleteSyncModeIfUnchanged removes the stored sync mode for the given mailing list +
	// committee pair only if it is still at revision expectedRevision (as returned by a prior
	// GetSyncMode call). If the value has since changed — e.g. a concurrent request wrote a
	// newer mode for the same pair — the delete is skipped and ok is false, so callers never
	// clobber a newer write. No-op (ok=true, not an error) when no mode was stored at all.
	DeleteSyncModeIfUnchanged(ctx context.Context, mailingListUID, committeeUID string, expectedRevision uint64) (ok bool, err error)
}
