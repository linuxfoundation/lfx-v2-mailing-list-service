// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// FakeCommitteeProjectLookup is a test double for port.CommitteeProjectLookup.
// Pre-populate Projects with committeeUID → projectUID entries as needed.
// Set Err to simulate a transient failure.
type FakeCommitteeProjectLookup struct {
	Projects map[string]string
	Err      error
}

var _ port.CommitteeProjectLookup = (*FakeCommitteeProjectLookup)(nil)

// NewFakeCommitteeProjectLookup returns a FakeCommitteeProjectLookup with an empty map.
func NewFakeCommitteeProjectLookup() *FakeCommitteeProjectLookup {
	return &FakeCommitteeProjectLookup{Projects: make(map[string]string)}
}

// GetCommitteeProject returns the pre-configured project UID for committeeUID.
// Returns Validation for empty UID and NotFound when no entry is set (matching the real implementation).
func (f *FakeCommitteeProjectLookup) GetCommitteeProject(_ context.Context, committeeUID string) (string, error) {
	if committeeUID == "" {
		return "", errs.NewValidation("committee UID is required")
	}
	if f.Err != nil {
		return "", f.Err
	}
	projectUID, ok := f.Projects[committeeUID]
	if !ok {
		return "", errs.NewNotFound("committee not found")
	}
	return projectUID, nil
}
