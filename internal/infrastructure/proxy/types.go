// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package proxy provides the ITX HTTP proxy client for GroupsIO operations.
package proxy

// serviceWire represents a GroupsIO service as returned by the ITX API.
type serviceWire struct {
	ID        string `json:"id,omitempty"`
	ProjectID string `json:"project_id,omitempty"` // v1 SFID
	Type      string `json:"type,omitempty"`
	GroupID   int64  `json:"group_id,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"last_modified_at,omitempty"`
}

// serviceRequestWire represents a create/update request for a GroupsIO service.
type serviceRequestWire struct {
	ProjectID string `json:"project_id,omitempty"` // v1 SFID
	Type      string `json:"type,omitempty"`
	GroupID   int64  `json:"group_id,omitempty"`
	Domain    string `json:"domain,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
	Status    string `json:"status,omitempty"`
}

// serviceListResponseWire represents a list response of GroupsIO services from the ITX API.
type serviceListResponseWire struct {
	Items []*serviceWire `json:"items"`
	Total int            `json:"total"`
}

// subgroupWire represents a GroupsIO subgroup (mailing list) as returned by the ITX API.
type subgroupWire struct {
	ID             string `json:"id,omitempty"`
	ProjectID      string `json:"project_id,omitempty"` // v1 SFID
	CommitteeID    string `json:"committee,omitempty"`  // v1 UUID
	ParentID       string `json:"parent_id,omitempty"`  // v1 Service ID
	GroupID        int64  `json:"group_id,omitempty"`
	Name           string `json:"group_name,omitempty"`
	Description    string `json:"description,omitempty"`
	Type           string `json:"type,omitempty"`
	AudienceAccess string `json:"visibility,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"last_modified_at,omitempty"`
}

// subgroupRequestWire represents a create/update request for a GroupsIO subgroup.
type subgroupRequestWire struct {
	ProjectID      string `json:"project_id,omitempty"` // v1 SFID
	CommitteeID    string `json:"committee,omitempty"`  // v1 UUID
	ParentID       string `json:"parent_id,omitempty"`  // v1 Service ID
	Name           string `json:"group_name,omitempty"`
	Description    string `json:"description,omitempty"`
	Type           string `json:"type,omitempty"`
	AudienceAccess string `json:"visibility,omitempty"`
}

// subgroupListResponseWire represents a list response of GroupsIO subgroups from the ITX API.
type subgroupListResponseWire struct {
	Items []*subgroupWire `json:"items"`
	Total int             `json:"total"`
}

// countResponseWire represents a count response from the ITX API.
type countResponseWire struct {
	Count int `json:"count"`
}

// memberWire represents a GroupsIO member as returned by the ITX API.
// POST responses use "member_id" (int); GET responses use "id" (string).
type memberWire struct {
	MemberID     int64  `json:"member_id,omitempty"` // POST create / list response
	ID           string `json:"id,omitempty"`        // GET single response
	Email        string `json:"email,omitempty"`
	Name         string `json:"full_name,omitempty"`
	MemberType   string `json:"member_type,omitempty"`
	DeliveryMode string `json:"delivery_mode,omitempty"`
	ModStatus    string `json:"mod_status,omitempty"`
	Status       string `json:"status,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	Organization string `json:"organization,omitempty"`
	JobTitle     string `json:"job_title,omitempty"`
	Username     string `json:"username,omitempty"`
	Role         string `json:"role,omitempty"`
	VotingStatus string `json:"voting_status,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	UpdatedAt    string `json:"last_modified_at,omitempty"`
}

// memberRequestWire represents a create/update request for a GroupsIO member.
type memberRequestWire struct {
	Email        string `json:"email,omitempty"`
	Name         string `json:"name,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	DeliveryMode string `json:"delivery_mode,omitempty"`
	MemberType   string `json:"member_type,omitempty"`
	ModStatus    string `json:"mod_status,omitempty"`
	Organization string `json:"organization,omitempty"`
	JobTitle     string `json:"job_title,omitempty"`
}

// memberListResponseWire represents a list response of GroupsIO members from the ITX API.
type memberListResponseWire struct {
	Data []*memberWire `json:"data"`
}

// checkSubscriberResponseWire represents the response to a check-subscriber request.
type checkSubscriberResponseWire struct {
	Subscribed bool `json:"subscribed"`
}

// checkSubscriberRequestWire represents the request body for checking a subscriber.
type checkSubscriberRequestWire struct {
	Email      string `json:"email"`
	SubgroupID string `json:"subgroup_id"`
}

// inviteMembersRequestWire represents an invite members request for a GroupsIO subgroup.
type inviteMembersRequestWire struct {
	Emails []string `json:"emails"`
}

// artifactUserWire represents a user reference on an artifact (creator or last modifier).
type artifactUserWire struct {
	ID             string `json:"id,omitempty"`
	Username       string `json:"username,omitempty"`
	Name           string `json:"name,omitempty"`
	Email          string `json:"email,omitempty"`
	ProfilePicture string `json:"profile_picture,omitempty"`
}

// artifactWire represents a GroupsIO subgroup artifact as returned by the ITX API.
type artifactWire struct {
	ArtifactID          string            `json:"artifact_id"`
	GroupID             uint64            `json:"group_id"`
	ProjectID           string            `json:"project_id,omitempty"`
	CommitteeID         string            `json:"committee_id,omitempty"`
	Type                string            `json:"type,omitempty"`
	MediaType           string            `json:"media_type,omitempty"`
	Filename            string            `json:"filename,omitempty"`
	LinkURL             string            `json:"link_url,omitempty"`
	DownloadURL         string            `json:"download_url,omitempty"`
	S3Key               string            `json:"s3_key,omitempty"`
	FileUploaded        *bool             `json:"file_uploaded,omitempty"`
	FileUploadStatus    string            `json:"file_upload_status,omitempty"`
	FileUploadedAt      string            `json:"file_uploaded_at,omitempty"`
	MessageIDs          []uint64          `json:"message_ids,omitempty"`
	LastPostedAt        string            `json:"last_posted_at,omitempty"`
	LastPostedMessageID uint64            `json:"last_posted_message_id,omitempty"`
	Description         string            `json:"description,omitempty"`
	CreatedBy           *artifactUserWire `json:"created_by,omitempty"`
	LastModifiedBy      *artifactUserWire `json:"last_modified_by,omitempty"`
	CreatedAt           string            `json:"created_at,omitempty"`
	UpdatedAt           string            `json:"last_modified_at,omitempty"`
}

// artifactDownloadResponseWire represents the presigned download URL response from the ITX API.
type artifactDownloadResponseWire struct {
	URL string `json:"url"`
}
