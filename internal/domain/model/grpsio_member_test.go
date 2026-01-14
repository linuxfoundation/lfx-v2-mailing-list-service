// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGrpsIOMember_BuildIndexKey(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		member      *GrpsIOMember
		description string
	}{
		{
			name: "committee member with all fields",
			member: &GrpsIOMember{
				UID:            uuid.New().String(),
				MailingListUID: "mailing-list-123",
				MemberID:       func() *int64 { id := int64(12345); return &id }(),
				GroupID:        func() *int64 { id := int64(67890); return &id }(),
				Username:       "testuser",
				FirstName:      "John",
				LastName:       "Doe",
				Email:          "john.doe@example.com",
				Organization:   "Acme Corp",
				JobTitle:       "Software Engineer",
				MemberType:     "committee",
				DeliveryMode:   "individual",
				ModStatus:      "none",
				Status:         "normal",
			},
			description: "Committee member should use mailing_list_uid and email for key generation",
		},
		{
			name: "direct member with minimal fields",
			member: &GrpsIOMember{
				UID:            uuid.New().String(),
				MailingListUID: "mailing-list-456",
				FirstName:      "Jane",
				LastName:       "Smith",
				Email:          "jane.smith@test.org",
				MemberType:     "direct",
				Status:         "pending",
			},
			description: "Direct member should use mailing_list_uid and email for key generation",
		},
		{
			name: "member with mixed case email",
			member: &GrpsIOMember{
				UID:            uuid.New().String(),
				MailingListUID: "MAILING-LIST-789",
				FirstName:      "Bob",
				LastName:       "Wilson",
				Email:          "Bob.Wilson@EXAMPLE.COM",
				MemberType:     "committee",
				Status:         "normal",
			},
			description: "Email and mailing list UID should be normalized to lowercase",
		},
		{
			name: "member with whitespace in fields",
			member: &GrpsIOMember{
				UID:            uuid.New().String(),
				MailingListUID: "  mailing-list-whitespace  ",
				FirstName:      "Alice",
				LastName:       "Brown",
				Email:          "  alice.brown@whitespace.com  ",
				MemberType:     "direct",
				Status:         "normal",
			},
			description: "Whitespace should be trimmed from key generation fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := tt.member.BuildIndexKey(ctx)
			key2 := tt.member.BuildIndexKey(ctx)

			// Keys should be consistent for same input
			assert.Equal(t, key1, key2, "Index keys should be consistent for same input")

			// Keys should be valid SHA-256 hex strings (64 characters)
			assert.Len(t, key1, 64, "Index key should be 64 characters (SHA-256 hex)")

			// Keys should only contain hex characters
			for _, char := range key1 {
				assert.True(t, (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f'),
					"Index key should only contain hex characters")
			}
		})
	}

	// Test that different members produce different keys
	t.Run("different members produce different keys", func(t *testing.T) {
		member1 := &GrpsIOMember{
			MailingListUID: "mailing-list-1",
			Email:          "user1@example.com",
			FirstName:      "User",
			LastName:       "One",
		}
		member2 := &GrpsIOMember{
			MailingListUID: "mailing-list-1",
			Email:          "user2@example.com",
			FirstName:      "User",
			LastName:       "Two",
		}

		key1 := member1.BuildIndexKey(ctx)
		key2 := member2.BuildIndexKey(ctx)

		assert.NotEqual(t, key1, key2, "Different members should produce different keys")
	})

	// Test same email in different mailing lists
	t.Run("same email different mailing lists produce different keys", func(t *testing.T) {
		email := "shared@example.com"

		member1 := &GrpsIOMember{
			MailingListUID: "mailing-list-1",
			Email:          email,
			FirstName:      "Shared",
			LastName:       "User",
		}
		member2 := &GrpsIOMember{
			MailingListUID: "mailing-list-2",
			Email:          email,
			FirstName:      "Shared",
			LastName:       "User",
		}

		key1 := member1.BuildIndexKey(ctx)
		key2 := member2.BuildIndexKey(ctx)

		assert.NotEqual(t, key1, key2, "Same email in different mailing lists should have different keys")
	})

	// Test duplicate email in same mailing list produces same key
	t.Run("duplicate email same mailing list produces same key", func(t *testing.T) {
		mailingListUID := "mailing-list-duplicate"
		email := "duplicate@example.com"

		member1 := &GrpsIOMember{
			UID:            uuid.New().String(),
			MailingListUID: mailingListUID,
			Email:          email,
			FirstName:      "First",
			LastName:       "Member",
			MemberType:     "committee",
		}
		member2 := &GrpsIOMember{
			UID:            uuid.New().String(),
			MailingListUID: mailingListUID,
			Email:          email,
			FirstName:      "Second",
			LastName:       "Member",
			MemberType:     "direct",
		}

		key1 := member1.BuildIndexKey(ctx)
		key2 := member2.BuildIndexKey(ctx)

		assert.Equal(t, key1, key2, "Same email in same mailing list should produce same key for uniqueness enforcement")
	})
}

