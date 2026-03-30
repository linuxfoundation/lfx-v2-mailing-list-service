// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package service provides provider functions for initializing service dependencies.
package service

import (
	"context"
	"encoding/base64"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/auth"
	infrastructure "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/proxy"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

var (
	natsPublisherClient port.MessagePublisher

	natsDoOnce sync.Once
	natsClient *nats.NATSClient
)

// AuthService initializes the authentication service implementation
func AuthService(ctx context.Context) port.Authenticator {
	var authService port.Authenticator

	authSource := os.Getenv("AUTH_SOURCE")
	if authSource == "" {
		authSource = "jwt"
	}

	switch authSource {
	case "mock":
		slog.InfoContext(ctx, "initializing mock authentication service")
		authService = infrastructure.NewMockAuthService()
	case "jwt":
		slog.InfoContext(ctx, "initializing JWT authentication service")
		jwtConfig := auth.JWTAuthConfig{
			JWKSURL:            os.Getenv("JWKS_URL"),
			Audience:           os.Getenv("JWT_AUDIENCE"),
			MockLocalPrincipal: os.Getenv("JWT_AUTH_DISABLED_MOCK_LOCAL_PRINCIPAL"),
		}
		jwtAuth, err := auth.NewJWTAuth(jwtConfig)
		if err != nil {
			log.Fatalf("failed to initialize JWT authentication service: %v", err)
		}
		authService = jwtAuth
	default:
		log.Fatalf("unsupported authentication service implementation: %s", authSource)
	}

	return authService
}

// Translator initializes the ID translator implementation.
// TRANSLATOR_SOURCE controls which backend is used (default: "nats").
// In mock mode, TRANSLATOR_MAPPINGS_FILE points to the YAML mappings file.
func Translator(ctx context.Context) port.Translator {
	source := os.Getenv("TRANSLATOR_SOURCE")
	if source == "" {
		source = "nats"
	}

	switch source {
	case "mock":
		filePath := os.Getenv("TRANSLATOR_MAPPINGS_FILE")
		if filePath == "" {
			filePath = "translator_mappings.yaml"
		}
		slog.InfoContext(ctx, "initializing mock translator", "file", filePath)
		t, err := infrastructure.NewMockTranslator(filePath)
		if err != nil {
			log.Fatalf("failed to initialize mock translator: %v", err)
		}
		return t
	case "nats":
		slog.InfoContext(ctx, "initializing NATS translator")
		return nats.NewNATSTranslatorFromClient(GetNATSClient(ctx), 5*time.Second)
	default:
		log.Fatalf("unsupported translator implementation: %s", source)
	}

	return nil
}

// ITXProxyConfig reads ITX proxy configuration from environment variables.
func ITXProxyConfig() proxy.Config {
	return proxy.Config{
		BaseURL:     os.Getenv("ITX_BASE_URL"),
		ClientID:    os.Getenv("ITX_CLIENT_ID"),
		PrivateKey:  decodePrivateKey(os.Getenv("ITX_CLIENT_PRIVATE_KEY")),
		Auth0Domain: os.Getenv("ITX_AUTH0_DOMAIN"),
		Audience:    os.Getenv("ITX_AUDIENCE"),
		Timeout:     30 * time.Second,
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

func natsInit(ctx context.Context) {
	natsDoOnce.Do(func() {
		natsURL := os.Getenv("NATS_URL")
		if natsURL == "" {
			natsURL = "nats://localhost:4222"
		}

		natsTimeout := os.Getenv("NATS_TIMEOUT")
		if natsTimeout == "" {
			natsTimeout = "10s"
		}
		natsTimeoutDuration, err := time.ParseDuration(natsTimeout)
		if err != nil {
			log.Fatalf("invalid NATS timeout duration: %v", err)
		}

		natsMaxReconnect := os.Getenv("NATS_MAX_RECONNECT")
		if natsMaxReconnect == "" {
			natsMaxReconnect = "3"
		}
		natsMaxReconnectInt, err := strconv.Atoi(natsMaxReconnect)
		if err != nil {
			log.Fatalf("invalid NATS max reconnect value %s: %v", natsMaxReconnect, err)
		}

		natsReconnectWait := os.Getenv("NATS_RECONNECT_WAIT")
		if natsReconnectWait == "" {
			natsReconnectWait = "2s"
		}
		natsReconnectWaitDuration, err := time.ParseDuration(natsReconnectWait)
		if err != nil {
			log.Fatalf("invalid NATS reconnect wait duration %s : %v", natsReconnectWait, err)
		}

		config := nats.Config{
			URL:           natsURL,
			Timeout:       natsTimeoutDuration,
			MaxReconnect:  natsMaxReconnectInt,
			ReconnectWait: natsReconnectWaitDuration,
		}

		client, errNewClient := nats.NewClient(ctx, config)
		if errNewClient != nil {
			log.Fatalf("failed to create NATS client: %v", errNewClient)
		}
		natsClient = client
		natsPublisherClient = nats.NewMessagePublisher(client)
	})
}

// GetNATSClient returns the initialized NATS client for subscriptions
func GetNATSClient(ctx context.Context) *nats.NATSClient {
	natsInit(ctx)
	return natsClient
}

// MappingReaderWriter initializes the v1-mappings KV abstraction used by the
// data stream event handler for idempotency tracking.
func MappingReaderWriter(ctx context.Context) port.MappingReaderWriter {
	client := GetNATSClient(ctx)
	kv, err := client.KeyValue(ctx, constants.KVBucketNameV1Mappings)
	if err != nil {
		log.Fatalf("failed to access %s KV bucket: %v", constants.KVBucketNameV1Mappings, err)
	}
	return nats.NewMappingReaderWriter(kv)
}

func natsPublisher(ctx context.Context) port.MessagePublisher {
	natsInit(ctx)
	return natsPublisherClient
}

// MessagePublisher initializes the service publisher implementation
func MessagePublisher(ctx context.Context) port.MessagePublisher {
	var publisher port.MessagePublisher

	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "nats"
	}

	switch repoSource {
	case "mock":
		slog.InfoContext(ctx, "initializing mock service publisher")
		publisher = infrastructure.NewMockMessagePublisher()

	case "nats":
		slog.InfoContext(ctx, "initializing NATS service publisher")
		natsPublisher := natsPublisher(ctx)
		if natsPublisher == nil {
			log.Fatalf("failed to initialize NATS publisher")
		}
		publisher = natsPublisher

	default:
		log.Fatalf("unsupported service publisher implementation: %s", repoSource)
	}

	return publisher
}
