// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package nats provides NATS messaging client implementation and related utilities.
package nats

import (
	"context"
	"log/slog"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NATSClient wraps the NATS connection and provides access control operations
type NATSClient struct {
	conn    *nats.Conn
	config  Config
	kvStore map[string]jetstream.KeyValue
	timeout time.Duration
}

// NATSClientInterface defines the interface for NATS operations
// This allows for easy mocking and testing
type NATSClientInterface interface {
	Close() error
	IsReady(ctx context.Context) error
}

// Close gracefully closes the NATS connection
func (c *NATSClient) Close() error {
	if c.conn != nil {
		c.conn.Close()
	}
	return nil
}

// IsReady checks if the NATS client is ready
func (c *NATSClient) IsReady(ctx context.Context) error {
	if c.conn == nil {
		slog.ErrorContext(ctx, "NATS client is not initialized or not connected")
		return errors.NewServiceUnavailable("NATS client is not initialized or not connected")
	}
	if !c.conn.IsConnected() || c.conn.IsDraining() {
		slog.ErrorContext(ctx, "NATS client is not ready",
			"connected", c.conn.IsConnected(),
			"draining", c.conn.IsDraining(),
		)
		return errors.NewServiceUnavailable("NATS client is not ready, connection is not established or is draining")
	}
	slog.DebugContext(ctx, "NATS client is ready", "url", c.conn.ConnectedUrl())
	return nil
}

// QueueSubscribe creates a queue subscription for load-balanced message processing
// Returns subscription handle and error
func (c *NATSClient) QueueSubscribe(subject, queue string, handler nats.MsgHandler) (*nats.Subscription, error) {
	if c.conn == nil {
		return nil, errors.NewServiceUnavailable("NATS connection not initialized")
	}
	if !c.conn.IsConnected() {
		return nil, errors.NewServiceUnavailable("NATS connection not ready")
	}
	return c.conn.QueueSubscribe(subject, queue, handler)
}

// KeyValueStore creates a JetStream client and gets the key-value store for projects.
func (c *NATSClient) KeyValueStore(ctx context.Context, bucketName string) error {
	js, err := jetstream.New(c.conn)
	if err != nil {
		slog.ErrorContext(ctx, "error creating NATS JetStream client",
			"error", err,
			"nats_url", c.conn.ConnectedUrl(),
		)
		return err
	}
	kvStore, err := js.KeyValue(ctx, bucketName)
	if err != nil {
		slog.ErrorContext(ctx, "error getting NATS JetStream key-value store",
			"error", err,
			"nats_url", c.conn.ConnectedUrl(),
			"bucket", bucketName,
		)
		return err
	}

	if c.kvStore == nil {
		c.kvStore = make(map[string]jetstream.KeyValue)
	}
	c.kvStore[bucketName] = kvStore
	return nil
}

// NewClient creates a new NATS client with the given configuration
func NewClient(ctx context.Context, config Config) (*NATSClient, error) {
	slog.InfoContext(ctx, "creating NATS client",
		"url", config.URL,
		"timeout", config.Timeout,
	)

	// Validate configuration
	if config.URL == "" {
		return nil, errors.NewUnexpected("NATS URL is required")
	}

	// Configure NATS connection options
	opts := []nats.Option{
		nats.Name(constants.ServiceName),
		nats.Timeout(config.Timeout),
		nats.MaxReconnects(config.MaxReconnect),
		nats.ReconnectWait(config.ReconnectWait),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			slog.WarnContext(ctx, "NATS disconnected",
				"error", err,
				"url", nc.ConnectedUrl(),
				"status", nc.Status(),
			)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.InfoContext(ctx, "NATS reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ErrorHandler(func(_ *nats.Conn, s *nats.Subscription, err error) {
			if s != nil {
				slog.With("error", err, "subject", s.Subject, "queue", s.Queue).Error("async NATS error")
			} else {
				slog.With("error", err).Error("async NATS error outside subscription")
			}
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			slog.InfoContext(ctx, "NATS connection closed",
				"url", nc.ConnectedUrl(),
				"status", nc.Status(),
			)
		}),
	}

	// Establish connection
	conn, err := nats.Connect(config.URL, opts...)
	if err != nil {
		return nil, errors.NewServiceUnavailable("failed to connect to NATS", err)
	}

	client := &NATSClient{
		conn:    conn,
		config:  config,
		timeout: config.Timeout,
	}

	// Initialize key-value stores for services, mailing lists, and members
	for _, bucketName := range []string{constants.KVBucketNameGroupsIOServices, constants.KVBucketNameGroupsIOMailingLists, constants.KVBucketNameGroupsIOMembers} {
		if err := client.KeyValueStore(ctx, bucketName); err != nil {
			slog.ErrorContext(ctx, "failed to initialize NATS key-value store",
				"error", err,
				"bucket", bucketName,
			)
			return nil, errors.NewServiceUnavailable("failed to initialize NATS key-value store", err)
		}
	}

	slog.InfoContext(ctx, "NATS client created successfully",
		"connected_url", conn.ConnectedUrl(),
		"status", conn.Status(),
	)

	return client, nil
}
