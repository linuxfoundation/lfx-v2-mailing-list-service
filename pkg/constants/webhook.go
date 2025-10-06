// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

// Webhook event types from Groups.io
const (
	SubGroupCreatedEvent       = "created_subgroup"
	SubGroupDeletedEvent       = "deleted_subgroup"
	SubGroupMemberAddedEvent   = "added_member"
	SubGroupMemberRemovedEvent = "removed_member"
	SubGroupMemberBannedEvent  = "ban_members"
)

// Webhook retry configuration
const (
	WebhookMaxRetries     = 3
	WebhookRetryBaseDelay = 100  // milliseconds
	WebhookRetryMaxDelay  = 5000 // milliseconds
)

// Webhook header
const (
	WebhookSignatureHeader = "x-groupsio-signature"
)
