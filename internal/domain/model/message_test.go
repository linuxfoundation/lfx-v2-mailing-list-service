// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

func TestIndexerMessage_Build(t *testing.T) {
	tests := []struct {
		name        string
		message     *IndexerMessage
		context     func() context.Context
		input       any
		expectError bool
		validate    func(t *testing.T, result *IndexerMessage, err error)
	}{
		{
			name: "build create action with context headers",
			message: &IndexerMessage{
				Action: ActionCreated,
				Tags:   []string{"test:tag"},
			},
			context: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, constants.AuthorizationContextID, "Bearer token123")
				ctx = context.WithValue(ctx, constants.PrincipalContextID, "user123")
				return ctx
			},
			input: map[string]interface{}{
				"uid":  "test-uid",
				"name": "test-name",
			},
			expectError: false,
			validate: func(t *testing.T, result *IndexerMessage, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Check action preserved
				assert.Equal(t, ActionCreated, result.Action)

				// Check tags preserved
				assert.Equal(t, []string{"test:tag"}, result.Tags)

				// Check headers extracted from context
				assert.Equal(t, "Bearer token123", result.Headers[constants.AuthorizationHeader])
				assert.Equal(t, "user123", result.Headers[constants.XOnBehalfOfHeader])

				// Check data is properly marshaled to map[string]any
				dataMap, ok := result.Data.(map[string]any)
				require.True(t, ok, "Data should be a map[string]any")
				assert.Equal(t, "test-uid", dataMap["uid"])
				assert.Equal(t, "test-name", dataMap["name"])
			},
		},
		{
			name: "build update action with context headers",
			message: &IndexerMessage{
				Action: ActionUpdated,
				Tags:   []string{"update:tag"},
			},
			context: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, constants.AuthorizationContextID, "Bearer updated-token")
				ctx = context.WithValue(ctx, constants.PrincipalContextID, "admin-user")
				return ctx
			},
			input: map[string]interface{}{
				"uid":         "updated-uid",
				"description": "updated description",
			},
			expectError: false,
			validate: func(t *testing.T, result *IndexerMessage, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)

				assert.Equal(t, ActionUpdated, result.Action)
				assert.Equal(t, "Bearer updated-token", result.Headers[constants.AuthorizationHeader])
				assert.Equal(t, "admin-user", result.Headers[constants.XOnBehalfOfHeader])

				dataMap, ok := result.Data.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "updated-uid", dataMap["uid"])
				assert.Equal(t, "updated description", dataMap["description"])
			},
		},
		{
			name: "build delete action",
			message: &IndexerMessage{
				Action: ActionDeleted,
				Tags:   []string{"delete:tag"},
			},
			context: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, constants.PrincipalContextID, "delete-user")
				return ctx
			},
			input:       "deleted-uid-123",
			expectError: false,
			validate: func(t *testing.T, result *IndexerMessage, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)

				assert.Equal(t, ActionDeleted, result.Action)
				assert.Equal(t, "delete-user", result.Headers[constants.XOnBehalfOfHeader])
				assert.NotContains(t, result.Headers, constants.AuthorizationHeader)

				// For delete actions, data should be the input directly (string)
				assert.Equal(t, "deleted-uid-123", result.Data)
			},
		},
		{
			name: "build without context values",
			message: &IndexerMessage{
				Action: ActionCreated,
			},
			context: func() context.Context {
				return context.Background()
			},
			input: map[string]interface{}{
				"uid": "no-context-uid",
			},
			expectError: false,
			validate: func(t *testing.T, result *IndexerMessage, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Headers should be empty when no context values
				assert.Empty(t, result.Headers)

				dataMap, ok := result.Data.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "no-context-uid", dataMap["uid"])
			},
		},
		{
			name: "build with struct input",
			message: &IndexerMessage{
				Action: ActionCreated,
			},
			context: func() context.Context {
				return context.Background()
			},
			input: struct {
				UID  string `json:"uid"`
				Name string `json:"name"`
			}{
				UID:  "struct-uid",
				Name: "struct-name",
			},
			expectError: false,
			validate: func(t *testing.T, result *IndexerMessage, err error) {
				require.NoError(t, err)

				dataMap, ok := result.Data.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "struct-uid", dataMap["uid"])
				assert.Equal(t, "struct-name", dataMap["name"])
			},
		},
		{
			name: "build with unmarshalable input for create action",
			message: &IndexerMessage{
				Action: ActionCreated,
			},
			context: func() context.Context {
				return context.Background()
			},
			input: make(chan int), // channels cannot be marshaled to JSON
			expectError: true,
			validate: func(t *testing.T, result *IndexerMessage, err error) {
				require.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), "unsupported type")
			},
		},
		{
			name: "build with complex input that fails JSON unmarshal",
			message: &IndexerMessage{
				Action: ActionUpdated,
			},
			context: func() context.Context {
				return context.Background()
			},
			input: struct {
				InvalidJSON func() `json:"invalid"`
			}{
				InvalidJSON: func() {},
			},
			expectError: true,
			validate: func(t *testing.T, result *IndexerMessage, err error) {
				require.Error(t, err)
				assert.Nil(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.context()
			result, err := tt.message.Build(ctx, tt.input)
			tt.validate(t, result, err)
		})
	}
}

