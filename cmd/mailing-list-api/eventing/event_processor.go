// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
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
	// ShutdownTimeout is the graceful in-flight completion window.
	ShutdownTimeout time.Duration
}

// EventProcessor is the interface for JetStream KV bucket event consumers.
// Start blocks until ctx is cancelled; Stop performs a graceful shutdown.
type EventProcessor interface {
	Start(ctx context.Context, streamConsumer port.DataStreamProcessor) error
	Stop(ctx context.Context) error
}

// natsEventProcessor is the NATS JetStream implementation of EventProcessor.
type natsEventProcessor struct {
	natsClient       consumerManager
	consumer         jetstream.Consumer
	consumeCtx       jetstream.ConsumeContext
	config           Config
	mu               sync.Mutex
	started          bool
	stopRequested    bool
	ready            chan struct{}
	processingCancel context.CancelFunc
	active           sync.WaitGroup
	waitOnce         sync.Once
	callbacksDone    chan struct{}
}

type consumerManager interface {
	CreateOrUpdateConsumer(context.Context, string, jetstream.ConsumerConfig) (jetstream.Consumer, error)
}

// NewEventProcessor creates an EventProcessor backed by the given NATSClient.
func NewEventProcessor(_ context.Context, cfg Config, natsClient *infraNATS.NATSClient) (EventProcessor, error) {
	return &natsEventProcessor{
		natsClient:    natsClient,
		config:        cfg,
		callbacksDone: make(chan struct{}),
	}, nil
}

// Start creates (or resumes) the durable JetStream consumer and processes messages
// until ctx is cancelled.
func (ep *natsEventProcessor) Start(ctx context.Context, streamConsumer port.DataStreamProcessor) error {
	// Lifecycle cancellation stops admission, not the already-running handler.
	// Keep its context usable through the graceful completion-write window.
	processingCtx, cancelProcessing := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelProcessing()
	ep.mu.Lock()
	if ep.started {
		ep.mu.Unlock()
		return fmt.Errorf("data stream processor already started")
	}
	ep.started = true
	ep.ready = make(chan struct{})
	ep.processingCancel = cancelProcessing
	if ep.callbacksDone == nil {
		ep.callbacksDone = make(chan struct{})
	}
	ready := ep.ready
	stopped := ep.stopRequested
	ep.mu.Unlock()
	var readyOnce sync.Once
	signalReady := func() { readyOnce.Do(func() { close(ready) }) }
	defer signalReady()
	if stopped {
		return context.Canceled
	}
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
			"$KV.v1-objects.itx-groupsio-v2-artifact.>",
			"$KV.v1-objects.itx-groupsio-v2-message.>",
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
	ep.mu.Lock()
	stopped = ep.stopRequested
	ep.mu.Unlock()
	if stopped {
		return context.Canceled
	}

	consumeCtx, err := consumer.Consume(
		func(jMsg jetstream.Msg) {
			ep.mu.Lock()
			if ep.stopRequested {
				ep.mu.Unlock()
				return
			} // Leave unprocessed messages for redelivery.
			ep.active.Add(1)
			ep.mu.Unlock()
			defer ep.active.Done()
			meta, err := jMsg.Metadata()
			if err != nil {
				slog.ErrorContext(processingCtx, "failed to read stream message metadata, ACKing to avoid poison pill",
					"subject", jMsg.Subject(), "error", err)
				_ = jMsg.Ack()
				return
			}
			streamConsumer.Process(processingCtx, model.StreamMessage{
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
	ep.mu.Lock()
	ep.consumeCtx = consumeCtx
	ep.mu.Unlock()
	signalReady()

	slog.InfoContext(ctx, "data stream processor started successfully")
	closed := consumeCtx.Closed()
	select {
	case <-ctx.Done():
		slog.InfoContext(ctx, "data stream processor context cancelled")
	case <-closed:
	}
	shutdownTimeout := ep.config.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 30 * time.Second
	}
	stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancelStop()
	err = ep.Stop(stopCtx)
	if err != nil {
		// The graceful budget expired. Cancel handler I/O, but keep its store
		// alive until the callback actually exits. The outer process shutdown
		// remains the hard deadline for a non-cooperative callback.
		cancelProcessing()
		<-ep.callbacksDone
	}
	return err
}

// Stop halts the JetStream consumer. The NATS connection lifecycle is managed
// by the caller (NATSClient).
func (ep *natsEventProcessor) Stop(ctx context.Context) error {
	slog.InfoContext(ctx, "stopping data stream processor")

	ep.mu.Lock()
	ep.stopRequested = true
	started, ready, cancelProcessing := ep.started, ep.ready, ep.processingCancel
	ep.mu.Unlock()
	if !started {
		return nil
	}
	select {
	case <-ready:
	case <-ctx.Done():
		if cancelProcessing != nil {
			cancelProcessing()
		}
		return ctx.Err()
	}
	ep.mu.Lock()
	consumeCtx := ep.consumeCtx
	ep.mu.Unlock()
	if consumeCtx == nil {
		if cancelProcessing != nil {
			cancelProcessing()
		}
		return nil
	}
	// Capture the SDK completion signal before requesting stop. Also wait on
	// our admission-protected callback barrier, including concurrent stops.
	closed := consumeCtx.Closed()
	consumeCtx.Stop()
	ep.waitOnce.Do(func() { go func() { ep.active.Wait(); close(ep.callbacksDone) }() })
	for _, done := range []<-chan struct{}{closed, ep.callbacksDone} {
		select {
		case <-done:
		case <-ctx.Done():
			if cancelProcessing != nil {
				cancelProcessing()
			}
			return ctx.Err()
		}
	}
	if cancelProcessing != nil {
		cancelProcessing()
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
