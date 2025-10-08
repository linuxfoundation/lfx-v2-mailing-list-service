// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/groupsio"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

func TestGrpsIOWriterOrchestrator_CreateGrpsIOMember(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*mock.MockRepository)
		inputMember   *model.GrpsIOMember
		expectedError error
		validate      func(t *testing.T, result *model.GrpsIOMember, revision uint64, mockRepo *mock.MockRepository)
	}{
		{
			name: "successful committee member creation",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				// Add a mailing list for the member to belong to
				testMailingList := &model.GrpsIOMailingList{
					UID:         "mailing-list-1",
					ServiceUID:  "service-1",
					Title:       "Test Committee List",
					Description: "Test committee mailing list",
					Type:        "discussion_open",
					CreatedAt:   time.Now().Add(-24 * time.Hour),
					UpdatedAt:   time.Now(),
				}
				mockRepo.AddMailingList(testMailingList)
			},
			inputMember: &model.GrpsIOMember{
				MailingListUID:   "mailing-list-1",
				GroupsIOMemberID: writerInt64Ptr(12345),
				GroupsIOGroupID:  writerInt64Ptr(67890),
				Username:         "committee-member",
				FirstName:        "Committee",
				LastName:         "Member",
				Email:            "committee.member@example.com",
				Organization:     "Test Organization",
				JobTitle:         "Committee Chair",
				MemberType:       "committee",
				DeliveryMode:     "individual",
				ModStatus:        "none",
				Status:           "normal",
				Source:           "webhook", // Webhook source preserves pre-provided IDs
			},
			expectedError: nil,
			validate: func(t *testing.T, result *model.GrpsIOMember, revision uint64, mockRepo *mock.MockRepository) {
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, "mailing-list-1", result.MailingListUID)
				require.NotNil(t, result.GroupsIOMemberID)
				assert.Equal(t, int64(12345), *result.GroupsIOMemberID)
				assert.Equal(t, "committee-member", result.Username)
				assert.Equal(t, "Committee", result.FirstName)
				assert.Equal(t, "Member", result.LastName)
				assert.Equal(t, "committee.member@example.com", result.Email)
				assert.Equal(t, "committee", result.MemberType)
				assert.Equal(t, "normal", result.Status)
				assert.Equal(t, uint64(1), revision)
				assert.Equal(t, 1, mockRepo.GetMemberCount())
				assert.NotZero(t, result.CreatedAt)
				assert.NotZero(t, result.UpdatedAt)
			},
		},
		{
			name: "successful direct member creation with minimal fields",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				// Add a mailing list for the member to belong to
				testMailingList := &model.GrpsIOMailingList{
					UID:         "mailing-list-2",
					ServiceUID:  "service-2",
					Title:       "Test Direct List",
					Description: "Test direct mailing list",
					Type:        "discussion_moderated",
					CreatedAt:   time.Now().Add(-24 * time.Hour),
					UpdatedAt:   time.Now(),
				}
				mockRepo.AddMailingList(testMailingList)
			},
			inputMember: &model.GrpsIOMember{
				MailingListUID: "mailing-list-2",
				FirstName:      "Direct",
				LastName:       "Member",
				Email:          "direct.member@example.com",
				MemberType:     "direct",
			},
			expectedError: nil,
			validate: func(t *testing.T, result *model.GrpsIOMember, revision uint64, mockRepo *mock.MockRepository) {
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, "mailing-list-2", result.MailingListUID)
				assert.Equal(t, "Direct", result.FirstName)
				assert.Equal(t, "Member", result.LastName)
				assert.Equal(t, "direct.member@example.com", result.Email)
				assert.Equal(t, "direct", result.MemberType)
				assert.Equal(t, "pending", result.Status) // Default status should be set
				assert.Equal(t, uint64(1), revision)
				assert.Equal(t, 1, mockRepo.GetMemberCount())
			},
		},
		{
			name: "member creation with server-generated UID",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				// Add a mailing list for the member to belong to
				testMailingList := &model.GrpsIOMailingList{
					UID:         "mailing-list-3",
					ServiceUID:  "service-3",
					Title:       "Test Server Generated UID List",
					Description: "Test mailing list with server-generated UID member",
					Type:        "created",
					CreatedAt:   time.Now().Add(-24 * time.Hour),
					UpdatedAt:   time.Now(),
				}
				mockRepo.AddMailingList(testMailingList)
			},
			inputMember: &model.GrpsIOMember{
				UID:            "client-provided-uid", // This should be ignored
				MailingListUID: "mailing-list-3",
				FirstName:      "Server",
				LastName:       "Generated",
				Email:          "servergen@example.com",
				MemberType:     "committee",
				Status:         "normal",
			},
			expectedError: nil,
			validate: func(t *testing.T, result *model.GrpsIOMember, revision uint64, mockRepo *mock.MockRepository) {
				assert.NotEqual(t, "client-provided-uid", result.UID) // Should NOT preserve client UID
				assert.NotEmpty(t, result.UID)                        // Should have server-generated UID
				assert.Equal(t, "mailing-list-3", result.MailingListUID)
				assert.Equal(t, "normal", result.Status) // Should preserve provided status
				assert.Equal(t, uint64(1), revision)
				assert.Equal(t, 1, mockRepo.GetMemberCount())
			},
		},
		{
			name: "mailing list not found error",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				// Don't add any mailing lists
			},
			inputMember: &model.GrpsIOMember{
				MailingListUID: "nonexistent-mailing-list",
				FirstName:      "Test",
				LastName:       "Member",
				Email:          "test@example.com",
				MemberType:     "committee",
			},
			expectedError: errs.NotFound{},
			validate: func(t *testing.T, result *model.GrpsIOMember, revision uint64, mockRepo *mock.MockRepository) {
				assert.Nil(t, result)
				assert.Equal(t, uint64(0), revision)
				assert.Equal(t, 0, mockRepo.GetMemberCount())
			},
		},
		{
			name: "empty mailing list UID error",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
			},
			inputMember: &model.GrpsIOMember{
				MailingListUID: "", // Empty mailing list UID
				FirstName:      "Test",
				LastName:       "Member",
				Email:          "test@example.com",
				MemberType:     "committee",
			},
			expectedError: errs.Validation{},
			validate: func(t *testing.T, result *model.GrpsIOMember, revision uint64, mockRepo *mock.MockRepository) {
				assert.Nil(t, result)
				assert.Equal(t, uint64(0), revision)
				assert.Equal(t, 0, mockRepo.GetMemberCount())
			},
		},
		{
			name: "member with complete audit fields",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				// Add a mailing list for the member to belong to
				testMailingList := &model.GrpsIOMailingList{
					UID:         "mailing-list-audit",
					ServiceUID:  "service-audit",
					Title:       "Test Audit List",
					Description: "Test audit mailing list",
					Type:        "created",
					CreatedAt:   time.Now().Add(-24 * time.Hour),
					UpdatedAt:   time.Now(),
				}
				mockRepo.AddMailingList(testMailingList)
			},
			inputMember: &model.GrpsIOMember{
				MailingListUID:   "mailing-list-audit",
				GroupsIOMemberID: writerInt64Ptr(99999),
				GroupsIOGroupID:  writerInt64Ptr(88888),
				Username:         "audit-member",
				FirstName:        "Audit",
				LastName:         "Member",
				Email:            "audit.member@example.com",
				Organization:     "Audit Organization",
				JobTitle:         "Audit Manager",
				MemberType:       "committee",
				DeliveryMode:     "digest",
				ModStatus:        "moderator",
				Status:           "normal",
				LastReviewedAt:   writerStringPtr("2024-01-01T00:00:00Z"),
				LastReviewedBy:   writerStringPtr("reviewer-uid"),
				Source:           "webhook", // Webhook source preserves pre-provided IDs
			},
			expectedError: nil,
			validate: func(t *testing.T, result *model.GrpsIOMember, revision uint64, mockRepo *mock.MockRepository) {
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, "mailing-list-audit", result.MailingListUID)
				require.NotNil(t, result.GroupsIOMemberID)
				assert.Equal(t, int64(99999), *result.GroupsIOMemberID)
				require.NotNil(t, result.GroupsIOGroupID)
				assert.Equal(t, int64(88888), *result.GroupsIOGroupID)
				assert.Equal(t, "audit-member", result.Username)
				assert.Equal(t, "Audit", result.FirstName)
				assert.Equal(t, "Member", result.LastName)
				assert.Equal(t, "audit.member@example.com", result.Email)
				assert.Equal(t, "Audit Organization", result.Organization)
				assert.Equal(t, "Audit Manager", result.JobTitle)
				assert.Equal(t, "committee", result.MemberType)
				assert.Equal(t, "digest", result.DeliveryMode)
				assert.Equal(t, "moderator", result.ModStatus)
				assert.Equal(t, "normal", result.Status)
				assert.NotNil(t, result.LastReviewedAt)
				assert.Equal(t, "2024-01-01T00:00:00Z", *result.LastReviewedAt)
				assert.NotNil(t, result.LastReviewedBy)
				assert.Equal(t, "reviewer-uid", *result.LastReviewedBy)
				assert.Equal(t, uint64(1), revision)
				assert.Equal(t, 1, mockRepo.GetMemberCount())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockRepo := mock.NewMockRepository()
			tc.setupMock(mockRepo)

			// Create writer orchestrator with all dependencies
			writer := NewGrpsIOWriterOrchestrator(
				WithGrpsIOWriterReader(mock.NewMockGrpsIOReader(mockRepo)),
				WithGrpsIOWriter(mock.NewMockGrpsIOWriter(mockRepo)),
				WithEntityAttributeReader(mock.NewMockEntityAttributeReader(mockRepo)),
				WithPublisher(mock.NewMockMessagePublisher()),
			)

			// Execute
			result, revision, err := writer.CreateGrpsIOMember(context.Background(), tc.inputMember)

			// Validate
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.IsType(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
			}

			tc.validate(t, result, revision, mockRepo)
		})
	}
}

