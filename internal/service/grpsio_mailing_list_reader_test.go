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
		UID:              testMailingListUID,
		GroupName:        "dev",
		Public:           true,
		Type:             "discussion_open",
		CommitteeUID:     "committee-1",
		CommitteeName:    "Technical Advisory Committee",
		CommitteeFilters: []string{"Voting Rep", "Observer"},
		Description:      "Development discussions and technical matters for the project",
		Title:            "Development List",
		SubjectTag:       "[DEV]",
		ServiceUID:       "service-1",
		ProjectUID:       "test-project-uid",
		ProjectName:      "Test Project",
		ProjectSlug:      "test-project",
		LastReviewedAt:   mailingListStringPtr("2024-01-01T00:00:00Z"),
		LastReviewedBy:   mailingListStringPtr("reviewer-uid"),
		Writers:          []string{"dev-admin@testproject.org"},
		Auditors:         []string{"auditor@testproject.org"},
		CreatedAt:        time.Now().Add(-18 * time.Hour),
		UpdatedAt:        time.Now().Add(-2 * time.Hour),
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
				assert.Equal(t, "committee-1", mailingList.CommitteeUID)
				assert.Equal(t, "Technical Advisory Committee", mailingList.CommitteeName)
				assert.Equal(t, []string{"Voting Rep", "Observer"}, mailingList.CommitteeFilters)
				assert.Equal(t, "Development discussions and technical matters for the project", mailingList.Description)
				assert.Equal(t, "Development List", mailingList.Title)
				assert.Equal(t, "[DEV]", mailingList.SubjectTag)
				assert.Equal(t, "service-1", mailingList.ServiceUID)
				assert.Equal(t, "test-project-uid", mailingList.ProjectUID)
				assert.Equal(t, "Test Project", mailingList.ProjectName)
				assert.Equal(t, "test-project", mailingList.ProjectSlug)
				assert.NotNil(t, mailingList.LastReviewedAt)
				assert.Equal(t, "2024-01-01T00:00:00Z", *mailingList.LastReviewedAt)
				assert.NotNil(t, mailingList.LastReviewedBy)
				assert.Equal(t, "reviewer-uid", *mailingList.LastReviewedBy)
				assert.Equal(t, []string{"dev-admin@testproject.org"}, mailingList.Writers)
				assert.Equal(t, []string{"auditor@testproject.org"}, mailingList.Auditors)
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
			mailingList, err := reader.GetGrpsIOMailingList(ctx, tt.mailingListUID)

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

