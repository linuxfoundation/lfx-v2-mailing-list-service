// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// FakeProjectLookup is a test double for port.ProjectLookup.
// Pre-populate Slugs with projectUID → slug entries as needed.
// Set Err to simulate a transient failure.
type FakeProjectLookup struct {
	Slugs map[string]string
	Err   error
}

var _ port.ProjectLookup = (*FakeProjectLookup)(nil)

// NewFakeProjectLookup returns a FakeProjectLookup with an empty slug map.
func NewFakeProjectLookup() *FakeProjectLookup {
	return &FakeProjectLookup{Slugs: make(map[string]string)}
}

// GetProjectSlug returns the pre-configured slug for projectUID, or an empty
// string if no entry is set (matching the real implementation's behaviour for
// projects without a slug).
func (f *FakeProjectLookup) GetProjectSlug(_ context.Context, projectUID string) (string, error) {
	if f.Err != nil {
		return "", f.Err
	}
	return f.Slugs[projectUID], nil
}
