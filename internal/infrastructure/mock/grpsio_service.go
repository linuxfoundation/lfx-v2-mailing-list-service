// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// Global mock repository instance to share data between all repositories
var (
	globalMockRepo     *MockRepository
	globalMockRepoOnce = &sync.Once{}
)

// MockRepository provides a mock implementation of all repository interfaces for testing
type MockRepository struct {
	services         map[string]*model.GrpsIOService
	serviceRevisions map[string]uint64
	projectSlugs     map[string]string // projectUID -> slug
	projectNames     map[string]string // projectUID -> name
	mu               sync.RWMutex      // Protect concurrent access to maps
}

// NewMockRepository creates a new mock repository with sample data
func NewMockRepository() *MockRepository {

	globalMockRepoOnce.Do(func() {
		now := time.Now()

		mock := &MockRepository{
			services:         make(map[string]*model.GrpsIOService),
			serviceRevisions: make(map[string]uint64),
			projectSlugs:     make(map[string]string),
			projectNames:     make(map[string]string),
		}

		// Add sample project data for testing
		mock.projectSlugs = map[string]string{
			"550e8400-e29b-41d4-a716-446655440001": "primary-project",
			"550e8400-e29b-41d4-a716-446655440002": "formation-project",
			"550e8400-e29b-41d4-a716-446655440003": "shared-project",
			"550e8400-e29b-41d4-a716-446655440004": "error-project",
			"550e8400-e29b-41d4-a716-446655440005": "get-test-project",
			"66666666-6666-6666-6666-666666666666": "delete-test-project",
			"7cad5a8d-19d0-41a4-81a6-043453daf9ee": "sample-project",
		}

		mock.projectNames = map[string]string{
			"550e8400-e29b-41d4-a716-446655440001": "Primary Test Project",
			"550e8400-e29b-41d4-a716-446655440002": "Formation Test Project",
			"550e8400-e29b-41d4-a716-446655440003": "Shared Test Project",
			"550e8400-e29b-41d4-a716-446655440004": "Error Test Project",
			"550e8400-e29b-41d4-a716-446655440005": "Get Test Project",
			"66666666-6666-6666-6666-666666666666": "Delete Test Project",
			"7cad5a8d-19d0-41a4-81a6-043453daf9ee": "Cloud Native Computing Foundation",
		}

		// Add sample data for testing
		sampleServices := []*model.GrpsIOService{
			{
				Type:         "primary",
				UID:          "service-1",
				Domain:       "lists.testproject.org",
				GroupID:      12345,
				Status:       "created",
				GlobalOwners: []string{"admin@testproject.org"},
				Prefix:       "",
				ProjectSlug:  "test-project",
				ProjectUID:   "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
				URL:          "https://lists.testproject.org",
				GroupName:    "test-project",
				Public:       true,
				CreatedAt:    now.Add(-24 * time.Hour),
				UpdatedAt:    now,
			},
			{
				Type:         "formation",
				UID:          "service-2",
				Domain:       "lists.formation.testproject.org",
				GroupID:      12346,
				Status:       "created",
				GlobalOwners: []string{"formation@testproject.org"},
				Prefix:       "formation",
				ProjectSlug:  "test-project",
				ProjectUID:   "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
				URL:          "https://lists.formation.testproject.org",
				GroupName:    "test-project-formation",
				Public:       false,
				CreatedAt:    now.Add(-12 * time.Hour),
				UpdatedAt:    now,
			},
			{
				Type:         "primary",
				UID:          "service-3",
				Domain:       "lists.example.org",
				GroupID:      12347,
				Status:       "pending",
				GlobalOwners: []string{"owner@example.org", "admin@example.org"},
				Prefix:       "",
				ProjectSlug:  "example-project",
				ProjectUID:   "8dbc6b9e-20e1-42b5-92b7-154564eaf0ff",
				URL:          "https://lists.example.org",
				GroupName:    "example-project",
				Public:       true,
				CreatedAt:    now.Add(-6 * time.Hour),
				UpdatedAt:    now.Add(-1 * time.Hour),
			},
		}

		// Store services by ID
		for _, service := range sampleServices {
			mock.services[service.UID] = service
			mock.serviceRevisions[service.UID] = 1
		}

		// Add project mappings
		mock.projectSlugs["7cad5a8d-19d0-41a4-81a6-043453daf9ee"] = "test-project"
		mock.projectNames["7cad5a8d-19d0-41a4-81a6-043453daf9ee"] = "Test Project"
		mock.projectSlugs["8dbc6b9e-20e1-42b5-92b7-154564eaf0ff"] = "example-project"
		mock.projectNames["8dbc6b9e-20e1-42b5-92b7-154564eaf0ff"] = "Example Project"

		globalMockRepo = mock
	})

	return globalMockRepo
}

