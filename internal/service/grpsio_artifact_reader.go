// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// GroupsIOArtifactReaderOrchestrator implements port.GroupsIOArtifactReader by wrapping an inner
// GroupsIOArtifactReader. Artifact and subgroup IDs are native ITX identifiers (UUID and numeric
// group ID respectively), so no v1/v2 ID translation is needed.
type GroupsIOArtifactReaderOrchestrator struct {
	reader port.GroupsIOArtifactReader
}

// ArtifactReaderOrchestratorOption configures a GroupsIOArtifactReaderOrchestrator.
type ArtifactReaderOrchestratorOption func(*GroupsIOArtifactReaderOrchestrator)

// WithArtifactReader sets the underlying reader (e.g. the ITX proxy client).
func WithArtifactReader(r port.GroupsIOArtifactReader) ArtifactReaderOrchestratorOption {
	return func(o *GroupsIOArtifactReaderOrchestrator) {
		o.reader = r
	}
}

// GetArtifact retrieves a single artifact by subgroup ID and artifact ID.
func (o *GroupsIOArtifactReaderOrchestrator) GetArtifact(ctx context.Context, subgroupID string, artifactID string) (*model.GroupsIOArtifact, error) {
	return o.reader.GetArtifact(ctx, subgroupID, artifactID)
}

// GetArtifactDownloadURL returns a presigned S3 download URL for a file artifact.
func (o *GroupsIOArtifactReaderOrchestrator) GetArtifactDownloadURL(ctx context.Context, subgroupID string, artifactID string) (string, error) {
	return o.reader.GetArtifactDownloadURL(ctx, subgroupID, artifactID)
}

// NewGroupsIOArtifactReaderOrchestrator creates a new artifact reader orchestrator with the given options.
func NewGroupsIOArtifactReaderOrchestrator(opts ...ArtifactReaderOrchestratorOption) port.GroupsIOArtifactReader {
	o := &GroupsIOArtifactReaderOrchestrator{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
