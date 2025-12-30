// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package model defines the domain models and entities for the mailing list service.
package model

import "time"

// CommitteeMemberCreatedEvent represents a committee member creation event from committee-api
type CommitteeMemberCreatedEvent struct {
	MemberUID    string                   `json:"member_uid"`
	CommitteeUID string                   `json:"committee_uid"`
	ProjectUID   string                   `json:"project_uid"`
	Member       CommitteeMemberEventData `json:"member"`
	Timestamp    time.Time                `json:"timestamp"`
}

// CommitteeMemberDeletedEvent represents a committee member deletion event from committee-api
type CommitteeMemberDeletedEvent struct {
	MemberUID    string    `json:"member_uid"`
	CommitteeUID string    `json:"committee_uid"`
	ProjectUID   string    `json:"project_uid"`
	Email        string    `json:"email"`
	Timestamp    time.Time `json:"timestamp"`
}

// CommitteeMemberUpdatedEvent represents a committee member update event from committee-api
type CommitteeMemberUpdatedEvent struct {
	MemberUID    string                   `json:"member_uid"`
	CommitteeUID string                   `json:"committee_uid"`
	ProjectUID   string                   `json:"project_uid"`
	OldMember    CommitteeMemberEventData `json:"old_member"`
	NewMember    CommitteeMemberEventData `json:"new_member"`
	Timestamp    time.Time                `json:"timestamp"`
}

// CommitteeMemberEventData contains member details embedded in committee events
type CommitteeMemberEventData struct {
	Email        string       `json:"email"`
	FirstName    string       `json:"first_name"`
	LastName     string       `json:"last_name"`
	Username     string       `json:"username"`
	VotingStatus string       `json:"voting_status"` // "Voting Rep", "Alternate Voting Rep", "Observer", etc.
	Organization Organization `json:"organization"`
	JobTitle     string       `json:"job_title"`
}

// Organization represents the organization details in committee member data
type Organization struct {
	Name string `json:"name"`
}

// CommitteeMemberBase represents the base committee member attributes
type CommitteeMember struct {
	UID             string                      `json:"uid"`
	Username        string                      `json:"username"`
	Email           string                      `json:"email"`
	FirstName       string                      `json:"first_name"`
	LastName        string                      `json:"last_name"`
	JobTitle        string                      `json:"job_title,omitempty"`
	Role            CommitteeMemberRole         `json:"role"`
	AppointedBy     string                      `json:"appointed_by"`
	Status          string                      `json:"status"`
	Voting          CommitteeMemberVotingInfo   `json:"voting"`
	Agency          string                      `json:"agency,omitempty"`
	Country         string                      `json:"country,omitempty"`
	Organization    CommitteeMemberOrganization `json:"organization"`
	CommitteeUID    string                      `json:"committee_uid"`
	CommitteeName   string                      `json:"committee_name"`
	LinkedInProfile string                      `json:"linkedin_profile,omitempty"`
	CreatedAt       time.Time                   `json:"created_at"`
	UpdatedAt       time.Time                   `json:"updated_at"`
}

// CommitteeMemberRole represents committee role information
type CommitteeMemberRole struct {
	Name      string `json:"name"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

// CommitteeMemberVotingInfo represents voting information for the committee member
type CommitteeMemberVotingInfo struct {
	Status    string `json:"status"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

// CommitteeMemberOrganization represents organization information for the committee member
type CommitteeMemberOrganization struct {
	Name    string `json:"name"`
	Website string `json:"website,omitempty"`
}
