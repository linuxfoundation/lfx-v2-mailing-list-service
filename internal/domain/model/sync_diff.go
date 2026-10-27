// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

// SyncDiffAdd is a committee member who should be added to the mailing list to
// reconcile it with its linked committee's membership.
type SyncDiffAdd struct {
	Email              string `json:"email"`
	CommitteeMemberUID string `json:"committee_member_uid"`
}

// SyncDiffRemove is a mailing list member, originally added by committee sync, who should be
// removed because they are no longer a member of the linked committee.
type SyncDiffRemove struct {
	Email               string `json:"email"`
	MailingListMemberID string `json:"mailing_list_member_id"`
}

// SyncDiff is the result of reconciling a mailing list's membership against its linked
// committee's membership for the committee's configured sync_mode direction.
type SyncDiff struct {
	MailingListID string           `json:"mailing_list_id"`
	CommitteeUID  string           `json:"committee_uid"`
	SyncMode      string           `json:"sync_mode"`
	Adds          []SyncDiffAdd    `json:"adds"`
	Removes       []SyncDiffRemove `json:"removes"`
}
