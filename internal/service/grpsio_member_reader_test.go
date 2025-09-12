// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

func TestGrpsIOMemberReaderOrchestrator_GetGrpsIOMember(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()

	// Setup test data
	testMemberUID := uuid.New().String()
	testMailingListUID := uuid.New().String()
	testMember := &model.GrpsIOMember{
		UID:              testMemberUID,
		MailingListUID:   testMailingListUID,
		GroupsIOMemberID: 12345,
		GroupsIOGroupID:  67890,
		Username:         "testuser",
		FirstName:        "John",
		LastName:         "Doe",
		Email:            "john.doe@example.com",
		Organization:     "Acme Corp",
		JobTitle:         "Software Engineer",
		MemberType:       "committee",
		DeliveryMode:     "individual",
		ModStatus:        "none",
		Status:           "normal",
		LastReviewedAt:   memberStringPtr("2024-01-01T00:00:00Z"),
		LastReviewedBy:   memberStringPtr("reviewer-uid"),
		Writers:          []string{"writer1", "writer2"},
		Auditors:         []string{"auditor1"},
		CreatedAt:        time.Now().Add(-24 * time.Hour),
		UpdatedAt:        time.Now(),
	}

	tests := []struct {
		name           string
		setupMock      func()
		memberUID      string
		expectedError  bool
		errorType      error
		validateMember func(*testing.T, *model.GrpsIOMember, uint64)
	}{
		{
			name: "successful member retrieval",
			setupMock: func() {
				mockRepo.ClearAll()
				// Store the member in mock repository
				mockRepo.AddMember(testMember)
			},
			memberUID:     testMemberUID,
			expectedError: false,
			validateMember: func(t *testing.T, member *model.GrpsIOMember, revision uint64) {
				assert.NotNil(t, member)
				assert.Equal(t, testMemberUID, member.UID)
				assert.Equal(t, testMailingListUID, member.MailingListUID)
				assert.Equal(t, int64(12345), member.GroupsIOMemberID)
				assert.Equal(t, int64(67890), member.GroupsIOGroupID)
				assert.Equal(t, "testuser", member.Username)
				assert.Equal(t, "John", member.FirstName)
				assert.Equal(t, "Doe", member.LastName)
				assert.Equal(t, "john.doe@example.com", member.Email)
				assert.Equal(t, "Acme Corp", member.Organization)
				assert.Equal(t, "Software Engineer", member.JobTitle)
				assert.Equal(t, "committee", member.MemberType)
				assert.Equal(t, "individual", member.DeliveryMode)
				assert.Equal(t, "none", member.ModStatus)
				assert.Equal(t, "normal", member.Status)
				assert.NotNil(t, member.LastReviewedAt)
				assert.Equal(t, "2024-01-01T00:00:00Z", *member.LastReviewedAt)
				assert.NotNil(t, member.LastReviewedBy)
				assert.Equal(t, "reviewer-uid", *member.LastReviewedBy)
				assert.Equal(t, []string{"writer1", "writer2"}, member.Writers)
				assert.Equal(t, []string{"auditor1"}, member.Auditors)
				assert.NotZero(t, member.CreatedAt)
				assert.NotZero(t, member.UpdatedAt)
				assert.Equal(t, uint64(1), revision) // Mock returns revision 1
			},
		},
		{
			name: "member not found error",
			setupMock: func() {
				mockRepo.ClearAll()
				// Don't store any member
			},
			memberUID:     "nonexistent-member-uid",
			expectedError: true,
			errorType:     errs.NotFound{},
			validateMember: func(t *testing.T, member *model.GrpsIOMember, revision uint64) {
				assert.Nil(t, member)
				assert.Equal(t, uint64(0), revision)
			},
		},
		{
			name: "empty member UID",
			setupMock: func() {
				mockRepo.ClearAll()
			},
			memberUID:     "",
			expectedError: true,
			errorType:     errs.NotFound{},
			validateMember: func(t *testing.T, member *model.GrpsIOMember, revision uint64) {
				assert.Nil(t, member)
				assert.Equal(t, uint64(0), revision)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tt.setupMock()

			// Create reader orchestrator
			reader := NewGrpsIOReaderOrchestrator(
				WithGrpsIOReader(mock.NewMockGrpsIOReader(mockRepo)),
			)

			// Execute
			member, revision, err := reader.GetGrpsIOMember(ctx, tt.memberUID)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorType != nil {
					assert.IsType(t, tt.errorType, err)
				}
			} else {
				require.NoError(t, err)
			}

			tt.validateMember(t, member, revision)
		})
	}
}

