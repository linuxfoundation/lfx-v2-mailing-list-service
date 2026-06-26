// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	inviteapi "github.com/linuxfoundation/lfx-v2-invite-service/pkg/api"
	"github.com/nats-io/nats.go"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

const inviteAcceptedCallTimeout = 30 * time.Second

// natsQueueSubscriber is the subset of *nats.NATSClient used by InviteAcceptedSubscriber.
// Keeping it as an interface makes the subscriber testable without a live NATS connection.
type natsQueueSubscriber interface {
	QueueSubscribe(subject, queue string, handler nats.MsgHandler) (*nats.Subscription, error)
}

// InviteAcceptedSubscriber subscribes to lfx.invite-service.invite_accepted events
// and calls the ITX GroupsIO backend to enrich all mailing-list member records tied
// to the acceptor's email with their new username.
type InviteAcceptedSubscriber struct {
	nc               natsQueueSubscriber
	acceptanceClient port.InviteAcceptanceClient
	logger           *slog.Logger
	sub              *nats.Subscription

	ctx    context.Context
	cancel context.CancelFunc
}

// NewInviteAcceptedSubscriber creates a new subscriber but does not start it.
func NewInviteAcceptedSubscriber(
	nc natsQueueSubscriber,
	acceptanceClient port.InviteAcceptanceClient,
	logger *slog.Logger,
) *InviteAcceptedSubscriber {
	return &InviteAcceptedSubscriber{
		nc:               nc,
		acceptanceClient: acceptanceClient,
		logger:           logger,
	}
}

// Start registers the NATS QueueSubscribe and begins processing acceptance events.
func (s *InviteAcceptedSubscriber) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	sub, err := s.nc.QueueSubscribe(
		inviteapi.InviteServiceAcceptedSubject,
		constants.InviteAcceptedQueueGroup,
		s.handle,
	)
	if err != nil {
		if s.cancel != nil {
			s.cancel()
		}
		return err
	}
	s.sub = sub
	s.logger.Info("invite_accepted subscriber started",
		"subject", inviteapi.InviteServiceAcceptedSubject,
		"queue_group", constants.InviteAcceptedQueueGroup,
	)
	return nil
}

// Stop drains the subscription (allowing in-flight handlers to complete), then cancels
// the context. Drain must precede cancel so that in-flight AcceptInvite calls are not
// aborted mid-request by context cancellation.
func (s *InviteAcceptedSubscriber) Stop() {
	if s.sub != nil {
		if err := s.sub.Drain(); err != nil {
			s.logger.Warn("error draining invite_accepted subscription", "error", err)
		}
	}
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *InviteAcceptedSubscriber) handle(msg *nats.Msg) {
	ctx, cancel := context.WithTimeout(s.ctx, inviteAcceptedCallTimeout)
	defer cancel()

	var evt inviteapi.InviteServiceAcceptedEvent
	if err := json.Unmarshal(msg.Data, &evt); err != nil {
		s.logger.Warn("failed to parse InviteServiceAcceptedEvent; discarding", "error", err)
		return
	}

	if err := processInviteAcceptedEvent(ctx, evt, s.acceptanceClient, s.logger); err != nil {
		s.logger.Warn("invite_accepted enrichment failed; best-effort, not retrying",
			"error", err,
			"email", evt.Recipient.Email,
			"username", evt.AcceptedBy,
		)
	}
}

// processInviteAcceptedEvent validates an invite acceptance event and calls ITX to enrich
// all mailing-list member records for the acceptor's email. Events for resource types
// other than mailing_list are silently ignored.
func processInviteAcceptedEvent(
	ctx context.Context,
	evt inviteapi.InviteServiceAcceptedEvent,
	client port.InviteAcceptanceClient,
	logger *slog.Logger,
) error {
	// Only process invites that belong to this service's resource type.
	// Resource.Type is empty for legacy events that pre-date the structured field;
	// those are processed to preserve backward compatibility.
	if evt.Resource.Type != "" && evt.Resource.Type != constants.ResourceTypeMailingList {
		logger.Debug("invite_accepted event is for a different resource type; ignoring",
			"resource_type", evt.Resource.Type)
		return nil
	}

	email := evt.Recipient.Email
	username := evt.AcceptedBy

	if email == "" || username == "" {
		logger.Warn("invite_accepted event missing required fields; discarding",
			"has_email", email != "",
			"has_username", username != "",
		)
		return nil
	}

	logger.Debug("received invite_accepted event",
		"email", email,
		"username", username,
	)

	if err := client.AcceptInvite(ctx, email, username); err != nil {
		return err
	}

	logger.Info("invite_accepted enrichment complete",
		"email", email,
		"username", username,
	)
	return nil
}
