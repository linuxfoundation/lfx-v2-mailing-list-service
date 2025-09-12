// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/mock"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

func TestGrpsIOWriterOrchestrator_CreateGrpsIOService(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*mock.MockRepository)
		inputService  *model.GrpsIOService
		expectedError error
		validate      func(t *testing.T, result *model.GrpsIOService, revision uint64, mockRepo *mock.MockRepository)
	}{
		{
			name: "successful primary service creation",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				mockRepo.AddProject("project-1", "test-project", "Test Project")
			},
			inputService: &model.GrpsIOService{
				Type:         "primary",
				Domain:       "lists.test.org",
				GroupID:      12345,
				GlobalOwners: []string{"admin@test.org"},
				Prefix:       "",
				ProjectUID:   "project-1",
				URL:          "https://lists.test.org",
				GroupName:    "test-project",
				Public:       true,
				Status:       "created",
			},
			expectedError: nil,
			validate: func(t *testing.T, result *model.GrpsIOService, revision uint64, mockRepo *mock.MockRepository) {
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, "primary", result.Type)
				assert.Equal(t, "project-1", result.ProjectUID)
				assert.Equal(t, "Test Project", result.ProjectName)
				assert.Equal(t, "test-project", result.ProjectSlug)
				assert.Equal(t, "lists.test.org", result.Domain)
				assert.Equal(t, uint64(1), revision)
				assert.Equal(t, 1, mockRepo.GetServiceCount())
			},
		},
		{
			name: "successful formation service creation with prefix",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				mockRepo.AddProject("project-2", "formation-project", "Formation Project")
			},
			inputService: &model.GrpsIOService{
				Type:         "formation",
				Domain:       "lists.formation.org",
				GroupID:      23456,
				GlobalOwners: []string{"admin@formation.org"},
				Prefix:       "form",
				ProjectUID:   "project-2",
				URL:          "https://lists.formation.org",
				GroupName:    "formation-project",
				Public:       true,
				Status:       "created",
			},
			expectedError: nil,
			validate: func(t *testing.T, result *model.GrpsIOService, revision uint64, mockRepo *mock.MockRepository) {
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, "formation", result.Type)
				assert.Equal(t, "form", result.Prefix)
				assert.Equal(t, "project-2", result.ProjectUID)
				assert.Equal(t, "Formation Project", result.ProjectName)
				assert.Equal(t, "formation-project", result.ProjectSlug)
				assert.Equal(t, uint64(1), revision)
				assert.Equal(t, 1, mockRepo.GetServiceCount())
			},
		},
		{
			name: "successful shared service creation with group ID",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				mockRepo.AddProject("project-3", "shared-project", "Shared Project")
			},
			inputService: &model.GrpsIOService{
				Type:         "shared",
				Domain:       "lists.shared.org",
				GroupID:      34567,
				GlobalOwners: []string{"admin@shared.org"},
				Prefix:       "",
				ProjectUID:   "project-3",
				URL:          "https://lists.shared.org",
				GroupName:    "shared-project",
				Public:       false,
				Status:       "created",
			},
			expectedError: nil,
			validate: func(t *testing.T, result *model.GrpsIOService, revision uint64, mockRepo *mock.MockRepository) {
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, "shared", result.Type)
				assert.Equal(t, int64(34567), result.GroupID)
				assert.False(t, result.Public)
				assert.Equal(t, "project-3", result.ProjectUID)
				assert.Equal(t, uint64(1), revision)
				assert.Equal(t, 1, mockRepo.GetServiceCount())
			},
		},
		{
			name: "project not found error",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				// Don't add any projects
			},
			inputService: &model.GrpsIOService{
				Type:         "primary",
				Domain:       "lists.test.org",
				GroupID:      12345,
				GlobalOwners: []string{"admin@test.org"},
				Prefix:       "",
				ProjectUID:   "nonexistent-project",
				URL:          "https://lists.test.org",
				GroupName:    "test-project",
				Public:       true,
				Status:       "created",
			},
			expectedError: errs.NotFound{},
			validate: func(t *testing.T, result *model.GrpsIOService, revision uint64, mockRepo *mock.MockRepository) {
				assert.Nil(t, result)
				assert.Equal(t, uint64(0), revision)
				assert.Equal(t, 0, mockRepo.GetServiceCount())
			},
		},
		{
			name: "service with writers and auditors",
			setupMock: func(mockRepo *mock.MockRepository) {
				mockRepo.ClearAll()
				mockRepo.AddProject("project-4", "audit-project", "Audit Project")
			},
			inputService: &model.GrpsIOService{
				Type:         "primary",
				Domain:       "lists.audit.org",
				GroupID:      45678,
				GlobalOwners: []string{"admin@audit.org"},
				Writers:      []string{"writer1@audit.org", "writer2@audit.org"},
				Auditors:     []string{"auditor1@audit.org"},
				Prefix:       "",
				ProjectUID:   "project-4",
				URL:          "https://lists.audit.org",
				GroupName:    "audit-project",
				Public:       true,
				Status:       "created",
			},
			expectedError: nil,
			validate: func(t *testing.T, result *model.GrpsIOService, revision uint64, mockRepo *mock.MockRepository) {
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, []string{"writer1@audit.org", "writer2@audit.org"}, result.Writers)
				assert.Equal(t, []string{"auditor1@audit.org"}, result.Auditors)
				assert.Equal(t, 1, mockRepo.GetServiceCount())
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
			result, revision, err := orchestrator.CreateGrpsIOService(ctx, tc.inputService)

			// Validate
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.IsType(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}

			tc.validate(t, result, revision, mockRepo)
		})
	}
}