func TestGrpsIOMember_Tags(t *testing.T) {
	tests := []struct {
		name         string
		member       *GrpsIOMember
		expectedTags []string
	}{
		{
			name:         "nil member",
			member:       nil,
			expectedTags: nil,
		},
		{
			name: "complete member",
			member: &GrpsIOMember{
				UID:            "member-123",
				MailingListUID: "mailing-list-456",
				Username:       "testuser",
				Email:          "test@example.com",
				Status:         "normal",
			},
			expectedTags: []string{
				"member-123",
				"member_uid:member-123",
				"mailing_list_uid:mailing-list-456",
				"username:testuser",
				"email:test@example.com",
				"status:normal",
			},
		},
		{
			name: "minimal member - empty fields",
			member: &GrpsIOMember{
				UID:            "",
				MailingListUID: "",
				Username:       "",
				Email:          "",
				Status:         "",
			},
			expectedTags: nil,
		},
		{
			name: "member with partial fields",
			member: &GrpsIOMember{
				UID:            "member-789",
				MailingListUID: "mailing-list-999",
				Email:          "partial@example.com",
			},
			expectedTags: []string{
				"member-789",
				"member_uid:member-789",
				"mailing_list_uid:mailing-list-999",
				"email:partial@example.com",
			},
		},
		{
			name: "member with only username",
			member: &GrpsIOMember{
				Username: "onlyusername",
			},
			expectedTags: []string{
				"username:onlyusername",
			},
		},
		{
			name: "member with only status",
			member: &GrpsIOMember{
				Status: "pending",
			},
			expectedTags: []string{
				"status:pending",
			},
		},
		{
			name: "member with all tag-generating fields",
			member: &GrpsIOMember{
				UID:            "comprehensive-member",
				MailingListUID: "comprehensive-list",
				Username:       "comprehensive-user",
				Email:          "comprehensive@example.com",
				Status:         "normal",
			},
			expectedTags: []string{
				"comprehensive-member",
				"member_uid:comprehensive-member",
				"mailing_list_uid:comprehensive-list",
				"username:comprehensive-user",
				"email:comprehensive@example.com",
				"status:normal",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := tt.member.Tags()
			assert.Equal(t, tt.expectedTags, tags)
		})
	}
}

// Test edge cases and boundary conditions
func TestGrpsIOMember_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("empty member", func(t *testing.T) {
		member := &GrpsIOMember{}

		// BuildIndexKey should still work with empty fields
		key := member.BuildIndexKey(ctx)
		assert.Len(t, key, 64, "Should produce valid key even with empty fields")

		// Tags should return empty slice
		tags := member.Tags()
		assert.Nil(t, tags, "Should return nil for member with no fields")
	})

	t.Run("member with special characters", func(t *testing.T) {
		member := &GrpsIOMember{
			UID:            "member-with-special-chars-@#$%",
			MailingListUID: "mailing-list-with-spaces and symbols!",
			Username:       "user-with-unicode-🚀",
			Email:          "special+chars@example-domain.com",
			Status:         "normal-status",
		}

		// BuildIndexKey should handle special characters
		key := member.BuildIndexKey(ctx)
		assert.Len(t, key, 64, "Should handle special characters in hash")

		// Tags should include special characters as-is
		tags := member.Tags()
		assert.Contains(t, tags, "username:user-with-unicode-🚀")
		assert.Contains(t, tags, "email:special+chars@example-domain.com")
		assert.Contains(t, tags, "member_uid:member-with-special-chars-@#$%")
	})

	t.Run("member with very long fields", func(t *testing.T) {
		longString := ""
		for i := 0; i < 1000; i++ {
			longString += "a"
		}

		member := &GrpsIOMember{
			MailingListUID: longString,
			Email:          "test@" + longString + ".com",
		}

		// BuildIndexKey should handle long strings (hash is fixed length)
		key := member.BuildIndexKey(ctx)
		assert.Len(t, key, 64, "Should produce fixed-length key regardless of input length")
	})

	t.Run("member with normalized email", func(t *testing.T) {
		member1 := &GrpsIOMember{
			MailingListUID: "TEST-LIST",
			Email:          "USER@EXAMPLE.COM",
		}
		member2 := &GrpsIOMember{
			MailingListUID: "test-list",
			Email:          "user@example.com",
		}

		key1 := member1.BuildIndexKey(ctx)
		key2 := member2.BuildIndexKey(ctx)

		assert.Equal(t, key1, key2, "Normalized emails and mailing list UIDs should produce same key")
	})
}