func TestGrpsIOReaderOrchestratorGetGrpsIOMailingListsByParent(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()

	// Setup test data
	parentUID := "service-1"
	testMailingLists := []*model.GrpsIOMailingList{
		{
			UID:         "mailing-list-1",
			GroupName:   "dev",
			Public:      true,
			Type:        "discussion_open",
			Description: "Development discussions for the project",
			Title:       "Development List",
			ServiceUID:  parentUID,
			ProjectUID:  "test-project-uid",
			Writers:     []string{"dev-admin@testproject.org"},
			Auditors:    []string{"auditor@testproject.org"},
			CreatedAt:   time.Now().Add(-18 * time.Hour),
			UpdatedAt:   time.Now().Add(-2 * time.Hour),
		},
		{
			UID:         "mailing-list-2",
			GroupName:   "announce",
			Public:      true,
			Type:        "announcement",
			Description: "Official announcements for the project",
			Title:       "Announcements",
			ServiceUID:  parentUID,
			ProjectUID:  "test-project-uid",
			Writers:     []string{"admin@testproject.org"},
			Auditors:    []string{"auditor@testproject.org"},
			CreatedAt:   time.Now().Add(-12 * time.Hour),
			UpdatedAt:   time.Now().Add(-1 * time.Hour),
		},
	}

	tests := []struct {
		name                 string
		setupMock            func()
		parentID             string
		expectedError        bool
		errorType            error
		validateMailingLists func(*testing.T, []*model.GrpsIOMailingList)
	}{
		{
			name: "successful mailing lists retrieval by parent",
			setupMock: func() {
				mockRepo.ClearAll()
				// Store the mailing lists in mock repository
				for _, ml := range testMailingLists {
					mockRepo.AddMailingList(ml)
				}
			},
			parentID:      parentUID,
			expectedError: false,
			validateMailingLists: func(t *testing.T, mailingLists []*model.GrpsIOMailingList) {
				assert.NotNil(t, mailingLists)
				assert.Len(t, mailingLists, 2)

				// Validate that all returned mailing lists have the correct parent
				for _, ml := range mailingLists {
					assert.Equal(t, parentUID, ml.ServiceUID)
					assert.Equal(t, "test-project-uid", ml.ProjectUID)
				}

				// Check specific mailing lists exist
				var foundDev, foundAnnounce bool
				for _, ml := range mailingLists {
					switch ml.GroupName {
					case "dev":
						foundDev = true
						assert.Equal(t, "discussion_open", ml.Type)
						assert.True(t, ml.Public)
					case "announce":
						foundAnnounce = true
						assert.Equal(t, "announcement", ml.Type)
						assert.True(t, ml.Public)
					}
				}
				assert.True(t, foundDev, "Expected to find 'dev' mailing list")
				assert.True(t, foundAnnounce, "Expected to find 'announce' mailing list")
			},
		},
		{
			name: "no mailing lists found for parent",
			setupMock: func() {
				mockRepo.ClearAll()
				// Don't store any mailing lists
			},
			parentID:      "nonexistent-parent-uid",
			expectedError: false,
			validateMailingLists: func(t *testing.T, mailingLists []*model.GrpsIOMailingList) {
				assert.Len(t, mailingLists, 0)
			},
		},
		{
			name: "empty parent ID",
			setupMock: func() {
				mockRepo.ClearAll()
			},
			parentID:      "",
			expectedError: false,
			validateMailingLists: func(t *testing.T, mailingLists []*model.GrpsIOMailingList) {
				assert.Len(t, mailingLists, 0)
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
			mailingLists, err := reader.GetGrpsIOMailingListsByParent(ctx, tt.parentID)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorType != nil {
					assert.IsType(t, tt.errorType, err)
				}
			} else {
				require.NoError(t, err)
			}

			tt.validateMailingLists(t, mailingLists)
		})
	}
}

