// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/cmd/mailing-list-api/eventing"
	infraNATS "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	svc "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// handleDataStream starts the durable JetStream consumer that processes DynamoDB KV
// change events for GroupsIO entities (service, subgroup, member).
//
// Enabled only when env.EventingEnabled is true. If disabled, the function
// is a no-op and returns nil.
//
// When inviteSender and userReader are non-nil and env.SelfServeBaseURL is non-empty,
// a MemberInviteHandler is constructed and wired into the member event processor.
func handleDataStream(
	ctx context.Context,
	wg *sync.WaitGroup,
	env environment,
	natsClient *infraNATS.NATSClient,
	mappings port.MappingReaderWriter,
	publisher port.MessagePublisher,
	inviteSender port.InviteSender,
	userReader port.UserReader,
) error {
	if !env.EventingEnabled {
		slog.InfoContext(ctx, "data stream processor disabled (EVENTING_ENABLED not set to true)")
		return nil
	}

	// Build the LFID invite handler for member events when fully configured.
	var memberInviteHandler *svc.MemberInviteHandler
	if inviteSender != nil && userReader != nil && env.SelfServeBaseURL != "" {
		v1ObjectsKV, kvErr := natsClient.KeyValue(ctx, constants.KVBucketV1Objects)
		if kvErr != nil {
			slog.WarnContext(ctx, "failed to open v1-objects KV for invite handler; invite sending disabled",
				"error", kvErr)
		} else {
			memberInviteHandler = svc.NewMemberInviteHandler(inviteSender, userReader, mappings, v1ObjectsKV, env.SelfServeBaseURL)
		}
	}

	handlerOpts := []eventing.EventHandlerOption{}
	if memberInviteHandler != nil {
		handlerOpts = append(handlerOpts, eventing.WithMemberInviteHandler(memberInviteHandler))
	}

	handler := eventing.NewEventHandler(publisher, mappings, infraNATS.NewNATSProjectLookup(natsClient), handlerOpts...)
	streamConsumer := infraNATS.NewDataStreamConsumer(handler)

	cfg := eventing.Config{
		ConsumerName:  env.EventingConsumerName,
		StreamName:    "KV_" + constants.KVBucketV1Objects,
		MaxDeliver:    env.EventingMaxDeliver,
		AckWait:       time.Duration(env.EventingAckWaitSecs) * time.Second,
		MaxAckPending: env.EventingMaxAckPending,
	}

	processor, err := eventing.NewEventProcessor(ctx, cfg, natsClient)
	if err != nil {
		return fmt.Errorf("failed to create data stream processor: %w", err)
	}

	slog.InfoContext(ctx, "data stream processor created",
		"consumer_name", cfg.ConsumerName,
		"stream_name", cfg.StreamName,
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := processor.Start(ctx, streamConsumer); err != nil {
			slog.ErrorContext(ctx, "data stream processor exited with error", "error", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		stopCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownSeconds*time.Second)
		defer cancel()
		if err := processor.Stop(stopCtx); err != nil {
			slog.ErrorContext(stopCtx, "error stopping data stream processor", "error", err)
		}
	}()

	return nil
}