func TestGrpsIOMemberReaderOrchestrator_CheckMemberExists(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()

	// Setup test data
	testMailingListUID := uuid.New().String()
	existingEmail := "existing@example.com"
	existingMember := &model.GrpsIOMember{
		UID:            uuid.New().String(),
		MailingListUID: testMailingListUID,
		FirstName:      "Existing",
		LastName:       "Member",
		Email:          existingEmail,
		MemberType:     "committee",
		Status:         "normal",
		CreatedAt:      time.Now().Add(-24 * time.Hour),
		UpdatedAt:      time.Now(),
	}

	tests := []struct {
		name             string
		setupMock        func()
		mailingListUID   string
		email            string
		expectedExists   bool
		expectedError    bool
		validateResponse func(*testing.T, bool)
	}{
		{
			name: "member exists in mailing list",
			setupMock: func() {
				mockRepo.ClearAll()
				mockRepo.AddMember(existingMember)
			},
			mailingListUID: testMailingListUID,
			email:          existingEmail,
			expectedExists: true,
			expectedError:  false,
			validateResponse: func(t *testing.T, exists bool) {
				assert.True(t, exists, "Should return true when member exists")
			},
		},
		{
			name: "member does not exist in mailing list",
			setupMock: func() {
				mockRepo.ClearAll()
				mockRepo.AddMember(existingMember)
			},
			mailingListUID: testMailingListUID,
			email:          "nonexistent@example.com",
			expectedExists: false,
			expectedError:  false,
			validateResponse: func(t *testing.T, exists bool) {
				assert.False(t, exists, "Should return false when member does not exist")
			},
		},
		{
			name: "email exists but in different mailing list",
			setupMock: func() {
				mockRepo.ClearAll()
				mockRepo.AddMember(existingMember)
			},
			mailingListUID: "different-mailing-list-uid",
			email:          existingEmail,
			expectedExists: false,
			expectedError:  false,
			validateResponse: func(t *testing.T, exists bool) {
				assert.False(t, exists, "Should return false when email exists in different mailing list")
			},
		},
		{
			name: "empty mailing list UID",
			setupMock: func() {
				mockRepo.ClearAll()
			},
			mailingListUID: "",
			email:          "test@example.com",
			expectedExists: false,
			expectedError:  false,
			validateResponse: func(t *testing.T, exists bool) {
				assert.False(t, exists, "Should return false for empty mailing list UID")
			},
		},
		{
			name: "empty email",
			setupMock: func() {
				mockRepo.ClearAll()
			},
			mailingListUID: testMailingListUID,
			email:          "",
			expectedExists: false,
			expectedError:  false,
			validateResponse: func(t *testing.T, exists bool) {
				assert.False(t, exists, "Should return false for empty email")
			},
		},
		{
			name: "case insensitive email check",
			setupMock: func() {
				mockRepo.ClearAll()
				mockRepo.AddMember(existingMember)
			},
			mailingListUID: testMailingListUID,
			email:          "EXISTING@EXAMPLE.COM",
			expectedExists: true,
			expectedError:  false,
			validateResponse: func(t *testing.T, exists bool) {
				assert.True(t, exists, "Should return true for case insensitive email match")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			tt.setupMock()

			// Create reader orchestrator
			reader := NewGrpsIOReaderOrchestrator(
				WithGrpsIOReader(mock.NewMockGrpsIOReader(mockRepo)),
			)

			// Execute
			exists, err := reader.CheckMemberExists(ctx, tt.mailingListUID, tt.email)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.expectedExists, exists, "Existence check should match expected result")
			tt.validateResponse(t, exists)
		})
	}
}