func TestGrpsIOReaderOrchestratorGetGrpsIOMailingListsByCommittee(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()

	// Setup test data
	committeeUID := "committee-1"
	testMailingLists := []*model.GrpsIOMailingList{
		{
			UID:              "mailing-list-1",
			GroupName:        "committee-dev",
			Public:           false,
			Type:             "discussion_moderated",
			CommitteeUID:     committeeUID,
			CommitteeName:    "Technical Committee",
			CommitteeFilters: []string{"Voting Rep"},
			Description:      "Committee development discussions",
			Title:            "Committee Development List",
			ServiceUID:       "service-1",
			ProjectUID:       "test-project-uid",
			Writers:          []string{"committee-admin@testproject.org"},
			Auditors:         []string{"auditor@testproject.org"},
			CreatedAt:        time.Now().Add(-18 * time.Hour),
			UpdatedAt:        time.Now().Add(-2 * time.Hour),
		},
		{
			UID:              "mailing-list-3",
			GroupName:        "committee-general",
			Public:           false,
			Type:             "discussion_open",
			CommitteeUID:     committeeUID,
			CommitteeName:    "Technical Committee",
			CommitteeFilters: []string{"Voting Rep", "Observer"},
			Description:      "General committee discussions",
			Title:            "Committee General List",
			ServiceUID:       "service-2",
			ProjectUID:       "test-project-uid",
			Writers:          []string{"committee-admin@testproject.org"},
			Auditors:         []string{"auditor@testproject.org"},
			CreatedAt:        time.Now().Add(-12 * time.Hour),
			UpdatedAt:        time.Now().Add(-1 * time.Hour),
		},
	}

	tests := []struct {
		name                 string
		setupMock            func()
		committeeID          string
		expectedError        bool
		errorType            error
		validateMailingLists func(*testing.T, []*model.GrpsIOMailingList)
	}{
		{
			name: "successful mailing lists retrieval by committee",
			setupMock: func() {
				mockRepo.ClearAll()
				// Store the mailing lists in mock repository
				for _, ml := range testMailingLists {
					mockRepo.AddMailingList(ml)
				}
				// Also add a mailing list without this committee to ensure filtering works
				otherCommitteeML := &model.GrpsIOMailingList{
					UID:           "mailing-list-other",
					GroupName:     "other-committee",
					CommitteeUID:  "committee-2",
					CommitteeName: "Other Committee",
					Description:   "Other committee discussions",
					Title:         "Other Committee List",
					ServiceUID:    "service-1",
					ProjectUID:    "test-project-uid",
					CreatedAt:     time.Now().Add(-6 * time.Hour),
					UpdatedAt:     time.Now(),
				}
				mockRepo.AddMailingList(otherCommitteeML)
			},
			committeeID:   committeeUID,
			expectedError: false,
			validateMailingLists: func(t *testing.T, mailingLists []*model.GrpsIOMailingList) {
				assert.NotNil(t, mailingLists)
				assert.Len(t, mailingLists, 2)

				// Validate that all returned mailing lists have the correct committee
				for _, ml := range mailingLists {
					assert.Equal(t, committeeUID, ml.CommitteeUID)
					assert.Equal(t, "Technical Committee", ml.CommitteeName)
					assert.Equal(t, "test-project-uid", ml.ProjectUID)
					assert.False(t, ml.Public) // Both committee lists are private
				}

				// Check specific mailing lists exist
				var foundDev, foundGeneral bool
				for _, ml := range mailingLists {
					switch ml.GroupName {
					case "committee-dev":
						foundDev = true
						assert.Equal(t, "discussion_moderated", ml.Type)
						assert.Equal(t, []string{"Voting Rep"}, ml.CommitteeFilters)
					case "committee-general":
						foundGeneral = true
						assert.Equal(t, "discussion_open", ml.Type)
						assert.Equal(t, []string{"Voting Rep", "Observer"}, ml.CommitteeFilters)
					}
				}
				assert.True(t, foundDev, "Expected to find 'committee-dev' mailing list")
				assert.True(t, foundGeneral, "Expected to find 'committee-general' mailing list")
			},
		},
		{
			name: "no mailing lists found for committee",
			setupMock: func() {
				mockRepo.ClearAll()
				// Don't store any mailing lists
			},
			committeeID:   "nonexistent-committee-uid",
			expectedError: false,
			validateMailingLists: func(t *testing.T, mailingLists []*model.GrpsIOMailingList) {
				assert.Len(t, mailingLists, 0)
			},
		},
		{
			name: "empty committee ID",
			setupMock: func() {
				mockRepo.ClearAll()
			},
			committeeID:   "",
			expectedError: false,
			validateMailingLists: func(t *testing.T, mailingLists []*model.GrpsIOMailingList) {
				assert.Len(t, mailingLists, 0)
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
			mailingLists, err := reader.GetGrpsIOMailingListsByCommittee(ctx, tt.committeeID)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorType != nil {
					assert.IsType(t, tt.errorType, err)
				}
			} else {
				require.NoError(t, err)
			}

			tt.validateMailingLists(t, mailingLists)
		})
	}
}

