// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// MailingListCreatedEvent represents a mailing list creation event
// Published to lfx.mailing-list-api.mailing_list_created
type MailingListCreatedEvent struct {
	// MailingList contains the full mailing list data that was created
	MailingList *GroupsIOMailingList `json:"mailing_list"`
}

// MailingListUpdatedEvent represents a mailing list update event
// Published to lfx.mailing-list-api.mailing_list_updated
type MailingListUpdatedEvent struct {
	// OldMailingList contains the mailing list state before the update
	OldMailingList *GroupsIOMailingList `json:"old_mailing_list"`
	// NewMailingList contains the mailing list state after the update
	NewMailingList *GroupsIOMailingList `json:"new_mailing_list"`
}

// CommitteeMailingListChangedEvent is published when a mailing list CRUD operation
// changes committee-related state. Additional fields can be added here as more
// committee attributes become driven by mailing list operations.
type CommitteeMailingListChangedEvent struct {
	CommitteeUID   string `json:"committee_uid"`
	HasMailingList bool   `json:"has_mailing_list"`
}