// MockGrpsIOServiceWriter implements GrpsIOServiceWriter interface
type MockGrpsIOServiceWriter struct {
	mock *MockRepository
}

// MockProjectRetriever implements ProjectRetriever interface
type MockProjectRetriever struct {
	mock *MockRepository
}

// Name returns the project name for a given UID
func (r *MockProjectRetriever) Name(ctx context.Context, uid string) (string, error) {
	slog.DebugContext(ctx, "mock project retriever: getting name", "uid", uid)

	r.mock.mu.RLock()
	defer r.mock.mu.RUnlock()

	name, exists := r.mock.projectNames[uid]
	if !exists {
		return "", errors.NewNotFound(fmt.Sprintf("project with UID %s not found", uid))
	}

	return name, nil
}

// Slug returns the project slug for a given UID
func (r *MockProjectRetriever) Slug(ctx context.Context, uid string) (string, error) {
	slog.DebugContext(ctx, "mock project retriever: getting slug", "uid", uid)

	r.mock.mu.RLock()
	defer r.mock.mu.RUnlock()

	slug, exists := r.mock.projectSlugs[uid]
	if !exists {
		return "", errors.NewNotFound(fmt.Sprintf("project with UID %s not found", uid))
	}

	return slug, nil
}

// GetGrpsIOService retrieves a single service by ID and returns ETag revision
func (m *MockRepository) GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "mock service: getting service", "service_uid", uid)

	m.mu.RLock()
	defer m.mu.RUnlock()

	service, exists := m.services[uid]
	if !exists {
		return nil, 0, errors.NewNotFound(fmt.Sprintf("service with UID %s not found", uid))
	}

	// Return a deep copy of the service to avoid data races
	serviceCopy := *service
	serviceCopy.GlobalOwners = make([]string, len(service.GlobalOwners))
	copy(serviceCopy.GlobalOwners, service.GlobalOwners)
	serviceCopy.Writers = append([]string(nil), service.Writers...)
	serviceCopy.Auditors = append([]string(nil), service.Auditors...)
	revision := m.serviceRevisions[uid]
	return &serviceCopy, revision, nil
}

// ================== GrpsIOServiceWriter implementation ==================

// CreateGrpsIOService creates a new service in the mock storage
func (w *MockGrpsIOServiceWriter) CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "mock service: creating service", "service_id", service.UID)

	w.mock.mu.Lock()
	defer w.mock.mu.Unlock()

	// Check if service already exists
	if _, exists := w.mock.services[service.UID]; exists {
		return nil, 0, errors.NewConflict(fmt.Sprintf("service with ID %s already exists", service.UID))
	}

	// Set created/updated timestamps
	now := time.Now()
	service.CreatedAt = now
	service.UpdatedAt = now

	// Store service copy to avoid external modifications
	serviceCopy := *service
	serviceCopy.GlobalOwners = make([]string, len(service.GlobalOwners))
	copy(serviceCopy.GlobalOwners, service.GlobalOwners)
	serviceCopy.Writers = append([]string(nil), service.Writers...)
	serviceCopy.Auditors = append([]string(nil), service.Auditors...)

	w.mock.services[service.UID] = &serviceCopy
	w.mock.serviceRevisions[service.UID] = 1

	// Return service copy
	resultCopy := serviceCopy
	resultCopy.GlobalOwners = make([]string, len(serviceCopy.GlobalOwners))
	copy(resultCopy.GlobalOwners, serviceCopy.GlobalOwners)
	resultCopy.Writers = append([]string(nil), serviceCopy.Writers...)
	resultCopy.Auditors = append([]string(nil), serviceCopy.Auditors...)

	return &resultCopy, 1, nil
}

