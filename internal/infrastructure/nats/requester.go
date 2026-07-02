// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"

	"github.com/nats-io/nats.go"
)

// Requester is a narrow interface for NATS request/reply. It is satisfied by
// *NATSClient and can be replaced with a mock in unit tests.
type Requester interface {
	RequestWithContext(ctx context.Context, subject string, data []byte) (*nats.Msg, error)
}

// Ensure NATSClient satisfies Requester.
var _ Requester = (*NATSClient)(nil)
