// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import "time"

// StreamMessage represents a single message from a JetStream KV change stream,
// decoupled from any NATS-specific types.
type StreamMessage struct {
	// Key is the bare KV key (subject prefix stripped).
	Key string
	// Data is the raw JSON payload. Nil for removal operations.
	Data []byte
	// IsRemoval is true for DEL/PURGE operations.
	IsRemoval bool
	// DeliveryCount is the number of times this message has been delivered.
	DeliveryCount uint64
	// Ack acknowledges successful processing.
	Ack func() error
	// Nak requeues the message with the given backoff delay.
	Nak func(delay time.Duration) error
}