func TestGrpsIOWriterOrchestrator_UpdateGrpsIOMember(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()

	t.Run("successful member update", func(t *testing.T) {
		mockRepo.ClearAll()

		// Add existing member
		existingMember := &model.GrpsIOMember{
			UID:            "test-member",
			MailingListUID: "test-list",
			FirstName:      "Original",
			LastName:       "Member",
			Email:          "original@example.com",
			MemberType:     "committee",
			Status:         "normal",
			CreatedAt:      time.Now().Add(-24 * time.Hour),
			UpdatedAt:      time.Now().Add(-1 * time.Hour),
		}
		mockRepo.AddMember(existingMember)

		// Create writer orchestrator
		writer := NewGrpsIOWriterOrchestrator(
			WithGrpsIOWriterReader(mock.NewMockGrpsIOReader(mockRepo)),
			WithGrpsIOWriter(mock.NewMockGrpsIOWriter(mockRepo)),
			WithPublisher(mock.NewMockMessagePublisher()),
		)

		// Execute update
		updatedMember := &model.GrpsIOMember{
			UID:            "test-member",
			MailingListUID: "test-list",
			FirstName:      "Updated",
			LastName:       "Member",
			Email:          "original@example.com", // Email should remain the same (immutable)
			MemberType:     "committee",
			Status:         "normal",
			CreatedAt:      existingMember.CreatedAt, // Preserve created time
			UpdatedAt:      time.Now(),               // This will be set by the orchestrator
		}

		result, revision, err := writer.UpdateGrpsIOMember(ctx, "test-member", updatedMember, 1)

		// Validate
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, uint64(2), revision) // Mock increments revision
		assert.Equal(t, "Updated", result.FirstName)
		assert.Equal(t, "Member", result.LastName)
		assert.Equal(t, "original@example.com", result.Email)
		assert.False(t, result.UpdatedAt.IsZero())
	})

	t.Run("update non-existent member", func(t *testing.T) {
		mockRepo.ClearAll()

		// Create writer orchestrator
		writer := NewGrpsIOWriterOrchestrator(
			WithGrpsIOWriterReader(mock.NewMockGrpsIOReader(mockRepo)),
			WithGrpsIOWriter(mock.NewMockGrpsIOWriter(mockRepo)),
			WithPublisher(mock.NewMockMessagePublisher()),
		)

		// Execute update on non-existent member
		member := &model.GrpsIOMember{
			UID:            "non-existent",
			MailingListUID: "test-list",
			FirstName:      "Updated",
			LastName:       "Member",
			Email:          "updated@example.com",
			MemberType:     "committee",
		}

		result, revision, err := writer.UpdateGrpsIOMember(ctx, "non-existent", member, 1)

		// Validate
		require.Error(t, err)
		assert.IsType(t, errs.NotFound{}, err)
		assert.Nil(t, result)
		assert.Equal(t, uint64(0), revision)
	})
}

