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

func TestGrpsIOReaderOrchestratorGetGrpsIOMailingList(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()

	// Setup test data
	testMailingListUID := uuid.New().String()
	testMailingList := &model.GrpsIOMailingList{
		UID:       testMailingListUID,
		GroupName: "dev",
		Public:    true,
		Type:      "discussion_open",
		Committees: []model.Committee{
			{UID: "committee-1", Name: "Technical Advisory Committee", AllowedVotingStatuses: []string{"Voting Rep", "Observer"}},
		},
		Description:    "Development discussions and technical matters for the project",
		Title:          "Development List",
		SubjectTag:     "[DEV]",
		ServiceUID:     "service-1",
		ProjectUID:     "test-project-uid",
		ProjectName:    "Test Project",
		ProjectSlug:    "test-project",
		CreatedAt:      time.Now().Add(-18 * time.Hour),
		UpdatedAt:      time.Now().Add(-2 * time.Hour),
	}

	tests := []struct {
		name                string
		setupMock           func()
		mailingListUID      string
		expectedError       bool
		errorType           error
		validateMailingList func(*testing.T, *model.GrpsIOMailingList)
	}{
		{
			name: "successful mailing list retrieval",
			setupMock: func() {
				mockRepo.ClearAll()
				// Store the mailing list in mock repository
				mockRepo.AddMailingList(testMailingList)
			},
			mailingListUID: testMailingListUID,
			expectedError:  false,
			validateMailingList: func(t *testing.T, mailingList *model.GrpsIOMailingList) {
				assert.NotNil(t, mailingList)
				assert.Equal(t, testMailingListUID, mailingList.UID)
				assert.Equal(t, "dev", mailingList.GroupName)
				assert.True(t, mailingList.Public)
				assert.Equal(t, "discussion_open", mailingList.Type)
				require.Len(t, mailingList.Committees, 1)
				assert.Equal(t, "committee-1", mailingList.Committees[0].UID)
				assert.Equal(t, "Technical Advisory Committee", mailingList.Committees[0].Name)
				assert.Equal(t, []string{"Voting Rep", "Observer"}, mailingList.Committees[0].AllowedVotingStatuses)
				assert.Equal(t, "Development discussions and technical matters for the project", mailingList.Description)
				assert.Equal(t, "Development List", mailingList.Title)
				assert.Equal(t, "[DEV]", mailingList.SubjectTag)
				assert.Equal(t, "service-1", mailingList.ServiceUID)
				assert.Equal(t, "test-project-uid", mailingList.ProjectUID)
				assert.Equal(t, "Test Project", mailingList.ProjectName)
				assert.Equal(t, "test-project", mailingList.ProjectSlug)
				assert.NotZero(t, mailingList.CreatedAt)
				assert.NotZero(t, mailingList.UpdatedAt)
			},
		},
		{
			name: "mailing list not found error",
			setupMock: func() {
				mockRepo.ClearAll()
				// Don't store any mailing list
			},
			mailingListUID: "nonexistent-mailing-list-uid",
			expectedError:  true,
			errorType:      errs.NotFound{},
			validateMailingList: func(t *testing.T, mailingList *model.GrpsIOMailingList) {
				assert.Nil(t, mailingList)
			},
		},
		{
			name: "empty mailing list UID",
			setupMock: func() {
				mockRepo.ClearAll()
			},
			mailingListUID: "",
			expectedError:  true,
			errorType:      errs.NotFound{},
			validateMailingList: func(t *testing.T, mailingList *model.GrpsIOMailingList) {
				assert.Nil(t, mailingList)
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
			mailingList, _, err := reader.GetGrpsIOMailingList(ctx, tt.mailingListUID)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorType != nil {
					assert.IsType(t, tt.errorType, err)
				}
			} else {
				require.NoError(t, err)
			}

			tt.validateMailingList(t, mailingList)
		})
	}
}

func TestGrpsIOReaderOrchestratorMailingListIntegration(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()
	mockRepo.ClearAll()

	// Setup comprehensive test data
	projectUID := "integration-test-project-uid"
	parentServiceUID := "integration-service-1"
	committeeUID := "integration-committee-1"

	testMailingLists := []*model.GrpsIOMailingList{
		{
			UID:       "integration-mailing-list-1",
			GroupName: "integration-dev",
			Public:    true,
			Type:      "discussion_open",
			Committees: []model.Committee{
				{UID: committeeUID, Name: "Integration Technical Committee", AllowedVotingStatuses: []string{"Voting Rep", "Observer"}},
			},
			Description:    "Integration development discussions and technical matters",
			Title:          "Integration Development List",
			SubjectTag:     "[INTEGRATION-DEV]",
			ServiceUID:     parentServiceUID,
			ProjectUID:     projectUID,
			ProjectName:    "Integration Test Project",
			ProjectSlug:    "integration-test-project",
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-3 * time.Hour),
		},
		{
			UID:            "integration-mailing-list-2",
			GroupName:      "integration-announce",
			Public:         true,
			Type:           "announcement",
			Description:    "Integration project announcements and important updates",
			Title:          "Integration Announcements",
			SubjectTag:     "[INTEGRATION-ANNOUNCE]",
			ServiceUID:     parentServiceUID,
			ProjectUID:     projectUID,
			ProjectName:    "Integration Test Project",
			ProjectSlug:    "integration-test-project",
			CreatedAt:      time.Now().Add(-36 * time.Hour),
			UpdatedAt:      time.Now().Add(-1 * time.Hour),
		},
	}

	// Store the mailing lists
	for _, ml := range testMailingLists {
		mockRepo.AddMailingList(ml)
	}

	// Create reader orchestrator
	reader := NewGrpsIOReaderOrchestrator(
		WithGrpsIOReader(mock.NewMockGrpsIOReader(mockRepo)),
	)

	t.Run("comprehensive mailing list operations", func(t *testing.T) {
		// Test individual mailing list retrieval
		ml1, _, err := reader.GetGrpsIOMailingList(ctx, "integration-mailing-list-1")
		require.NoError(t, err)
		require.NotNil(t, ml1)
		assert.Equal(t, "integration-dev", ml1.GroupName)
		assert.True(t, ml1.IsCommitteeBased())

		ml2, _, err := reader.GetGrpsIOMailingList(ctx, "integration-mailing-list-2")
		require.NoError(t, err)
		require.NotNil(t, ml2)
		assert.Equal(t, "integration-announce", ml2.GroupName)
		assert.False(t, ml2.IsCommitteeBased()) // No committee fields set

		// Validate complete data integrity
		assert.Equal(t, parentServiceUID, ml1.ServiceUID)
		assert.Equal(t, parentServiceUID, ml2.ServiceUID)
		assert.Equal(t, projectUID, ml1.ProjectUID)
		assert.Equal(t, projectUID, ml2.ProjectUID)

		// Validate committee-based mailing list
		require.Len(t, ml1.Committees, 1)
		assert.Equal(t, committeeUID, ml1.Committees[0].UID)
		assert.Equal(t, "Integration Technical Committee", ml1.Committees[0].Name)
		assert.Equal(t, []string{"Voting Rep", "Observer"}, ml1.Committees[0].AllowedVotingStatuses)

		// Validate project details
		assert.Equal(t, "Integration Test Project", ml1.ProjectName)
		assert.Equal(t, "integration-test-project", ml1.ProjectSlug)
		assert.Equal(t, "Integration Test Project", ml2.ProjectName)
		assert.Equal(t, "integration-test-project", ml2.ProjectSlug)
	})
}

// Helper function to create string pointer for mailing list tests
func mailingListStringPtr(s string) *string {
	return &s
}
