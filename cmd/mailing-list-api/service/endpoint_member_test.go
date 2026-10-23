// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strp(s string) *string { return &s }

// fakeAccessChecker is a test double for port.AccessChecker.
type fakeAccessChecker struct {
	isWriter bool
	err      error
}

func (f *fakeAccessChecker) HasRelation(_ context.Context, _, _, _ string) (bool, error) {
	return f.isWriter, f.err
}

func TestAuthorizeAddGroupsioMember(t *testing.T) {
	tests := []struct {
		name                     string
		principal                any
		callerEmail              any
		checker                  *fakeAccessChecker
		payload                  *mailinglist.AddGroupsioMemberPayload
		expectErr                bool
		expectServiceUnavailable bool
	}{
		{
			name:      "verified writer may add another member with privileged fields",
			principal: "writer1",
			checker:   &fakeAccessChecker{isWriter: true},
			payload: &mailinglist.AddGroupsioMemberPayload{
				SubgroupID: "list-123",
				Email:      strp("someone-else@example.com"),
				MemberType: strp("direct"),
				ModStatus:  strp("owner"),
			},
			expectErr: false,
		},
		{
			name:        "verified writer may add themselves with privileged fields",
			principal:   "writer1",
			callerEmail: "writer1@example.com",
			checker:     &fakeAccessChecker{isWriter: true},
			payload: &mailinglist.AddGroupsioMemberPayload{
				SubgroupID: "list-123",
				Email:      strp("writer1@example.com"),
				ModStatus:  strp("moderator"),
			},
			expectErr: false,
		},
		{
			name:        "non-writer self-service add with no privileged fields is allowed",
			principal:   "user1",
			callerEmail: "self@example.com",
			checker:     &fakeAccessChecker{isWriter: false},
			payload: &mailinglist.AddGroupsioMemberPayload{
				SubgroupID: "list-123",
				Email:      strp("self@example.com"),
			},
			expectErr: false,
		},
		{
			name:        "non-writer self-service add is case-insensitive on email match",
			principal:   "user1",
			callerEmail: "Self@Example.com",
			checker:     &fakeAccessChecker{isWriter: false},
			payload: &mailinglist.AddGroupsioMemberPayload{
				SubgroupID: "list-123",
				Email:      strp("self@example.com"),
			},
			expectErr: false,
		},
		{
			name:        "non-writer self-service add with non-direct member_type is rejected",
			principal:   "user1",
			callerEmail: "self@example.com",
			checker:     &fakeAccessChecker{isWriter: false},
			payload: &mailinglist.AddGroupsioMemberPayload{
				SubgroupID: "list-123",
				Email:      strp("self@example.com"),
				MemberType: strp("invite"),
			},
			expectErr: true,
		},
		{
			name:        "non-writer self-service add with moderator mod_status is rejected",
			principal:   "user1",
			callerEmail: "self@example.com",
			checker:     &fakeAccessChecker{isWriter: false},
			payload: &mailinglist.AddGroupsioMemberPayload{
				SubgroupID: "list-123",
				Email:      strp("self@example.com"),
				ModStatus:  strp(constants.ModStatusModerator),
			},
			expectErr: true,
		},
		{
			name:        "non-writer caller submitting someone else's email is rejected",
			principal:   "user1",
			callerEmail: "user1@example.com",
			checker:     &fakeAccessChecker{isWriter: false},
			payload: &mailinglist.AddGroupsioMemberPayload{
				SubgroupID: "list-123",
				Email:      strp("someone-else@example.com"),
			},
			expectErr: true,
		},
		{
			name:      "missing principal is rejected",
			principal: nil,
			checker:   &fakeAccessChecker{isWriter: true},
			payload: &mailinglist.AddGroupsioMemberPayload{
				SubgroupID: "list-123",
				Email:      strp("someone@example.com"),
			},
			expectErr: true,
		},
		{
			name:      "access check failure returns service unavailable",
			principal: "user1",
			checker:   &fakeAccessChecker{err: errors.New("fga-sync unavailable")},
			payload: &mailinglist.AddGroupsioMemberPayload{
				SubgroupID: "list-123",
				Email:      strp("someone@example.com"),
			},
			expectErr:                true,
			expectServiceUnavailable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.principal != nil {
				ctx = context.WithValue(ctx, constants.PrincipalContextID, tt.principal)
			}
			if tt.callerEmail != nil {
				ctx = context.WithValue(ctx, constants.EmailContextID, tt.callerEmail)
			}

			s := &mailingListAPI{accessChecker: tt.checker}
			err := s.authorizeAddGroupsioMember(ctx, tt.payload)

			if tt.expectErr {
				require.Error(t, err)
				if tt.expectServiceUnavailable {
					var serviceUnavailable errs.ServiceUnavailable
					assert.ErrorAs(t, err, &serviceUnavailable)
				} else {
					var unauthorized errs.Unauthorized
					assert.ErrorAs(t, err, &unauthorized)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
