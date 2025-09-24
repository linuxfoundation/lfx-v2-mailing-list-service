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
		GroupsIOMemberID: memberInt64Ptr(12345),
		GroupsIOGroupID:  memberInt64Ptr(67890),
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


// Helper function to create string pointer
func memberStringPtr(s string) *string {
	return &s
}

// Helper function to create int64 pointer
func memberInt64Ptr(i int64) *int64 {
	return &i
}