// UpdateGrpsIOService updates an existing service with revision checking
func (w *MockGrpsIOServiceWriter) UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "mock service: updating service", "service_uid", uid, "expected_revision", expectedRevision)

	w.mock.mu.Lock()
	defer w.mock.mu.Unlock()

	// Check if service exists
	existingService, exists := w.mock.services[uid]
	if !exists {
		return nil, 0, errors.NewNotFound(fmt.Sprintf("service with UID %s not found", uid))
	}

	// Check revision
	currentRevision := w.mock.serviceRevisions[uid]
	if currentRevision != expectedRevision {
		return nil, 0, errors.NewConflict(fmt.Sprintf("revision mismatch: expected %d, got %d", expectedRevision, currentRevision))
	}

	// Preserve created timestamp, update updated timestamp
	service.CreatedAt = existingService.CreatedAt
	service.UpdatedAt = time.Now()
	service.UID = uid // Ensure ID matches

	// Store service copy
	serviceCopy := *service
	serviceCopy.GlobalOwners = make([]string, len(service.GlobalOwners))
	copy(serviceCopy.GlobalOwners, service.GlobalOwners)
	serviceCopy.Writers = append([]string(nil), service.Writers...)
	serviceCopy.Auditors = append([]string(nil), service.Auditors...)

	w.mock.services[uid] = &serviceCopy
	newRevision := currentRevision + 1
	w.mock.serviceRevisions[uid] = newRevision

	// Return service copy
	resultCopy := serviceCopy
	resultCopy.GlobalOwners = make([]string, len(serviceCopy.GlobalOwners))
	copy(resultCopy.GlobalOwners, serviceCopy.GlobalOwners)
	resultCopy.Writers = append([]string(nil), serviceCopy.Writers...)
	resultCopy.Auditors = append([]string(nil), serviceCopy.Auditors...)

	return &resultCopy, newRevision, nil
}

// DeleteGrpsIOService deletes a service with revision checking
func (w *MockGrpsIOServiceWriter) DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64) error {
	slog.DebugContext(ctx, "mock service: deleting service", "service_uid", uid, "expected_revision", expectedRevision)

	w.mock.mu.Lock()
	defer w.mock.mu.Unlock()

	// Check if service exists
	if _, exists := w.mock.services[uid]; !exists {
		return errors.NewNotFound(fmt.Sprintf("service with UID %s not found", uid))
	}

	// Check revision
	currentRevision := w.mock.serviceRevisions[uid]
	if currentRevision != expectedRevision {
		return errors.NewConflict(fmt.Sprintf("revision mismatch: expected %d, got %d", expectedRevision, currentRevision))
	}

	// Delete service
	delete(w.mock.services, uid)
	delete(w.mock.serviceRevisions, uid)

	return nil
}

// UniqueProjectType validates that only one primary service exists per project (mock implementation)
func (w *MockGrpsIOServiceWriter) UniqueProjectType(ctx context.Context, service *model.GrpsIOService) (string, error) {
	slog.DebugContext(ctx, "mock constraint validation: unique project type", "project_uid", service.ProjectUID, "type", service.Type)

	// Mock implementation - always allows constraint creation
	constraintKey := fmt.Sprintf("mock_constraint_%s_%s", service.ProjectUID, service.Type)
	return constraintKey, nil
}

// UniqueProjectPrefix validates that the prefix is unique within the project for formation services (mock implementation)
func (w *MockGrpsIOServiceWriter) UniqueProjectPrefix(ctx context.Context, service *model.GrpsIOService) (string, error) {
	slog.DebugContext(ctx, "mock constraint validation: unique project prefix", "project_uid", service.ProjectUID, "prefix", service.Prefix)

	// Mock implementation - always allows constraint creation
	constraintKey := fmt.Sprintf("mock_constraint_%s_%s_%s", service.ProjectUID, service.Type, service.Prefix)
	return constraintKey, nil
}