func TestGrpsIOWriterOrchestrator_DeleteGrpsIOMember(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()

	t.Run("successful member deletion", func(t *testing.T) {
		mockRepo.ClearAll()

		// Add existing member
		existingMember := &model.GrpsIOMember{
			UID:            "test-member",
			MailingListUID: "test-list",
			FirstName:      "Test",
			LastName:       "Member",
			Email:          "test@example.com",
			MemberType:     "committee",
			Status:         "normal",
			CreatedAt:      time.Now().Add(-24 * time.Hour),
			UpdatedAt:      time.Now().Add(-1 * time.Hour),
		}
		mockRepo.AddMember(existingMember)

		// Create writer orchestrator
		writer := NewGrpsIOWriterOrchestrator(
			WithGrpsIOWriterReader(mock.NewMockGrpsIOReader(mockRepo)),
			WithGrpsIOWriter(mock.NewMockGrpsIOWriter(mockRepo)),
			WithPublisher(mock.NewMockMessagePublisher()),
		)

		// Execute delete
		err := writer.DeleteGrpsIOMember(ctx, "test-member", 1, existingMember)

		// Validate
		require.NoError(t, err)

		// Verify member is deleted (should not be found) using reader orchestrator
		reader := NewGrpsIOReaderOrchestrator(
			WithGrpsIOReader(mock.NewMockGrpsIOReader(mockRepo)),
		)
		_, _, err = reader.GetGrpsIOMember(ctx, "test-member")
		require.Error(t, err)
		assert.IsType(t, errs.NotFound{}, err)
	})

	t.Run("delete non-existent member", func(t *testing.T) {
		mockRepo.ClearAll()

		// Create writer orchestrator
		writer := NewGrpsIOWriterOrchestrator(
			WithGrpsIOWriterReader(mock.NewMockGrpsIOReader(mockRepo)),
			WithGrpsIOWriter(mock.NewMockGrpsIOWriter(mockRepo)),
			WithPublisher(mock.NewMockMessagePublisher()),
		)

		// Execute delete on non-existent member
		err := writer.DeleteGrpsIOMember(ctx, "non-existent", 1, nil)

		// Validate
		require.Error(t, err)
		assert.IsType(t, errs.NotFound{}, err)
	})

	t.Run("delete with wrong revision", func(t *testing.T) {
		mockRepo.ClearAll()

		// Add existing member
		existingMember := &model.GrpsIOMember{
			UID:            "test-member",
			MailingListUID: "test-list",
			FirstName:      "Test",
			LastName:       "Member",
			Email:          "test@example.com",
			MemberType:     "committee",
			Status:         "normal",
			CreatedAt:      time.Now().Add(-24 * time.Hour),
			UpdatedAt:      time.Now().Add(-1 * time.Hour),
		}
		mockRepo.AddMember(existingMember)

		// Create writer orchestrator
		writer := NewGrpsIOWriterOrchestrator(
			WithGrpsIOWriterReader(mock.NewMockGrpsIOReader(mockRepo)),
			WithGrpsIOWriter(mock.NewMockGrpsIOWriter(mockRepo)),
			WithPublisher(mock.NewMockMessagePublisher()),
		)

		// Execute delete with wrong revision (mock expects revision 1, but we pass 999)
		err := writer.DeleteGrpsIOMember(ctx, "test-member", 999, existingMember)

		// Validate - mock should return conflict error for revision mismatch
		require.Error(t, err)
		assert.IsType(t, errs.Conflict{}, err)
	})
}

