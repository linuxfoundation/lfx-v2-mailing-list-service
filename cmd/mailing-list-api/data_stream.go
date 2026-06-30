// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/cmd/mailing-list-api/eventing"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/cmd/mailing-list-api/service"
	infraNATS "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	svc "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// handleDataStream starts the durable JetStream consumer that processes DynamoDB KV
// change events for GroupsIO entities (service, subgroup, member).
//
// Enabled only when EVENTING_ENABLED=true. If disabled, the function
// is a no-op and returns nil.
//
// When inviteSender and userReader are non-nil and selfServeBaseURL is non-empty,
// a MemberInviteHandler is constructed and wired into the member event processor.
func handleDataStream(
	ctx context.Context,
	wg *sync.WaitGroup,
	inviteSender port.InviteSender,
	userReader port.UserReader,
	selfServeBaseURL string,
) error {
	if !dataStreamEnabled() {
		slog.InfoContext(ctx, "data stream processor disabled (EVENTING_ENABLED not set to true)")
		return nil
	}

	natsClient := service.GetNATSClient(ctx)
	mappings := service.MappingReaderWriter(ctx)

	// Build the LFID invite handler for member events when fully configured.
	var memberInviteHandler *svc.MemberInviteHandler
	if inviteSender != nil && userReader != nil && selfServeBaseURL != "" {
		v1ObjectsKV, kvErr := natsClient.KeyValue(ctx, constants.KVBucketV1Objects)
		if kvErr != nil {
			slog.WarnContext(ctx, "failed to open v1-objects KV for invite handler; invite sending disabled",
				"error", kvErr)
		} else {
			memberInviteHandler = svc.NewMemberInviteHandler(inviteSender, userReader, mappings, v1ObjectsKV, selfServeBaseURL)
		}
	}

	handlerOpts := []eventing.EventHandlerOption{}
	if memberInviteHandler != nil {
		handlerOpts = append(handlerOpts, eventing.WithMemberInviteHandler(memberInviteHandler))
	}

	handler := eventing.NewEventHandler(service.MessagePublisher(ctx), mappings, infraNATS.NewNATSProjectLookup(natsClient), handlerOpts...)
	streamConsumer := infraNATS.NewDataStreamConsumer(handler)

	cfg := dataStreamConfig()
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

// dataStreamEnabled reports whether the data stream processor has been opted into.
func dataStreamEnabled() bool {
	return os.Getenv("EVENTING_ENABLED") == "true"
}

// dataStreamConfig builds eventing.Config from environment variables with
// sensible defaults.
func dataStreamConfig() eventing.Config {
	consumerName := os.Getenv("EVENTING_CONSUMER_NAME")
	if consumerName == "" {
		consumerName = "mailing-list-service-kv-consumer"
	}

	maxDeliver := envInt("EVENTING_MAX_DELIVER", 3)
	maxAckPending := envInt("EVENTING_MAX_ACK_PENDING", 1000)
	ackWaitSecs := envInt("EVENTING_ACK_WAIT_SECS", 30)

	return eventing.Config{
		ConsumerName:  consumerName,
		StreamName:    "KV_" + constants.KVBucketV1Objects,
		MaxDeliver:    maxDeliver,
		AckWait:       time.Duration(ackWaitSecs) * time.Second,
		MaxAckPending: maxAckPending,
	}
}

// envInt reads an integer environment variable, returning defaultVal if the
// variable is absent or cannot be parsed.
func envInt(key string, defaultVal int) int {
	s := os.Getenv(key)
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return n
}
