// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service provides factory functions for initializing service dependencies.
package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/auth"
	infrastructure "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/proxy"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// NewAuthService initializes and returns the authentication service implementation.
// source controls the backend: "jwt" (default) or "mock".
func NewAuthService(ctx context.Context, source, jwksURL, jwtAudience, mockPrincipal string) (port.Authenticator, error) {
	switch source {
	case "mock":
		slog.InfoContext(ctx, "initializing mock authentication service")
		return infrastructure.NewMockAuthService(), nil
	case "jwt":
		slog.InfoContext(ctx, "initializing JWT authentication service")
		jwtConfig := auth.JWTAuthConfig{
			JWKSURL:            jwksURL,
			Audience:           jwtAudience,
			MockLocalPrincipal: mockPrincipal,
		}
		return auth.NewJWTAuth(jwtConfig)
	default:
		return nil, fmt.Errorf("unsupported authentication service implementation: %s", source)
	}
}

// NewTranslator initializes and returns the ID translator implementation.
// source controls the backend: "nats" (default) or "mock".
func NewTranslator(ctx context.Context, source, mappingsFile string, natsClient *nats.NATSClient) (port.Translator, error) {
	switch source {
	case "mock":
		slog.InfoContext(ctx, "initializing mock translator", "file", mappingsFile)
		t, err := infrastructure.NewMockTranslator(mappingsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize mock translator: %w", err)
		}
		return t, nil
	case "nats":
		slog.InfoContext(ctx, "initializing NATS translator")
		return nats.NewNATSTranslatorFromClient(natsClient, 5*time.Second), nil
	default:
		return nil, fmt.Errorf("unsupported translator implementation: %s", source)
	}
}

// NewITXProxyConfig builds the ITX proxy configuration from the provided values.
// privateKey is base64-decoded transparently if necessary.
func NewITXProxyConfig(baseURL, clientID, privateKey, auth0Domain, audience string) proxy.Config {
	return proxy.Config{
		BaseURL:     baseURL,
		ClientID:    clientID,
		PrivateKey:  decodePrivateKey(privateKey),
		Auth0Domain: auth0Domain,
		Audience:    audience,
		Timeout:     30 * time.Second,
	}
}

// NewMappingReaderWriter initializes the v1-mappings KV abstraction used by the
// data stream event handler for idempotency tracking.
func NewMappingReaderWriter(ctx context.Context, natsClient *nats.NATSClient) (port.MappingReaderWriter, error) {
	kv, err := natsClient.KeyValue(ctx, constants.KVBucketNameV1Mappings)
	if err != nil {
		return nil, fmt.Errorf("failed to access %s KV bucket: %w", constants.KVBucketNameV1Mappings, err)
	}
	return nats.NewMappingReaderWriter(kv), nil
}

// NewMessagePublisher initializes the message publisher implementation.
// source controls the backend: "nats" (default) or "mock".
func NewMessagePublisher(ctx context.Context, source string, natsClient *nats.NATSClient) (port.MessagePublisher, error) {
	switch source {
	case "mock":
		slog.InfoContext(ctx, "initializing mock message publisher")
		return infrastructure.NewMockMessagePublisher(), nil
	case "nats":
		slog.InfoContext(ctx, "initializing NATS message publisher")
		return nats.NewMessagePublisher(natsClient), nil
	default:
		return nil, fmt.Errorf("unsupported message publisher implementation: %s", source)
	}
}

// NewCommitteeProjectLookup initializes the committee project lookup implementation.
// source controls the backend: "nats" (default) or "mock".
func NewCommitteeProjectLookup(ctx context.Context, source string, natsClient *nats.NATSClient) (port.CommitteeProjectLookup, error) {
	switch source {
	case "mock":
		slog.InfoContext(ctx, "initializing mock committee project lookup")
		return infrastructure.NewFakeCommitteeProjectLookup(), nil
	case "nats":
		slog.InfoContext(ctx, "initializing NATS committee project lookup")
		return nats.NewNATSCommitteeProjectLookup(natsClient), nil
	default:
		return nil, fmt.Errorf("unsupported committee project lookup implementation: %s", source)
	}
}

// decodePrivateKey returns the raw PEM key, base64-decoding it first if needed.
// Secrets stored in AWS Secrets Manager (and injected via External Secrets Operator)
// are sometimes base64-encoded before storage; this handles both cases transparently.
func decodePrivateKey(key string) string {
	if strings.HasPrefix(key, "-----") {
		return key
	}
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return key
	}
	return string(decoded)
}
