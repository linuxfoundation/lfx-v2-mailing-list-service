// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GroupsIOArtifactReader defines the application-level interface for GroupsIO artifact read operations.
type GroupsIOArtifactReader interface {
	// GetArtifact retrieves a single artifact by subgroup ID and artifact ID.
	GetArtifact(ctx context.Context, subgroupID string, artifactID string) (*model.GroupsIOArtifact, error)

	// GetArtifactDownloadURL returns a presigned S3 download URL for a file artifact.
	GetArtifactDownloadURL(ctx context.Context, subgroupID string, artifactID string) (string, error)
}
