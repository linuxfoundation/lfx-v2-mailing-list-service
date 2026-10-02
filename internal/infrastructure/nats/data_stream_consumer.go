// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	msgpack "github.com/vmihailenco/msgpack/v5"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// dataStreamConsumer is the NATS JetStream implementation of port.DataStreamProcessor.
type dataStreamConsumer struct {
	handler           port.DataEventHandler
	heartbeatInterval time.Duration
}

// Process routes a single stream message to the appropriate DataEventHandler method
// (HandleChange or HandleRemoval), then ACKs or NAKs with exponential backoff based
// on the handler's return value.
//
// Unrecoverable parse errors (invalid JSON) are always ACKed to prevent a poison-pill loop.
func (c *dataStreamConsumer) Process(ctx context.Context, msg model.StreamMessage) {
	stopProgress := func() {}
	if msg.InProgress != nil {
		interval := c.heartbeatInterval
		if interval <= 0 {
			interval = time.Millisecond
		}
		// Service-domain fan-out can exceed AckWait when many lists belong to one
		// service. Keep this delivery owned until processing ends (ACK or NAK).
		stop := make(chan struct{})
		done := make(chan struct{})
		var once sync.Once
		stopProgress = func() {
			once.Do(func() { close(stop) })
			<-done
		}
		go func() {
			defer close(done)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					if err := msg.InProgress(); err != nil {
						slog.WarnContext(ctx, "failed to extend data stream ACK deadline", "key", msg.Key, "error", err)
					}
				}
			}
		}()
	}
	defer func() { stopProgress() }()
	if msg.IsRemoval {
		if nak := c.handler.HandleRemoval(ctx, msg.Key); nak {
			stopProgress()
			c.nak(ctx, msg)
			return
		}
		stopProgress()
		c.ack(ctx, msg)
		return
	}

	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		if msgErr := msgpack.Unmarshal(msg.Data, &data); msgErr != nil {
			slog.ErrorContext(ctx, "failed to unmarshal stream message payload as JSON or msgpack, ACKing to avoid poison pill",
				"key", msg.Key, "json_error", err, "msgpack_error", msgErr)
			stopProgress()
			c.ack(ctx, msg)
			return
		}
		slog.DebugContext(ctx, "decoded stream message payload as msgpack", "key", msg.Key)
	}

	if nak := c.handler.HandleChange(ctx, msg.Key, data); nak {
		stopProgress()
		c.nak(ctx, msg)
		return
	}
	stopProgress()
	c.ack(ctx, msg)
}

func (c *dataStreamConsumer) ack(ctx context.Context, msg model.StreamMessage) {
	if err := ctx.Err(); err != nil {
		slog.WarnContext(ctx, "processing cancelled before ACK, NAKing for retry", "key", msg.Key, "error", err)
		c.nak(ctx, msg)
		return
	}
	if err := msg.Ack(); err != nil {
		slog.ErrorContext(ctx, "failed to ACK stream message", "key", msg.Key, "error", err)
	}
}

func (c *dataStreamConsumer) nak(ctx context.Context, msg model.StreamMessage) {
	delay := nakDelay(msg.DeliveryCount)
	if err := msg.Nak(delay); err != nil {
		slog.ErrorContext(ctx, "failed to NAK stream message",
			"key", msg.Key, "delay", delay, "error", err)
	}
}

func nakDelay(numDelivered uint64) time.Duration {
	switch numDelivered {
	case 1:
		return 2 * time.Second
	case 2:
		return 10 * time.Second
	default:
		return 20 * time.Second
	}
}

// NewDataStreamConsumer dispatches events and heartbeats before the configured AckWait.
func NewDataStreamConsumer(handler port.DataEventHandler, ackWait ...time.Duration) port.DataStreamProcessor {
	wait := 30 * time.Second
	if len(ackWait) > 0 && ackWait[0] > 0 {
		wait = ackWait[0]
	}
	interval := wait / 3
	if interval < time.Millisecond {
		interval = time.Millisecond
	}
	return &dataStreamConsumer{handler: handler, heartbeatInterval: interval}
}