func TestGrpsIOMemberReaderOrchestratorIntegration(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()
	mockRepo.ClearAll()

	// Setup test data
	testMemberUID := uuid.New().String()
	testMailingListUID := uuid.New().String()
	testMember := &model.GrpsIOMember{
		UID:              testMemberUID,
		MailingListUID:   testMailingListUID,
		GroupsIOMemberID: 54321,
		GroupsIOGroupID:  98765,
		Username:         "integrationuser",
		FirstName:        "Integration",
		LastName:         "Test",
		Email:            "integration.test@example.com",
		Organization:     "Integration Test Corp",
		JobTitle:         "Test Manager",
		MemberType:       "direct",
		DeliveryMode:     "digest",
		ModStatus:        "moderator",
		Status:           "pending",
		LastReviewedAt:   memberStringPtr("2024-02-01T12:00:00Z"),
		LastReviewedBy:   memberStringPtr("integration-reviewer"),
		Writers:          []string{"integration-writer1", "integration-writer2", "integration-writer3"},
		Auditors:         []string{"integration-auditor1", "integration-auditor2"},
		CreatedAt:        time.Now().Add(-48 * time.Hour),
		UpdatedAt:        time.Now().Add(-1 * time.Hour),
	}

	// Store the member
	mockRepo.AddMember(testMember)

	// Create reader orchestrator
	reader := NewGrpsIOReaderOrchestrator(
		WithGrpsIOReader(mock.NewMockGrpsIOReader(mockRepo)),
	)

	t.Run("get member and check existence for same member", func(t *testing.T) {
		// Get member
		member, revision, err := reader.GetGrpsIOMember(ctx, testMemberUID)
		require.NoError(t, err)
		require.NotNil(t, member)

		// Check if member exists
		exists, err := reader.CheckMemberExists(ctx, testMailingListUID, "integration.test@example.com")
		require.NoError(t, err)

		// Validate that both operations return consistent data
		assert.Equal(t, testMemberUID, member.UID)
		assert.True(t, exists, "Member should exist in mailing list")

		// Validate complete member data
		assert.Equal(t, testMailingListUID, member.MailingListUID)
		assert.Equal(t, int64(54321), member.GroupsIOMemberID)
		assert.Equal(t, int64(98765), member.GroupsIOGroupID)
		assert.Equal(t, "integrationuser", member.Username)
		assert.Equal(t, "Integration", member.FirstName)
		assert.Equal(t, "Test", member.LastName)
		assert.Equal(t, "integration.test@example.com", member.Email)
		assert.Equal(t, "Integration Test Corp", member.Organization)
		assert.Equal(t, "Test Manager", member.JobTitle)
		assert.Equal(t, "direct", member.MemberType)
		assert.Equal(t, "digest", member.DeliveryMode)
		assert.Equal(t, "moderator", member.ModStatus)
		assert.Equal(t, "pending", member.Status)

		// Validate audit fields
		assert.NotNil(t, member.LastReviewedAt)
		assert.Equal(t, "2024-02-01T12:00:00Z", *member.LastReviewedAt)
		assert.NotNil(t, member.LastReviewedBy)
		assert.Equal(t, "integration-reviewer", *member.LastReviewedBy)
		assert.Equal(t, []string{"integration-writer1", "integration-writer2", "integration-writer3"}, member.Writers)
		assert.Equal(t, []string{"integration-auditor1", "integration-auditor2"}, member.Auditors)

		// Validate timestamps
		assert.False(t, member.CreatedAt.IsZero())
		assert.False(t, member.UpdatedAt.IsZero())

		// Validate revision from get operation
		assert.Equal(t, uint64(1), revision) // Mock returns revision 1
	})

	t.Run("check existence for different emails", func(t *testing.T) {
		// Check existing email
		exists, err := reader.CheckMemberExists(ctx, testMailingListUID, "integration.test@example.com")
		require.NoError(t, err)
		assert.True(t, exists, "Existing email should be found")

		// Check non-existing email
		exists, err = reader.CheckMemberExists(ctx, testMailingListUID, "nonexistent@example.com")
		require.NoError(t, err)
		assert.False(t, exists, "Non-existing email should not be found")

		// Check email in different mailing list
		exists, err = reader.CheckMemberExists(ctx, "different-mailing-list", "integration.test@example.com")
		require.NoError(t, err)
		assert.False(t, exists, "Email should not be found in different mailing list")
	})

	t.Run("case insensitive email checking", func(t *testing.T) {
		// Test uppercase email
		exists, err := reader.CheckMemberExists(ctx, testMailingListUID, "INTEGRATION.TEST@EXAMPLE.COM")
		require.NoError(t, err)
		assert.True(t, exists, "Uppercase email should match")

		// Test mixed case email
		exists, err = reader.CheckMemberExists(ctx, testMailingListUID, "Integration.Test@Example.Com")
		require.NoError(t, err)
		assert.True(t, exists, "Mixed case email should match")
	})
}