// Test member creation with duplicate email in same mailing list
func TestGrpsIOWriterOrchestrator_CreateGrpsIOMember_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()

	t.Run("duplicate email in same mailing list should succeed in mock", func(t *testing.T) {
		mockRepo.ClearAll()

		// Add a mailing list
		testMailingList := &model.GrpsIOMailingList{
			UID:         "test-mailing-list",
			ServiceUID:  "test-service",
			Title:       "Test List",
			Description: "Test mailing list for duplicate email",
			Type:        "created",
			CreatedAt:   time.Now().Add(-24 * time.Hour),
			UpdatedAt:   time.Now(),
		}
		mockRepo.AddMailingList(testMailingList)

		// Create writer orchestrator
		writer := NewGrpsIOWriterOrchestrator(
			WithGrpsIOWriterReader(mock.NewMockGrpsIOReader(mockRepo)),
			WithGrpsIOWriter(mock.NewMockGrpsIOWriter(mockRepo)),
			WithPublisher(mock.NewMockMessagePublisher()),
		)

		sharedEmail := "duplicate@example.com"

		// Create first member
		member1 := &model.GrpsIOMember{
			MailingListUID: "test-mailing-list",
			FirstName:      "First",
			LastName:       "Member",
			Email:          sharedEmail,
			MemberType:     "committee",
			Status:         "normal",
		}

		result1, revision1, err1 := writer.CreateGrpsIOMember(ctx, member1)
		require.NoError(t, err1) // Mock allows duplicate
		assert.NotEmpty(t, result1.UID)
		assert.Equal(t, uint64(1), revision1)

		// Create second member with same email (should return existing member - idempotent)
		member2 := &model.GrpsIOMember{
			MailingListUID: "test-mailing-list",
			FirstName:      "Second",
			LastName:       "Member",
			Email:          sharedEmail,
			MemberType:     "direct",
			Status:         "pending",
		}

		result2, revision2, err2 := writer.CreateGrpsIOMember(ctx, member2)
		require.NoError(t, err2) // Idempotent - returns existing member
		assert.NotEmpty(t, result2.UID)
		assert.Equal(t, result1.UID, result2.UID) // Same UID (idempotent)
		assert.Equal(t, uint64(1), revision2)
		assert.Equal(t, 1, mockRepo.GetMemberCount()) // Only one member stored (idempotent)
	})
}

