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
