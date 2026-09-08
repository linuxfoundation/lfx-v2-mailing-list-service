// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
)

// ---- GroupsIO Artifact endpoints ----

func (s *mailingListAPI) GetGroupsioArtifact(ctx context.Context, p *mailinglist.GetGroupsioArtifactPayload) (*mailinglist.GroupsioArtifact, error) {
	artifact, err := s.artifactReader.GetArtifact(ctx, p.SubgroupID, p.ArtifactID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return convertArtifact(artifact), nil
}

func (s *mailingListAPI) GetGroupsioArtifactDownload(ctx context.Context, p *mailinglist.GetGroupsioArtifactDownloadPayload) (*mailinglist.GroupsioArtifactDownload, error) {
	url, err := s.artifactReader.GetArtifactDownloadURL(ctx, p.SubgroupID, p.ArtifactID)
	if err != nil {
		return nil, mapDomainError(err)
	}
	return &mailinglist.GroupsioArtifactDownload{URL: url}, nil
}
