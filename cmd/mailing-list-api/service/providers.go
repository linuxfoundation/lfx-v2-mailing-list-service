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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/groupsio"
	infrastructure "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
)

var (
	natsStorageClient      port.GrpsIOReaderWriter
	natsMessagingClient    port.EntityAttributeReader
	natsPublisherClient    port.MessagePublisher
	groupsIOClient         groupsio.ClientInterface
	grpsioWebhookValidator port.GrpsIOWebhookValidator

	natsDoOnce                 sync.Once
	groupsIOClientOnce         sync.Once
	grpsioWebhookValidatorOnce sync.Once
)

// ValidateProviderConfiguration checks for configuration mismatches that could cause issues
// Call this during application startup to fail fast on misconfiguration
func ValidateProviderConfiguration(ctx context.Context) {
	groupsioSource := os.Getenv("GROUPSIO_SOURCE")
	if groupsioSource == "" {
		groupsioSource = "groupsio"
	}

	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "nats"
	}

	authSource := os.Getenv("AUTH_SOURCE")
	if authSource == "" {
		authSource = "jwt"
	}

	slog.InfoContext(ctx, "provider configuration validation",
		"auth_source", authSource,
		"repository_source", repoSource,
		"groupsio_source", groupsioSource,
	)

	// Warn about potentially dangerous mismatches
	if groupsioSource == "mock" && repoSource != "mock" {
		slog.WarnContext(ctx,
			"CONFIGURATION MISMATCH: mock GroupsIO with real repository - mock validator will accept all webhooks into production storage!",
			"groupsio_source", groupsioSource,
			"repository_source", repoSource,
		)
	}

	if authSource == "mock" && repoSource != "mock" {
		slog.WarnContext(ctx,
			"CONFIGURATION MISMATCH: mock auth with real repository - authentication is bypassed but data is stored in production!",
			"auth_source", authSource,
			"repository_source", repoSource,
		)
	}

	// Validate webhook secret in production mode
	if groupsioSource != "mock" {
		webhookSecret := os.Getenv("GROUPSIO_WEBHOOK_SECRET")
		if webhookSecret == "" {
			slog.WarnContext(ctx, "GROUPSIO_WEBHOOK_SECRET not set - webhook validation will fail in production!")
		}
	}

	slog.InfoContext(ctx, "provider configuration validation completed")
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
		natsStorageClient = nats.NewStorage(client)
		natsMessagingClient = nats.NewEntityAttributeReader(client)
		natsPublisherClient = nats.NewMessagePublisher(client)
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

// GrpsIOReader initializes the service reader implementation
func GrpsIOReader(ctx context.Context) port.GrpsIOReader {
	var grpsIOReader port.GrpsIOReader

	// Repository implementation configuration
	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "nats"
	}

	switch repoSource {
	case "mock":
		slog.InfoContext(ctx, "initializing mock grpsio service reader")
		grpsIOReader = infrastructure.NewMockGrpsIOReader(infrastructure.NewMockRepository())

	case "nats":
		slog.InfoContext(ctx, "initializing NATS service")
		natsClient := natsStorage(ctx)
		if natsClient == nil {
			log.Fatalf("failed to initialize NATS client")
		}
		grpsIOReader = natsClient

	default:
		log.Fatalf("unsupported service reader implementation: %s", repoSource)
	}

	return grpsIOReader
}

