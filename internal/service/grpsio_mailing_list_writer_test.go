// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// TestMockMailingListWriter implements proper reservation logic for testing
type TestMockMailingListWriter struct {
	mock         *mock.MockRepository
	reservations map[string]string // key -> reservationID for rollback
}

func NewTestMockMailingListWriter(mockRepo *mock.MockRepository) *TestMockMailingListWriter {
	return &TestMockMailingListWriter{
		mock:         mockRepo,
		reservations: make(map[string]string),
	}
}

func (w *TestMockMailingListWriter) CreateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, uint64, error) {
	// Generate UID if not set
	if mailingList.UID == "" {
		mailingList.UID = uuid.New().String()
	}

	now := time.Now()
	mailingList.CreatedAt = now
	mailingList.UpdatedAt = now

	// Store mailing list
	return w.mock.CreateGrpsIOMailingList(ctx, mailingList)
}

func (w *TestMockMailingListWriter) UpdateGrpsIOMailingList(ctx context.Context, uid string, mailingList *model.GrpsIOMailingList, expectedRevision uint64) (*model.GrpsIOMailingList, uint64, error) {
	mockWriter := mock.NewMockGrpsIOWriter(w.mock)
	return mockWriter.UpdateGrpsIOMailingList(ctx, uid, mailingList, expectedRevision)
}

func (w *TestMockMailingListWriter) DeleteGrpsIOMailingList(ctx context.Context, uid string, expectedRevision uint64, mailingList *model.GrpsIOMailingList) error {
	mockWriter := mock.NewMockGrpsIOWriter(w.mock)
	return mockWriter.DeleteGrpsIOMailingList(ctx, uid, expectedRevision, mailingList)
}

// UniqueMailingListGroupName reserves a unique group name within parent service
func (w *TestMockMailingListWriter) UniqueMailingListGroupName(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error) {
	groupNameKey := mailingList.BuildIndexKey(ctx)

	// Use the mock's existing logic but invert the result for proper reservation behavior
	mockWriter := mock.NewMockGrpsIOWriter(w.mock)
	existingUID, err := mockWriter.UniqueMailingListGroupName(ctx, mailingList)

	// If we get a conflict error, that means it already exists - return the conflict
	if err != nil {
		var conflictErr errs.Conflict
		if errors.As(err, &conflictErr) {
			return existingUID, err
		}
		// If it's a "not found" error, that means it's unique - we can reserve it
		reservationID := uuid.New().String()
		w.reservations[groupNameKey] = reservationID
		return reservationID, nil
	}

	// Should not reach here with the current mock implementation
	return existingUID, err
}

// CreateSecondaryIndices creates secondary indices for mailing list
func (w *TestMockMailingListWriter) CreateSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) ([]string, error) {
	mockWriter := mock.NewMockGrpsIOWriter(w.mock)
	return mockWriter.CreateSecondaryIndices(ctx, mailingList)
}

// GetKeyRevision gets revision for a key
func (w *TestMockMailingListWriter) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	mockWriter := mock.NewMockGrpsIOWriter(w.mock)
	return mockWriter.GetKeyRevision(ctx, key)
}

// Delete deletes a key with revision
func (w *TestMockMailingListWriter) Delete(ctx context.Context, key string, revision uint64) error {
	mockWriter := mock.NewMockGrpsIOWriter(w.mock)
	return mockWriter.Delete(ctx, key, revision)
}

