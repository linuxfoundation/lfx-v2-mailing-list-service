// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// CommitteeProjectLookup resolves a v2 committee UID to its owning v2 project UID.
// Implementations query lfx-v2-committee-service via NATS request-reply on
// constants.CommitteeGetProjectSubject.
type CommitteeProjectLookup interface {
	// GetCommitteeProject returns the v2 project UID that owns the given v2 committee UID.
	// Returns NotFound when no committee exists for the supplied UID.
	GetCommitteeProject(ctx context.Context, committeeUID string) (string, error)
}
