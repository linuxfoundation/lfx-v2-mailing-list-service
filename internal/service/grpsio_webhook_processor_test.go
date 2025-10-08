// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		event := map[string]interface{}{
			"action": "created_subgroup",
			"group": map[string]interface{}{
				"id":              123,
				"name":            "test-group",
				"parent_group_id": 456,
			},
			"extra": "developers",
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupCreatedEvent, data)
		assert.NoError(t, err)
	})

	t.Run("created_subgroup event with missing group info", func(t *testing.T) {
		event := map[string]interface{}{
			"action": "created_subgroup",
			"extra":  "developers",
			// Missing group field
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupCreatedEvent, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing group information")
	})

	t.Run("created_subgroup event with empty extra", func(t *testing.T) {
		event := map[string]interface{}{
			"action": "created_subgroup",
			"group": map[string]interface{}{
				"id":              123,
				"name":            "test-group",
				"parent_group_id": 456,
			},
			"extra": "",
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupCreatedEvent, data)
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
		event := map[string]interface{}{
			"action":   "deleted_subgroup",
			"extra_id": 789,
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupDeletedEvent, data)
		assert.NoError(t, err)
	})

	t.Run("deleted_subgroup event with zero extra_id", func(t *testing.T) {
		event := map[string]interface{}{
			"action":   "deleted_subgroup",
			"extra_id": 0,
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupDeletedEvent, data)
		assert.NoError(t, err)
	})

	t.Run("deleted_subgroup event with missing extra_id", func(t *testing.T) {
		event := map[string]interface{}{
			"action": "deleted_subgroup",
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupDeletedEvent, data)
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
		WithMemberWriter(mock.NewMockGrpsIOWriter(mockRepo)),
	)
	ctx := context.Background()

	t.Run("valid added_member event", func(t *testing.T) {
		event := map[string]interface{}{
			"action": "added_member",
			"member_info": map[string]interface{}{
				"id":         1,
				"user_id":    2,
				"group_id":   123,
				"group_name": "test-group",
				"email":      "test@example.com",
				"status":     "approved",
			},
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupMemberAddedEvent, data)
		assert.NoError(t, err)
	})

	t.Run("added_member event with missing member_info", func(t *testing.T) {
		event := map[string]interface{}{
			"action": "added_member",
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupMemberAddedEvent, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing member info")
	})

	t.Run("added_member event with partial member_info", func(t *testing.T) {
		event := map[string]interface{}{
			"action": "added_member",
			"member_info": map[string]interface{}{
				"email": "test@example.com",
				// Missing other fields
			},
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupMemberAddedEvent, data)
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
		WithMemberWriter(mock.NewMockGrpsIOWriter(mockRepo)),
	)
	ctx := context.Background()

	t.Run("valid removed_member event", func(t *testing.T) {
		event := map[string]interface{}{
			"action": "removed_member",
			"member_info": map[string]interface{}{
				"id":         1,
				"user_id":    2,
				"group_id":   123,
				"group_name": "test-group",
				"email":      "test@example.com",
				"status":     "approved",
			},
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupMemberRemovedEvent, data)
		assert.NoError(t, err)
	})

	t.Run("removed_member event with missing member_info", func(t *testing.T) {
		event := map[string]interface{}{
			"action": "removed_member",
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupMemberRemovedEvent, data)
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
		WithMemberWriter(mock.NewMockGrpsIOWriter(mockRepo)),
	)
	ctx := context.Background()

	t.Run("valid ban_members event", func(t *testing.T) {
		event := map[string]interface{}{
			"action": "ban_members",
			"member_info": map[string]interface{}{
				"id":         1,
				"user_id":    2,
				"group_id":   123,
				"group_name": "test-group",
				"email":      "test@example.com",
				"status":     "banned",
			},
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupMemberBannedEvent, data)
		assert.NoError(t, err)
	})

	t.Run("ban_members event with missing member_info", func(t *testing.T) {
		event := map[string]interface{}{
			"action": "ban_members",
		}
		data, err := json.Marshal(event)
		require.NoError(t, err)

		err = processor.ProcessEvent(ctx, constants.SubGroupMemberBannedEvent, data)
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

	event := map[string]interface{}{
		"action": "unknown_event_type",
	}
	data, err := json.Marshal(event)
	require.NoError(t, err)

	err = processor.ProcessEvent(ctx, "unknown_event_type", data)
	assert.NoError(t, err) // Unknown events are ignored, not errors
}

// TestProcessEvent_InvalidJSON tests invalid JSON handling
func TestProcessEvent_InvalidJSON(t *testing.T) {
	mockRepo := mock.NewMockRepository()
	processor := NewGrpsIOWebhookProcessor(
		WithServiceReader(mockRepo),
		WithMailingListReader(mockRepo),
		WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
	)
	ctx := context.Background()

	invalidJSON := []byte("{invalid json")

	err := processor.ProcessEvent(ctx, constants.SubGroupCreatedEvent, invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
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