func TestIndexerMessage_BuildWithValidInput(t *testing.T) {
	// Test with a real domain model
	ml := &GrpsIOMailingList{
		UID:       "ml-123",
		GroupName: "test-group",
		Public:    true,
		Type:      TypeDiscussionOpen,
	}

	message := &IndexerMessage{
		Action: ActionCreated,
		Tags:   ml.Tags(),
	}

	ctx := context.WithValue(context.Background(), constants.PrincipalContextID, "test-user")
	result, err := message.Build(ctx, ml)

	require.NoError(t, err)
	assert.Equal(t, ActionCreated, result.Action)
	assert.Equal(t, ml.Tags(), result.Tags)
	assert.Equal(t, "test-user", result.Headers[constants.XOnBehalfOfHeader])

	// Verify the mailing list data was properly marshaled
	dataMap, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "ml-123", dataMap["uid"])
	assert.Equal(t, "test-group", dataMap["group_name"])
	assert.Equal(t, true, dataMap["public"])
	assert.Equal(t, "discussion_open", dataMap["type"])
}

func TestMessageAction_Constants(t *testing.T) {
	// Test that message action constants have expected values
	assert.Equal(t, MessageAction("created"), ActionCreated)
	assert.Equal(t, MessageAction("updated"), ActionUpdated)
	assert.Equal(t, MessageAction("deleted"), ActionDeleted)
}

func TestAccessMessage_Struct(t *testing.T) {
	// Test that AccessMessage struct can be properly marshaled/unmarshaled
	accessMsg := AccessMessage{
		UID:        "access-123",
		ObjectType: "groupsio_service",
		Public:     true,
		Relations:  map[string][]string{"admin": {"user123"}},
		References: map[string]string{"project": "project-456"},
	}

	// Test JSON marshaling
	data, err := json.Marshal(accessMsg)
	require.NoError(t, err)

	// Test JSON unmarshaling
	var unmarshaled AccessMessage
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, accessMsg.UID, unmarshaled.UID)
	assert.Equal(t, accessMsg.ObjectType, unmarshaled.ObjectType)
	assert.Equal(t, accessMsg.Public, unmarshaled.Public)
	assert.Equal(t, accessMsg.Relations, unmarshaled.Relations)
	assert.Equal(t, accessMsg.References, unmarshaled.References)
}

// Benchmark for Build method with realistic data
func BenchmarkIndexerMessage_Build(b *testing.B) {
	ml := createValidTestMailingList()
	message := &IndexerMessage{
		Action: ActionCreated,
		Tags:   ml.Tags(),
	}
	ctx := context.WithValue(context.Background(), constants.PrincipalContextID, "bench-user")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = message.Build(ctx, ml)
	}
}