// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// SyncApplyJobBinding records which mailing list and committee a sync-apply job was started
// against, so a later poll can be authenticated against this server-recorded binding instead of
// trusting an API caller to faithfully echo it back (see internal/service/grpsio_sync_apply.go).
type SyncApplyJobBinding struct {
	MailingListID string
	CommitteeUID  string
}

// SyncApplyJobBindingStore persists the binding StartSyncApply establishes for an ITX sync-apply
// job, so GetSyncApplyStatus can look it up by the job ID alone rather than decoding
// caller-supplied state.
type SyncApplyJobBindingStore interface {
	// Put records binding for itxJobID. Implementations may expire entries after a bounded TTL —
	// callers must treat a subsequent Get miss as "unknown or expired job", not "invalid job ID".
	Put(ctx context.Context, itxJobID string, binding SyncApplyJobBinding) error

	// Get returns the binding recorded for itxJobID and true, or false if none was found (the job
	// ID is unknown, never started by this service, or its binding has expired).
	Get(ctx context.Context, itxJobID string) (SyncApplyJobBinding, bool, error)
}
