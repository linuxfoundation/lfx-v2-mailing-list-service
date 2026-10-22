// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"

	mailinglist "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strp(s string) *string { return &s }

func TestAuthorizeAddGroupsioMember(t *testing.T) {
	tests := []struct {
		name        string
		callerEmail any
		payload     *mailinglist.AddGroupsioMemberPayload
		expectErr   bool
	}{
		{
			name:        "no caller email context value falls back to Heimdall gate",
			callerEmail: nil,
			payload:     &mailinglist.AddGroupsioMemberPayload{Email: strp("someone@example.com")},
			expectErr:   false,
		},
		{
			name:        "empty caller email falls back to Heimdall gate",
			callerEmail: "",
			payload:     &mailinglist.AddGroupsioMemberPayload{Email: strp("someone@example.com")},
			expectErr:   false,
		},
		{
			name:        "caller email does not match payload email falls back to Heimdall gate",
			callerEmail: "writer@example.com",
			payload: &mailinglist.AddGroupsioMemberPayload{
				Email:      strp("someone-else@example.com"),
				MemberType: strp("direct"),
				ModStatus:  strp("owner"),
			},
			expectErr: false,
		},
		{
			name:        "self-service add with no privileged fields is allowed",
			callerEmail: "self@example.com",
			payload:     &mailinglist.AddGroupsioMemberPayload{Email: strp("self@example.com")},
			expectErr:   false,
		},
		{
			name:        "self-service add is case-insensitive on email match",
			callerEmail: "Self@Example.com",
			payload:     &mailinglist.AddGroupsioMemberPayload{Email: strp("self@example.com")},
			expectErr:   false,
		},
		{
			name:        "self-service add with member_type direct is allowed",
			callerEmail: "self@example.com",
			payload: &mailinglist.AddGroupsioMemberPayload{
				Email:      strp("self@example.com"),
				MemberType: strp("direct"),
			},
			expectErr: false,
		},
		{
			name:        "self-service add with mod_status none is allowed",
			callerEmail: "self@example.com",
			payload: &mailinglist.AddGroupsioMemberPayload{
				Email:     strp("self@example.com"),
				ModStatus: strp(constants.ModStatusNone),
			},
			expectErr: false,
		},
		{
			name:        "self-service add with non-direct member_type is rejected",
			callerEmail: "self@example.com",
			payload: &mailinglist.AddGroupsioMemberPayload{
				Email:      strp("self@example.com"),
				MemberType: strp("invite"),
			},
			expectErr: true,
		},
		{
			name:        "self-service add with moderator mod_status is rejected",
			callerEmail: "self@example.com",
			payload: &mailinglist.AddGroupsioMemberPayload{
				Email:     strp("self@example.com"),
				ModStatus: strp(constants.ModStatusModerator),
			},
			expectErr: true,
		},
		{
			name:        "self-service add with owner mod_status is rejected",
			callerEmail: "self@example.com",
			payload: &mailinglist.AddGroupsioMemberPayload{
				Email:     strp("self@example.com"),
				ModStatus: strp(constants.ModStatusOwner),
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.callerEmail != nil {
				ctx = context.WithValue(ctx, constants.EmailContextID, tt.callerEmail)
			}

			err := authorizeAddGroupsioMember(ctx, tt.payload)

			if tt.expectErr {
				require.Error(t, err)
				var unauthorized errs.Unauthorized
				assert.ErrorAs(t, err, &unauthorized)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
