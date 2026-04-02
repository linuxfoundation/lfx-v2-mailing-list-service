// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import "time"

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
	ProjectID           string        `json:"project_id,omitempty"`
	CommitteeID         string        `json:"committee_id,omitempty"`
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
	LastPostedMessageID uint64        `json:"last_posted_message_id,omitempty"`
	Description         string        `json:"description,omitempty"`
	CreatedBy           *ArtifactUser `json:"created_by,omitempty"`
	LastModifiedBy      *ArtifactUser `json:"last_modified_by,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
}