func TestGrpsIOWriterOrchestrator_CreateGrpsIOMailingList(t *testing.T) {
	testCases := []struct {
		name             string
		setupMock        func(*mock.MockRepository)
		inputMailingList *model.GrpsIOMailingList
		expectedError    error
		validate         func(t *testing.T, result *model.GrpsIOMailingList, mockRepo *mock.MockRepository)
	}{
		{
			name: "successful mailing list creation without committee",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()

				// Add parent service
				service := &model.GrpsIOService{
					UID:         "service-1",
					Type:        "primary",
					ProjectUID:  "project-1",
					ProjectName: "Test Project",
					ProjectSlug: "test-project",
					Prefix:      "",
					Domain:      "lists.test.org",
					GroupName:   "test-project",
					Public:      true,
					Status:      "created",
				}
				mockRepo.AddService(service)
			},
			inputMailingList: &model.GrpsIOMailingList{
				GroupName:   "announce",
				Public:      true,
				Type:        "announcement",
				Description: "Test announcement mailing list for the project",
				Title:       "Test Announcements",
				ServiceUID:  "service-1",
			},
			expectedError: nil,
			validate: func(t *testing.T, result *model.GrpsIOMailingList, mockRepo *mock.MockRepository) {
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, "project-1", result.ProjectUID)
				assert.Equal(t, "Test Project", result.ProjectName)
				assert.Equal(t, "test-project", result.ProjectSlug)
				assert.Equal(t, "announce", result.GroupName)
				assert.Equal(t, "service-1", result.ServiceUID)
				assert.Empty(t, result.CommitteeUID)
				assert.Empty(t, result.CommitteeName)
				assert.Equal(t, 1, mockRepo.GetMailingListCount())
			},
		},
		{
			name: "successful mailing list creation with committee",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()

				// Add parent service
				service := &model.GrpsIOService{
					UID:         "service-1",
					Type:        "primary",
					ProjectUID:  "project-1",
					ProjectName: "Test Project",
					ProjectSlug: "test-project",
					Prefix:      "",
					Domain:      "lists.test.org",
					GroupName:   "test-project",
					Public:      true,
					Status:      "created",
				}
				mockRepo.AddService(service)

				// Add committee
				mockRepo.AddCommittee("committee-1", "Technical Committee")
			},
			inputMailingList: &model.GrpsIOMailingList{
				GroupName:    "tsc-discuss",
				Public:       false,
				Type:         "discussion_moderated",
				CommitteeUID: "committee-1",
				Description:  "Technical Steering Committee discussion list",
				Title:        "TSC Discussion",
				ServiceUID:   "service-1",
			},
			expectedError: nil,
			validate: func(t *testing.T, result *model.GrpsIOMailingList, mockRepo *mock.MockRepository) {
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, "committee-1", result.CommitteeUID)
				assert.Equal(t, "Technical Committee", result.CommitteeName)
				assert.Equal(t, "tsc-discuss", result.GroupName)
				assert.False(t, result.Public)
				assert.Equal(t, "discussion_moderated", result.Type)
				assert.Equal(t, 1, mockRepo.GetMailingListCount())
			},
		},
		{
			name: "successful creation with formation service prefix validation",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()

				// Add formation service with prefix
				service := &model.GrpsIOService{
					UID:         "service-2",
					Type:        "formation",
					ProjectUID:  "project-2",
					ProjectName: "Formation Project",
					ProjectSlug: "formation-project",
					Prefix:      "form",
					Domain:      "lists.formation.org",
					GroupName:   "formation-project",
					Public:      true,
					Status:      "created",
				}
				mockRepo.AddService(service)
			},
			inputMailingList: &model.GrpsIOMailingList{
				GroupName:   "form-announce",
				Public:      true,
				Type:        "announcement",
				Description: "Formation project announcements",
				Title:       "Formation Announcements",
				ServiceUID:  "service-2",
			},
			expectedError: nil,
			validate: func(t *testing.T, result *model.GrpsIOMailingList, mockRepo *mock.MockRepository) {
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, "form-announce", result.GroupName)
				assert.Equal(t, "project-2", result.ProjectUID)
				assert.Equal(t, 1, mockRepo.GetMailingListCount())
			},
		},
		{
			name: "parent service not found error",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				// Don't add any services
			},
			inputMailingList: &model.GrpsIOMailingList{
				GroupName:   "announce",
				Public:      true,
				Type:        "announcement",
				Description: "Test announcement mailing list",
				Title:       "Test Announcements",
				ServiceUID:  "nonexistent-service",
			},
			expectedError: errs.NotFound{},
			validate: func(t *testing.T, result *model.GrpsIOMailingList, mockRepo *mock.MockRepository) {
				assert.Nil(t, result)
				assert.Equal(t, 0, mockRepo.GetMailingListCount())
			},
		},
		{
			name: "committee not found error",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()

				// Add parent service
				service := &model.GrpsIOService{
					UID:         "service-1",
					Type:        "primary",
					ProjectUID:  "project-1",
					ProjectName: "Test Project",
					ProjectSlug: "test-project",
					Prefix:      "",
					Domain:      "lists.test.org",
					GroupName:   "test-project",
					Public:      true,
					Status:      "created",
				}
				mockRepo.AddService(service)
				// Don't add committee
			},
			inputMailingList: &model.GrpsIOMailingList{
				GroupName:    "committee-discuss",
				Public:       false,
				Type:         "discussion_moderated",
				CommitteeUID: "nonexistent-committee",
				Description:  "Committee discussion list",
				Title:        "Committee Discussion",
				ServiceUID:   "service-1",
			},
			expectedError: errs.NotFound{},
			validate: func(t *testing.T, result *model.GrpsIOMailingList, mockRepo *mock.MockRepository) {
				assert.Nil(t, result)
				assert.Equal(t, 0, mockRepo.GetMailingListCount())
			},
		},
		{
			name: "group name already exists error",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()

				// Add parent service
				service := &model.GrpsIOService{
					UID:         "service-1",
					Type:        "primary",
					ProjectUID:  "project-1",
					ProjectName: "Test Project",
					ProjectSlug: "test-project",
					Prefix:      "",
					Domain:      "lists.test.org",
					GroupName:   "test-project",
					Public:      true,
					Status:      "created",
				}
				mockRepo.AddService(service)

				// Add existing mailing list with same group name
				existingMailingList := &model.GrpsIOMailingList{
					UID:         "existing-list",
					GroupName:   "announce",
					ServiceUID:  "service-1",
					ProjectUID:  "project-1",
					Type:        "announcement",
					Description: "Existing announcement list",
					Title:       "Existing Announcements",
					Public:      true,
					CreatedAt:   time.Now().Add(-24 * time.Hour),
					UpdatedAt:   time.Now(),
				}
				mockRepo.AddMailingList(existingMailingList)
			},
			inputMailingList: &model.GrpsIOMailingList{
				GroupName:   "announce", // Same group name as existing
				Public:      true,
				Type:        "announcement",
				Description: "New announcement list",
				Title:       "New Announcements",
				ServiceUID:  "service-1",
			},
			expectedError: errs.Conflict{},
			validate: func(t *testing.T, result *model.GrpsIOMailingList, mockRepo *mock.MockRepository) {
				assert.Nil(t, result)
				assert.Equal(t, 1, mockRepo.GetMailingListCount()) // Only the existing one
			},
		},
		{
			name: "invalid group name prefix for formation service",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()

				// Add formation service with prefix
				service := &model.GrpsIOService{
					UID:         "service-2",
					Type:        "formation",
					ProjectUID:  "project-2",
					ProjectName: "Formation Project",
					ProjectSlug: "formation-project",
					Prefix:      "form",
					Domain:      "lists.formation.org",
					GroupName:   "formation-project",
					Public:      true,
					Status:      "created",
				}
				mockRepo.AddService(service)
			},
			inputMailingList: &model.GrpsIOMailingList{
				GroupName:   "announce", // Should be form-announce for formation service
				Public:      true,
				Type:        "announcement",
				Description: "Invalid group name without prefix",
				Title:       "Invalid Announcements",
				ServiceUID:  "service-2",
			},
			expectedError: errs.Validation{},
			validate: func(t *testing.T, result *model.GrpsIOMailingList, mockRepo *mock.MockRepository) {
				assert.Nil(t, result)
				assert.Equal(t, 0, mockRepo.GetMailingListCount())
			},
		},
		{
			name: "description too short validation error",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()

				// Add parent service
				service := &model.GrpsIOService{
					UID:         "service-1",
					Type:        "primary",
					ProjectUID:  "project-1",
					ProjectName: "Test Project",
					ProjectSlug: "test-project",
					Prefix:      "",
					Domain:      "lists.test.org",
					GroupName:   "test-project",
					Public:      true,
					Status:      "created",
				}
				mockRepo.AddService(service)
			},
			inputMailingList: &model.GrpsIOMailingList{
				GroupName:   "announce",
				Public:      true,
				Type:        "announcement",
				Description: "Short", // Too short (less than 11 characters)
				Title:       "Test Announcements",
				ServiceUID:  "service-1",
			},
			expectedError: errs.Validation{},
			validate: func(t *testing.T, result *model.GrpsIOMailingList, mockRepo *mock.MockRepository) {
				assert.Nil(t, result)
				assert.Equal(t, 0, mockRepo.GetMailingListCount())
			},
		},
		{
			name: "invalid mailing list type validation error",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()

				// Add parent service
				service := &model.GrpsIOService{
					UID:         "service-1",
					Type:        "primary",
					ProjectUID:  "project-1",
					ProjectName: "Test Project",
					ProjectSlug: "test-project",
					Prefix:      "",
					Domain:      "lists.test.org",
					GroupName:   "test-project",
					Public:      true,
					Status:      "created",
				}
				mockRepo.AddService(service)
			},
			inputMailingList: &model.GrpsIOMailingList{
				GroupName:   "announce",
				Public:      true,
				Type:        "invalid_type", // Invalid type
				Description: "Test announcement mailing list",
				Title:       "Test Announcements",
				ServiceUID:  "service-1",
			},
			expectedError: errs.Validation{},
			validate: func(t *testing.T, result *model.GrpsIOMailingList, mockRepo *mock.MockRepository) {
				assert.Nil(t, result)
				assert.Equal(t, 0, mockRepo.GetMailingListCount())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockRepo := mock.NewMockRepository()
			tc.setupMock(mockRepo)

			grpsIOReader := mock.NewMockGrpsIOReader(mockRepo)
			grpsIOWriter := mock.NewMockGrpsIOWriter(mockRepo)
			entityReader := mock.NewMockEntityAttributeReader(mockRepo)
			publisher := mock.NewMockMessagePublisher()

			orchestrator := NewGrpsIOWriterOrchestrator(
				WithGrpsIOWriterReader(grpsIOReader),
				WithGrpsIOWriter(grpsIOWriter),
				WithEntityAttributeReader(entityReader),
				WithPublisher(publisher),
			)

			// Execute
			ctx := context.Background()
			result, revision, err := orchestrator.CreateGrpsIOMailingList(ctx, tc.inputMailingList)

			// Validate
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.IsType(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Greater(t, revision, uint64(0), "revision should be greater than 0")
			}

			tc.validate(t, result, mockRepo)
		})
	}
}