// GrpsIOReaderWriter initializes the service reader writer implementation
func GrpsIOReaderWriter(ctx context.Context) port.GrpsIOReaderWriter {
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

// GrpsIOWriter initializes the service writer implementation
func GrpsIOWriter(ctx context.Context) port.GrpsIOWriter {
	var grpsIOWriter port.GrpsIOWriter

	// Repository implementation configuration
	repoSource := os.Getenv("REPOSITORY_SOURCE")
	if repoSource == "" {
		repoSource = "nats"
	}

	switch repoSource {
	case "mock":
		slog.InfoContext(ctx, "initializing mock grpsio service writer")
		grpsIOWriter = infrastructure.NewMockGrpsIOWriter(infrastructure.NewMockRepository())

	case "nats":
		slog.InfoContext(ctx, "initializing NATS service writer")
		natsClient := natsStorage(ctx)
		if natsClient == nil {
			log.Fatalf("failed to initialize NATS client")
		}
		grpsIOWriter = natsClient

	default:
		log.Fatalf("unsupported service writer implementation: %s", repoSource)
	}

	return grpsIOWriter
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

// GroupsIOClient initializes the GroupsIO client with singleton pattern
func GroupsIOClient(ctx context.Context) groupsio.ClientInterface {
	groupsIOClientOnce.Do(func() {
		var client groupsio.ClientInterface

		// Repository implementation configuration
		source := os.Getenv("GROUPSIO_SOURCE")
		if source == "" {
			source = "groupsio" // Default to production GroupsIO client
		}

		switch source {
		case "mock":
			slog.InfoContext(ctx, "initializing mock groupsio client")
			client = infrastructure.NewMockGroupsIOClient()

		case "groupsio":
			slog.InfoContext(ctx, "initializing groupsio client")
			config := groupsio.NewConfigFromEnv()

			var err error
			client, err = groupsio.NewClient(config)
			if err != nil {
				log.Fatalf("failed to initialize GroupsIO client - missing required configuration: %v", err)
			}
			slog.InfoContext(ctx, "groupsio client initialized successfully")

		default:
			log.Fatalf("unsupported groupsio client implementation: %s", source)
		}

		groupsIOClient = client
	})

	return groupsIOClient
}

// GrpsIOReaderOrchestrator initializes the service reader orchestrator
func GrpsIOReaderOrchestrator(ctx context.Context) service.GrpsIOReader {
	grpsIOReader := GrpsIOReader(ctx)

	return service.NewGrpsIOReaderOrchestrator(
		service.WithGrpsIOReader(grpsIOReader),
	)
}

// GrpsIOWriterOrchestrator initializes the service writer orchestrator
func GrpsIOWriterOrchestrator(ctx context.Context) service.GrpsIOWriter {
	grpsIOWriter := GrpsIOWriter(ctx)
	grpsIOReader := GrpsIOReader(ctx)
	entityReader := EntityAttributeRetriever(ctx)
	publisher := MessagePublisher(ctx)
	groupsClient := GroupsIOClient(ctx) // May be nil for mock/disabled

	return service.NewGrpsIOWriterOrchestrator(
		service.WithGrpsIOWriter(grpsIOWriter),
		service.WithGrpsIOWriterReader(grpsIOReader),
		service.WithEntityAttributeReader(entityReader),
		service.WithPublisher(publisher),
		service.WithGroupsIOClient(groupsClient),
	)
}

// GrpsIOWebhookValidator initializes the GroupsIO webhook validator with mock support and singleton pattern
func GrpsIOWebhookValidator(ctx context.Context) port.GrpsIOWebhookValidator {
	grpsioWebhookValidatorOnce.Do(func() {
		var validator port.GrpsIOWebhookValidator

		// Repository implementation configuration
		source := os.Getenv("GROUPSIO_SOURCE")
		if source == "" {
			source = "groupsio" // Default to production GroupsIO webhook validation
		}

		switch source {
		case "mock":
			slog.InfoContext(ctx, "initializing mock groupsio webhook validator")
			validator = infrastructure.NewMockGrpsIOWebhookValidator()

		case "groupsio":
			slog.InfoContext(ctx, "initializing groupsio webhook validator")
			secret := os.Getenv("GROUPSIO_WEBHOOK_SECRET")
			if secret == "" {
				log.Fatalf("GROUPSIO_WEBHOOK_SECRET is required for groupsio webhook validation")
			}
			validator = groupsio.NewGrpsIOWebhookValidator(secret)

		default:
			log.Fatalf("unsupported groupsio webhook validator implementation: %s", source)
		}

		grpsioWebhookValidator = validator
	})

	return grpsioWebhookValidator
}

// GrpsIOWebhookProcessor creates GroupsIO webhook processor with explicit dependency injection
func GrpsIOWebhookProcessor(ctx context.Context) port.GrpsIOWebhookProcessor {
	slog.InfoContext(ctx, "initializing groupsio webhook processor with dependency injection")

	return service.NewGrpsIOWebhookProcessor(
		service.WithServiceReader(GrpsIOReader(ctx)),
		service.WithMailingListReader(GrpsIOReader(ctx)),
		service.WithMailingListWriter(GrpsIOWriter(ctx)),
		service.WithMemberReader(GrpsIOReader(ctx)),
		service.WithMemberWriter(GrpsIOWriter(ctx)),
	)
}
