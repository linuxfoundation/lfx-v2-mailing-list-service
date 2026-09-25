// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGrpsIOWebhookEvent_JSONMarshaling tests JSON marshaling/unmarshaling
func TestGrpsIOWebhookEvent_JSONMarshaling(t *testing.T) {
	t.Run("marshal complete webhook event", func(t *testing.T) {
		now := time.Now().UTC()
		event := &GrpsIOWebhookEvent{
			ID:     123,
			Action: "created_subgroup",
			Group: &GroupInfo{
				ID:            456,
				Name:          "test-group",
				ParentGroupID: 789,
			},
			MemberInfo: &MemberInfo{
				ID:        1,
				UserID:    2,
				GroupID:   123,
				GroupName: "test-group",
				Email:     "test@example.com",
				Status:    "approved",
			},
			Extra:      "developers",
			ExtraID:    999,
			ReceivedAt: now,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		// Unmarshal and verify
		var unmarshaled GrpsIOWebhookEvent
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, event.ID, unmarshaled.ID)
		assert.Equal(t, event.Action, unmarshaled.Action)
		assert.Equal(t, event.Extra, unmarshaled.Extra)
		assert.Equal(t, event.ExtraID, unmarshaled.ExtraID)
		assert.Equal(t, event.Group.ID, unmarshaled.Group.ID)
		assert.Equal(t, event.MemberInfo.Email, unmarshaled.MemberInfo.Email)
	})

	t.Run("unmarshal webhook event with only group info", func(t *testing.T) {
		jsonData := `{
			"id": 123,
			"action": "created_subgroup",
			"group": {
				"id": 456,
				"name": "test-group",
				"parent_group_id": 789
			},
			"extra": "developers"
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Equal(t, 123, event.ID)
		assert.Equal(t, "created_subgroup", event.Action)
		assert.NotNil(t, event.Group)
		assert.Equal(t, 456, event.Group.ID)
		assert.Equal(t, "test-group", event.Group.Name)
		assert.Equal(t, 789, event.Group.ParentGroupID)
		assert.Equal(t, "developers", event.Extra)
		assert.Nil(t, event.MemberInfo)
	})

	t.Run("unmarshal webhook event with only member info", func(t *testing.T) {
		jsonData := `{
			"id": 123,
			"action": "added_member",
			"member_info": {
				"id": 1,
				"user_id": 2,
				"group_id": 456,
				"group_name": "test-group",
				"email": "test@example.com",
				"status": "approved"
			}
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Equal(t, 123, event.ID)
		assert.Equal(t, "added_member", event.Action)
		assert.Nil(t, event.Group)
		assert.NotNil(t, event.MemberInfo)
		assert.Equal(t, 1, event.MemberInfo.ID)
		assert.Equal(t, 2, event.MemberInfo.UserID)
		assert.Equal(t, uint64(456), event.MemberInfo.GroupID)
		assert.Equal(t, "test-group", event.MemberInfo.GroupName)
		assert.Equal(t, "test@example.com", event.MemberInfo.Email)
		assert.Equal(t, "approved", event.MemberInfo.Status)
	})

	t.Run("unmarshal minimal webhook event", func(t *testing.T) {
		jsonData := `{
			"action": "unknown_event"
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Equal(t, 0, event.ID)
		assert.Equal(t, "unknown_event", event.Action)
		assert.Nil(t, event.Group)
		assert.Nil(t, event.MemberInfo)
		assert.Empty(t, event.Extra)
		assert.Equal(t, 0, event.ExtraID)
	})

	t.Run("unmarshal deleted_subgroup event", func(t *testing.T) {
		jsonData := `{
			"action": "deleted_subgroup",
			"extra_id": 999
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Equal(t, "deleted_subgroup", event.Action)
		assert.Equal(t, 999, event.ExtraID)
		assert.Nil(t, event.Group)
		assert.Nil(t, event.MemberInfo)
	})
}

// TestGroupInfo_JSONMarshaling tests GroupInfo JSON operations
func TestGroupInfo_JSONMarshaling(t *testing.T) {
	t.Run("marshal group info", func(t *testing.T) {
		group := &GroupInfo{
			ID:            123,
			Name:          "test-group",
			ParentGroupID: 456,
		}

		data, err := json.Marshal(group)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		var unmarshaled GroupInfo
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, group.ID, unmarshaled.ID)
		assert.Equal(t, group.Name, unmarshaled.Name)
		assert.Equal(t, group.ParentGroupID, unmarshaled.ParentGroupID)
	})

	t.Run("unmarshal group info with missing fields", func(t *testing.T) {
		jsonData := `{"name": "test-group"}`

		var group GroupInfo
		err := json.Unmarshal([]byte(jsonData), &group)
		require.NoError(t, err)

		assert.Equal(t, 0, group.ID)
		assert.Equal(t, "test-group", group.Name)
		assert.Equal(t, 0, group.ParentGroupID)
	})

	t.Run("unmarshal empty group info", func(t *testing.T) {
		jsonData := `{}`

		var group GroupInfo
		err := json.Unmarshal([]byte(jsonData), &group)
		require.NoError(t, err)

		assert.Equal(t, 0, group.ID)
		assert.Empty(t, group.Name)
		assert.Equal(t, 0, group.ParentGroupID)
	})
}

// TestMemberInfo_JSONMarshaling tests MemberInfo JSON operations
func TestMemberInfo_JSONMarshaling(t *testing.T) {
	t.Run("marshal member info", func(t *testing.T) {
		member := &MemberInfo{
			ID:        1,
			UserID:    2,
			GroupID:   123,
			GroupName: "test-group",
			Email:     "test@example.com",
			Status:    "approved",
		}

		data, err := json.Marshal(member)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		var unmarshaled MemberInfo
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, member.ID, unmarshaled.ID)
		assert.Equal(t, member.UserID, unmarshaled.UserID)
		assert.Equal(t, member.GroupID, unmarshaled.GroupID)
		assert.Equal(t, member.GroupName, unmarshaled.GroupName)
		assert.Equal(t, member.Email, unmarshaled.Email)
		assert.Equal(t, member.Status, unmarshaled.Status)
	})

	t.Run("unmarshal member info with partial data", func(t *testing.T) {
		jsonData := `{
			"email": "test@example.com",
			"status": "approved"
		}`

		var member MemberInfo
		err := json.Unmarshal([]byte(jsonData), &member)
		require.NoError(t, err)

		assert.Equal(t, 0, member.ID)
		assert.Equal(t, 0, member.UserID)
		assert.Equal(t, uint64(0), member.GroupID)
		assert.Empty(t, member.GroupName)
		assert.Equal(t, "test@example.com", member.Email)
		assert.Equal(t, "approved", member.Status)
	})

	t.Run("unmarshal empty member info", func(t *testing.T) {
		jsonData := `{}`

		var member MemberInfo
		err := json.Unmarshal([]byte(jsonData), &member)
		require.NoError(t, err)

		assert.Equal(t, 0, member.ID)
		assert.Equal(t, 0, member.UserID)
		assert.Equal(t, uint64(0), member.GroupID)
		assert.Empty(t, member.GroupName)
		assert.Empty(t, member.Email)
		assert.Empty(t, member.Status)
	})

	t.Run("unmarshal member info with large group_id", func(t *testing.T) {
		jsonData := `{
			"group_id": 18446744073709551615
		}`

		var member MemberInfo
		err := json.Unmarshal([]byte(jsonData), &member)
		require.NoError(t, err)

		assert.Equal(t, uint64(18446744073709551615), member.GroupID)
	})
}

