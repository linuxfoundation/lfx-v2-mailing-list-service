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

func TestGrpsIOReaderOrchestratorGetGrpsIOService(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()

	// Setup test data
	testServiceUID := uuid.New().String()
	testService := &model.GrpsIOService{
		Type:           "primary",
		UID:            testServiceUID,
		Domain:         "lists.testproject.org",
		GroupID:        12345,
		Status:         "created",
		GlobalOwners:   []string{"admin@testproject.org"},
		Prefix:         "",
		ProjectSlug:    "test-project",
		ProjectName:    "Test Project",
		ProjectUID:     "test-project-uid",
		URL:            "https://lists.testproject.org",
		GroupName:      "test-project",
		Public:         true,
		LastReviewedAt: serviceStringPtr("2024-01-01T00:00:00Z"),
		LastReviewedBy: serviceStringPtr("reviewer-uid"),
		Writers:        []string{"writer1", "writer2"},
		Auditors:       []string{"auditor1"},
		CreatedAt:      time.Now().Add(-24 * time.Hour),
		UpdatedAt:      time.Now(),
	}

	tests := []struct {
		name            string
		setupMock       func()
		serviceUID      string
		expectedError   bool
		errorType       error
		validateService func(*testing.T, *model.GrpsIOService, uint64)
	}{
		{
			name: "successful service retrieval",
			setupMock: func() {
				mockRepo.ClearAll()
				// Store the service in mock repository
				mockRepo.AddService(testService)
			},
			serviceUID:    testServiceUID,
			expectedError: false,
			validateService: func(t *testing.T, service *model.GrpsIOService, revision uint64) {
				assert.NotNil(t, service)
				assert.Equal(t, testServiceUID, service.UID)
				assert.Equal(t, "primary", service.Type)
				assert.Equal(t, "lists.testproject.org", service.Domain)
				assert.Equal(t, int64(12345), service.GroupID)
				assert.Equal(t, "created", service.Status)
				assert.Equal(t, []string{"admin@testproject.org"}, service.GlobalOwners)
				assert.Equal(t, "", service.Prefix)
				assert.Equal(t, "test-project", service.ProjectSlug)
				assert.Equal(t, "Test Project", service.ProjectName)
				assert.Equal(t, "test-project-uid", service.ProjectUID)
				assert.Equal(t, "https://lists.testproject.org", service.URL)
				assert.Equal(t, "test-project", service.GroupName)
				assert.True(t, service.Public)
				assert.NotNil(t, service.LastReviewedAt)
				assert.Equal(t, "2024-01-01T00:00:00Z", *service.LastReviewedAt)
				assert.NotNil(t, service.LastReviewedBy)
				assert.Equal(t, "reviewer-uid", *service.LastReviewedBy)
				assert.Equal(t, []string{"writer1", "writer2"}, service.Writers)
				assert.Equal(t, []string{"auditor1"}, service.Auditors)
				assert.NotZero(t, service.CreatedAt)
				assert.NotZero(t, service.UpdatedAt)
				assert.Equal(t, uint64(1), revision) // Mock returns revision 1
			},
		},
		{
			name: "service not found error",
			setupMock: func() {
				mockRepo.ClearAll()
				// Don't store any service
			},
			serviceUID:    "nonexistent-service-uid",
			expectedError: true,
			errorType:     errs.NotFound{},
			validateService: func(t *testing.T, service *model.GrpsIOService, revision uint64) {
				assert.Nil(t, service)
				assert.Equal(t, uint64(0), revision)
			},
		},
		{
			name: "empty service UID",
			setupMock: func() {
				mockRepo.ClearAll()
			},
			serviceUID:    "",
			expectedError: true,
			errorType:     errs.NotFound{},
			validateService: func(t *testing.T, service *model.GrpsIOService, revision uint64) {
				assert.Nil(t, service)
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
			service, revision, err := reader.GetGrpsIOService(ctx, tt.serviceUID)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorType != nil {
					assert.IsType(t, tt.errorType, err)
				}
			} else {
				require.NoError(t, err)
			}

			tt.validateService(t, service, revision)
		})
	}
}

func TestGrpsIOReaderOrchestratorGetRevision(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()

	// Setup test data
	testServiceUID := uuid.New().String()
	testService := &model.GrpsIOService{
		Type:        "primary",
		UID:         testServiceUID,
		Domain:      "lists.testproject.org",
		GroupID:     12345,
		Status:      "created",
		ProjectSlug: "test-project",
		ProjectUID:  "test-project-uid",
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		UpdatedAt:   time.Now(),
	}

	tests := []struct {
		name             string
		setupMock        func()
		serviceUID       string
		expectedError    bool
		errorType        error
		validateRevision func(*testing.T, uint64)
	}{
		{
			name: "successful revision retrieval",
			setupMock: func() {
				mockRepo.ClearAll()
				// Store the service in mock repository
				mockRepo.AddService(testService)
			},
			serviceUID:    testServiceUID,
			expectedError: false,
			validateRevision: func(t *testing.T, revision uint64) {
				assert.Equal(t, uint64(1), revision) // Mock returns revision 1
			},
		},
		{
			name: "service not found error",
			setupMock: func() {
				mockRepo.ClearAll()
				// Don't store any service
			},
			serviceUID:    "nonexistent-service-uid",
			expectedError: true,
			errorType:     errs.NotFound{},
			validateRevision: func(t *testing.T, revision uint64) {
				assert.Equal(t, uint64(0), revision)
			},
		},
		{
			name: "empty service UID",
			setupMock: func() {
				mockRepo.ClearAll()
			},
			serviceUID:    "",
			expectedError: true,
			errorType:     errs.NotFound{},
			validateRevision: func(t *testing.T, revision uint64) {
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
			revision, err := reader.GetRevision(ctx, tt.serviceUID)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorType != nil {
					assert.IsType(t, tt.errorType, err)
				}
			} else {
				require.NoError(t, err)
			}

			tt.validateRevision(t, revision)
		})
	}
}