// Test member creation validation scenarios
func TestGrpsIOWriterOrchestrator_CreateGrpsIOMember_ValidationScenarios(t *testing.T) {
	ctx := context.Background()

	validationTests := []struct {
		name           string
		setupMock      func(*mock.MockRepository)
		inputMember    *model.GrpsIOMember
		expectedError  bool
		errorContains  string
		validateResult func(*testing.T, *model.GrpsIOMember, uint64)
	}{
		{
			name: "member with all required fields",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				testMailingList := &model.GrpsIOMailingList{
					UID:        "valid-list",
					ServiceUID: "valid-service",
					Title:      "Valid List",
					Type:       "created",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
				}
				mockRepo.AddMailingList(testMailingList)
			},
			inputMember: &model.GrpsIOMember{
				MailingListUID: "valid-list",
				FirstName:      "Valid",
				LastName:       "Member",
				Email:          "valid@example.com",
				MemberType:     "committee",
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *model.GrpsIOMember, revision uint64) {
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, "pending", result.Status) // Default status should be set
				assert.Equal(t, uint64(1), revision)
			},
		},
		{
			name: "member creation with message publishing",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				testMailingList := &model.GrpsIOMailingList{
					UID:        "messaging-list",
					ServiceUID: "messaging-service",
					Title:      "Messaging List",
					Type:       "created",
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
				}
				mockRepo.AddMailingList(testMailingList)
			},
			inputMember: &model.GrpsIOMember{
				MailingListUID: "messaging-list",
				Username:       "messaging-user",
				FirstName:      "Messaging",
				LastName:       "Member",
				Email:          "messaging@example.com",
				MemberType:     "committee",
				Status:         "normal",
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *model.GrpsIOMember, revision uint64) {
				assert.NotNil(t, result)
				assert.Equal(t, "messaging-user", result.Username)
				assert.Equal(t, "normal", result.Status) // Should preserve provided status
				assert.Equal(t, uint64(1), revision)
			},
		},
	}

	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := mock.NewMockRepository()
			tt.setupMock(mockRepo)

			// Create writer orchestrator with message publisher
			writer := NewGrpsIOWriterOrchestrator(
				WithGrpsIOWriterReader(mock.NewMockGrpsIOReader(mockRepo)),
				WithGrpsIOWriter(mock.NewMockGrpsIOWriter(mockRepo)),
				WithEntityAttributeReader(mock.NewMockEntityAttributeReader(mockRepo)),
				WithPublisher(mock.NewMockMessagePublisher()),
			)

			// Execute
			result, revision, err := writer.CreateGrpsIOMember(ctx, tt.inputMember)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}

			tt.validateResult(t, result, revision)
		})
	}
}

