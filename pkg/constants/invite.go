// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package constants

// InviteRoleMember is the invite-service role for mailing-list members who do not yet
// have an LFID.
const InviteRoleMember = "Member"

// InviteAcceptedQueueGroup is the NATS queue group used by the mailing list service
// when subscribing to invite_accepted events. Using a unique queue group ensures only
// one mailing-list-service replica processes each event.
const InviteAcceptedQueueGroup = "mailing-list-service-invite-accepted"

// KVMemberLFIDInviteSentPrefix is the v1-mappings key prefix used to track whether an
// LFID invite has already been sent for a given mailing list member. The full key is
// "<prefix>.<memberUID>". A "pending" value indicates the invite is in-flight; any
// other non-empty value is the InviteUID returned by the invite service.
const KVMemberLFIDInviteSentPrefix = "groupsio-member-lfid-invite-sent"