// Test with multiple members in different mailing lists
func TestGrpsIOMemberReaderOrchestrator_MultipleMembers(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()
	mockRepo.ClearAll()

	// Setup test data with multiple members
	mailingList1 := uuid.New().String()
	mailingList2 := uuid.New().String()
	sharedEmail := "shared@example.com"

	member1 := &model.GrpsIOMember{
		UID:            uuid.New().String(),
		MailingListUID: mailingList1,
		FirstName:      "Member",
		LastName:       "One",
		Email:          sharedEmail,
		MemberType:     "committee",
		Status:         "normal",
		CreatedAt:      time.Now().Add(-24 * time.Hour),
		UpdatedAt:      time.Now(),
	}

	member2 := &model.GrpsIOMember{
		UID:            uuid.New().String(),
		MailingListUID: mailingList2,
		FirstName:      "Member",
		LastName:       "Two",
		Email:          sharedEmail,
		MemberType:     "direct",
		Status:         "pending",
		CreatedAt:      time.Now().Add(-12 * time.Hour),
		UpdatedAt:      time.Now(),
	}

	member3 := &model.GrpsIOMember{
		UID:            uuid.New().String(),
		MailingListUID: mailingList1,
		FirstName:      "Member",
		LastName:       "Three",
		Email:          "unique@example.com",
		MemberType:     "committee",
		Status:         "normal",
		CreatedAt:      time.Now().Add(-6 * time.Hour),
		UpdatedAt:      time.Now(),
	}

	// Store members
	mockRepo.AddMember(member1)
	mockRepo.AddMember(member2)
	mockRepo.AddMember(member3)

	// Create reader orchestrator
	reader := NewGrpsIOReaderOrchestrator(
		WithGrpsIOReader(mock.NewMockGrpsIOReader(mockRepo)),
	)

	t.Run("shared email exists in both mailing lists", func(t *testing.T) {
		// Check shared email in mailing list 1
		exists1, err := reader.CheckMemberExists(ctx, mailingList1, sharedEmail)
		require.NoError(t, err)
		assert.True(t, exists1, "Shared email should exist in mailing list 1")

		// Check shared email in mailing list 2
		exists2, err := reader.CheckMemberExists(ctx, mailingList2, sharedEmail)
		require.NoError(t, err)
		assert.True(t, exists2, "Shared email should exist in mailing list 2")
	})

	t.Run("unique email exists in only one mailing list", func(t *testing.T) {
		// Check unique email in mailing list 1 (should exist)
		exists1, err := reader.CheckMemberExists(ctx, mailingList1, "unique@example.com")
		require.NoError(t, err)
		assert.True(t, exists1, "Unique email should exist in mailing list 1")

		// Check unique email in mailing list 2 (should not exist)
		exists2, err := reader.CheckMemberExists(ctx, mailingList2, "unique@example.com")
		require.NoError(t, err)
		assert.False(t, exists2, "Unique email should not exist in mailing list 2")
	})

	t.Run("can retrieve all members individually", func(t *testing.T) {
		// Get member 1
		retrievedMember1, revision1, err := reader.GetGrpsIOMember(ctx, member1.UID)
		require.NoError(t, err)
		assert.Equal(t, member1.UID, retrievedMember1.UID)
		assert.Equal(t, mailingList1, retrievedMember1.MailingListUID)
		assert.Equal(t, uint64(1), revision1)

		// Get member 2
		retrievedMember2, revision2, err := reader.GetGrpsIOMember(ctx, member2.UID)
		require.NoError(t, err)
		assert.Equal(t, member2.UID, retrievedMember2.UID)
		assert.Equal(t, mailingList2, retrievedMember2.MailingListUID)
		assert.Equal(t, uint64(1), revision2)

		// Get member 3
		retrievedMember3, revision3, err := reader.GetGrpsIOMember(ctx, member3.UID)
		require.NoError(t, err)
		assert.Equal(t, member3.UID, retrievedMember3.UID)
		assert.Equal(t, mailingList1, retrievedMember3.MailingListUID)
		assert.Equal(t, uint64(1), revision3)
	})
}

// Helper function to create string pointer
func memberStringPtr(s string) *string {
	return &s
}
