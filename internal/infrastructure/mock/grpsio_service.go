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

// MockGrpsIOService provides a mock implementation of GrpsIOServiceReaderWriter
type MockGrpsIOService struct {
	services         map[string]*model.GrpsIOService
	serviceRevisions map[string]uint64
	mu               sync.RWMutex
}

// NewMockService creates a new mock service with sample data
func NewMockService() port.GrpsIOServiceReaderWriter {
	now := time.Now()

	mock := &MockGrpsIOService{
		services:         make(map[string]*model.GrpsIOService),
		serviceRevisions: make(map[string]uint64),
	}

	// Add sample data for testing
	sampleServices := []*model.GrpsIOService{
		{
			Type:         "v2_primary",
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
			Type:         "v2_formation",
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
			Type:         "v2_primary",
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

	return mock
}

// GetGrpsIOService retrieves a single service by ID and returns ETag revision
func (m *MockGrpsIOService) GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error) {
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
	revision := m.serviceRevisions[uid]
	return &serviceCopy, revision, nil
}

// CreateGrpsIOService creates a new service in the mock storage
func (m *MockGrpsIOService) CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "mock service: creating service", "service_id", service.UID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if service already exists
	if _, exists := m.services[service.UID]; exists {
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

	m.services[service.UID] = &serviceCopy
	m.serviceRevisions[service.UID] = 1

	// Return service copy
	resultCopy := serviceCopy
	resultCopy.GlobalOwners = make([]string, len(serviceCopy.GlobalOwners))
	copy(resultCopy.GlobalOwners, serviceCopy.GlobalOwners)

	return &resultCopy, 1, nil
}

// UpdateGrpsIOService updates an existing service with revision checking
func (m *MockGrpsIOService) UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "mock service: updating service", "service_uid", uid, "expected_revision", expectedRevision)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if service exists
	existingService, exists := m.services[uid]
	if !exists {
		return nil, 0, errors.NewNotFound(fmt.Sprintf("service with UID %s not found", uid))
	}

	// Check revision
	currentRevision := m.serviceRevisions[uid]
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

	m.services[uid] = &serviceCopy
	newRevision := currentRevision + 1
	m.serviceRevisions[uid] = newRevision

	// Return service copy
	resultCopy := serviceCopy
	resultCopy.GlobalOwners = make([]string, len(serviceCopy.GlobalOwners))
	copy(resultCopy.GlobalOwners, serviceCopy.GlobalOwners)

	return &resultCopy, newRevision, nil
}

// DeleteGrpsIOService deletes a service with revision checking
func (m *MockGrpsIOService) DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64) error {
	slog.DebugContext(ctx, "mock service: deleting service", "service_uid", uid, "expected_revision", expectedRevision)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if service exists
	if _, exists := m.services[uid]; !exists {
		return errors.NewNotFound(fmt.Sprintf("service with UID %s not found", uid))
	}

	// Check revision
	currentRevision := m.serviceRevisions[uid]
	if currentRevision != expectedRevision {
		return errors.NewConflict(fmt.Sprintf("revision mismatch: expected %d, got %d", expectedRevision, currentRevision))
	}

	// Delete service
	delete(m.services, uid)
	delete(m.serviceRevisions, uid)

	return nil
}

// UniqueProjectType validates that only one primary service exists per project (mock implementation)
func (m *MockGrpsIOService) UniqueProjectType(ctx context.Context, service *model.GrpsIOService) (string, error) {
	slog.DebugContext(ctx, "mock constraint validation: unique project type", "project_uid", service.ProjectUID, "type", service.Type)

	// Mock implementation - always allows constraint creation
	constraintKey := fmt.Sprintf("mock_constraint_%s_%s", service.ProjectUID, service.Type)
	return constraintKey, nil
}

// UniqueProjectPrefix validates that the prefix is unique within the project for formation services (mock implementation)
func (m *MockGrpsIOService) UniqueProjectPrefix(ctx context.Context, service *model.GrpsIOService) (string, error) {
	slog.DebugContext(ctx, "mock constraint validation: unique project prefix", "project_uid", service.ProjectUID, "prefix", service.Prefix)

	// Mock implementation - always allows constraint creation
	constraintKey := fmt.Sprintf("mock_constraint_%s_%s_%s", service.ProjectUID, service.Type, service.Prefix)
	return constraintKey, nil
}

// UniqueProjectGroupID validates that the group_id is unique within the project for shared services (mock implementation)
func (m *MockGrpsIOService) UniqueProjectGroupID(ctx context.Context, service *model.GrpsIOService) (string, error) {
	slog.DebugContext(ctx, "mock constraint validation: unique project group_id", "project_uid", service.ProjectUID, "group_id", service.GroupID)

	// Mock implementation - always allows constraint creation
	constraintKey := fmt.Sprintf("mock_constraint_%s_%s_%d", service.ProjectUID, service.Type, service.GroupID)
	return constraintKey, nil
}

// GetRevision retrieves only the revision for a given UID (reader interface)
func (m *MockGrpsIOService) GetRevision(ctx context.Context, uid string) (uint64, error) {
	slog.DebugContext(ctx, "mock get service revision", "service_uid", uid)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if rev, exists := m.serviceRevisions[uid]; exists {
		return rev, nil
	}

	return 0, errors.NewNotFound("service not found")
}

// GetKeyRevision retrieves the revision for a given key (writer interface - used for cleanup operations)
func (m *MockGrpsIOService) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	slog.DebugContext(ctx, "mock get key revision", "key", key)

	// Mock implementation - return a mock revision
	return 1, nil
}

// Delete removes a key with the given revision (mock implementation)
func (m *MockGrpsIOService) Delete(ctx context.Context, key string, revision uint64) error {
	slog.DebugContext(ctx, "mock delete key", "key", key, "revision", revision)

	// Mock implementation - always succeeds
	return nil
}

// IsReady checks if the service is ready (always returns nil for mocks)
func (m *MockGrpsIOService) IsReady(ctx context.Context) error {
	slog.DebugContext(ctx, "mock service ready check: always ready")
	return nil
}
