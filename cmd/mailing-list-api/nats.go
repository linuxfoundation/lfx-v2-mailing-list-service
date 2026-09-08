// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"

	infraNATS "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
)

// setupNATS creates and returns a connected NATS client using the provided environment config.
// The caller is responsible for calling Close() on the returned client when done.
func setupNATS(ctx context.Context, env environment) (*infraNATS.NATSClient, error) {
	slog.InfoContext(ctx, "connecting to NATS", "nats_url", env.NatsURL)
	cfg := infraNATS.Config{
		URL:           env.NatsURL,
		Timeout:       env.NatsTimeout,
		MaxReconnect:  env.NatsMaxReconnect,
		ReconnectWait: env.NatsReconnectWait,
	}
	client, err := infraNATS.NewClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "NATS connection established", "nats_url", env.NatsURL)
	return client, nil
}
