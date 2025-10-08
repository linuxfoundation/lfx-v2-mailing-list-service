// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
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

// Helper function to generate HMAC-SHA1 signature with base64 encoding (matches production)
func generateSignature(body []byte, secret string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// createProductionWebhookPayload simulates production GroupsIO payload structure after GOA decoding
func createProductionWebhookPayload(bodyBytes []byte, signature string) *mailinglistservice.GroupsioWebhookPayload {
	var eventJSON map[string]interface{}
	json.Unmarshal(bodyBytes, &eventJSON)

	payload := &mailinglistservice.GroupsioWebhookPayload{
		Signature: signature,
	}

	// Extract action (required field)
	if action, ok := eventJSON["action"].(string); ok {
		payload.Action = action
	}

	// Populate fields based on event type (simulating GOA decoder)
	if group, ok := eventJSON["group"]; ok {
		payload.Group = group
	}
	if memberInfo, ok := eventJSON["member_info"]; ok {
		payload.MemberInfo = memberInfo
	}
	if extra, ok := eventJSON["extra"].(string); ok {
		payload.Extra = &extra
	}
	if extraID, ok := eventJSON["extra_id"].(float64); ok {
		id := int(extraID)
		payload.ExtraID = &id
	}

	return payload
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

	// Create production-like webhook event payload (use float64 for numbers as JSON unmarshaling does)
	event := map[string]interface{}{
		"action": "created_subgroup",
		"group": map[string]interface{}{
			"id":              float64(142630),
			"name":            "lfx-test-1759227480",
			"parent_group_id": float64(141234),
			"title":           "LFX Test Project",
			"type":            "sub_group",
			"privacy":         "private",
		},
		"extra": "developers",
	}
	bodyBytes, err := json.Marshal(event)
	require.NoError(t, err)

	// Generate valid base64 signature
	signature := generateSignature(bodyBytes, testWebhookSecret)

	// Create context with body
	ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

	// Create payload with GOA-style populated fields (not Body field)
	payload := createProductionWebhookPayload(bodyBytes, signature)

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

	// Production-like payload with float64 for numbers
	event := map[string]interface{}{
		"action": "created_subgroup",
		"group": map[string]interface{}{
			"id":              float64(142630),
			"name":            "lfx-test-1759227480",
			"parent_group_id": float64(141234),
		},
		"extra": "developers",
	}
	bodyBytes, err := json.Marshal(event)
	require.NoError(t, err)

	// Invalid signature
	invalidSignature := "invalid-signature-12345"

	ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

	// Create payload with populated fields
	payload := createProductionWebhookPayload(bodyBytes, invalidSignature)

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

	// Create minimal payload
	event := map[string]interface{}{
		"action": "created_subgroup",
	}
	bodyBytes, _ := json.Marshal(event)

	payload := createProductionWebhookPayload(bodyBytes, "some-signature")

	err := svc.(*mailingListService).GroupsioWebhook(ctx, payload)

	// Verify 400 Bad Request
	require.Error(t, err)
	badRequestErr, ok := err.(*mailinglistservice.BadRequestError)
	assert.True(t, ok, "Expected BadRequestError")
	assert.Equal(t, "missing webhook body", badRequestErr.Message)
}

// TestWebhook_MalformedPayload tests webhook with malformed event (missing action field)
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

	// Malformed event: missing required 'action' field
	event := map[string]interface{}{
		"group": map[string]interface{}{
			"id": float64(123),
		},
	}
	bodyBytes, _ := json.Marshal(event)

	ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

	// Create payload without action field
	payload := &mailinglistservice.GroupsioWebhookPayload{
		Signature: "some-signature",
		Group:     event["group"],
	}

	err := svc.(*mailingListService).GroupsioWebhook(ctx, payload)

	// Should fail validation or return error
	// Handler returns nil (204) to prevent retries, but logs error internally
	assert.NoError(t, err)
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

	payload := createProductionWebhookPayload(bodyBytes, signature)

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

	// Production-like payload with float64 for numbers
	event := map[string]interface{}{
		"action": "created_subgroup",
		"group": map[string]interface{}{
			"id":              float64(142630),
			"name":            "lfx-test-1759227480",
			"parent_group_id": float64(141234),
		},
		"extra": "developers",
	}
	bodyBytes, err := json.Marshal(event)
	require.NoError(t, err)

	// No signature needed in mock mode
	ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

	payload := createProductionWebhookPayload(bodyBytes, "any-signature-works-in-mock")

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

			// Add required fields based on event type (use float64 for numbers)
			switch eventType {
			case "created_subgroup":
				event["group"] = map[string]interface{}{
					"id":              float64(142630),
					"name":            "lfx-test-1759227480",
					"parent_group_id": float64(141234),
					"title":           "LFX Test Project",
				}
				event["extra"] = "developers"
			case "deleted_subgroup":
				event["extra_id"] = float64(789)
			case "added_member", "removed_member", "ban_members":
				event["member_info"] = map[string]interface{}{
					"id":         float64(12345),
					"user_id":    float64(67890),
					"group_id":   float64(142630),
					"group_name": "lfx-test-1759227480+developers",
					"email":      "user@example.com",
					"status":     "approved",
					"object":     "member",
				}
			}

			bodyBytes, err := json.Marshal(event)
			require.NoError(t, err)

			ctx := context.WithValue(context.Background(), constants.GrpsIOWebhookBodyContextKey, bodyBytes)

			payload := createProductionWebhookPayload(bodyBytes, "mock-signature")

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

	payload := createProductionWebhookPayload(bodyBytes, "mock-signature")

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

	payload := createProductionWebhookPayload(bodyBytes, "mock-signature")

	err = svc.(*mailingListService).GroupsioWebhook(ctx, payload)

	// Should still return 204 (logged error, but always returns nil to prevent retries)
	assert.NoError(t, err)
}
