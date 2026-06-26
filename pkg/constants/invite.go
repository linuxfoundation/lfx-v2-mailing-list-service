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

// KVMemberLFIDInviteSentPrefix is the v1-mappings key prefix used to dedup LFID invites
// for mailing-list members. The full key is "<prefix>.<memberUID>". The key is created
// atomically with value "pending" before SendInvite is called (preventing concurrent
// duplicate sends); on success it is overwritten with the InviteUID; on failure it is
// purged so JetStream redelivery can retry.
const KVMemberLFIDInviteSentPrefix = "groupsio-member-lfid-invite-sent"
