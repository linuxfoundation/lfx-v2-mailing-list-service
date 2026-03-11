// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/nats-io/nats.go/jetstream"
)

// DataStreamProcessor routes JetStream KV messages to a DataEventHandler and
// manages ACK/NAK with exponential backoff. It is the interface used by the
// eventing layer to decouple message dispatch from NATS internals.
type DataStreamProcessor interface {
	Process(ctx context.Context, msg jetstream.Msg)
}

// dataStreamConsumer is the NATS JetStream implementation of DataStreamProcessor.
type dataStreamConsumer struct {
	handler port.DataEventHandler
}

// Process routes a single JetStream KV message to the appropriate DataEventHandler
// method (HandleChange or HandleRemoval), then ACKs or NAKs with exponential backoff
// based on the handler's return value.
//
// Unrecoverable parse errors (bad metadata, invalid JSON) are always ACKed to
// prevent a poison-pill loop.
func (c *dataStreamConsumer) Process(ctx context.Context, msg jetstream.Msg) {
	meta, err := msg.Metadata()
	if err != nil {
		slog.ErrorContext(ctx, "failed to read stream message metadata, ACKing to avoid poison pill",
			"subject", msg.Subject(), "error", err)
		c.ack(ctx, msg)
		return
	}

	key := keyFromSubject(msg.Subject())

	var nak bool
	if isRemoval(msg) {
		nak = c.handler.HandleRemoval(ctx, key)
	} else {
		var data map[string]any
		if jsonErr := json.Unmarshal(msg.Data(), &data); jsonErr != nil {
			slog.ErrorContext(ctx, "failed to unmarshal stream message payload, ACKing to avoid poison pill",
				"key", key, "error", jsonErr)
			c.ack(ctx, msg)
			return
		}
		nak = c.handler.HandleChange(ctx, key, data)
	}

	if nak {
		delay := nakDelay(meta.NumDelivered)
		if nakErr := msg.NakWithDelay(delay); nakErr != nil {
			slog.ErrorContext(ctx, "failed to NAK stream message",
				"key", key, "delay", delay, "error", nakErr)
		}
		return
	}

	c.ack(ctx, msg)
}

func (c *dataStreamConsumer) ack(ctx context.Context, msg jetstream.Msg) {
	if err := msg.Ack(); err != nil {
		slog.ErrorContext(ctx, "failed to ACK stream message", "subject", msg.Subject(), "error", err)
	}
}

// keyFromSubject strips the "$KV.<bucket>." prefix from a JetStream KV subject,
// returning the bare key. Subject format: $KV.<bucket>.<key>
func keyFromSubject(subject string) string {
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

func isRemoval(msg jetstream.Msg) bool {
	op := msg.Headers().Get("Kv-Operation")
	return op == "DEL" || op == "PURGE"
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

// NewDataStreamConsumer creates a DataStreamProcessor that dispatches messages to handler.
func NewDataStreamConsumer(handler port.DataEventHandler) DataStreamProcessor {
	return &dataStreamConsumer{handler: handler}
}