func TestGrpsIOWriterOrchestrator_CreateGrpsIOService_PublishingErrors(t *testing.T) {
	testCases := []struct {
		name           string
		indexerError   error
		accessError    error
		expectComplete bool // Should service still be created despite publishing errors?
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
			mockRepo.AddProject("project-1", "test-project", "Test Project")

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

			service := &model.GrpsIOService{
				Type:         "primary",
				Domain:       "lists.test.org",
				GroupID:      12345,
				GlobalOwners: []string{"admin@test.org"},
				Prefix:       "",
				ProjectUID:   "project-1",
				URL:          "https://lists.test.org",
				GroupName:    "test-project",
				Public:       true,
				Status:       "created",
			}

			// Execute
			ctx := context.Background()
			result, revision, err := orchestrator.CreateGrpsIOService(ctx, service)

			// Validate
			if tc.expectComplete {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.UID)
				assert.Equal(t, uint64(1), revision)
				assert.Equal(t, 1, mockRepo.GetServiceCount())
			} else {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Equal(t, uint64(0), revision)
			}
		})
	}
}

// Note: UpdateGrpsIOService tests are complex due to mock implementation limitations
// The update functionality is tested through integration tests

func TestGrpsIOWriterOrchestrator_DeleteGrpsIOService(t *testing.T) {
	testCases := []struct {
		name          string
		setupMock     func(*mock.MockRepository) (*model.GrpsIOService, uint64)
		uid           string
		revision      uint64
		expectedError error
		validate      func(t *testing.T, mockRepo *mock.MockRepository)
	}{
		{
			name: "successful service deletion",
			setupMock: func(mockRepo *mock.MockRepository) (*model.GrpsIOService, uint64) {
				mockRepo.AddProject("project-1", "test-project", "Test Project")

				service := &model.GrpsIOService{
					UID:          "service-1",
					Type:         "primary",
					Domain:       "lists.test.org",
					ProjectUID:   "project-1",
					ProjectName:  "Test Project",
					ProjectSlug:  "test-project",
					GroupName:    "test-project",
					GlobalOwners: []string{"admin@test.org"},
					Public:       true,
					Status:       "created",
				}
				mockRepo.AddService(service)
				return service, uint64(1)
			},
			uid:           "service-1",
			revision:      uint64(1),
			expectedError: nil,
			validate: func(t *testing.T, mockRepo *mock.MockRepository) {
				// Verify service is deleted by trying to get it
				_, _, err := mockRepo.GetGrpsIOService(context.Background(), "service-1")
				var notFoundErr errs.NotFound
				assert.True(t, errors.As(err, &notFoundErr), "Service should be deleted")
			},
		},
		{
			name: "delete non-existent service",
			setupMock: func(mockRepo *mock.MockRepository) (*model.GrpsIOService, uint64) {
				mockRepo.AddProject("project-1", "test-project", "Test Project")
				return nil, uint64(1)
			},
			uid:           "non-existent-service",
			revision:      uint64(1),
			expectedError: errs.NotFound{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockRepo := mock.NewMockRepository()
			mockRepo.ClearAll()

			var setupService *model.GrpsIOService
			if tc.setupMock != nil {
				setupService, _ = tc.setupMock(mockRepo)
			}

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
			err := orchestrator.DeleteGrpsIOService(ctx, tc.uid, tc.revision)

			// Validate
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.IsType(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
				if tc.validate != nil {
					tc.validate(t, mockRepo)
				}
			}

			// Additional validation for successful deletes
			if tc.expectedError == nil && setupService != nil {
				// Verify that the service is no longer accessible
				_, _, err := mockRepo.GetGrpsIOService(ctx, tc.uid)
				var notFoundErr errs.NotFound
				assert.True(t, errors.As(err, &notFoundErr), "Service should be deleted from repository")
			}
		})
	}
}