func TestNewGrpsIOReaderOrchestrator(t *testing.T) {
	mockRepo := mock.NewMockRepository()

	tests := []struct {
		name     string
		options  []grpsIOReaderOrchestratorOption
		validate func(*testing.T, GrpsIOReader)
	}{
		{
			name:    "create with no options panics",
			options: []grpsIOReaderOrchestratorOption{},
			validate: func(t *testing.T, reader GrpsIOReader) {
				// This should never be called since we expect a panic
				t.Fatal("Expected panic but got reader")
			},
		},
		{
			name: "create with grpsio reader option",
			options: []grpsIOReaderOrchestratorOption{
				WithGrpsIOReader(mock.NewMockGrpsIOReader(mockRepo)),
			},
			validate: func(t *testing.T, reader GrpsIOReader) {
				assert.NotNil(t, reader)
				orchestrator, ok := reader.(*grpsIOReaderOrchestrator)
				assert.True(t, ok)
				assert.NotNil(t, orchestrator.grpsIOReader)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "create with no options panics" {
				assert.Panics(t, func() {
					NewGrpsIOReaderOrchestrator(tt.options...)
				}, "Expected panic when creating orchestrator without dependencies")
			} else {
				// Execute normally for other tests
				reader := NewGrpsIOReaderOrchestrator(tt.options...)
				// Validate
				tt.validate(t, reader)
			}
		})
	}
}

func TestGrpsIOReaderOrchestratorIntegration(t *testing.T) {
	ctx := context.Background()
	mockRepo := mock.NewMockRepository()
	mockRepo.ClearAll()

	// Setup test data
	testServiceUID := uuid.New().String()
	testService := &model.GrpsIOService{
		Type:           "formation",
		UID:            testServiceUID,
		Domain:         "lists.formation.testproject.org",
		GroupID:        67890,
		Status:         "created",
		GlobalOwners:   []string{"formation@testproject.org", "admin@testproject.org"},
		Prefix:         "formation",
		ProjectSlug:    "integration-test-project",
		ProjectName:    "Integration Test Project",
		ProjectUID:     "integration-test-project-uid",
		URL:            "https://lists.formation.testproject.org",
		GroupName:      "integration-test-project-formation",
		Public:         false,
		LastReviewedAt: serviceStringPtr("2024-02-01T12:00:00Z"),
		LastReviewedBy: serviceStringPtr("integration-reviewer"),
		Writers:        []string{"integration-writer1", "integration-writer2", "integration-writer3"},
		Auditors:       []string{"integration-auditor1", "integration-auditor2"},
		CreatedAt:      time.Now().Add(-48 * time.Hour),
		UpdatedAt:      time.Now().Add(-1 * time.Hour),
	}

	// Store the service
	mockRepo.AddService(testService)

	// Create reader orchestrator
	reader := NewGrpsIOReaderOrchestrator(
		WithGrpsIOReader(mock.NewMockGrpsIOReader(mockRepo)),
	)

	t.Run("get service and revision for same service", func(t *testing.T) {
		// Get service
		service, serviceRevision, err := reader.GetGrpsIOService(ctx, testServiceUID)
		require.NoError(t, err)
		require.NotNil(t, service)

		// Get revision
		revision, err := reader.GetRevision(ctx, testServiceUID)
		require.NoError(t, err)

		// Validate that both operations return consistent data
		assert.Equal(t, testServiceUID, service.UID)
		assert.Equal(t, serviceRevision, revision) // Should be same revision in mock

		// Validate complete service data
		assert.Equal(t, "formation", service.Type)
		assert.Equal(t, "lists.formation.testproject.org", service.Domain)
		assert.Equal(t, int64(67890), service.GroupID)
		assert.Equal(t, "created", service.Status)
		assert.Equal(t, []string{"formation@testproject.org", "admin@testproject.org"}, service.GlobalOwners)
		assert.Equal(t, "formation", service.Prefix)
		assert.Equal(t, "integration-test-project", service.ProjectSlug)
		assert.Equal(t, "Integration Test Project", service.ProjectName)
		assert.Equal(t, "integration-test-project-uid", service.ProjectUID)
		assert.Equal(t, "https://lists.formation.testproject.org", service.URL)
		assert.Equal(t, "integration-test-project-formation", service.GroupName)
		assert.False(t, service.Public)

		// Validate audit fields
		assert.NotNil(t, service.LastReviewedAt)
		assert.Equal(t, "2024-02-01T12:00:00Z", *service.LastReviewedAt)
		assert.NotNil(t, service.LastReviewedBy)
		assert.Equal(t, "integration-reviewer", *service.LastReviewedBy)
		assert.Equal(t, []string{"integration-writer1", "integration-writer2", "integration-writer3"}, service.Writers)
		assert.Equal(t, []string{"integration-auditor1", "integration-auditor2"}, service.Auditors)

		// Validate timestamps
		assert.False(t, service.CreatedAt.IsZero())
		assert.False(t, service.UpdatedAt.IsZero())
	})
}

// Helper function to create string pointer
func serviceStringPtr(s string) *string {
	return &s
}
