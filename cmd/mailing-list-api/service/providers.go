// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"log"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/auth"
	infrastructure "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
)

var (
	natsStorageClient   port.GrpsIOReaderWriter
	natsMessagingClient port.EntityAttributeReader
	natsPublisherClient port.MessagePublisher

	natsDoOnce sync.Once
)

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
		natsStorageClient = nats.NewStorage(client)
		natsMessagingClient = nats.NewEntityAttributeReader(client)
		natsPublisherClient = nats.NewGrpsIOServicePublisher(client)
	})
}

func natsStorage(ctx context.Context) port.GrpsIOReaderWriter {
	natsInit(ctx)
	return natsStorageClient
}

func natsMessaging(ctx context.Context) port.EntityAttributeReader {
	natsInit(ctx)
	return natsMessagingClient
}

func natsPublisher(ctx context.Context) port.MessagePublisher {
	natsInit(ctx)
	return natsPublisherClient
}

// AuthService initializes the authentication service implementation
func AuthService(ctx context.Context) port.Authenticator {
	var authService port.Authenticator

	// Repository implementation configuration
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
			JWKSURL:  os.Getenv("JWKS_URL"),
			Audience: os.Getenv("JWT_AUDIENCE"),
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

// EntityAttributeRetriever initializes the entity attribute retriever implementation based on the repository source
func EntityAttributeRetriever(ctx context.Context) port.EntityAttributeReader {
	var entityReader port.EntityAttributeReader

	// Repository implementation configuration
	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "nats"
	}

	switch repoSource {
	case "mock":
		slog.InfoContext(ctx, "initializing mock entity attribute retriever")
		entityReader = infrastructure.NewMockEntityAttributeReader(infrastructure.NewMockRepository())

	case "nats":
		slog.InfoContext(ctx, "initializing NATS entity attribute retriever")
		natsClient := natsMessaging(ctx)
		if natsClient == nil {
			log.Fatalf("failed to initialize NATS client")
		}
		entityReader = natsClient

	default:
		log.Fatalf("unsupported entity attribute reader implementation: %s", repoSource)
	}

	return entityReader
}

// GrpsIOServiceReader initializes the service reader implementation
func GrpsIOServiceReader(ctx context.Context) port.GrpsIOServiceReader {
	var grpsIOServiceReader port.GrpsIOServiceReader

	// Repository implementation configuration
	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "nats"
	}

	switch repoSource {
	case "mock":
		slog.InfoContext(ctx, "initializing mock grpsio service reader")
		grpsIOServiceReader = infrastructure.NewMockGrpsIOServiceReader(infrastructure.NewMockRepository())

	case "nats":
		slog.InfoContext(ctx, "initializing NATS service")
		natsClient := natsStorage(ctx)
		if natsClient == nil {
			log.Fatalf("failed to initialize NATS client")
		}
		grpsIOServiceReader = natsClient

	default:
		log.Fatalf("unsupported service reader implementation: %s", repoSource)
	}

	return grpsIOServiceReader
}

func GrpsIOServiceReaderWriter(ctx context.Context) port.GrpsIOReaderWriter {
	var storage port.GrpsIOReaderWriter
	// Repository implementation configuration
	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "nats"
	}

	switch repoSource {
	case "mock":
		slog.InfoContext(ctx, "initializing mock grpsio storage reader writer")
		storage = infrastructure.NewMockGrpsIOReaderWriter(infrastructure.NewMockRepository())

	case "nats":
		slog.InfoContext(ctx, "initializing NATS service")
		natsClient := natsStorage(ctx)
		if natsClient == nil {
			log.Fatalf("failed to initialize NATS client")
		}
		storage = natsClient

	default:
		log.Fatalf("unsupported service reader implementation: %s", repoSource)
	}

	return storage
}

// GrpsIOServiceWriter initializes the service writer implementation
func GrpsIOServiceWriter(ctx context.Context) port.GrpsIOServiceWriter {
	var grpsIOServiceWriter port.GrpsIOServiceWriter

	// Repository implementation configuration
	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "nats"
	}

	switch repoSource {
	case "mock":
		slog.InfoContext(ctx, "initializing mock grpsio service writer")
		grpsIOServiceWriter = infrastructure.NewMockGrpsIOServiceWriter(infrastructure.NewMockRepository())

	case "nats":
		slog.InfoContext(ctx, "initializing NATS service writer")
		natsClient := natsStorage(ctx)
		if natsClient == nil {
			log.Fatalf("failed to initialize NATS client")
		}
		grpsIOServiceWriter = natsClient

	default:
		log.Fatalf("unsupported service writer implementation: %s", repoSource)
	}

	return grpsIOServiceWriter
}

// MessagePublisher initializes the service publisher implementation
func MessagePublisher(ctx context.Context) port.MessagePublisher {
	var publisher port.MessagePublisher

	// Repository implementation configuration
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

// GrpsIOServiceReaderOrchestrator initializes the service reader orchestrator
func GrpsIOServiceReaderOrchestrator(ctx context.Context) service.GrpsIOServiceReader {
	serviceReader := GrpsIOServiceReader(ctx)

	slog.InfoContext(ctx, "initializing service reader orchestrator")

	return service.NewGrpsIOServiceReaderOrchestrator(
		service.WithServiceReader(serviceReader),
	)
}

// GrpsIOServiceWriterOrchestrator initializes the service writer orchestrator
func GrpsIOServiceWriterOrchestrator(ctx context.Context) service.GrpsIOServiceWriter {
	serviceWriter := GrpsIOServiceWriter(ctx)
	serviceReader := GrpsIOServiceReader(ctx)
	entityReader := EntityAttributeRetriever(ctx)
	publisher := MessagePublisher(ctx)

	slog.InfoContext(ctx, "initializing service writer orchestrator with concurrent message publishing")

	return service.NewGrpsIOServiceWriterOrchestrator(
		service.WithServiceWriter(serviceWriter),
		service.WithGrpsIOServiceReader(serviceReader),
		service.WithEntityAttributeReader(entityReader),
		service.WithPublisher(publisher),
	)
}
