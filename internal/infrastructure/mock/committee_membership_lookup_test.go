// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeCommitteeMembershipLookup_EmptyUID_ReturnsValidationError(t *testing.T) {
	lookup := NewFakeCommitteeMembershipLookup()
	members, err := lookup.GetCommitteeMembers(context.Background(), "")
	require.Error(t, err)
	var validationErr errs.Validation
	assert.True(t, errors.As(err, &validationErr))
	assert.Nil(t, members)
}

func TestFakeCommitteeMembershipLookup_Unconfigured_ReturnsNotFound(t *testing.T) {
	lookup := NewFakeCommitteeMembershipLookup()
	members, err := lookup.GetCommitteeMembers(context.Background(), "committee-1")
	require.Error(t, err)
	var notFoundErr errs.NotFound
	assert.True(t, errors.As(err, &notFoundErr))
	assert.Nil(t, members)
}

func TestFakeCommitteeMembershipLookup_Configured_ReturnsMembers(t *testing.T) {
	lookup := NewFakeCommitteeMembershipLookup()
	lookup.Members["committee-1"] = []port.CommitteeMember{
		{UID: "member-1", Email: "alice@example.com"},
		{UID: "member-2", Email: "bob@example.com"},
	}

	members, err := lookup.GetCommitteeMembers(context.Background(), "committee-1")
	require.NoError(t, err)
	assert.Equal(t, []port.CommitteeMember{
		{UID: "member-1", Email: "alice@example.com"},
		{UID: "member-2", Email: "bob@example.com"},
	}, members)
}

func TestFakeCommitteeMembershipLookup_DifferentCommitteesAreIndependent(t *testing.T) {
	lookup := NewFakeCommitteeMembershipLookup()
	lookup.Members["committee-1"] = []port.CommitteeMember{{UID: "member-1", Email: "alice@example.com"}}
	lookup.Members["committee-2"] = []port.CommitteeMember{{UID: "member-2", Email: "bob@example.com"}}

	members1, err := lookup.GetCommitteeMembers(context.Background(), "committee-1")
	require.NoError(t, err)
	members2, err := lookup.GetCommitteeMembers(context.Background(), "committee-2")
	require.NoError(t, err)

	assert.Equal(t, "alice@example.com", members1[0].Email)
	assert.Equal(t, "bob@example.com", members2[0].Email)
}

func TestFakeCommitteeMembershipLookup_Err_ReturnsConfiguredError(t *testing.T) {
	lookup := NewFakeCommitteeMembershipLookup()
	lookup.Members["committee-1"] = []port.CommitteeMember{{UID: "member-1", Email: "alice@example.com"}}
	lookup.Err = errs.NewServiceUnavailable("nats unavailable")

	members, err := lookup.GetCommitteeMembers(context.Background(), "committee-1")
	require.Error(t, err)
	var unavailableErr errs.ServiceUnavailable
	assert.True(t, errors.As(err, &unavailableErr))
	assert.Nil(t, members)
}
