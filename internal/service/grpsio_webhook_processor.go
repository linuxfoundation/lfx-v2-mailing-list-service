// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// GrpsIOWebhookProcessor handles GroupsIO webhook event processing
type GrpsIOWebhookProcessor interface {
	ProcessEvent(ctx context.Context, eventType string, data []byte) error
}

// SIMPLIFIED FOR MVP: No dependencies, no functional options
// PR #2 will add orchestrator pattern with:
// - grpsIOWriter, grpsIOReader
// - entityReader, publisher, groupsClient
// - Functional options pattern
type grpsIOWebhookProcessor struct {
	// Empty struct for MVP
	// Dependencies added in PR #2 (Business Logic)
}

// NewGrpsIOWebhookProcessor creates a new GroupsIO webhook processor
func NewGrpsIOWebhookProcessor() GrpsIOWebhookProcessor {
	return &grpsIOWebhookProcessor{}
}

// ProcessEvent routes webhook events to appropriate handlers
func (p *grpsIOWebhookProcessor) ProcessEvent(ctx context.Context, eventType string, data []byte) error {
	slog.InfoContext(ctx, "processing groupsio webhook event", "event_type", eventType)

	var event model.GrpsIOWebhookEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal groupsio webhook event: %w", err)
	}
	event.Action = eventType

	switch eventType {
	case constants.SubGroupCreatedEvent:
		return p.handleSubGroupCreated(ctx, &event)
	case constants.SubGroupDeletedEvent:
		return p.handleSubGroupDeleted(ctx, &event)
	case constants.SubGroupMemberAddedEvent:
		return p.handleMemberAdded(ctx, &event)
	case constants.SubGroupMemberRemovedEvent:
		return p.handleMemberRemoved(ctx, &event)
	case constants.SubGroupMemberBannedEvent:
		return p.handleMemberBanned(ctx, &event)
	default:
		slog.WarnContext(ctx, "unknown groupsio webhook event type", "event_type", eventType)
		return nil // Ignore unknown events
	}
}

// MINIMAL HANDLERS - Log and validate only

func (p *grpsIOWebhookProcessor) handleSubGroupCreated(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	if event.Group == nil {
		return fmt.Errorf("missing group information in created_subgroup event")
	}

	subGroupName := fmt.Sprintf("%s+%s", event.Group.Name, event.Extra)

	slog.InfoContext(ctx, "received created_subgroup event",
		"subgroup_name", subGroupName,
		"parent_group_id", event.Group.ParentGroupID,
		"subgroup_id", event.Group.ID,
	)

	// TODO (PR #2): Implement subgroup adoption logic
	// - Find parent service by parent_group_id
	// - Validate prefix matching (v2_formation_, v2_shared_, etc)
	// - Create mailing list entry in NATS KV

	return nil
}

func (p *grpsIOWebhookProcessor) handleSubGroupDeleted(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	subGroupID := uint64(event.ExtraID)

	slog.InfoContext(ctx, "received deleted_subgroup event",
		"subgroup_id", subGroupID,
	)

	// TODO (PR #2): Implement deletion logic
	// - Find mailing list by group_id
	// - Delete from NATS KV
	// - Check if last subgroup (EnabledServices event in PR #3)

	return nil
}

func (p *grpsIOWebhookProcessor) handleMemberAdded(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	if event.MemberInfo == nil {
		return fmt.Errorf("missing member info in added_member event")
	}

	slog.InfoContext(ctx, "received added_member event",
		"group_id", event.MemberInfo.GroupID,
		"email", event.MemberInfo.Email,
		"status", event.MemberInfo.Status,
	)

	// TODO (PR #3): Publish to NATS subject "groupsio.member.added"
	// This will replace the old SNS publishing to Zoom service

	return nil
}

func (p *grpsIOWebhookProcessor) handleMemberRemoved(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	if event.MemberInfo == nil {
		return fmt.Errorf("missing member info in removed_member event")
	}

	slog.InfoContext(ctx, "received removed_member event",
		"group_id", event.MemberInfo.GroupID,
		"email", event.MemberInfo.Email,
	)

	// TODO (PR #3): Publish to NATS subject "groupsio.member.removed"

	return nil
}

func (p *grpsIOWebhookProcessor) handleMemberBanned(ctx context.Context, event *model.GrpsIOWebhookEvent) error {
	if event.MemberInfo == nil {
		return fmt.Errorf("missing member info in ban_members event")
	}

	slog.InfoContext(ctx, "received ban_members event",
		"group_id", event.MemberInfo.GroupID,
		"email", event.MemberInfo.Email,
	)

	// TODO (PR #3): Publish to NATS subject "groupsio.member.banned"

	return nil
}

// RetryConfig holds retry configuration for webhook processing
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultRetryConfig returns default retry configuration for webhooks
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: constants.WebhookMaxRetries,
		BaseDelay:   constants.WebhookRetryBaseDelay * time.Millisecond,
		MaxDelay:    constants.WebhookRetryMaxDelay * time.Millisecond,
	}
}

// RetryWithExponentialBackoff executes a function with exponential backoff retry logic
func RetryWithExponentialBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff: baseDelay * 2^(attempt-1)
			delay := time.Duration(1<<uint(attempt-1)) * config.BaseDelay
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}

			slog.WarnContext(ctx, "retrying webhook event processing",
				"attempt", attempt+1,
				"total_attempts", config.MaxAttempts,
				"retry_delay_ms", delay.Milliseconds(),
			)

			// Wait with context cancellation support
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled: %w", ctx.Err())
			}
		}

		err := fn()
		if err == nil {
			if attempt > 0 {
				slog.InfoContext(ctx, "retry succeeded",
					"attempt", attempt+1,
					"total_attempts", config.MaxAttempts,
				)
			}
			return nil
		}

		lastErr = err
		slog.ErrorContext(ctx, "webhook event processing attempt failed",
			"attempt", attempt+1,
			"total_attempts", config.MaxAttempts,
			"error", err,
		)
	}

	return fmt.Errorf("failed after %d attempts: %w", config.MaxAttempts, lastErr)
}