// Test member creation and validation scenarios
func TestGrpsIOMember_ValidationScenarios(t *testing.T) {
	now := time.Now()

	t.Run("complete committee member", func(t *testing.T) {
		member := &GrpsIOMember{
			UID:            uuid.New().String(),
			MailingListUID: "committee-mailing-list",
			MemberID:       int64Ptr(12345),
			GroupID:        int64Ptr(67890),
			Username:       "committee-member",
			FirstName:      "Committee",
			LastName:       "Member",
			Email:          "committee.member@example.com",
			Organization:   "Example Organization",
			JobTitle:       "Committee Chair",
			MemberType:     "committee",
			DeliveryMode:   "individual",
			ModStatus:      "moderator",
			Status:         "normal",
			LastReviewedAt: stringPtr("2024-01-01T00:00:00Z"),
			LastReviewedBy: stringPtr("reviewer-uid"),
			CreatedAt:      now.Add(-24 * time.Hour),
			UpdatedAt:      now,
		}

		// Verify member can generate index key
		key := member.BuildIndexKey(context.Background())
		assert.Len(t, key, 64, "Committee member should generate valid index key")

		// Verify member can generate tags
		tags := member.Tags()
		assert.Greater(t, len(tags), 0, "Committee member should generate tags")
		assert.Contains(t, tags, "member_uid:"+member.UID)
		assert.Contains(t, tags, "email:committee.member@example.com")
		assert.Contains(t, tags, "status:normal")
		assert.Contains(t, tags, "username:committee-member")
	})

	t.Run("minimal direct member", func(t *testing.T) {
		member := &GrpsIOMember{
			UID:            uuid.New().String(),
			MailingListUID: "direct-mailing-list",
			FirstName:      "Direct",
			LastName:       "Member",
			Email:          "direct.member@example.com",
			MemberType:     "direct",
			Status:         "pending",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		// Verify member can generate index key
		key := member.BuildIndexKey(context.Background())
		assert.Len(t, key, 64, "Direct member should generate valid index key")

		// Verify member can generate tags
		tags := member.Tags()
		assert.Greater(t, len(tags), 0, "Direct member should generate tags")
		assert.Contains(t, tags, "status:pending")
		assert.Contains(t, tags, "email:direct.member@example.com")
	})
}

// Benchmark tests for performance-critical functions
func BenchmarkGrpsIOMember_BuildIndexKey(b *testing.B) {
	ctx := context.Background()

	tests := []struct {
		name   string
		member *GrpsIOMember
	}{
		{
			name: "committee member",
			member: &GrpsIOMember{
				MailingListUID: "mailing-list-" + uuid.New().String(),
				Email:          "committee@example.com",
				MemberType:     "committee",
			},
		},
		{
			name: "direct member",
			member: &GrpsIOMember{
				MailingListUID: "mailing-list-" + uuid.New().String(),
				Email:          "direct@example.com",
				MemberType:     "direct",
			},
		},
		{
			name: "member with long email",
			member: &GrpsIOMember{
				MailingListUID: "mailing-list-" + uuid.New().String(),
				Email:          "very-long-email-address-for-benchmarking-purposes@very-long-domain-name-for-testing.example.com",
				MemberType:     "committee",
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = tt.member.BuildIndexKey(ctx)
			}
		})
	}
}

func BenchmarkGrpsIOMember_Tags(b *testing.B) {
	member := &GrpsIOMember{
		UID:            "member-" + uuid.New().String(),
		MailingListUID: "mailing-list-" + uuid.New().String(),
		Username:       "benchmark-member",
		Email:          "benchmark@example.com",
		Status:         "normal",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = member.Tags()
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
