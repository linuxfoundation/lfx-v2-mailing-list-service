// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package main is the ITX mailing list proxy service that provides a lightweight proxy
// layer to the ITX GroupsIO API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/cmd/mailing-list-api/eventing"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/cmd/mailing-list-api/service"
	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	infraNATS "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/proxy"
	orchestrator "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	logging "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/log"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/utils"

	"goa.design/clue/debug"
)

// Build-time variables set via ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

const (
	defaultPort             = "8080"
	gracefulShutdownSeconds = 25
)

func init() {
	logging.InitStructureLogConfig()
}

func main() {
	os.Exit(run())
}

// run is the entry point for the service logic. It returns a non-zero exit code on
// startup failure so that orchestrators and CI can trigger crash-loop backoff or
// mark the pipeline step as failed.
// Deferred functions (OTel shutdown, NATS close) are called when run() returns,
// before os.Exit fires in main.
func run() int {
	env := parseEnv()
	flags := parseFlags(env.Port)

	ctx := context.Background()

	// Set up OpenTelemetry SDK.
	// Command-line/environment OTEL_SERVICE_VERSION takes precedence over
	// the build-time Version variable.
	otelConfig := utils.OTelConfigFromEnv()
	if otelConfig.ServiceVersion == "" {
		otelConfig.ServiceVersion = Version
	}
	otelShutdown, err := utils.SetupOTelSDKWithConfig(ctx, otelConfig)
	if err != nil {
		slog.ErrorContext(ctx, "error setting up OpenTelemetry SDK", "error", err)
		return 1
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownSeconds*time.Second)
		defer cancel()
		if shutdownErr := otelShutdown(shutdownCtx); shutdownErr != nil {
			slog.ErrorContext(ctx, "error shutting down OpenTelemetry SDK", "error", shutdownErr)
		}
	}()

	slog.InfoContext(ctx, "starting ITX mailing list proxy service",
		"port", env.Port,
		"version", Version,
		"build-time", BuildTime,
		"git-commit", GitCommit,
	)

	// Connect to NATS — all infrastructure that depends on it is built below.
	natsClient, err := setupNATS(ctx, env)
	if err != nil {
		slog.ErrorContext(ctx, "error connecting to NATS", "error", err)
		return 1
	}
	defer func() {
		if closeErr := natsClient.Close(); closeErr != nil {
			slog.ErrorContext(ctx, "error closing NATS client", "error", closeErr)
		}
	}()

	// Initialize authentication service.
	authSvc, err := service.NewAuthService(ctx, env.AuthSource, env.JWKSURL, env.JWTAudience, env.JWTMockPrincipal)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing authentication service", "error", err)
		return 1
	}

	// Initialize ID translator.
	translator, err := service.NewTranslator(ctx, env.TranslatorSource, env.TranslatorMappings, natsClient)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing translator", "error", err)
		return 1
	}

	// Initialize ITX proxy client.
	slog.InfoContext(ctx, "initializing GroupsIO service proxy")
	proxyClient, err := proxy.NewProxy(ctx, service.NewITXProxyConfig(env.ITXBaseURL, env.ITXClientID, env.ITXClientPrivateKey, env.ITXAuth0Domain, env.ITXAudience))
	if err != nil {
		slog.ErrorContext(ctx, "failed to initialize ITX proxy client", "error", err)
		return 1
	}
	slog.InfoContext(ctx, "ITX proxy client initialized")

	// Initialize message publisher.
	publisher, err := service.NewMessagePublisher(ctx, env.RepositorySource, natsClient)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing message publisher", "error", err)
		return 1
	}

	// Initialize committee project lookup.
	committeeLookup, err := service.NewCommitteeProjectLookup(ctx, env.RepositorySource, natsClient)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing committee project lookup", "error", err)
		return 1
	}

	// Initialize v1-mappings KV store for the data stream idempotency tracker.
	mappings, err := service.NewMappingReaderWriter(ctx, natsClient)
	if err != nil {
		slog.ErrorContext(ctx, "error initializing mapping reader/writer", "error", err)
		return 1
	}

	// Build orchestrators.
	serviceReaderOrchestrator := orchestrator.NewGroupsIOServiceReaderOrchestrator(
		orchestrator.WithServiceReader(proxyClient),
		orchestrator.WithServiceReaderTranslator(translator),
	)
	serviceOrchestrator := orchestrator.NewGroupsIOServiceWriterOrchestrator(
		orchestrator.WithServiceWriter(proxyClient),
		orchestrator.WithServiceTranslator(translator),
	)
	mailingListReaderOrchestrator := orchestrator.NewGroupsIOMailingListReaderOrchestrator(
		orchestrator.WithMailingListReader(proxyClient),
		orchestrator.WithMailingListReaderTranslator(translator),
	)
	mailingListOrchestrator := orchestrator.NewGroupsIOMailingListOrchestrator(
		orchestrator.WithMailingListWriter(proxyClient),
		orchestrator.WithMailingListTranslator(translator),
		orchestrator.WithMailingListEventReader(mailingListReaderOrchestrator),
		orchestrator.WithMailingListPublisher(publisher),
		orchestrator.WithMailingListServiceReader(serviceReaderOrchestrator),
		orchestrator.WithMailingListCommitteeProjectLookup(committeeLookup),
	)
	memberReaderOrchestrator := orchestrator.NewGroupsIOMailingListMemberReaderOrchestrator(
		orchestrator.WithMemberReader(proxyClient),
	)
	memberWriterOrchestrator := orchestrator.NewGroupsIOMailingListMemberWriterOrchestrator(
		orchestrator.WithMemberWriter(proxyClient),
	)
	artifactReaderOrchestrator := orchestrator.NewGroupsIOArtifactReaderOrchestrator(
		orchestrator.WithArtifactReader(proxyClient),
	)

	// ---- LFID invite feature ----
	// Initialise invite deps when InvitesEnabled=true.  The acceptance subscriber
	// starts independently of EventingEnabled so that enrichment works even when
	// the data stream consumer is not running on this replica.
	var (
		inviteSender *infraNATS.NATSInviteSender
		userReader   *infraNATS.NATSUserReader
		inviteAccSub *eventing.InviteAcceptedSubscriber
	)
	if env.InvitesEnabled {
		inviteSender = infraNATS.NewInviteSender(natsClient, slog.Default())
		userReader = infraNATS.NewUserReader(natsClient, slog.Default())

		inviteAccSub = eventing.NewInviteAcceptedSubscriber(natsClient, proxyClient, slog.Default())
		if err := inviteAccSub.Start(ctx); err != nil {
			slog.ErrorContext(ctx, "failed to start invite_accepted subscriber; continuing without it",
				"error", err)
			inviteAccSub = nil
		} else {
			slog.InfoContext(ctx, "invite_accepted subscriber started")
		}
	} else {
		slog.InfoContext(ctx, "LFID invite feature disabled (INVITES_ENABLED not set to true)")
	}

	// Create the mailing list API service.
	mailingListSvc := service.NewMailingListAPI(
		authSvc,
		natsClient,
		serviceReaderOrchestrator,
		serviceOrchestrator,
		mailingListReaderOrchestrator,
		mailingListOrchestrator,
		memberReaderOrchestrator,
		memberWriterOrchestrator,
		artifactReaderOrchestrator,
	)

	// Wrap the service in GOA endpoints.
	mailingListServiceEndpoints := mailinglistservice.NewEndpoints(mailingListSvc)
	if flags.Debug {
		mailingListServiceEndpoints.Use(debug.LogPayloads())
	}

	errc := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)

	addr := ":" + env.Port
	if flags.Bind != "*" {
		addr = flags.Bind + ":" + env.Port
	}

	setupHTTPServer(ctx, addr, mailingListServiceEndpoints, &wg, errc, flags.Debug, env.KODataPath)

	// Start data stream processor for v1 DynamoDB KV events (optional).
	if err := handleDataStream(ctx, &wg, env, natsClient, mappings, publisher, inviteSender, userReader); err != nil {
		slog.ErrorContext(ctx, "FATAL: failed to start data stream processor", "error", err)
		cancel()
		return 1
	}

	// Wait for shutdown signal.
	slog.InfoContext(ctx, "received shutdown signal, stopping servers",
		"signal", <-errc,
	)

	// Stop the invite_accepted subscriber before cancelling the context so that
	// in-flight AcceptInvite calls can complete gracefully.
	if inviteAccSub != nil {
		inviteAccSub.Stop()
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gracefulShutdownSeconds*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.InfoContext(ctx, "graceful shutdown completed")
	case <-shutdownCtx.Done():
		slog.WarnContext(ctx, "graceful shutdown timed out")
	}

	return 0
}