// UniqueProjectGroupID validates that the group_id is unique within the project for shared services (mock implementation)
func (w *MockGrpsIOServiceWriter) UniqueProjectGroupID(ctx context.Context, service *model.GrpsIOService) (string, error) {
	slog.DebugContext(ctx, "mock constraint validation: unique project group_id", "project_uid", service.ProjectUID, "group_id", service.GroupID)

	// Mock implementation - always allows constraint creation
	constraintKey := fmt.Sprintf("mock_constraint_%s_%s_%d", service.ProjectUID, service.Type, service.GroupID)
	return constraintKey, nil
}

// GetRevision retrieves only the revision for a given UID (reader interface)
func (m *MockRepository) GetRevision(ctx context.Context, uid string) (uint64, error) {
	slog.DebugContext(ctx, "mock get service revision", "service_uid", uid)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if rev, exists := m.serviceRevisions[uid]; exists {
		return rev, nil
	}

	return 0, errors.NewNotFound("service not found")
}

// GetKeyRevision retrieves the revision for a given key (writer interface - used for cleanup operations)
func (w *MockGrpsIOServiceWriter) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	slog.DebugContext(ctx, "mock get key revision", "key", key)

	// Mock implementation - return a mock revision
	return 1, nil
}

// Delete removes a key with the given revision (mock implementation)
func (w *MockGrpsIOServiceWriter) Delete(ctx context.Context, key string, revision uint64) error {
	slog.DebugContext(ctx, "mock delete key", "key", key, "revision", revision)

	// Mock implementation - always succeeds
	return nil
}

// MockGrpsIOServiceReaderWriter combines reader and writer functionality
type MockGrpsIOServiceReaderWriter struct {
	port.GrpsIOServiceReader
	port.GrpsIOServiceWriter
}

// IsReady checks if the service is ready (always returns nil for mocks)
func (m *MockGrpsIOServiceReaderWriter) IsReady(ctx context.Context) error {
	slog.DebugContext(ctx, "mock service ready check: always ready")
	return nil
}

// Helper functions

// NewMockGrpsIOServiceReader creates a mock grpsio service reader
func NewMockGrpsIOServiceReader(mock *MockRepository) port.GrpsIOServiceReader {
	return mock
}

// NewMockGrpsIOServiceWriter creates a mock grpsio service writer
func NewMockGrpsIOServiceWriter(mock *MockRepository) port.GrpsIOServiceWriter {
	return &MockGrpsIOServiceWriter{mock: mock}
}

// NewMockGrpsIOServiceReaderWriter creates a mock grpsio service reader writer
func NewMockGrpsIOServiceReaderWriter(mock *MockRepository) port.GrpsIOServiceReaderWriter {
	return &MockGrpsIOServiceReaderWriter{
		GrpsIOServiceReader: mock,
		GrpsIOServiceWriter: &MockGrpsIOServiceWriter{mock: mock},
	}
}

// NewMockProjectRetriever creates a mock project retriever
func NewMockProjectRetriever(mock *MockRepository) port.ProjectReader {
	return &MockProjectRetriever{mock: mock}
}

// Utility functions for mock data manipulation

// AddService adds a service to the mock data (useful for testing)
func (m *MockRepository) AddService(service *model.GrpsIOService) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.services[service.UID] = service
	m.serviceRevisions[service.UID] = 1
}

// AddProject adds both project slug and name mappings (useful for testing)
func (m *MockRepository) AddProject(uid, slug, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.projectSlugs[uid] = slug
	m.projectNames[uid] = name
}

// ClearAll clears all mock data (useful for testing)
func (m *MockRepository) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.services = make(map[string]*model.GrpsIOService)
	m.serviceRevisions = make(map[string]uint64)
	m.projectSlugs = make(map[string]string)
	m.projectNames = make(map[string]string)
}

// GetServiceCount returns the total number of services
func (m *MockRepository) GetServiceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.services)
}
