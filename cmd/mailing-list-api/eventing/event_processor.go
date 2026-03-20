// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	infraNATS "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/nats-io/nats.go/jetstream"
)

// Config holds the configuration for an EventProcessor.
type Config struct {
	// ConsumerName is the durable consumer name (survives restarts).
	ConsumerName string
	// StreamName is the JetStream stream to consume from (e.g. "KV_v1-objects").
	StreamName string
	// MaxDeliver is the maximum number of delivery attempts before giving up.
	MaxDeliver int
	// AckWait is how long the server waits for an ACK before redelivering.
	AckWait time.Duration
	// MaxAckPending is the maximum number of unacknowledged messages in flight.
	MaxAckPending int
}

// EventProcessor is the interface for JetStream KV bucket event consumers.
// Start blocks until ctx is cancelled; Stop performs a graceful shutdown.
type EventProcessor interface {
	Start(ctx context.Context, streamConsumer port.DataStreamProcessor) error
	Stop(ctx context.Context) error
}

// natsEventProcessor is the NATS JetStream implementation of EventProcessor.
type natsEventProcessor struct {
	natsClient *infraNATS.NATSClient
	consumer   jetstream.Consumer
	consumeCtx jetstream.ConsumeContext
	config     Config
}

// NewEventProcessor creates an EventProcessor backed by the given NATSClient.
func NewEventProcessor(_ context.Context, cfg Config, natsClient *infraNATS.NATSClient) (EventProcessor, error) {
	return &natsEventProcessor{
		natsClient: natsClient,
		config:     cfg,
	}, nil
}

// Start creates (or resumes) the durable JetStream consumer and processes messages
// until ctx is cancelled.
func (ep *natsEventProcessor) Start(ctx context.Context, streamConsumer port.DataStreamProcessor) error {
	slog.InfoContext(ctx, "starting data stream processor", "consumer_name", ep.config.ConsumerName)

	consumer, err := ep.natsClient.CreateOrUpdateConsumer(ctx, ep.config.StreamName, jetstream.ConsumerConfig{
		Name:    ep.config.ConsumerName,
		Durable: ep.config.ConsumerName,
		// DeliverLastPerSubjectPolicy resumes from the last seen record per KV key after a
		// restart, avoiding a full replay while ensuring no in-flight event is dropped.
		DeliverPolicy: jetstream.DeliverLastPerSubjectPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubjects: []string{
			"$KV.v1-objects.itx-groupsio-v2-service.>",
			"$KV.v1-objects.itx-groupsio-v2-subgroup.>",
			"$KV.v1-objects.itx-groupsio-v2-member.>",
		},
		MaxDeliver:    ep.config.MaxDeliver,
		AckWait:       ep.config.AckWait,
		MaxAckPending: ep.config.MaxAckPending,
		Description:   "Durable KV watcher for mailing-list-service GroupsIO entities",
	})
	if err != nil {
		return fmt.Errorf("failed to create or update consumer: %w", err)
	}
	ep.consumer = consumer

	consumeCtx, err := consumer.Consume(
		func(jMsg jetstream.Msg) {
			meta, err := jMsg.Metadata()
			if err != nil {
				slog.ErrorContext(ctx, "failed to read stream message metadata, ACKing to avoid poison pill",
					"subject", jMsg.Subject(), "error", err)
				_ = jMsg.Ack()
				return
			}
			streamConsumer.Process(ctx, model.StreamMessage{
				Key:           kvKey(jMsg.Subject()),
				Data:          jMsg.Data(),
				IsRemoval:     isKVRemoval(jMsg),
				DeliveryCount: meta.NumDelivered,
				Ack:           jMsg.Ack,
				Nak:           jMsg.NakWithDelay,
			})
		},
		jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
			slog.With("error", err).Error("data stream KV consumer error")
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming messages: %w", err)
	}
	ep.consumeCtx = consumeCtx

	slog.InfoContext(ctx, "data stream processor started successfully")
	<-ctx.Done()
	slog.InfoContext(ctx, "data stream processor context cancelled")
	return nil
}

// Stop halts the JetStream consumer. The NATS connection lifecycle is managed
// by the caller (NATSClient).
func (ep *natsEventProcessor) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "stopping data stream processor")

	if ep.consumeCtx != nil {
		ep.consumeCtx.Stop()
		slog.InfoContext(ctx, "data stream consumer stopped")
	}

	slog.InfoContext(ctx, "data stream processor stopped")
	return nil
}

// kvKey strips the "$KV.<bucket>." prefix from a JetStream KV subject,
// returning the bare key. Subject format: $KV.<bucket>.<key>
func kvKey(subject string) string {
	idx := strings.Index(subject, ".")
	if idx == -1 {
		return subject
	}
	idx2 := strings.Index(subject[idx+1:], ".")
	if idx2 == -1 {
		return subject
	}
	return subject[idx+idx2+2:]
}

func isKVRemoval(msg jetstream.Msg) bool {
	op := msg.Headers().Get("Kv-Operation")
	return op == "DEL" || op == "PURGE"
}
