// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// ---------------------------------------------------------------------------
// Stub acceptance client
// ---------------------------------------------------------------------------

type stubAcceptanceClient struct {
	calls []acceptCall
	err   error
}

type acceptCall struct {
	email    string
	username string
}

func (s *stubAcceptanceClient) AcceptInvite(_ context.Context, email, username string) error {
	s.calls = append(s.calls, acceptCall{email: email, username: username})
	return s.err
}

func silentLog() *slog.Logger { return slog.New(slog.DiscardHandler) }

// ---------------------------------------------------------------------------
// processInviteAcceptedEvent
// ---------------------------------------------------------------------------

func TestProcessInviteAcceptedEvent_WrongResourceType_Ignored(t *testing.T) {
	client := &stubAcceptanceClient{}
	evt := inviteapi.InviteServiceAcceptedEvent{
		Invite: inviteapi.Invite{
			Recipient:  inviteapi.Recipient{Email: "user@example.com"},
			Resource:   inviteapi.Resource{Type: "voting"},
			AcceptedBy: "jsmith",
		},
	}
	err := processInviteAcceptedEvent(context.Background(), evt, client, silentLog())
	require.NoError(t, err)
	assert.Empty(t, client.calls, "non-mailing_list resource type should be ignored")
}

func TestProcessInviteAcceptedEvent_EmptyResourceType_BackwardCompat(t *testing.T) {
	// Legacy events that pre-date the Resource.Type field have an empty type.
	// They must be processed to preserve backward compatibility.
	client := &stubAcceptanceClient{}
	evt := inviteapi.InviteServiceAcceptedEvent{
		Invite: inviteapi.Invite{
			Recipient:  inviteapi.Recipient{Email: "legacy@example.com"},
			Resource:   inviteapi.Resource{Type: ""}, // empty
			AcceptedBy: "legacyuser",
		},
	}
	err := processInviteAcceptedEvent(context.Background(), evt, client, silentLog())
	require.NoError(t, err)
	require.Len(t, client.calls, 1)
	assert.Equal(t, "legacy@example.com", client.calls[0].email)
	assert.Equal(t, "legacyuser", client.calls[0].username)
}

func TestProcessInviteAcceptedEvent_MailingListResourceType_Enriched(t *testing.T) {
	client := &stubAcceptanceClient{}
	evt := inviteapi.InviteServiceAcceptedEvent{
		Invite: inviteapi.Invite{
			Recipient:  inviteapi.Recipient{Email: "alice@example.com"},
			Resource:   inviteapi.Resource{Type: constants.ResourceTypeMailingList},
			AcceptedBy: "alice",
		},
	}
	err := processInviteAcceptedEvent(context.Background(), evt, client, silentLog())
	require.NoError(t, err)
	require.Len(t, client.calls, 1)
	assert.Equal(t, "alice@example.com", client.calls[0].email)
	assert.Equal(t, "alice", client.calls[0].username)
}

func TestProcessInviteAcceptedEvent_MissingEmail_Discards(t *testing.T) {
	client := &stubAcceptanceClient{}
	evt := inviteapi.InviteServiceAcceptedEvent{
		Invite: inviteapi.Invite{
			Recipient:  inviteapi.Recipient{Email: ""},
			Resource:   inviteapi.Resource{Type: constants.ResourceTypeMailingList},
			AcceptedBy: "bob",
		},
	}
	err := processInviteAcceptedEvent(context.Background(), evt, client, silentLog())
	require.NoError(t, err)
	assert.Empty(t, client.calls, "event with no email should be discarded without calling client")
}

func TestProcessInviteAcceptedEvent_MissingUsername_Discards(t *testing.T) {
	client := &stubAcceptanceClient{}
	evt := inviteapi.InviteServiceAcceptedEvent{
		Invite: inviteapi.Invite{
			Recipient:  inviteapi.Recipient{Email: "bob@example.com"},
			Resource:   inviteapi.Resource{Type: constants.ResourceTypeMailingList},
			AcceptedBy: "",
		},
	}
	err := processInviteAcceptedEvent(context.Background(), evt, client, silentLog())
	require.NoError(t, err)
	assert.Empty(t, client.calls, "event with no username should be discarded without calling client")
}

func TestProcessInviteAcceptedEvent_EnrichmentError_Propagated(t *testing.T) {
	itxErr := errors.New("ITX internal error")
	client := &stubAcceptanceClient{err: itxErr}
	evt := inviteapi.InviteServiceAcceptedEvent{
		Invite: inviteapi.Invite{
			Recipient:  inviteapi.Recipient{Email: "charlie@example.com"},
			Resource:   inviteapi.Resource{Type: constants.ResourceTypeMailingList},
			AcceptedBy: "charlie",
		},
	}
	err := processInviteAcceptedEvent(context.Background(), evt, client, silentLog())
	assert.ErrorIs(t, err, itxErr, "enrichment error should be propagated to the caller")
	require.Len(t, client.calls, 1)
}

func TestProcessInviteAcceptedEvent_OtherResourceType_NotEnriched(t *testing.T) {
	// Survey and voting resource types must not trigger mailing-list ITX calls.
	for _, rt := range []string{"survey", "vote", "project"} {
		rt := rt
		t.Run(rt, func(t *testing.T) {
			client := &stubAcceptanceClient{}
			evt := inviteapi.InviteServiceAcceptedEvent{
				Invite: inviteapi.Invite{
					Recipient:  inviteapi.Recipient{Email: "user@example.com"},
					Resource:   inviteapi.Resource{Type: rt},
					AcceptedBy: "someuser",
				},
			}
			err := processInviteAcceptedEvent(context.Background(), evt, client, silentLog())
			require.NoError(t, err)
			assert.Empty(t, client.calls, "resource type %q should not trigger enrichment", rt)
		})
	}
}
