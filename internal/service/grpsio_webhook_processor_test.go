// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/stretchr/testify/assert"
)

// TestProcessEvent_CreatedSubgroup tests created_subgroup event processing
func TestProcessEvent_CreatedSubgroup(t *testing.T) {
	mockRepo := mock.NewMockRepository()
	processor := NewGrpsIOWebhookProcessor(
		WithServiceReader(mockRepo),
		WithMailingListReader(mockRepo),
		WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
	)
	ctx := context.Background()

	t.Run("valid created_subgroup event", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action: constants.SubGroupCreatedEvent,
			Group: &model.GroupInfo{
				ID:            123,
				Name:          "test-group",
				ParentGroupID: 456,
			},
			Extra: "developers",
		}

		err := processor.ProcessEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("created_subgroup event with missing group info", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action: constants.SubGroupCreatedEvent,
			Extra:  "developers",
		}

		err := processor.ProcessEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing group information")
	})

	t.Run("created_subgroup event with empty extra", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action: constants.SubGroupCreatedEvent,
			Group: &model.GroupInfo{
				ID:            123,
				Name:          "test-group",
				ParentGroupID: 456,
			},
			Extra: "",
		}

		err := processor.ProcessEvent(ctx, event)
		assert.NoError(t, err)
	})
}

// TestProcessEvent_DeletedSubgroup tests deleted_subgroup event processing
func TestProcessEvent_DeletedSubgroup(t *testing.T) {
	mockRepo := mock.NewMockRepository()
	processor := NewGrpsIOWebhookProcessor(
		WithServiceReader(mockRepo),
		WithMailingListReader(mockRepo),
		WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
	)
	ctx := context.Background()

	t.Run("valid deleted_subgroup event", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action:  constants.SubGroupDeletedEvent,
			ExtraID: 789,
		}

		err := processor.ProcessEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("deleted_subgroup event with zero extra_id", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action:  constants.SubGroupDeletedEvent,
			ExtraID: 0,
		}

		err := processor.ProcessEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("deleted_subgroup event with missing extra_id", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action: constants.SubGroupDeletedEvent,
		}

		err := processor.ProcessEvent(ctx, event)
		assert.NoError(t, err) // extra_id defaults to 0
	})
}

// TestProcessEvent_MemberAdded tests added_member event processing
func TestProcessEvent_MemberAdded(t *testing.T) {
	mockRepo := mock.NewMockRepository()
	processor := NewGrpsIOWebhookProcessor(
		WithServiceReader(mockRepo),
		WithMailingListReader(mockRepo),
		WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
		WithMemberReader(mockRepo),
		WithMemberWriter(mock.NewMockGrpsIOMemberWriter(mockRepo)),
	)
	ctx := context.Background()

	t.Run("valid added_member event", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action: constants.SubGroupMemberAddedEvent,
			MemberInfo: &model.MemberInfo{
				ID:      1,
				GroupID: 123,
				Email:   "test@example.com",
				Status:  "approved",
			},
		}

		err := processor.ProcessEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("added_member event with missing member_info", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action: constants.SubGroupMemberAddedEvent,
		}

		err := processor.ProcessEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing member info")
	})

	t.Run("added_member event with partial member_info", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action: constants.SubGroupMemberAddedEvent,
			MemberInfo: &model.MemberInfo{
				Email: "test@example.com",
				// Missing other fields
			},
		}

		err := processor.ProcessEvent(ctx, event)
		assert.NoError(t, err) // Partial info is allowed
	})
}

// TestProcessEvent_MemberRemoved tests removed_member event processing
func TestProcessEvent_MemberRemoved(t *testing.T) {
	mockRepo := mock.NewMockRepository()
	processor := NewGrpsIOWebhookProcessor(
		WithServiceReader(mockRepo),
		WithMailingListReader(mockRepo),
		WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
		WithMemberReader(mockRepo),
		WithMemberWriter(mock.NewMockGrpsIOMemberWriter(mockRepo)),
	)
	ctx := context.Background()

	t.Run("valid removed_member event", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action: constants.SubGroupMemberRemovedEvent,
			MemberInfo: &model.MemberInfo{
				ID:      1,
				GroupID: 123,
				Email:   "test@example.com",
				Status:  "approved",
			},
		}

		err := processor.ProcessEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("removed_member event with missing member_info", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action: constants.SubGroupMemberRemovedEvent,
		}

		err := processor.ProcessEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing member info")
	})
}

// TestProcessEvent_MemberBanned tests ban_members event processing
func TestProcessEvent_MemberBanned(t *testing.T) {
	mockRepo := mock.NewMockRepository()
	processor := NewGrpsIOWebhookProcessor(
		WithServiceReader(mockRepo),
		WithMailingListReader(mockRepo),
		WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
		WithMemberReader(mockRepo),
		WithMemberWriter(mock.NewMockGrpsIOMemberWriter(mockRepo)),
	)
	ctx := context.Background()

	t.Run("valid ban_members event", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action: constants.SubGroupMemberBannedEvent,
			MemberInfo: &model.MemberInfo{
				ID:      1,
				GroupID: 123,
				Email:   "test@example.com",
				Status:  "banned",
			},
		}

		err := processor.ProcessEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("ban_members event with missing member_info", func(t *testing.T) {
		event := &model.GrpsIOWebhookEvent{
			Action: constants.SubGroupMemberBannedEvent,
		}

		err := processor.ProcessEvent(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing member info")
	})
}

// TestProcessEvent_UnknownEventType tests unknown event type handling
func TestProcessEvent_UnknownEventType(t *testing.T) {
	mockRepo := mock.NewMockRepository()
	processor := NewGrpsIOWebhookProcessor(
		WithServiceReader(mockRepo),
		WithMailingListReader(mockRepo),
		WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
	)
	ctx := context.Background()

	event := &model.GrpsIOWebhookEvent{
		Action: "unknown_event_type",
	}

	err := processor.ProcessEvent(ctx, event)
	assert.NoError(t, err) // Unknown events are ignored, not errors
}

// NOTE: Retry logic tests have been moved to pkg/utils/retry_test.go
// The retry utilities (RetryConfig, RetryWithExponentialBackoff) are now
// general-purpose utilities in the utils package.

// TestNewGrpsIOWebhookProcessor tests processor creation
func TestNewGrpsIOWebhookProcessor(t *testing.T) {
	mockRepo := mock.NewMockRepository()
	processor := NewGrpsIOWebhookProcessor(
		WithServiceReader(mockRepo),
		WithMailingListReader(mockRepo),
		WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
	)

	assert.NotNil(t, processor)
	assert.Implements(t, (*GrpsIOWebhookProcessor)(nil), processor)
}
