// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"testing"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/groupsio"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/service"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWebhookSecret = "test-secret-123"

// Helper function to generate HMAC-SHA1 signature
func generateSignature(body []byte, secret string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// TestWebhook_ValidSignature tests webhook with valid HMAC-SHA1 signature
func TestWebhook_ValidSignature(t *testing.T) {
	// Create webhook service with mock dependencies
	mockRepo := mock.NewMockRepository()
	grpsioWebhookValidator := groupsio.NewGrpsIOWebhookValidator(testWebhookSecret)
	grpsioWebhookProcessor := service.NewGrpsIOWebhookProcessor(
		service.WithServiceReader(mockRepo),
		service.WithMailingListReader(mockRepo),
		service.WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
	)

	svc := NewMailingList(
		mock.NewMockAuthService(),
		nil, // grpsIOReaderOrchestrator
		nil, // grpsIOWriterOrchestrator
		nil, // storage
		grpsioWebhookValidator,
		grpsioWebhookProcessor,
	)

	// Create webhook event payload
	event := map[string]interface{}{
		"action": "created_subgroup",
		"group": map[string]interface{}{
			"id":              123,
			"name":            "test-group",
			"parent_group_id": 456,
		},
		"extra": "developers",
	}
	bodyBytes, err := json.Marshal(event)
	require.NoError(t, err)

	// Generate valid signature
	signature := generateSignature(bodyBytes, testWebhookSecret)

	// Create context with body
	ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

	// Create payload
	payload := &mailinglistservice.GroupsioWebhookPayload{
		Signature: signature,
		Body:      bodyBytes,
	}

	// Call webhook handler
	err = svc.(*mailingListService).GroupsioWebhook(ctx, payload)

	// Verify 204 No Content (nil error)
	assert.NoError(t, err)
}

// TestWebhook_InvalidSignature tests webhook with invalid signature
func TestWebhook_InvalidSignature(t *testing.T) {
	mockRepo := mock.NewMockRepository()
	grpsioWebhookValidator := groupsio.NewGrpsIOWebhookValidator(testWebhookSecret)
	grpsioWebhookProcessor := service.NewGrpsIOWebhookProcessor(
		service.WithServiceReader(mockRepo),
		service.WithMailingListReader(mockRepo),
		service.WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
	)

	svc := NewMailingList(
		mock.NewMockAuthService(),
		nil,
		nil,
		nil,
		grpsioWebhookValidator,
		grpsioWebhookProcessor,
	)

	event := map[string]interface{}{
		"action": "created_subgroup",
		"group": map[string]interface{}{
			"id":              123,
			"name":            "test-group",
			"parent_group_id": 456,
		},
		"extra": "developers",
	}
	bodyBytes, err := json.Marshal(event)
	require.NoError(t, err)

	// Invalid signature
	invalidSignature := "invalid-signature-12345"

	ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

	payload := &mailinglistservice.GroupsioWebhookPayload{
		Signature: invalidSignature,
		Body:      bodyBytes,
	}

	err = svc.(*mailingListService).GroupsioWebhook(ctx, payload)

	// Verify 401 Unauthorized
	require.Error(t, err)
	unauthorizedErr, ok := err.(*mailinglistservice.UnauthorizedError)
	assert.True(t, ok, "Expected UnauthorizedError")
	assert.Equal(t, "invalid webhook signature", unauthorizedErr.Message)
}

// TestWebhook_MissingBody tests webhook without body in context
func TestWebhook_MissingBody(t *testing.T) {
	grpsioWebhookValidator := mock.NewMockGrpsIOWebhookValidator()
	grpsioWebhookProcessor := service.NewGrpsIOWebhookProcessor()

	svc := NewMailingList(
		mock.NewMockAuthService(),
		nil,
		nil,
		nil,
		grpsioWebhookValidator,
		grpsioWebhookProcessor,
	)

	// Context without body
	ctx := context.Background()

	payload := &mailinglistservice.GroupsioWebhookPayload{
		Signature: "some-signature",
		Body:      []byte("{}"),
	}

	err := svc.(*mailingListService).GroupsioWebhook(ctx, payload)

	// Verify 400 Bad Request
	require.Error(t, err)
	badRequestErr, ok := err.(*mailinglistservice.BadRequestError)
	assert.True(t, ok, "Expected BadRequestError")
	assert.Equal(t, "missing webhook body", badRequestErr.Message)
}

// TestWebhook_MalformedPayload tests webhook with invalid JSON
func TestWebhook_MalformedPayload(t *testing.T) {
	grpsioWebhookValidator := mock.NewMockGrpsIOWebhookValidator()
	grpsioWebhookProcessor := service.NewGrpsIOWebhookProcessor()

	svc := NewMailingList(
		mock.NewMockAuthService(),
		nil,
		nil,
		nil,
		grpsioWebhookValidator,
		grpsioWebhookProcessor,
	)

	// Invalid JSON
	bodyBytes := []byte("{invalid json")

	ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

	payload := &mailinglistservice.GroupsioWebhookPayload{
		Signature: "some-signature",
		Body:      bodyBytes,
	}

	err := svc.(*mailingListService).GroupsioWebhook(ctx, payload)

	// Verify 400 Bad Request
	require.Error(t, err)
	badRequestErr, ok := err.(*mailinglistservice.BadRequestError)
	assert.True(t, ok, "Expected BadRequestError")
	assert.Equal(t, "invalid event format", badRequestErr.Message)
}

// TestWebhook_UnsupportedEventType tests webhook with unsupported event type
func TestWebhook_UnsupportedEventType(t *testing.T) {
	grpsioWebhookValidator := groupsio.NewGrpsIOWebhookValidator(testWebhookSecret)
	grpsioWebhookProcessor := service.NewGrpsIOWebhookProcessor()

	svc := NewMailingList(
		mock.NewMockAuthService(),
		nil,
		nil,
		nil,
		grpsioWebhookValidator,
		grpsioWebhookProcessor,
	)

	event := map[string]interface{}{
		"action": "unsupported_event",
	}
	bodyBytes, err := json.Marshal(event)
	require.NoError(t, err)

	signature := generateSignature(bodyBytes, testWebhookSecret)

	ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

	payload := &mailinglistservice.GroupsioWebhookPayload{
		Signature: signature,
		Body:      bodyBytes,
	}

	err = svc.(*mailingListService).GroupsioWebhook(ctx, payload)

	// Verify 400 Bad Request
	require.Error(t, err)
	badRequestErr, ok := err.(*mailinglistservice.BadRequestError)
	assert.True(t, ok, "Expected BadRequestError")
	assert.Contains(t, badRequestErr.Message, "unsupported event type")
}

// TestWebhook_MockMode tests webhook in mock mode (always valid)
func TestWebhook_MockMode(t *testing.T) {
	mockRepo := mock.NewMockRepository()
	grpsioWebhookValidator := mock.NewMockGrpsIOWebhookValidator()
	grpsioWebhookProcessor := service.NewGrpsIOWebhookProcessor(
		service.WithServiceReader(mockRepo),
		service.WithMailingListReader(mockRepo),
		service.WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
	)

	svc := NewMailingList(
		mock.NewMockAuthService(),
		nil,
		nil,
		nil,
		grpsioWebhookValidator,
		grpsioWebhookProcessor,
	)

	event := map[string]interface{}{
		"action": "created_subgroup",
		"group": map[string]interface{}{
			"id":              123,
			"name":            "test-group",
			"parent_group_id": 456,
		},
		"extra": "developers",
	}
	bodyBytes, err := json.Marshal(event)
	require.NoError(t, err)

	// No signature needed in mock mode
	ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

	payload := &mailinglistservice.GroupsioWebhookPayload{
		Signature: "any-signature-works-in-mock",
		Body:      bodyBytes,
	}

	err = svc.(*mailingListService).GroupsioWebhook(ctx, payload)

	// Verify success (204 No Content)
	assert.NoError(t, err)
}

// TestWebhook_AllEventTypes tests all 5 supported event types
func TestWebhook_AllEventTypes(t *testing.T) {
	eventTypes := []string{
		"created_subgroup",
		"deleted_subgroup",
		"added_member",
		"removed_member",
		"ban_members",
	}

	mockRepo := mock.NewMockRepository()
	grpsioWebhookValidator := mock.NewMockGrpsIOWebhookValidator()
	grpsioWebhookProcessor := service.NewGrpsIOWebhookProcessor(
		service.WithServiceReader(mockRepo),
		service.WithMailingListReader(mockRepo),
		service.WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
		service.WithMemberReader(mockRepo),
		service.WithMemberWriter(mock.NewMockGrpsIOWriter(mockRepo)),
	)

	svc := NewMailingList(
		mock.NewMockAuthService(),
		nil,
		nil,
		nil,
		grpsioWebhookValidator,
		grpsioWebhookProcessor,
	)

	for _, eventType := range eventTypes {
		t.Run(eventType, func(t *testing.T) {
			event := map[string]interface{}{
				"action": eventType,
			}

			// Add required fields based on event type
			switch eventType {
			case "created_subgroup":
				event["group"] = map[string]interface{}{
					"id":              123,
					"name":            "test-group",
					"parent_group_id": 456,
				}
				event["extra"] = "developers"
			case "deleted_subgroup":
				event["extra_id"] = 789
			case "added_member", "removed_member", "ban_members":
				event["member_info"] = map[string]interface{}{
					"id":         1,
					"user_id":    2,
					"group_id":   123,
					"group_name": "test-group",
					"email":      "test@example.com",
					"status":     "approved",
				}
			}

			bodyBytes, err := json.Marshal(event)
			require.NoError(t, err)

			ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

			payload := &mailinglistservice.GroupsioWebhookPayload{
				Signature: "mock-signature",
				Body:      bodyBytes,
			}

			err = svc.(*mailingListService).GroupsioWebhook(ctx, payload)

			// All should succeed (204 No Content)
			assert.NoError(t, err, "Event type %s should succeed", eventType)
		})
	}
}

// TestWebhook_CreatedSubgroupMissingGroupInfo tests created_subgroup with missing group info
func TestWebhook_CreatedSubgroupMissingGroupInfo(t *testing.T) {
	grpsioWebhookValidator := mock.NewMockGrpsIOWebhookValidator()
	grpsioWebhookProcessor := service.NewGrpsIOWebhookProcessor()

	svc := NewMailingList(
		mock.NewMockAuthService(),
		nil,
		nil,
		nil,
		grpsioWebhookValidator,
		grpsioWebhookProcessor,
	)

	event := map[string]interface{}{
		"action": "created_subgroup",
		// Missing group field
		"extra": "developers",
	}
	bodyBytes, err := json.Marshal(event)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

	payload := &mailinglistservice.GroupsioWebhookPayload{
		Signature: "mock-signature",
		Body:      bodyBytes,
	}

	err = svc.(*mailingListService).GroupsioWebhook(ctx, payload)

	// Should still return 204 (logged error, but always returns nil to prevent retries)
	assert.NoError(t, err)
}

// TestWebhook_MemberEventMissingMemberInfo tests member events with missing member info
func TestWebhook_MemberEventMissingMemberInfo(t *testing.T) {
	mockRepo := mock.NewMockRepository()
	grpsioWebhookValidator := mock.NewMockGrpsIOWebhookValidator()
	grpsioWebhookProcessor := service.NewGrpsIOWebhookProcessor(
		service.WithServiceReader(mockRepo),
		service.WithMailingListReader(mockRepo),
		service.WithMailingListWriter(mock.NewMockGrpsIOMailingListWriter(mockRepo)),
		service.WithMemberReader(mockRepo),
		service.WithMemberWriter(mock.NewMockGrpsIOWriter(mockRepo)),
	)

	svc := NewMailingList(
		mock.NewMockAuthService(),
		nil,
		nil,
		nil,
		grpsioWebhookValidator,
		grpsioWebhookProcessor,
	)

	event := map[string]interface{}{
		"action": "added_member",
		// Missing member_info field
	}
	bodyBytes, err := json.Marshal(event)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

	payload := &mailinglistservice.GroupsioWebhookPayload{
		Signature: "mock-signature",
		Body:      bodyBytes,
	}

	err = svc.(*mailingListService).GroupsioWebhook(ctx, payload)

	// Should still return 204 (logged error, but always returns nil to prevent retries)
	assert.NoError(t, err)
}