func TestGrpsIOWriterOrchestrator_UpdateGrpsIOService_ConflictHandling(t *testing.T) {
	testCases := []struct {
		name             string
		setupMock        func(*mock.MockRepository) (*model.GrpsIOService, uint64)
		uid              string
		expectedRevision uint64
		actualRevision   uint64
		expectedError    error
		validate         func(t *testing.T, mockRepo *mock.MockRepository)
	}{
		{
			name: "revision mismatch returns conflict error",
			setupMock: func(mockRepo *mock.MockRepository) (*model.GrpsIOService, uint64) {
				mockRepo.ClearAll()
				mockRepo.AddProject("project-1", "test-project", "Test Project")

				service := &model.GrpsIOService{
					UID:          "service-1",
					Type:         "primary",
					Domain:       "lists.test.org",
					ProjectUID:   "project-1",
					ProjectName:  "Test Project",
					ProjectSlug:  "test-project",
					GroupName:    "test-project",
					GlobalOwners: []string{"admin@test.org"},
					Public:       true,
					Status:       "created",
				}

				// Create service and simulate revision mismatch
				// First add the service normally (revision 1)
				mockRepo.AddService(service)
				// Then create another copy and update it to increment revision
				serviceCopy := *service
				serviceCopy.Status = "updated to increment revision"
				tempWriter := mock.NewMockGrpsIOServiceWriter(mockRepo)
				_, _, _ = tempWriter.UpdateGrpsIOService(context.Background(), service.UID, &serviceCopy, 1) //nolint:errcheck // Test setup
				// Now the service has revision 2, but client will try with revision 1

				return service, 2
			},
			uid:              "service-1",
			expectedRevision: 1, // Client thinks revision is 1
			actualRevision:   2, // But actual revision is 2
			expectedError:    errs.Conflict{},
			validate: func(t *testing.T, mockRepo *mock.MockRepository) {
				// Service should still exist with revision 2
				_, rev, err := mockRepo.GetGrpsIOService(context.Background(), "service-1")
				assert.NoError(t, err)
				assert.Equal(t, uint64(2), rev, "Revision should remain unchanged after conflict")
			},
		},
		{
			name: "service not found during update",
			setupMock: func(mockRepo *mock.MockRepository) (*model.GrpsIOService, uint64) {
				mockRepo.ClearAll()
				mockRepo.AddProject("project-1", "test-project", "Test Project")
				return nil, 0
			},
			uid:              "non-existent-service",
			expectedRevision: 1,
			actualRevision:   0,
			expectedError:    errs.NotFound{},
			validate: func(t *testing.T, mockRepo *mock.MockRepository) {
				assert.Equal(t, 0, mockRepo.GetServiceCount(), "No services should exist")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockRepo := mock.NewMockRepository()
			setupService, _ := tc.setupMock(mockRepo)

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

			// Prepare update data
			var updateService *model.GrpsIOService
			if setupService != nil {
				updateService = &model.GrpsIOService{
					UID:          tc.uid,
					Type:         setupService.Type,
					Domain:       setupService.Domain,
					ProjectUID:   setupService.ProjectUID,
					ProjectName:  setupService.ProjectName,
					ProjectSlug:  setupService.ProjectSlug,
					GroupName:    setupService.GroupName,
					GlobalOwners: []string{"updated@test.org"}, // Changed field
					Public:       setupService.Public,
					Status:       setupService.Status,
				}
			} else {
				updateService = &model.GrpsIOService{
					UID:          tc.uid,
					Type:         "primary",
					ProjectUID:   "project-1", // Add required fields to avoid validation errors
					GlobalOwners: []string{"admin@test.org"},
				}
			}

			// Execute
			ctx := context.Background()
			result, revision, err := orchestrator.UpdateGrpsIOService(ctx, tc.uid, updateService, tc.expectedRevision)

			// Validate error type
			if tc.expectedError != nil {
				assert.Error(t, err)

				switch tc.expectedError.(type) {
				case errs.Conflict:
					var conflictErr errs.Conflict
					assert.True(t, errors.As(err, &conflictErr), "Expected Conflict error, got %T", err)
				case errs.NotFound:
					var notFoundErr errs.NotFound
					assert.True(t, errors.As(err, &notFoundErr), "Expected NotFound error, got %T", err)
				}
				assert.Nil(t, result)
				assert.Equal(t, uint64(0), revision)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Greater(t, revision, uint64(0))
			}

			// Run additional validation
			if tc.validate != nil {
				tc.validate(t, mockRepo)
			}
		})
	}
}

