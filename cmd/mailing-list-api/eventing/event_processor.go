// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	infraNATS "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/nats-io/nats.go"
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
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// natsEventProcessor is the NATS JetStream implementation of EventProcessor.
// It takes ownership of conn — draining and closing it on Stop.
type natsEventProcessor struct {
	natsConn       *nats.Conn
	jsInstance     jetstream.JetStream
	consumer       jetstream.Consumer
	consumeCtx     jetstream.ConsumeContext
	streamConsumer infraNATS.DataStreamProcessor
	config         Config
}

// NewEventProcessor creates an EventProcessor from an existing NATS connection and handler.
// conn ownership is transferred — the processor drains and closes it on Stop.
func NewEventProcessor(ctx context.Context, cfg Config, conn *nats.Conn, streamConsumer infraNATS.DataStreamProcessor) (EventProcessor, error) {
	js, err := jetstream.New(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &natsEventProcessor{
		natsConn:       conn,
		jsInstance:     js,
		streamConsumer: streamConsumer,
		config:         cfg,
	}, nil
}

// Start creates (or resumes) the durable JetStream consumer and processes messages
// until ctx is cancelled.
func (ep *natsEventProcessor) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "starting data stream processor", "consumer_name", ep.config.ConsumerName)

	consumer, err := ep.jsInstance.CreateOrUpdateConsumer(ctx, ep.config.StreamName, jetstream.ConsumerConfig{
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
		func(msg jetstream.Msg) {
			ep.streamConsumer.Process(ctx, msg)
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

// Stop drains and closes the dedicated NATS connection.
func (ep *natsEventProcessor) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "stopping data stream processor")

	if ep.consumeCtx != nil {
		ep.consumeCtx.Stop()
		slog.InfoContext(ctx, "data stream consumer stopped")
	}

	if ep.natsConn != nil {
		if err := ep.natsConn.Drain(); err != nil {
			slog.ErrorContext(ctx, "error draining data stream NATS connection", "error", err)
		}
		ep.natsConn.Close()
		slog.InfoContext(ctx, "data stream NATS connection closed")
	}

	slog.InfoContext(ctx, "data stream processor stopped")
	return nil
}
