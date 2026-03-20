// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package main is the ITX mailing list proxy service that provides a lightweight proxy
// layer to the ITX GroupsIO API.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/cmd/mailing-list-api/service"
	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/proxy"
	logging "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/log"

	"goa.design/clue/debug"
)

const (
	defaultPort             = "8080"
	gracefulShutdownSeconds = 25
)

func init() {
	logging.InitStructureLogConfig()
}

func main() {
	var (
		dbgF = flag.Bool("d", false, "enable debug logging")
		port = flag.String("p", defaultPort, "listen port")
		bind = flag.String("bind", "*", "interface to bind on")
	)
	flag.Usage = func() {
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()

	ctx := context.Background()
	slog.InfoContext(ctx, "Starting ITX mailing list proxy service",
		"bind", *bind,
		"http-port", *port,
		"graceful-shutdown-seconds", gracefulShutdownSeconds,
	)

	// Initialize authentication service
	authService := service.AuthService(ctx)

	// Initialize ID mapper for v1/v2 ID conversions
	idMapper := service.IDMapper(ctx)

	// Initialize ITX proxy client
	itxConfig := service.ITXProxyConfig()
	itxClient := proxy.NewClient(itxConfig)

	// Initialize ITX GroupsIO services
	svcService := service.GroupsioServiceService(ctx, itxClient, idMapper)
	subgroupService := service.GroupsioSubgroupService(ctx, itxClient, idMapper)
	memberService := service.GroupsioMemberService(ctx, itxClient)

	slog.InfoContext(ctx, "ITX proxy client initialized")

	// Create the mailing list API service
	mailingListSvc := service.NewMailingListAPI(
		authService,
		svcService,
		subgroupService,
		memberService,
	)

	// Wrap the services in endpoints
	mailingListServiceEndpoints := mailinglistservice.NewEndpoints(mailingListSvc)
	mailingListServiceEndpoints.Use(debug.LogPayloads())

	errc := make(chan error)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)

	addr := ":" + *port
	if *bind != "*" {
		addr = *bind + ":" + *port
	}

	handleHTTPServer(ctx, addr, mailingListServiceEndpoints, &wg, errc, *dbgF)

	// Start committee sync - critical for data consistency
	if err := handleCommitteeSync(ctx, &wg); err != nil {
		slog.ErrorContext(ctx, "FATAL: failed to start committee sync - service cannot maintain data consistency", "error", err)
		os.Exit(1)
	}

	// Start mailing list sync - critical for data consistency
	if err := handleMailingListSync(ctx, &wg); err != nil {
		slog.ErrorContext(ctx, "FATAL: failed to start mailing list sync - service cannot maintain data consistency", "error", err)
		os.Exit(1)
	}

	// Start data stream processor for v1 DynamoDB KV events (optional — enabled via env var)
	if err := handleDataStream(ctx, &wg); err != nil {
		slog.ErrorContext(ctx, "FATAL: failed to start data stream processor", "error", err)
		os.Exit(1)
	}

	// Wait for signal.
	slog.InfoContext(ctx, "received shutdown signal, stopping servers",
		"signal", <-errc,
	)

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

	slog.InfoContext(ctx, "exited")
}