// TestGrpsIOWebhookEvent_RealWorldPayloads tests real-world GroupsIO webhook payloads
func TestGrpsIOWebhookEvent_RealWorldPayloads(t *testing.T) {
	t.Run("real created_subgroup payload", func(t *testing.T) {
		// Example from GroupsIO documentation
		jsonData := `{
			"id": 12345,
			"action": "created_subgroup",
			"group": {
				"id": 67890,
				"name": "myproject",
				"parent_group_id": 11111
			},
			"extra": "developers"
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Equal(t, 12345, event.ID)
		assert.Equal(t, "created_subgroup", event.Action)
		assert.NotNil(t, event.Group)
		assert.Equal(t, 67890, event.Group.ID)
		assert.Equal(t, "myproject", event.Group.Name)
		assert.Equal(t, 11111, event.Group.ParentGroupID)
		assert.Equal(t, "developers", event.Extra)
	})

	t.Run("real deleted_subgroup payload", func(t *testing.T) {
		jsonData := `{
			"id": 12346,
			"action": "deleted_subgroup",
			"extra_id": 67890
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Equal(t, 12346, event.ID)
		assert.Equal(t, "deleted_subgroup", event.Action)
		assert.Equal(t, 67890, event.ExtraID)
	})

	t.Run("real added_member payload", func(t *testing.T) {
		jsonData := `{
			"id": 12347,
			"action": "added_member",
			"member_info": {
				"id": 55555,
				"user_id": 66666,
				"group_id": 67890,
				"group_name": "myproject+developers",
				"email": "developer@example.com",
				"status": "approved"
			}
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Equal(t, 12347, event.ID)
		assert.Equal(t, "added_member", event.Action)
		assert.NotNil(t, event.MemberInfo)
		assert.Equal(t, 55555, event.MemberInfo.ID)
		assert.Equal(t, 66666, event.MemberInfo.UserID)
		assert.Equal(t, uint64(67890), event.MemberInfo.GroupID)
		assert.Equal(t, "myproject+developers", event.MemberInfo.GroupName)
		assert.Equal(t, "developer@example.com", event.MemberInfo.Email)
		assert.Equal(t, "approved", event.MemberInfo.Status)
	})

	t.Run("real ban_members payload", func(t *testing.T) {
		jsonData := `{
			"id": 12348,
			"action": "ban_members",
			"member_info": {
				"id": 77777,
				"user_id": 88888,
				"group_id": 67890,
				"group_name": "myproject+developers",
				"email": "banned@example.com",
				"status": "banned"
			}
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Equal(t, 12348, event.ID)
		assert.Equal(t, "ban_members", event.Action)
		assert.NotNil(t, event.MemberInfo)
		assert.Equal(t, "banned", event.MemberInfo.Status)
	})
}

// TestGrpsIOWebhookEvent_EdgeCases tests edge cases
func TestGrpsIOWebhookEvent_EdgeCases(t *testing.T) {
	t.Run("event with null group", func(t *testing.T) {
		jsonData := `{
			"action": "created_subgroup",
			"group": null
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Nil(t, event.Group)
	})

	t.Run("event with null member_info", func(t *testing.T) {
		jsonData := `{
			"action": "added_member",
			"member_info": null
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Nil(t, event.MemberInfo)
	})

	t.Run("event with negative IDs", func(t *testing.T) {
		jsonData := `{
			"id": -1,
			"action": "test",
			"extra_id": -999
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Equal(t, -1, event.ID)
		assert.Equal(t, -999, event.ExtraID)
	})

	t.Run("event with special characters in strings", func(t *testing.T) {
		jsonData := `{
			"action": "created_subgroup",
			"group": {
				"name": "test-group+special_chars@123"
			},
			"extra": "sub+group_name-123"
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Equal(t, "test-group+special_chars@123", event.Group.Name)
		assert.Equal(t, "sub+group_name-123", event.Extra)
	})

	t.Run("event with unicode characters", func(t *testing.T) {
		jsonData := `{
			"action": "created_subgroup",
			"group": {
				"name": "测试组"
			},
			"extra": "开发者"
		}`

		var event GrpsIOWebhookEvent
		err := json.Unmarshal([]byte(jsonData), &event)
		require.NoError(t, err)

		assert.Equal(t, "测试组", event.Group.Name)
		assert.Equal(t, "开发者", event.Extra)
	})
}