// MockMessagePublisherWithError is a mock publisher that can return errors for testing
type MockMessagePublisherWithError struct {
	indexerError error
	accessError  error
}

func (p *MockMessagePublisherWithError) Indexer(ctx context.Context, subject string, message interface{}) error {
	if p.indexerError != nil {
		return p.indexerError
	}
	return nil
}

func (p *MockMessagePublisherWithError) Access(ctx context.Context, subject string, message interface{}) error {
	if p.accessError != nil {
		return p.accessError
	}
	return nil
}

func TestGrpsIOWriterOrchestrator_CreateGrpsIOMailingList_PublishingErrors(t *testing.T) {
	testCases := []struct {
		name           string
		indexerError   error
		accessError    error
		expectComplete bool // Should mailing list still be created despite publishing errors?
	}{
		{
			name:           "indexer error does not fail creation",
			indexerError:   errors.New("indexer publishing failed"),
			accessError:    nil,
			expectComplete: true,
		},
		{
			name:           "access error does not fail creation",
			indexerError:   nil,
			accessError:    errors.New("access publishing failed"),
			expectComplete: true,
		},
		{
			name:           "both publishing errors do not fail creation",
			indexerError:   errors.New("indexer publishing failed"),
			accessError:    errors.New("access publishing failed"),
			expectComplete: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockRepo := mock.NewMockRepository()
			mockRepo.ClearAll()

			// Add parent service
			service := &model.GrpsIOService{
				UID:         "service-1",
				Type:        "primary",
				ProjectUID:  "project-1",
				ProjectName: "Test Project",
				ProjectSlug: "test-project",
				Prefix:      "",
				Domain:      "lists.test.org",
				GroupName:   "test-project",
				Public:      true,
				Status:      "created",
			}
			mockRepo.AddService(service)

			grpsIOReader := mock.NewMockGrpsIOReader(mockRepo)
			grpsIOWriter := mock.NewMockGrpsIOWriter(mockRepo)
			entityReader := mock.NewMockEntityAttributeReader(mockRepo)

			// Use custom publisher that can return errors
			publisher := &MockMessagePublisherWithError{
				indexerError: tc.indexerError,
				accessError:  tc.accessError,
			}

			orchestrator := NewGrpsIOWriterOrchestrator(
				WithGrpsIOWriterReader(grpsIOReader),
				WithGrpsIOWriter(grpsIOWriter),
				WithEntityAttributeReader(entityReader),
				WithPublisher(publisher),
			)

			mailingList := &model.GrpsIOMailingList{
				GroupName:   "announce",
				Public:      true,
				Type:        "announcement",
				Description: "Test announcement mailing list for publishing errors",
				Title:       "Test Announcements",
				ServiceUID:  "service-1",
			}

			// Execute
			ctx := context.Background()
			result, revision, err := orchestrator.CreateGrpsIOMailingList(ctx, mailingList)

			// Validate
			if tc.expectComplete {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.UID)
				assert.Greater(t, revision, uint64(0), "revision should be greater than 0")
				assert.Equal(t, 1, mockRepo.GetMailingListCount())
			} else {
				assert.Error(t, err)
				assert.Nil(t, result)
			}
		})
	}
}

