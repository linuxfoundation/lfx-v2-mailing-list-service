// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessEvent_CreatedSubgroup tests created_subgroup event processing
func TestProcessEvent_CreatedSubgroup(t *testing.T) {
	processor := NewGrpsIOWebhookProcessor()
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
	processor := NewGrpsIOWebhookProcessor()
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
	processor := NewGrpsIOWebhookProcessor()
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
	processor := NewGrpsIOWebhookProcessor()
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
	processor := NewGrpsIOWebhookProcessor()
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
	processor := NewGrpsIOWebhookProcessor()
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
	processor := NewGrpsIOWebhookProcessor()
	ctx := context.Background()

	invalidJSON := []byte("{invalid json")

	err := processor.ProcessEvent(ctx, constants.SubGroupCreatedEvent, invalidJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

// TestRetryWithExponentialBackoff tests retry logic
func TestRetryWithExponentialBackoff(t *testing.T) {
	ctx := context.Background()

	t.Run("succeeds on first attempt", func(t *testing.T) {
		config := RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    100 * time.Millisecond,
		}

		attempts := 0
		err := RetryWithExponentialBackoff(ctx, config, func() error {
			attempts++
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("succeeds on second attempt", func(t *testing.T) {
		config := RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    100 * time.Millisecond,
		}

		attempts := 0
		err := RetryWithExponentialBackoff(ctx, config, func() error {
			attempts++
			if attempts < 2 {
				return errors.New("transient error")
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 2, attempts)
	})

	t.Run("fails after max attempts", func(t *testing.T) {
		config := RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond,
			MaxDelay:    100 * time.Millisecond,
		}

		attempts := 0
		expectedErr := errors.New("persistent error")
		err := RetryWithExponentialBackoff(ctx, config, func() error {
			attempts++
			return expectedErr
		})

		assert.Error(t, err)
		assert.Equal(t, 3, attempts)
		assert.Contains(t, err.Error(), "failed after 3 attempts")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		config := RetryConfig{
			MaxAttempts: 10,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    1000 * time.Millisecond,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		attempts := 0
		err := RetryWithExponentialBackoff(ctx, config, func() error {
			attempts++
			return errors.New("error")
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "retry cancelled")
		assert.Less(t, attempts, 10) // Should not reach max attempts
	})

	t.Run("exponential backoff delay calculation", func(t *testing.T) {
		config := RetryConfig{
			MaxAttempts: 5,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    500 * time.Millisecond,
		}

		attempts := 0
		startTime := time.Now()

		err := RetryWithExponentialBackoff(ctx, config, func() error {
			attempts++
			if attempts < 4 {
				return errors.New("error")
			}
			return nil
		})

		elapsed := time.Since(startTime)

		assert.NoError(t, err)
		assert.Equal(t, 4, attempts)
		// Expected delays: 100ms (1st retry), 200ms (2nd retry), 400ms (3rd retry)
		// Total: ~700ms
		assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(600))
		assert.LessOrEqual(t, elapsed.Milliseconds(), int64(900))
	})

	t.Run("respects max delay cap", func(t *testing.T) {
		config := RetryConfig{
			MaxAttempts: 5,
			BaseDelay:   100 * time.Millisecond,
			MaxDelay:    150 * time.Millisecond, // Cap at 150ms
		}

		attempts := 0
		startTime := time.Now()

		err := RetryWithExponentialBackoff(ctx, config, func() error {
			attempts++
			if attempts < 4 {
				return errors.New("error")
			}
			return nil
		})

		elapsed := time.Since(startTime)

		assert.NoError(t, err)
		assert.Equal(t, 4, attempts)
		// Expected delays: 100ms, 150ms (capped), 150ms (capped)
		// Total: ~400ms
		assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(350))
		assert.LessOrEqual(t, elapsed.Milliseconds(), int64(550))
	})
}

// TestDefaultRetryConfig tests default retry configuration
func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.Equal(t, constants.WebhookMaxRetries, config.MaxAttempts)
	assert.Equal(t, constants.WebhookRetryBaseDelay*time.Millisecond, config.BaseDelay)
	assert.Equal(t, constants.WebhookRetryMaxDelay*time.Millisecond, config.MaxDelay)
}

// TestNewGrpsIOWebhookProcessor tests processor creation
func TestNewGrpsIOWebhookProcessor(t *testing.T) {
	processor := NewGrpsIOWebhookProcessor()

	assert.NotNil(t, processor)
	assert.Implements(t, (*GrpsIOWebhookProcessor)(nil), processor)
}