func TestGrpsIOReaderOrchestratorGetGrpsIOMailingListsByProject(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()

	// Setup test data
	projectUID := "test-project-uid"
	testMailingLists := []*model.GrpsIOMailingList{
		{
			UID:         "mailing-list-1",
			GroupName:   "project-dev",
			Public:      true,
			Type:        "discussion_open",
			Description: "Project development discussions",
			Title:       "Project Development List",
			ServiceUID:  "service-1",
			ProjectUID:  projectUID,
			ProjectName: "Test Project",
			ProjectSlug: "test-project",
			Writers:     []string{"dev-admin@testproject.org"},
			Auditors:    []string{"auditor@testproject.org"},
			CreatedAt:   time.Now().Add(-18 * time.Hour),
			UpdatedAt:   time.Now().Add(-2 * time.Hour),
		},
		{
			UID:         "mailing-list-2",
			GroupName:   "project-announce",
			Public:      true,
			Type:        "announcement",
			Description: "Project announcements",
			Title:       "Project Announcements",
			ServiceUID:  "service-1",
			ProjectUID:  projectUID,
			ProjectName: "Test Project",
			ProjectSlug: "test-project",
			Writers:     []string{"admin@testproject.org"},
			Auditors:    []string{"auditor@testproject.org"},
			CreatedAt:   time.Now().Add(-12 * time.Hour),
			UpdatedAt:   time.Now().Add(-1 * time.Hour),
		},
	}

	tests := []struct {
		name                 string
		setupMock            func()
		projectID            string
		expectedError        bool
		errorType            error
		validateMailingLists func(*testing.T, []*model.GrpsIOMailingList)
	}{
		{
			name: "successful mailing lists retrieval by project",
			setupMock: func() {
				mockRepo.ClearAll()
				// Store the mailing lists in mock repository
				for _, ml := range testMailingLists {
					mockRepo.AddMailingList(ml)
				}
				// Also add a mailing list from different project to ensure filtering works
				otherProjectML := &model.GrpsIOMailingList{
					UID:         "mailing-list-other",
					GroupName:   "other-project",
					Description: "Other project discussions",
					Title:       "Other Project List",
					ServiceUID:  "service-2",
					ProjectUID:  "other-project-uid",
					ProjectName: "Other Project",
					CreatedAt:   time.Now().Add(-6 * time.Hour),
					UpdatedAt:   time.Now(),
				}
				mockRepo.AddMailingList(otherProjectML)
			},
			projectID:     projectUID,
			expectedError: false,
			validateMailingLists: func(t *testing.T, mailingLists []*model.GrpsIOMailingList) {
				assert.NotNil(t, mailingLists)
				assert.Len(t, mailingLists, 2)

				// Validate that all returned mailing lists have the correct project
				for _, ml := range mailingLists {
					assert.Equal(t, projectUID, ml.ProjectUID)
					assert.Equal(t, "Test Project", ml.ProjectName)
					assert.Equal(t, "test-project", ml.ProjectSlug)
					assert.True(t, ml.Public) // Both project lists are public
				}

				// Check specific mailing lists exist
				var foundDev, foundAnnounce bool
				for _, ml := range mailingLists {
					switch ml.GroupName {
					case "project-dev":
						foundDev = true
						assert.Equal(t, "discussion_open", ml.Type)
					case "project-announce":
						foundAnnounce = true
						assert.Equal(t, "announcement", ml.Type)
					}
				}
				assert.True(t, foundDev, "Expected to find 'project-dev' mailing list")
				assert.True(t, foundAnnounce, "Expected to find 'project-announce' mailing list")
			},
		},
		{
			name: "no mailing lists found for project",
			setupMock: func() {
				mockRepo.ClearAll()
				// Don't store any mailing lists
			},
			projectID:     "nonexistent-project-uid",
			expectedError: false,
			validateMailingLists: func(t *testing.T, mailingLists []*model.GrpsIOMailingList) {
				assert.Len(t, mailingLists, 0)
			},
		},
		{
			name: "empty project ID",
			setupMock: func() {
				mockRepo.ClearAll()
			},
			projectID:     "",
			expectedError: false,
			validateMailingLists: func(t *testing.T, mailingLists []*model.GrpsIOMailingList) {
				assert.Len(t, mailingLists, 0)
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
			mailingLists, err := reader.GetGrpsIOMailingListsByProject(ctx, tt.projectID)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorType != nil {
					assert.IsType(t, tt.errorType, err)
				}
			} else {
				require.NoError(t, err)
			}

			tt.validateMailingLists(t, mailingLists)
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
			UID:              "integration-mailing-list-1",
			GroupName:        "integration-dev",
			Public:           true,
			Type:             "discussion_open",
			CommitteeUID:     committeeUID,
			CommitteeName:    "Integration Technical Committee",
			CommitteeFilters: []string{"Voting Rep", "Observer"},
			Description:      "Integration development discussions and technical matters",
			Title:            "Integration Development List",
			SubjectTag:       "[INTEGRATION-DEV]",
			ServiceUID:       parentServiceUID,
			ProjectUID:       projectUID,
			ProjectName:      "Integration Test Project",
			ProjectSlug:      "integration-test-project",
			LastReviewedAt:   mailingListStringPtr("2024-03-01T15:00:00Z"),
			LastReviewedBy:   mailingListStringPtr("integration-reviewer"),
			Writers:          []string{"integration-dev-admin@testproject.org", "integration-lead@testproject.org"},
			Auditors:         []string{"integration-auditor1@testproject.org", "integration-auditor2@testproject.org"},
			CreatedAt:        time.Now().Add(-48 * time.Hour),
			UpdatedAt:        time.Now().Add(-3 * time.Hour),
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
			LastReviewedAt: mailingListStringPtr("2024-03-02T10:30:00Z"),
			LastReviewedBy: mailingListStringPtr("integration-admin"),
			Writers:        []string{"integration-admin@testproject.org"},
			Auditors:       []string{"integration-auditor1@testproject.org"},
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
		ml1, err := reader.GetGrpsIOMailingList(ctx, "integration-mailing-list-1")
		require.NoError(t, err)
		require.NotNil(t, ml1)
		assert.Equal(t, "integration-dev", ml1.GroupName)
		assert.True(t, ml1.IsCommitteeBased())

		ml2, err := reader.GetGrpsIOMailingList(ctx, "integration-mailing-list-2")
		require.NoError(t, err)
		require.NotNil(t, ml2)
		assert.Equal(t, "integration-announce", ml2.GroupName)
		assert.False(t, ml2.IsCommitteeBased()) // No committee fields set

		// Test retrieval by parent service
		mlsByParent, err := reader.GetGrpsIOMailingListsByParent(ctx, parentServiceUID)
		require.NoError(t, err)
		assert.Len(t, mlsByParent, 2)
		for _, ml := range mlsByParent {
			assert.Equal(t, parentServiceUID, ml.ServiceUID)
			assert.Equal(t, projectUID, ml.ProjectUID)
		}

		// Test retrieval by committee (should find only the committee-based one)
		mlsByCommittee, err := reader.GetGrpsIOMailingListsByCommittee(ctx, committeeUID)
		require.NoError(t, err)
		assert.Len(t, mlsByCommittee, 1)
		assert.Equal(t, "integration-dev", mlsByCommittee[0].GroupName)
		assert.Equal(t, committeeUID, mlsByCommittee[0].CommitteeUID)
		assert.Equal(t, "Integration Technical Committee", mlsByCommittee[0].CommitteeName)
		assert.Equal(t, []string{"Voting Rep", "Observer"}, mlsByCommittee[0].CommitteeFilters)

		// Test retrieval by project
		mlsByProject, err := reader.GetGrpsIOMailingListsByProject(ctx, projectUID)
		require.NoError(t, err)
		assert.Len(t, mlsByProject, 2)
		for _, ml := range mlsByProject {
			assert.Equal(t, projectUID, ml.ProjectUID)
			assert.Equal(t, "Integration Test Project", ml.ProjectName)
			assert.Equal(t, "integration-test-project", ml.ProjectSlug)
		}

		// Validate complete data integrity across all operations
		allMLUIDs := make(map[string]bool)
		allMLUIDs[ml1.UID] = true
		allMLUIDs[ml2.UID] = true

		for _, ml := range mlsByParent {
			assert.Contains(t, allMLUIDs, ml.UID)
		}
		for _, ml := range mlsByProject {
			assert.Contains(t, allMLUIDs, ml.UID)
		}
	})
}

// Helper function to create string pointer for mailing list tests
func mailingListStringPtr(s string) *string {
	return &s
}