func TestGrpsIOWriterOrchestrator_DeleteGrpsIOService_ConflictHandling(t *testing.T) {
	testCases := []struct {
		name             string
		setupMock        func(*mock.MockRepository) (*model.GrpsIOService, uint64)
		uid              string
		expectedRevision uint64
		expectedError    error
		validate         func(t *testing.T, mockRepo *mock.MockRepository)
	}{
		{
			name: "revision mismatch on delete returns conflict error",
			setupMock: func(mockRepo *mock.MockRepository) (*model.GrpsIOService, uint64) {
				mockRepo.ClearAll()
				mockRepo.AddProject("project-1", "test-project", "Test Project")

				service := &model.GrpsIOService{
					UID:          "service-1",
					Type:         "formation", // Non-primary service can be deleted
					Domain:       "lists.test.org",
					ProjectUID:   "project-1",
					ProjectName:  "Test Project",
					ProjectSlug:  "test-project",
					GroupName:    "form-test",
					Prefix:       "form-",
					GlobalOwners: []string{"admin@test.org"},
					Public:       true,
					Status:       "created",
				}

				// Create service and simulate revision mismatch
				// First add the service normally (revision 1)
				mockRepo.AddService(service)
				// Then update it twice to get revision 3
				serviceCopy := *service
				serviceCopy.Status = "updated to increment revision"
				tempWriter := mock.NewMockGrpsIOServiceWriter(mockRepo)
				_, _, _ = tempWriter.UpdateGrpsIOService(context.Background(), service.UID, &serviceCopy, 1) //nolint:errcheck // Test setup
				_, _, _ = tempWriter.UpdateGrpsIOService(context.Background(), service.UID, &serviceCopy, 2) //nolint:errcheck // Test setup
				// Now the service has revision 3, but client will try with revision 1

				return service, 3
			},
			uid:              "service-1",
			expectedRevision: 1, // Client thinks revision is 1
			expectedError:    errs.Conflict{},
			validate: func(t *testing.T, mockRepo *mock.MockRepository) {
				// Service should still exist after failed delete
				_, rev, err := mockRepo.GetGrpsIOService(context.Background(), "service-1")
				assert.NoError(t, err)
				assert.Equal(t, uint64(3), rev, "Service should still exist with original revision")
				assert.Equal(t, 1, mockRepo.GetServiceCount(), "Service should not be deleted")
			},
		},
		{
			name: "delete non-existent service returns not found",
			setupMock: func(mockRepo *mock.MockRepository) (*model.GrpsIOService, uint64) {
				mockRepo.ClearAll()
				return nil, 0
			},
			uid:              "non-existent-service",
			expectedRevision: 1,
			expectedError:    errs.NotFound{},
			validate: func(t *testing.T, mockRepo *mock.MockRepository) {
				assert.Equal(t, 0, mockRepo.GetServiceCount(), "No services should exist")
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
			err := orchestrator.DeleteGrpsIOService(ctx, tc.uid, tc.expectedRevision)

			// Validate error type
			if tc.expectedError != nil {
				assert.Error(t, err)

				switch tc.expectedError.(type) {
				case errs.Conflict:
					var conflictErr errs.Conflict
					assert.True(t, errors.As(err, &conflictErr), "Expected Conflict error, got %T", err)
				case errs.NotFound:
					var notFoundErr errs.NotFound
					assert.True(t, errors.As(err, &notFoundErr), "Expected NotFound error, got %T", err)
				}
			} else {
				assert.NoError(t, err)
			}

			// Run additional validation
			if tc.validate != nil {
				tc.validate(t, mockRepo)
			}
		})
	}
}

// Note: buildServiceIndexerMessage and buildServiceAccessControlMessage methods are private
// and are tested indirectly through the Create/Update/Delete operations above
