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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/cmd/mailing-list-api/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	infraNATS "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
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
	publisher port.MessagePublisher,
	inviteSender port.InviteSender,
	userReader port.UserReader,
) error {
	if !env.EventingEnabled {
		slog.InfoContext(ctx, "data stream processor disabled (EVENTING_ENABLED not set to true)")
		return nil
	}

	// Open v1-mappings for idempotency and the subgroup service index for lookups.
	// Deferred until here so deployments with eventing disabled don't require either bucket.
	mappings, err := service.NewMappingReaderWriter(ctx, natsClient)
	if err != nil {
		return err
	}
	v1ObjectsKV, err := natsClient.KeyValue(ctx, constants.KVBucketV1Objects)
	if err != nil {
		return fmt.Errorf("failed to access %s KV bucket: %w", constants.KVBucketV1Objects, err)
	}
	lockKV, err := natsClient.KeyValue(ctx, constants.KVBucketServiceDomainLocks)
	if err != nil {
		return fmt.Errorf("failed to access %s KV bucket: %w", constants.KVBucketServiceDomainLocks, err)
	}

	// Build the LFID invite handler for member events when fully configured.
	var memberInviteHandler *svc.MemberInviteHandler
	if inviteSender != nil && userReader != nil && env.SelfServeBaseURL != "" {
		memberInviteHandler = svc.NewMemberInviteHandler(inviteSender, userReader, mappings, v1ObjectsKV, env.SelfServeBaseURL)
	}

	handlerOpts := []eventing.EventHandlerOption{
		eventing.WithSubgroupObjectReader(infraNATS.NewSubgroupObjectReader(v1ObjectsKV)),
		eventing.WithServiceDomainLock(infraNATS.NewServiceDomainLock(lockKV)),
	}
	if memberInviteHandler != nil {
		handlerOpts = append(handlerOpts, eventing.WithMemberInviteHandler(memberInviteHandler))
	}

	handler := eventing.NewEventHandler(publisher, mappings, infraNATS.NewNATSProjectLookup(natsClient), handlerOpts...)
	streamConsumer := infraNATS.NewDataStreamConsumer(handler, time.Duration(env.EventingAckWaitSecs)*time.Second)

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
