// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// MailingListCreatedEvent represents a mailing list creation event
// Published to lfx.mailing-list-api.mailing_list_created
type MailingListCreatedEvent struct {
	// MailingList contains the full mailing list data that was created
	MailingList *GrpsIOMailingList `json:"mailing_list"`
}

// MailingListUpdatedEvent represents a mailing list update event
// Published to lfx.mailing-list-api.mailing_list_updated
type MailingListUpdatedEvent struct {
	// OldMailingList contains the mailing list state before the update
	OldMailingList *GrpsIOMailingList `json:"old_mailing_list"`
	// NewMailingList contains the mailing list state after the update
	NewMailingList *GrpsIOMailingList `json:"new_mailing_list"`
}
