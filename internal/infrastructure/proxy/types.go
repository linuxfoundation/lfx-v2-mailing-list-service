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
	ID               string   `json:"id,omitempty"`
	ProjectID        string   `json:"project_id,omitempty"`        // v1 SFID
	CommitteeID      string   `json:"committee,omitempty"`         // v1 UUID
	CommitteeFilters []string `json:"committee_filters,omitempty"` // allowed voting statuses, if the committee link is filtered
	SyncMode         string   `json:"sync_mode,omitempty"`         // auto_c_to_ml, auto_ml_to_c, or manual
	ParentID         string   `json:"parent_id,omitempty"`         // v1 Service ID
	GroupID          int64    `json:"group_id,omitempty"`
	Name             string   `json:"group_name,omitempty"`
	Description      string   `json:"description,omitempty"`
	Type             string   `json:"type,omitempty"`
	AudienceAccess   string   `json:"visibility,omitempty"`
	CreatedAt        string   `json:"created_at,omitempty"`
	UpdatedAt        string   `json:"last_modified_at,omitempty"`
}

// subgroupRequestWire represents a create/update request for a GroupsIO subgroup.
type subgroupRequestWire struct {
	ProjectID      string `json:"project_id,omitempty"` // v1 SFID
	CommitteeID    string `json:"committee,omitempty"`  // v1 UUID
	SyncMode       string `json:"sync_mode,omitempty"`  // auto_c_to_ml, auto_ml_to_c, or manual
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

// acceptInviteRequestWire is the JSON payload for POST /v2/groupsio/invite_accepted.
// It instructs the ITX GroupsIO backend to enrich all member records whose email
// matches the acceptor with the newly-resolved username.
type acceptInviteRequestWire struct {
	Email    string `json:"email"`
	Username string `json:"username"`
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
	LastPostedMessageID *uint64           `json:"last_posted_message_id,omitempty"`
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

// ---- sync-diff / sync-apply job wire types ----
//
// ITX computes and applies committee<->mailing-list membership reconciliation itself, as async
// jobs: POST .../sync-diff and .../sync-apply return 202 with a job_id, and the corresponding GET
// endpoint polls the job's status/results. See pkg/subgroups/sync_diff.go and
// groups_subgroups_sync.go in itx-service-groupsio.

// syncDiffDirectionCToML and syncDiffDirectionMLToC are ITX's values for the sync-diff/sync-apply
// "direction" field — distinct from the subgroup-level sync_mode values (auto_c_to_ml/
// auto_ml_to_c/manual).
const (
	syncDiffDirectionCToML = "c_to_ml"
	syncDiffDirectionMLToC = "ml_to_c"
)

// ITX sync job status values, shared by sync-diff and sync-apply jobs.
const (
	syncJobStatusPending   = "pending"
	syncJobStatusOngoing   = "ongoing"
	syncJobStatusCompleted = "completed"
	syncJobStatusFailed    = "failed"
)

// postSyncDiffRequestWire is the POST .../sync-diff request body.
type postSyncDiffRequestWire struct {
	Direction string `json:"direction"`
}

// postSyncJobResponseWire is the 202 response shared by POST .../sync-diff and
// POST .../sync-apply.
type postSyncJobResponseWire struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

// syncDiffMemberWire is one entry in a sync-diff job's to_add/to_remove lists.
type syncDiffMemberWire struct {
	MemberID string `json:"member_id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

// syncDiffConflictWire is one entry in a sync-diff job's email_conflicts list.
type syncDiffConflictWire struct {
	MLEmail        string `json:"ml_email"`
	CommitteeEmail string `json:"committee_email"`
	FullName       string `json:"full_name"`
	MemberID       string `json:"member_id"`
}

// syncDiffJobWire is the GET .../sync-diff/:job_id response body.
type syncDiffJobWire struct {
	ID             string                 `json:"id"`
	SubgroupID     uint64                 `json:"subgroup_id"`
	CommitteeID    string                 `json:"committee_id,omitempty"`
	Direction      string                 `json:"direction"`
	Status         string                 `json:"status"`
	ToAdd          []syncDiffMemberWire   `json:"to_add"`
	ToRemove       []syncDiffMemberWire   `json:"to_remove"`
	EmailConflicts []syncDiffConflictWire `json:"email_conflicts"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
}

// postSyncApplyRequestWire is the POST .../sync-apply request body. ApprovedAdds carries emails
// (matching a to_add entry's email); ApprovedRemoves carries mailing-list member IDs (matching a
// to_remove entry's member_id) — ITX has no other way to identify which existing list row to
// delete.
type postSyncApplyRequestWire struct {
	Direction       string   `json:"direction"`
	ApprovedAdds    []string `json:"approved_adds,omitempty"`
	ApprovedRemoves []string `json:"approved_removes,omitempty"`
	JobID           string   `json:"job_id,omitempty"`
}

// syncApplyResultWire is one entry in a sync-apply job's results list.
type syncApplyResultWire struct {
	Email  string `json:"email"`
	Action string `json:"action"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// syncApplyJobWire is the GET .../sync-apply/:apply_job_id response body.
type syncApplyJobWire struct {
	ID           string                `json:"id"`
	SubgroupID   uint64                `json:"subgroup_id"`
	CommitteeID  string                `json:"committee_id,omitempty"`
	Direction    string                `json:"direction"`
	Status       string                `json:"status"`
	Results      []syncApplyResultWire `json:"results"`
	ErrorMessage string                `json:"error_message,omitempty"`
}
