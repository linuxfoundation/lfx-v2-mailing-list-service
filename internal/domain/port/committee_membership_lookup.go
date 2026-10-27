// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import "context"

// CommitteeMember is the minimal committee membership record needed to
// reconcile a mailing list's membership against its linked committee.
type CommitteeMember struct {
	UID   string `json:"uid"`
	Email string `json:"email"`
	// Name is the committee member's display name, when lfx-v2-committee-service's response
	// includes one. It is only a label for human review — no search-by-name capability exists
	// upstream to turn it into a real identity-match fallback (see
	// linuxfoundation/lfx-self-serve#4364).
	Name string `json:"name,omitempty"`
}

// CommitteeMembershipLookup resolves the current member list for a v2 committee.
// Implementations query lfx-v2-committee-service via NATS request-reply on
// constants.CommitteeListMembersSubject.
type CommitteeMembershipLookup interface {
	// GetCommitteeMembers returns the current members of the given v2 committee UID.
	// Returns NotFound when no committee exists for the supplied UID.
	GetCommitteeMembers(ctx context.Context, committeeUID string) ([]CommitteeMember, error)
}
