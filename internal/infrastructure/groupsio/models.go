// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package groupsio

import "time"

// GroupObject represents a Groups.io group (expanded to match production structure)
type GroupObject struct {
	ID         uint64 `json:"id"`
	Object     string `json:"object"`
	Created    string `json:"created"`
	Updated    string `json:"updated"`
	Title      string `json:"title"`
	Name       string `json:"name"`
	Alias      string `json:"alias"`
	Desc       string `json:"desc"`
	PlainDesc  string `json:"plain_desc"`
	SubjectTag string `json:"subject_tag"`
	Footer     string `json:"footer"`
	Website    string `json:"website"`
	Announce   bool   `json:"announce"`
	Restricted bool   `json:"restricted"`
	Moderated  bool   `json:"moderated"`
	Privacy    string `json:"privacy"`
	OrgID      uint64 `json:"org_id"`
	OrgDomain  string `json:"org_domain"`
	SubsCount  uint64 `json:"subs_count"`
	GroupURL   string `json:"group_url"`
}

// SubgroupObject represents a Groups.io subgroup (mailing list)
type SubgroupObject struct {
	ID          uint64 `json:"id"`
	GroupID     uint64 `json:"group_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Public      bool   `json:"public"`
	Type        string `json:"type"` // announcement, discussion_moderated, discussion_open
	CreatedAt   string `json:"created"`
	UpdatedAt   string `json:"updated"`
}

// MemberObject represents a Groups.io member (simplified from go-groupsio)
type MemberObject struct {
	ID           uint64 `json:"id"`
	GroupID      uint64 `json:"group_id"`
	SubgroupID   uint64 `json:"subgroup_id,omitempty"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	Status       string `json:"status"`     // normal, pending, bouncing, etc.
	ModStatus    string `json:"mod_status"` // none, moderator, owner
	DeliveryMode string `json:"delivery"`   // individual, digest, no_email
	JoinedAt     string `json:"joined"`
	UpdatedAt    string `json:"updated"`
}

// LoginObject represents the Groups.io login response
type LoginObject struct {
	Token     string      `json:"token"`
	User      interface{} `json:"user,omitempty"` // Can be object or array, we don't need to parse it
	UserID    uint64      `json:"user_id,omitempty"`
	Email     string      `json:"email,omitempty"`
	ExpiresAt string      `json:"expires_at,omitempty"`
}

// ErrorObject represents a Groups.io API error response
type ErrorObject struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

// GroupCreateOptions represents options for creating a group (matches production go-groupsio)
type GroupCreateOptions struct {
	GroupName      string `url:"group_name"`
	Desc           string `url:"desc"`
	Privacy        string `url:"privacy"`
	SubGroupAccess string `url:"sub_group_access"`

	// Creator subscription options (from production)
	EmailDelivery     string `url:"email_delivery,omitempty"`
	MessageSelection  string `url:"message_selection,omitempty"`
	AutoFollowReplies bool   `url:"auto_follow_replies,omitempty"`
}

// SubgroupCreateOptions represents options for creating a subgroup (matches production go-groupsio)
type SubgroupCreateOptions struct {
	// Subgroup options (production field names)
	ParentGroupID   uint64 `url:"group_id,omitempty"`   // Parent group ID
	ParentGroupName string `url:"group_name,omitempty"` // Parent group name
	GroupName       string `url:"sub_group_name"`       // REQUIRED by Groups.io API: must be "sub_group_name" not "subgroup_name" per API spec
	Desc            string `url:"desc"`                 // REQUIRED by Groups.io API: must be "desc" not "description" per API spec
	Privacy         string `url:"privacy,omitempty"`    // Privacy setting (optional - may inherit from parent)

	// Creator subscription options (from production)
	EmailDelivery     string `url:"email_delivery,omitempty"`
	MessageSelection  string `url:"message_selection,omitempty"`
	AutoFollowReplies bool   `url:"auto_follow_replies,omitempty"`
	MaxAttachmentSize string `url:"max_attachment_size,omitempty"`
}

// MemberUpdateOptions represents options for updating a member
type MemberUpdateOptions struct {
	ModStatus    string `url:"mod_status,omitempty"` // none, moderator, owner
	DeliveryMode string `url:"delivery,omitempty"`   // individual, digest, no_email
	Status       string `url:"status,omitempty"`     // normal, pending, bouncing
	FirstName    string `url:"first_name,omitempty"`
	LastName     string `url:"last_name,omitempty"`
}

// GroupUpdateOptions represents options for updating a Groups.io group/service
type GroupUpdateOptions struct {
	GlobalOwners          []string `url:"global_owners,omitempty"`
	Announce              *bool    `url:"announce,omitempty"`
	ReplyTo               string   `url:"reply_to,omitempty"`
	MembersVisible        string   `url:"members_visible,omitempty"`
	CalendarAccess        string   `url:"calendar_access,omitempty"`
	FilesAccess           string   `url:"files_access,omitempty"`
	DatabaseAccess        string   `url:"database_access,omitempty"`
	WikiAccess            string   `url:"wiki_access,omitempty"`
	PhotosAccess          string   `url:"photos_access,omitempty"`
	MemberDirectoryAccess string   `url:"member_directory_access,omitempty"`
	PollsAccess           string   `url:"polls_access,omitempty"`
	ChatAccess            string   `url:"chat_access,omitempty"`
}

// SubgroupUpdateOptions represents options for updating a Groups.io subgroup/mailing list
type SubgroupUpdateOptions struct {
	Title       string `url:"title,omitempty"`
	Description string `url:"description,omitempty"`
	SubjectTag  string `url:"subject_tag,omitempty"`
}

// TokenCache represents a cached authentication token
type TokenCache struct {
	Token     string
	ExpiresAt time.Time
}
