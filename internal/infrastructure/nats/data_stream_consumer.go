// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
)

// dataStreamConsumer is the NATS JetStream implementation of port.DataStreamProcessor.
type dataStreamConsumer struct {
	handler port.DataEventHandler
}

// Process routes a single stream message to the appropriate DataEventHandler method
// (HandleChange or HandleRemoval), then ACKs or NAKs with exponential backoff based
// on the handler's return value.
//
// Unrecoverable parse errors (invalid JSON) are always ACKed to prevent a poison-pill loop.
func (c *dataStreamConsumer) Process(ctx context.Context, msg model.StreamMessage) {
	if msg.IsRemoval {
		if nak := c.handler.HandleRemoval(ctx, msg.Key); nak {
			c.nak(ctx, msg)
			return
		}
		c.ack(ctx, msg)
		return
	}

	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal stream message payload, ACKing to avoid poison pill",
			"key", msg.Key, "error", err)
		c.ack(ctx, msg)
		return
	}

	if nak := c.handler.HandleChange(ctx, msg.Key, data); nak {
		c.nak(ctx, msg)
		return
	}
	c.ack(ctx, msg)
}

func (c *dataStreamConsumer) ack(ctx context.Context, msg model.StreamMessage) {
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

// NewDataStreamConsumer creates a port.DataStreamProcessor that dispatches messages to handler.
func NewDataStreamConsumer(handler port.DataEventHandler) port.DataStreamProcessor {
	return &dataStreamConsumer{handler: handler}
}