func TestGrpsIOWriterOrchestrator_buildMailingListIndexerMessage(t *testing.T) {
	testCases := []struct {
		name          string
		mailingList   *model.GrpsIOMailingList
		expectedError bool
	}{
		{
			name: "successful indexer message build",
			mailingList: &model.GrpsIOMailingList{
				UID:         "test-list",
				ServiceUID:  "test-service",
				GroupName:   "announce",
				ProjectUID:  "test-project",
				Type:        "announcement",
				Public:      true,
				Description: "Test announcement list",
				Title:       "Test Announcements",
			},
			expectedError: false,
		},
		{
			name:          "build with nil mailing list",
			mailingList:   nil,
			expectedError: false, // The Build method doesn't validate nil input
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			orchestrator := &grpsIOWriterOrchestrator{}
			ctx := context.Background()

			// Execute
			result, err := orchestrator.buildMailingListIndexerMessage(ctx, tc.mailingList, model.ActionCreated)

			// Validate
			if tc.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, model.ActionCreated, result.Action)

				// For nil mailing list case, validate that data is nil
				if tc.mailingList == nil {
					assert.Nil(t, result.Data)
				}
			}
		})
	}
}

func TestGrpsIOWriterOrchestrator_buildMailingListAccessControlMessage(t *testing.T) {
	testCases := []struct {
		name        string
		mailingList *model.GrpsIOMailingList
		expected    *model.AccessMessage
	}{
		{
			name: "mailing list without committee",
			mailingList: &model.GrpsIOMailingList{
				UID:        "list-1",
				ServiceUID: "service-1",
				ProjectUID: "project-1",
				Public:     true,
			},
			expected: &model.AccessMessage{
				UID:        "list-1",
				ObjectType: constants.ObjectTypeGroupsIOMailingList,
				Public:     true,
				Relations:  map[string][]string{},
				References: map[string]string{
					constants.RelationGroupsIOService: "service-1",
				},
			},
		},
		{
			name: "mailing list with committee",
			mailingList: &model.GrpsIOMailingList{
				UID:          "list-2",
				ServiceUID:   "service-2",
				ProjectUID:   "project-2",
				CommitteeUID: "committee-1",
				Public:       false,
			},
			expected: &model.AccessMessage{
				UID:        "list-2",
				ObjectType: constants.ObjectTypeGroupsIOMailingList,
				Public:     false,
				Relations:  map[string][]string{},
				References: map[string]string{
					"committee":                       "committee-1",
					constants.RelationGroupsIOService: "service-2",
				},
			},
		},
		{
			name: "mailing list with writers",
			mailingList: &model.GrpsIOMailingList{
				UID:        "list-3",
				ServiceUID: "service-3",
				ProjectUID: "project-3",
				Public:     true,
				Writers:    []string{"user1", "user2"},
			},
			expected: &model.AccessMessage{
				UID:        "list-3",
				ObjectType: constants.ObjectTypeGroupsIOMailingList,
				Public:     true,
				Relations: map[string][]string{
					constants.RelationWriter: {"user1", "user2"},
				},
				References: map[string]string{
					constants.RelationGroupsIOService: "service-3",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			orchestrator := &grpsIOWriterOrchestrator{}

			// Execute
			result := orchestrator.buildMailingListAccessControlMessage(tc.mailingList)

			// Validate
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestGrpsIOWriterOrchestrator_UpdateGrpsIOMailingList tests the update functionality
func TestGrpsIOWriterOrchestrator_UpdateGrpsIOMailingList(t *testing.T) {
	testCases := []struct {
		name               string
		setupMock          func(*mock.MockRepository)
		existingUID        string
		updatedMailingList *model.GrpsIOMailingList
		expectedRevision   uint64
		expectedError      error
		validate           func(*testing.T, *model.GrpsIOMailingList, *mock.MockRepository)
	}{
		{
			name: "successful_update_without_committee_change_preserves_name",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()

				// Add parent service
				service := &model.GrpsIOService{
					UID:         "service-1",
					Type:        "primary",
					ProjectUID:  "project-1",
					ProjectName: "Test Project",
					ProjectSlug: "test-project",
					Domain:      "lists.test.org",
					GroupName:   "test-project",
					Public:      true,
					Status:      "created",
				}
				mockRepo.AddService(service)

				// Add committee
				mockRepo.AddCommittee("committee-1", "Technical Steering Committee")

				// Create existing mailing list
				existing := &model.GrpsIOMailingList{
					UID:           "list-1",
					GroupName:     "tsc-discuss",
					Public:        false,
					Type:          "discussion_moderated",
					CommitteeUID:  "committee-1",
					CommitteeName: "Technical Steering Committee",
					Description:   "Technical steering committee discussions",
					Title:         "TSC Discussion List",
					ServiceUID:    "service-1",
					ProjectUID:    "project-1",
					ProjectName:   "Test Project",
					ProjectSlug:   "test-project",
					CreatedAt:     time.Now().Add(-time.Hour),
					UpdatedAt:     time.Now().Add(-time.Hour),
				}
				mockRepo.AddMailingList(existing)
			},
			existingUID: "list-1",
			updatedMailingList: &model.GrpsIOMailingList{
				GroupName:    "tsc-discuss",
				Public:       true, // Changed
				Type:         "discussion_moderated",
				CommitteeUID: "committee-1",                                      // Same committee
				Description:  "Updated technical steering committee discussions", // Changed
				Title:        "TSC Discussion List",
				ServiceUID:   "service-1",
			},
			expectedRevision: 1,
			expectedError:    nil,
			validate: func(t *testing.T, result *model.GrpsIOMailingList, mockRepo *mock.MockRepository) {
				assert.Equal(t, "committee-1", result.CommitteeUID)
				assert.Equal(t, "Technical Steering Committee", result.CommitteeName)                   // Should be preserved
				assert.Equal(t, true, result.Public)                                                    // Should be updated
				assert.Equal(t, "Updated technical steering committee discussions", result.Description) // Should be updated
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockRepo := mock.NewMockRepository()
			tc.setupMock(mockRepo)

			grpsIOReader := mock.NewMockGrpsIOReader(mockRepo)
			grpsIOWriter := mock.NewMockGrpsIOWriter(mockRepo)
			entityReader := mock.NewMockEntityAttributeReader(mockRepo)
			publisher := mock.NewMockMessagePublisher()

			orchestrator := NewGrpsIOWriterOrchestrator(
				WithGrpsIOWriterReader(grpsIOReader),
				WithGrpsIOWriter(grpsIOWriter),
				WithEntityAttributeReader(entityReader),
				WithPublisher(publisher),
			)

			// Execute
			ctx := context.Background()
			result, revision, err := orchestrator.UpdateGrpsIOMailingList(ctx, tc.existingUID, tc.updatedMailingList, tc.expectedRevision)

			// Validate
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.IsType(t, tc.expectedError, err)
				assert.Nil(t, result)
				assert.Equal(t, uint64(0), revision)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Greater(t, revision, uint64(0))

				// Validate immutable fields are preserved
				assert.Equal(t, tc.existingUID, result.UID)
				assert.Equal(t, tc.updatedMailingList.ServiceUID, result.ServiceUID)
			}

			tc.validate(t, result, mockRepo)
		})
	}
}

// TestGrpsIOWriterOrchestrator_mergeMailingListData tests the merge logic in isolation
func TestGrpsIOWriterOrchestrator_mergeMailingListData(t *testing.T) {
	testCases := []struct {
		name     string
		existing *model.GrpsIOMailingList
		updated  *model.GrpsIOMailingList
		validate func(*testing.T, *model.GrpsIOMailingList)
	}{
		{
			name: "preserve_committee_name_when_uid_unchanged",
			existing: &model.GrpsIOMailingList{
				UID:           "list-1",
				GroupName:     "tsc-discuss",
				CommitteeUID:  "committee-1",
				CommitteeName: "Technical Steering Committee",
				CreatedAt:     time.Now().Add(-time.Hour),
				ServiceUID:    "service-1",
				ProjectUID:    "project-1",
				ProjectName:   "Test Project",
				ProjectSlug:   "test-project",
			},
			updated: &model.GrpsIOMailingList{
				CommitteeUID: "committee-1", // Same committee
				Public:       true,          // Changed field
			},
			validate: func(t *testing.T, result *model.GrpsIOMailingList) {
				assert.Equal(t, "Technical Steering Committee", result.CommitteeName)
				assert.Equal(t, "committee-1", result.CommitteeUID)
				assert.Equal(t, true, result.Public)
			},
		},
		{
			name: "clear_committee_name_when_uid_removed",
			existing: &model.GrpsIOMailingList{
				UID:           "list-2",
				GroupName:     "general-discuss",
				CommitteeUID:  "committee-1",
				CommitteeName: "Technical Steering Committee",
				CreatedAt:     time.Now().Add(-time.Hour),
				ServiceUID:    "service-1",
				ProjectUID:    "project-1",
				ProjectName:   "Test Project",
				ProjectSlug:   "test-project",
			},
			updated: &model.GrpsIOMailingList{
				CommitteeUID: "", // Empty committee (removing committee)
				Public:       true,
			},
			validate: func(t *testing.T, result *model.GrpsIOMailingList) {
				assert.Equal(t, "", result.CommitteeName, "Committee name should be cleared when UID is removed")
				assert.Equal(t, "", result.CommitteeUID)
				assert.Equal(t, true, result.Public)
			},
		},
		{
			name: "use_new_committee_name_when_uid_changes",
			existing: &model.GrpsIOMailingList{
				UID:           "list-3",
				GroupName:     "committee-discuss",
				CommitteeUID:  "committee-1",
				CommitteeName: "Technical Steering Committee",
				CreatedAt:     time.Now().Add(-time.Hour),
				ServiceUID:    "service-1",
				ProjectUID:    "project-1",
				ProjectName:   "Test Project",
				ProjectSlug:   "test-project",
			},
			updated: &model.GrpsIOMailingList{
				CommitteeUID:  "committee-2",          // Different committee
				CommitteeName: "Governance Committee", // New committee name
				Public:        true,
			},
			validate: func(t *testing.T, result *model.GrpsIOMailingList) {
				// When CommitteeUID changes, new CommitteeName should be used as provided
				assert.Equal(t, "Governance Committee", result.CommitteeName)
				assert.Equal(t, "committee-2", result.CommitteeUID)
				assert.Equal(t, true, result.Public)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			ctx := context.Background()
			orchestrator := &grpsIOWriterOrchestrator{}

			// Execute
			orchestrator.mergeMailingListData(ctx, tc.existing, tc.updated)

			// Validate immutable fields are always preserved
			assert.Equal(t, tc.existing.UID, tc.updated.UID)
			assert.Equal(t, tc.existing.CreatedAt, tc.updated.CreatedAt)
			assert.Equal(t, tc.existing.ServiceUID, tc.updated.ServiceUID)
			assert.Equal(t, tc.existing.GroupName, tc.updated.GroupName)
			assert.Equal(t, tc.existing.ProjectUID, tc.updated.ProjectUID)
			assert.Equal(t, tc.existing.ProjectName, tc.updated.ProjectName)
			assert.Equal(t, tc.existing.ProjectSlug, tc.updated.ProjectSlug)

			// UpdatedAt should be set to current time
			assert.True(t, tc.updated.UpdatedAt.After(tc.existing.CreatedAt))

			// Run custom validation
			tc.validate(t, tc.updated)
		})
	}
}

// Helper functions (if needed in future)
