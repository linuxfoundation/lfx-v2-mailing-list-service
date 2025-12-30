// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/cmd/mailing-list-api/service"
	internalService "github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/nats-io/nats.go"
)

// handleCommitteeSync sets up and starts committee event subscriptions
// Pattern: mirrors handleHTTPServer - does both setup and start in one function
func handleCommitteeSync(ctx context.Context, wg *sync.WaitGroup) error {
	slog.InfoContext(ctx, "starting committee sync")

	// Get dependencies (all inside function - mirrors handleHTTPServer)
	mailingListReader := service.GrpsIOReader(ctx)
	memberWriter := service.GrpsIOWriterOrchestrator(ctx) // Use orchestrator for message publishing
	memberReader := service.GrpsIOReader(ctx)
	entityReader := service.EntityAttributeRetriever(ctx)
	natsClient := service.GetNATSClient(ctx)

	// Create committee sync service
	syncService := internalService.NewCommitteeSyncService(
		mailingListReader,
		memberWriter,
		memberReader,
		entityReader,
	)

	// Subscribe to all committee event subjects
	subjects := []string{
		constants.CommitteeMemberCreatedSubject,
		constants.CommitteeMemberDeletedSubject,
		constants.CommitteeMemberUpdatedSubject,
	}

	for _, subject := range subjects {
		// Capture loop variable for closure
		subject := subject
		_, subErr := natsClient.QueueSubscribe(
			subject,
			constants.MailingListAPIQueue,
			func(msg *nats.Msg) {
				// Check if service is shutting down
				select {
				case <-ctx.Done():
					slog.InfoContext(ctx, "rejecting message - service shutting down",
						"subject", msg.Subject)
					if nakErr := msg.Nak(); nakErr != nil {
						slog.ErrorContext(ctx, "failed to nak message during shutdown", "error", nakErr)
					}
					return
				default:
					// Continue processing
				}

				// Create fresh context with timeout for this message
				// Not derived from shutdown context to avoid cancellation issues
				msgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				// Process message with proper error handling and acknowledgment
				if handleErr := syncService.HandleMessage(msgCtx, msg); handleErr != nil {
					slog.ErrorContext(msgCtx, "failed to process committee event, will retry",
						"error", handleErr,
						"subject", msg.Subject)
					if nakErr := msg.Nak(); nakErr != nil {
						slog.ErrorContext(msgCtx, "failed to nak message", "error", nakErr)
					}
				} else {
					// Success - acknowledge message
					if ackErr := msg.Ack(); ackErr != nil {
						slog.ErrorContext(msgCtx, "failed to ack message", "error", ackErr)
					}
				}
			},
		)
		if subErr != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", subject, subErr)
		}
		slog.InfoContext(ctx, "subscribed to committee event",
			"subject", subject,
			"queue", constants.MailingListAPIQueue)
	}

	slog.InfoContext(ctx, "committee sync started successfully")

	// Graceful shutdown (mirrors handleHTTPServer)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		slog.InfoContext(ctx, "shutting down committee sync")
		// NATS client cleanup handled by existing Close() in main shutdown
	}()

	return nil
}
