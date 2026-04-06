// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"fmt"
	"time"
)

// ArtifactUser represents a user reference on an artifact (creator or last modifier).
type ArtifactUser struct {
	ID             string `json:"id,omitempty"`
	Username       string `json:"username,omitempty"`
	Name           string `json:"name,omitempty"`
	Email          string `json:"email,omitempty"`
	ProfilePicture string `json:"profile_picture,omitempty"`
}

// GroupsIOArtifact represents a GroupsIO subgroup artifact.
type GroupsIOArtifact struct {
	ArtifactID          string        `json:"artifact_id"`
	GroupID             uint64        `json:"group_id"`
	ProjectUID          string        `json:"project_uid,omitempty"`
	CommitteeUID        string        `json:"committee_uid,omitempty"`
	Type                string        `json:"type,omitempty"`
	MediaType           string        `json:"media_type,omitempty"`
	Filename            string        `json:"filename,omitempty"`
	LinkURL             string        `json:"link_url,omitempty"`
	DownloadURL         string        `json:"download_url,omitempty"`
	S3Key               string        `json:"s3_key,omitempty"`
	FileUploaded        *bool         `json:"file_uploaded,omitempty"`
	FileUploadStatus    string        `json:"file_upload_status,omitempty"`
	FileUploadedAt      *time.Time    `json:"file_uploaded_at,omitempty"`
	MessageIDs          []uint64      `json:"message_ids,omitempty"`
	LastPostedAt        *time.Time    `json:"last_posted_at,omitempty"`
	LastPostedMessageID *uint64       `json:"last_posted_message_id,omitempty"`
	Description         string        `json:"description,omitempty"`
	CreatedBy           *ArtifactUser `json:"created_by,omitempty"`
	LastModifiedBy      *ArtifactUser `json:"last_modified_by,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
}

// ParentRefs returns the parent resource references for indexing (project, committee, group).
func (a *GroupsIOArtifact) ParentRefs() []string {
	if a == nil {
		return nil
	}
	var refs []string
	if a.ProjectUID != "" {
		refs = append(refs, fmt.Sprintf("project:%s", a.ProjectUID))
	}
	if a.CommitteeUID != "" {
		refs = append(refs, fmt.Sprintf("committee:%s", a.CommitteeUID))
	}
	if a.GroupID != 0 {
		refs = append(refs, fmt.Sprintf("groupsio_mailing_list:%d", a.GroupID))
	}
	return refs
}

// NameAndAliases returns display names for search (filename or link URL depending on type).
func (a *GroupsIOArtifact) NameAndAliases() []string {
	if a == nil {
		return nil
	}
	var names []string
	if a.Filename != "" {
		names = append(names, a.Filename)
	}
	if a.LinkURL != "" {
		names = append(names, a.LinkURL)
	}
	return names
}

// SortName returns the primary sort name for the artifact (filename, falling back to link URL).
func (a *GroupsIOArtifact) SortName() string {
	if a == nil {
		return ""
	}
	if a.Filename != "" {
		return a.Filename
	}
	return a.LinkURL
}

// Fulltext returns a concatenated string for full-text search.
func (a *GroupsIOArtifact) Fulltext() string {
	if a == nil {
		return ""
	}
	name := a.SortName()
	if a.Description == "" {
		return name
	}
	if name == "" {
		return a.Description
	}
	return name + " " + a.Description
}

// Tags generates a consistent set of tags for the artifact.
func (a *GroupsIOArtifact) Tags() []string {
	if a == nil {
		return nil
	}
	var tags []string
	if a.ArtifactID != "" {
		tags = append(tags, a.ArtifactID)
		tags = append(tags, fmt.Sprintf("group_artifact_id:%s", a.ArtifactID))
	}
	if a.GroupID != 0 {
		tags = append(tags, fmt.Sprintf("group_id:%d", a.GroupID))
	}
	if a.ProjectUID != "" {
		tags = append(tags, fmt.Sprintf("project_uid:%s", a.ProjectUID))
	}
	if a.CommitteeUID != "" {
		tags = append(tags, fmt.Sprintf("committee_uid:%s", a.CommitteeUID))
	}
	return tags
}
