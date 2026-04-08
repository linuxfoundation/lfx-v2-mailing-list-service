// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// ProjectLookup fetches project attributes from the project service via NATS.
type ProjectLookup interface {
	// GetProjectSlug returns the URL slug for the given project UID.
	// Returns an error on transient failures (e.g. NATS timeout); returns an
	// empty string when the project exists but has no slug assigned.
	GetProjectSlug(ctx context.Context, projectUID string) (string, error)
}
