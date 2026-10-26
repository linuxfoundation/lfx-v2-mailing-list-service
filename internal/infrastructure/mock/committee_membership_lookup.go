// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// FakeCommitteeMembershipLookup is a test double for port.CommitteeMembershipLookup.
// Pre-populate Members with committeeUID → member-list entries as needed.
// Set Err to simulate a transient failure.
type FakeCommitteeMembershipLookup struct {
	Members map[string][]port.CommitteeMember
	Err     error
}

var _ port.CommitteeMembershipLookup = (*FakeCommitteeMembershipLookup)(nil)

// NewFakeCommitteeMembershipLookup returns a FakeCommitteeMembershipLookup with an empty map.
func NewFakeCommitteeMembershipLookup() *FakeCommitteeMembershipLookup {
	return &FakeCommitteeMembershipLookup{Members: make(map[string][]port.CommitteeMember)}
}

// GetCommitteeMembers returns the pre-configured member list for committeeUID.
// Returns Validation for empty UID and NotFound when no entry is set (matching the real implementation).
func (f *FakeCommitteeMembershipLookup) GetCommitteeMembers(_ context.Context, committeeUID string) ([]port.CommitteeMember, error) {
	if committeeUID == "" {
		return nil, errs.NewValidation("committee UID is required")
	}
	if f.Err != nil {
		return nil, f.Err
	}
	members, ok := f.Members[committeeUID]
	if !ok {
		return nil, errs.NewNotFound("committee not found")
	}
	return members, nil
}