// Test member creation with different member types
func TestGrpsIOWriterOrchestrator_CreateGrpsIOMember_MemberTypes(t *testing.T) {
	ctx := context.Background()

	memberTypes := []struct {
		memberType  string
		description string
	}{
		{"committee", "Committee member type"},
		{"direct", "Direct member type"},
	}

	for _, mt := range memberTypes {
		t.Run(mt.description, func(t *testing.T) {
			mockRepo := mock.NewMockRepository()
			mockRepo.ClearAll()

			// Add a mailing list
			testMailingList := &model.GrpsIOMailingList{
				UID:        "type-test-list",
				ServiceUID: "type-test-service",
				Title:      "Type Test List",
				Type:       "created",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			mockRepo.AddMailingList(testMailingList)

			// Create writer orchestrator
			writer := NewGrpsIOWriterOrchestrator(
				WithGrpsIOWriterReader(mock.NewMockGrpsIOReader(mockRepo)),
				WithGrpsIOWriter(mock.NewMockGrpsIOWriter(mockRepo)),
				WithEntityAttributeReader(mock.NewMockEntityAttributeReader(mockRepo)),
				WithPublisher(mock.NewMockMessagePublisher()),
			)

			// Create member with specific type
			member := &model.GrpsIOMember{
				MailingListUID: "type-test-list",
				FirstName:      "Type",
				LastName:       "Test",
				Email:          "type.test@example.com",
				MemberType:     mt.memberType,
			}

			result, revision, err := writer.CreateGrpsIOMember(ctx, member)

			// Validate
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Equal(t, mt.memberType, result.MemberType)
			assert.Equal(t, uint64(1), revision)
			assert.Equal(t, 1, mockRepo.GetMemberCount())
		})
	}
}

// Helper function to create string pointer
func writerStringPtr(s string) *string {
	return &s
}

// Helper function to create int64 pointer
func writerInt64Ptr(i int64) *int64 {
	return &i
}

// TestGrpsIOWriterOrchestrator_syncMemberToGroupsIO tests the syncMemberToGroupsIO method
// Note: This method returns void and only logs errors/warnings. Comprehensive testing would
// require log capture or refactoring the method to return an error/status.
// These tests verify that guard clauses prevent panics in edge cases.
func TestGrpsIOWriterOrchestrator_syncMemberToGroupsIO(t *testing.T) {
	testCases := []struct {
		name       string
		setupMocks func() *grpsIOWriterOrchestrator
		member     *model.GrpsIOMember
		updates    groupsio.MemberUpdateOptions
	}{
		{
			name: "skip sync when Groups.io client is nil",
			setupMocks: func() *grpsIOWriterOrchestrator {
				return &grpsIOWriterOrchestrator{
					groupsClient: nil, // No client - should skip gracefully
				}
			},
			member: &model.GrpsIOMember{
				UID:              "member-1",
				GroupsIOMemberID: func() *int64 { i := int64(12345); return &i }(),
				FirstName:        "John",
				LastName:         "Doe",
			},
			updates: groupsio.MemberUpdateOptions{
				FirstName: "John",
				LastName:  "Doe",
			},
		},
		{
			name: "skip sync when member GroupsIOMemberID is nil",
			setupMocks: func() *grpsIOWriterOrchestrator {
				return &grpsIOWriterOrchestrator{
					groupsClient: nil, // Could be any value, but GroupsIOMemberID is nil
				}
			},
			member: &model.GrpsIOMember{
				UID:              "member-2",
				GroupsIOMemberID: nil, // No member ID - should skip gracefully
				FirstName:        "Jane",
				LastName:         "Smith",
			},
			updates: groupsio.MemberUpdateOptions{
				FirstName: "Jane",
				LastName:  "Smith",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			ctx := context.Background()
			orchestrator := tc.setupMocks()

			// Execute - should not panic regardless of nil clients or missing data
			require.NotPanics(t, func() {
				orchestrator.syncMemberToGroupsIO(ctx, tc.member, tc.updates)
			}, "syncMemberToGroupsIO should handle nil clients and missing data gracefully")

			// Note: Without log capture or return values, we can only verify no panic occurs.
			// The guard clauses at lines 468-471 in grpsio_member_writer.go ensure safe exit.
		})
	}
}
