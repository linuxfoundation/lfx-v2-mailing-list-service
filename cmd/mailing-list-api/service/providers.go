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
)

var (
	natsStorageClient   port.GrpsIOServiceReaderWriter
	natsMessagingClient port.ProjectReader

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
		natsMessagingClient = nats.NewMessageRequest(client)
	})
}

func natsStorage(ctx context.Context) port.GrpsIOServiceReaderWriter {
	natsInit(ctx)
	return natsStorageClient
}

func natsMessaging(ctx context.Context) port.ProjectReader {
	natsInit(ctx)
	return natsMessagingClient
}

// TODO: MailingListStorage - Add when MailingListReaderWriter port is implemented

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

// ProjectRetriever initializes the project retriever implementation based on the repository source
func ProjectRetriever(ctx context.Context) port.ProjectReader {
	var projectReader port.ProjectReader

	// Repository implementation configuration
	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "nats"
	}

	switch repoSource {
	case "nats":
		slog.InfoContext(ctx, "initializing NATS project retriever")
		natsClient := natsMessaging(ctx)
		if natsClient == nil {
			log.Fatalf("failed to initialize NATS client")
		}
		projectReader = natsClient

	default:
		log.Fatalf("unsupported project reader implementation: %s", repoSource)
	}

	return projectReader
}

// GrpsIOServiceReader initializes the service reader implementation
func GrpsIOServiceReader(ctx context.Context) port.GrpsIOServiceReader {
	var grpsIOServiceReader port.GrpsIOServiceReader

	// Repository implementation configuration
	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "mock"
	}

	switch repoSource {
	case "mock":
		slog.InfoContext(ctx, "initializing mock service")
		grpsIOServiceReader = infrastructure.NewMockService()

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

func GrpsIOServiceReaderWriter(ctx context.Context) port.GrpsIOServiceReaderWriter {
	var storage port.GrpsIOServiceReaderWriter
	// Repository implementation configuration
	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "mock"
	}

	switch repoSource {
	case "mock":
		slog.InfoContext(ctx, "initializing mock service")
		storage = infrastructure.NewMockService()

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
