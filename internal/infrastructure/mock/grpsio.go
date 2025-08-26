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
	mailingLists     map[string]*model.GrpsIOMailingList
	projectSlugs     map[string]string // projectUID -> slug
	projectNames     map[string]string // projectUID -> name
	committeeNames   map[string]string // committeeUID -> name
	mu               sync.RWMutex      // Protect concurrent access to maps
}

// NewMockRepository creates a new mock repository with sample data
func NewMockRepository() *MockRepository {

	globalMockRepoOnce.Do(func() {
		now := time.Now()

		mock := &MockRepository{
			services:         make(map[string]*model.GrpsIOService),
			serviceRevisions: make(map[string]uint64),
			mailingLists:     make(map[string]*model.GrpsIOMailingList),
			projectSlugs:     make(map[string]string),
			projectNames:     make(map[string]string),
			committeeNames:   make(map[string]string),
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

		// Add sample committee data for testing
		mock.committeeNames = map[string]string{
			"committee-1": "Technical Oversight Committee",
			"committee-2": "Security Committee",
			"committee-3": "Architecture Committee",
			"committee-4": "Marketing Committee",
			"committee-5": "Governance Committee",
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

// MockEntityAttributeReader implements EntityAttributeReader interface
type MockEntityAttributeReader struct {
	mock *MockRepository
}

// ProjectName returns the project name for a given UID
func (r *MockEntityAttributeReader) ProjectName(ctx context.Context, uid string) (string, error) {
	slog.DebugContext(ctx, "mock entity attribute reader: getting project name", "uid", uid)

	r.mock.mu.RLock()
	defer r.mock.mu.RUnlock()

	name, exists := r.mock.projectNames[uid]
	if !exists {
		return "", errors.NewNotFound(fmt.Sprintf("project with UID %s not found", uid))
	}

	return name, nil
}

// ProjectSlug returns the project slug for a given UID
func (r *MockEntityAttributeReader) ProjectSlug(ctx context.Context, uid string) (string, error) {
	slog.DebugContext(ctx, "mock entity attribute reader: getting project slug", "uid", uid)

	r.mock.mu.RLock()
	defer r.mock.mu.RUnlock()

	slug, exists := r.mock.projectSlugs[uid]
	if !exists {
		return "", errors.NewNotFound(fmt.Sprintf("project with UID %s not found", uid))
	}

	return slug, nil
}

// CommitteeName returns the committee name for a given UID
func (r *MockEntityAttributeReader) CommitteeName(ctx context.Context, uid string) (string, error) {
	slog.DebugContext(ctx, "mock entity attribute reader: getting committee name", "uid", uid)

	r.mock.mu.RLock()
	defer r.mock.mu.RUnlock()

	name, exists := r.mock.committeeNames[uid]
	if !exists {
		return "", errors.NewNotFound(fmt.Sprintf("committee with UID %s not found", uid))
	}

	return name, nil
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

// MockGrpsIOStorageReaderWriter combines all storage functionality for services and mailing lists
type MockGrpsIOStorageReaderWriter struct {
	*MockRepository
	serviceWriter *MockGrpsIOServiceWriter
}

// IsReady checks if the service is ready (always returns nil for mocks)
func (m *MockGrpsIOStorageReaderWriter) IsReady(ctx context.Context) error {
	slog.DebugContext(ctx, "mock storage ready check: always ready")
	return nil
}

// ==================== SERVICE WRITER DELEGATION ====================

// CreateGrpsIOService delegates to the service writer
func (m *MockGrpsIOStorageReaderWriter) CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService) (*model.GrpsIOService, uint64, error) {
	return m.serviceWriter.CreateGrpsIOService(ctx, service)
}

// UpdateGrpsIOService delegates to the service writer
func (m *MockGrpsIOStorageReaderWriter) UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error) {
	return m.serviceWriter.UpdateGrpsIOService(ctx, uid, service, expectedRevision)
}

// DeleteGrpsIOService delegates to the service writer
func (m *MockGrpsIOStorageReaderWriter) DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64) error {
	return m.serviceWriter.DeleteGrpsIOService(ctx, uid, expectedRevision)
}

// UniqueProjectType delegates to the service writer
func (m *MockGrpsIOStorageReaderWriter) UniqueProjectType(ctx context.Context, service *model.GrpsIOService) (string, error) {
	return m.serviceWriter.UniqueProjectType(ctx, service)
}

// UniqueProjectPrefix delegates to the service writer
func (m *MockGrpsIOStorageReaderWriter) UniqueProjectPrefix(ctx context.Context, service *model.GrpsIOService) (string, error) {
	return m.serviceWriter.UniqueProjectPrefix(ctx, service)
}

// UniqueProjectGroupID delegates to the service writer
func (m *MockGrpsIOStorageReaderWriter) UniqueProjectGroupID(ctx context.Context, service *model.GrpsIOService) (string, error) {
	return m.serviceWriter.UniqueProjectGroupID(ctx, service)
}

// GetKeyRevision delegates to the service writer
func (m *MockGrpsIOStorageReaderWriter) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	return m.serviceWriter.GetKeyRevision(ctx, key)
}

// Delete delegates to the service writer
func (m *MockGrpsIOStorageReaderWriter) Delete(ctx context.Context, key string, revision uint64) error {
	return m.serviceWriter.Delete(ctx, key, revision)
}

// ==================== MAILING LIST METHODS ====================

// UniqueMailingListGroupName validates that group name is unique within parent service (always succeeds for mocks)
func (m *MockGrpsIOStorageReaderWriter) UniqueMailingListGroupName(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error) {
	constraintKey := fmt.Sprintf("lookup:mailing_list:constraint:%s:%s", mailingList.ParentUID, mailingList.GroupName)
	slog.DebugContext(ctx, "mock: validating unique mailing list group name", "constraint_key", constraintKey)
	return constraintKey, nil
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

// NewMockGrpsIOStorageReaderWriter creates a mock grpsio unified storage reader writer
func NewMockGrpsIOReaderWriter(mock *MockRepository) port.GrpsIOReaderWriter {
	return &MockGrpsIOStorageReaderWriter{
		MockRepository: mock,
		serviceWriter:  &MockGrpsIOServiceWriter{mock: mock},
	}
}

// NewMockEntityAttributeReader creates a mock entity attribute reader
func NewMockEntityAttributeReader(mock *MockRepository) port.EntityAttributeReader {
	return &MockEntityAttributeReader{mock: mock}
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

// AddCommittee adds committee name mapping (useful for testing)
func (m *MockRepository) AddCommittee(uid, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.committeeNames[uid] = name
}

// ClearAll clears all mock data (useful for testing)
func (m *MockRepository) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.services = make(map[string]*model.GrpsIOService)
	m.serviceRevisions = make(map[string]uint64)
	m.mailingLists = make(map[string]*model.GrpsIOMailingList)
	m.projectSlugs = make(map[string]string)
	m.projectNames = make(map[string]string)
	m.committeeNames = make(map[string]string)
}

// GetServiceCount returns the total number of services
func (m *MockRepository) GetServiceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.services)
}

// ==================== MAILING LIST READER OPERATIONS ====================

// GetGrpsIOMailingList retrieves a mailing list by UID
func (m *MockRepository) GetGrpsIOMailingList(ctx context.Context, uid string) (*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "mock mailing list: getting mailing list", "mailing_list_uid", uid)

	m.mu.RLock()
	defer m.mu.RUnlock()

	mailingList, exists := m.mailingLists[uid]
	if !exists {
		return nil, errors.NewNotFound("mailing list not found")
	}

	// Return a deep copy to avoid data races
	mailingListCopy := *mailingList
	mailingListCopy.CommitteeFilters = make([]string, len(mailingList.CommitteeFilters))
	copy(mailingListCopy.CommitteeFilters, mailingList.CommitteeFilters)
	mailingListCopy.Writers = append([]string(nil), mailingList.Writers...)
	mailingListCopy.Auditors = append([]string(nil), mailingList.Auditors...)

	return &mailingListCopy, nil
}

// GetGrpsIOMailingListsByParent retrieves mailing lists by parent service ID
func (m *MockRepository) GetGrpsIOMailingListsByParent(ctx context.Context, parentID string) ([]*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "mock mailing list: getting mailing lists by parent", "parent_id", parentID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*model.GrpsIOMailingList
	for _, mailingList := range m.mailingLists {
		if mailingList.ParentUID == parentID {
			// Create a deep copy
			mailingListCopy := *mailingList
			mailingListCopy.CommitteeFilters = make([]string, len(mailingList.CommitteeFilters))
			copy(mailingListCopy.CommitteeFilters, mailingList.CommitteeFilters)
			mailingListCopy.Writers = append([]string(nil), mailingList.Writers...)
			mailingListCopy.Auditors = append([]string(nil), mailingList.Auditors...)
			result = append(result, &mailingListCopy)
		}
	}

	return result, nil
}

// GetGrpsIOMailingListsByCommittee retrieves mailing lists by committee ID
func (m *MockRepository) GetGrpsIOMailingListsByCommittee(ctx context.Context, committeeID string) ([]*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "mock mailing list: getting mailing lists by committee", "committee_id", committeeID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*model.GrpsIOMailingList
	for _, mailingList := range m.mailingLists {
		if mailingList.CommitteeUID == committeeID {
			// Create a deep copy
			mailingListCopy := *mailingList
			mailingListCopy.CommitteeFilters = make([]string, len(mailingList.CommitteeFilters))
			copy(mailingListCopy.CommitteeFilters, mailingList.CommitteeFilters)
			mailingListCopy.Writers = append([]string(nil), mailingList.Writers...)
			mailingListCopy.Auditors = append([]string(nil), mailingList.Auditors...)
			result = append(result, &mailingListCopy)
		}
	}

	return result, nil
}

// GetGrpsIOMailingListsByProject retrieves mailing lists by project ID
func (m *MockRepository) GetGrpsIOMailingListsByProject(ctx context.Context, projectID string) ([]*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "mock mailing list: getting mailing lists by project", "project_id", projectID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*model.GrpsIOMailingList
	for _, mailingList := range m.mailingLists {
		if mailingList.ProjectUID == projectID {
			// Create a deep copy
			mailingListCopy := *mailingList
			mailingListCopy.CommitteeFilters = make([]string, len(mailingList.CommitteeFilters))
			copy(mailingListCopy.CommitteeFilters, mailingList.CommitteeFilters)
			mailingListCopy.Writers = append([]string(nil), mailingList.Writers...)
			mailingListCopy.Auditors = append([]string(nil), mailingList.Auditors...)
			result = append(result, &mailingListCopy)
		}
	}

	return result, nil
}

// CheckMailingListExists checks if a mailing list with the given name exists in parent service
func (m *MockRepository) CheckMailingListExists(ctx context.Context, parentID, groupName string) (bool, error) {
	slog.DebugContext(ctx, "mock mailing list: checking mailing list existence", "parent_id", parentID, "group_name", groupName)

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, mailingList := range m.mailingLists {
		if mailingList.ParentUID == parentID && mailingList.GroupName == groupName {
			return true, nil
		}
	}

	return false, nil
}

// ==================== MAILING LIST WRITER OPERATIONS ====================

// CreateGrpsIOMailingList creates a new mailing list in the mock storage
func (m *MockRepository) CreateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "mock mailing list: creating mailing list", "mailing_list_id", mailingList.UID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if mailing list already exists
	if _, exists := m.mailingLists[mailingList.UID]; exists {
		return nil, errors.NewConflict("mailing list already exists")
	}

	// Set created/updated timestamps if not set
	if mailingList.CreatedAt.IsZero() {
		mailingList.CreatedAt = time.Now()
	}
	if mailingList.UpdatedAt.IsZero() {
		mailingList.UpdatedAt = time.Now()
	}

	// Store mailing list copy to avoid external modifications
	mailingListCopy := *mailingList
	mailingListCopy.CommitteeFilters = make([]string, len(mailingList.CommitteeFilters))
	copy(mailingListCopy.CommitteeFilters, mailingList.CommitteeFilters)
	mailingListCopy.Writers = append([]string(nil), mailingList.Writers...)
	mailingListCopy.Auditors = append([]string(nil), mailingList.Auditors...)

	m.mailingLists[mailingList.UID] = &mailingListCopy

	// Return mailing list copy
	resultCopy := mailingListCopy
	resultCopy.CommitteeFilters = make([]string, len(mailingListCopy.CommitteeFilters))
	copy(resultCopy.CommitteeFilters, mailingListCopy.CommitteeFilters)
	resultCopy.Writers = append([]string(nil), mailingListCopy.Writers...)
	resultCopy.Auditors = append([]string(nil), mailingListCopy.Auditors...)

	return &resultCopy, nil
}

// UpdateGrpsIOMailingList updates an existing mailing list
func (m *MockRepository) UpdateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "mock mailing list: updating mailing list", "mailing_list_uid", mailingList.UID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if mailing list exists
	existingMailingList, exists := m.mailingLists[mailingList.UID]
	if !exists {
		return nil, errors.NewNotFound("mailing list not found")
	}

	// Preserve created timestamp, update updated timestamp
	mailingList.CreatedAt = existingMailingList.CreatedAt
	mailingList.UpdatedAt = time.Now()

	// Store mailing list copy
	mailingListCopy := *mailingList
	mailingListCopy.CommitteeFilters = make([]string, len(mailingList.CommitteeFilters))
	copy(mailingListCopy.CommitteeFilters, mailingList.CommitteeFilters)
	mailingListCopy.Writers = append([]string(nil), mailingList.Writers...)
	mailingListCopy.Auditors = append([]string(nil), mailingList.Auditors...)

	m.mailingLists[mailingList.UID] = &mailingListCopy

	// Return mailing list copy
	resultCopy := mailingListCopy
	resultCopy.CommitteeFilters = make([]string, len(mailingListCopy.CommitteeFilters))
	copy(resultCopy.CommitteeFilters, mailingListCopy.CommitteeFilters)
	resultCopy.Writers = append([]string(nil), mailingListCopy.Writers...)
	resultCopy.Auditors = append([]string(nil), mailingListCopy.Auditors...)

	return &resultCopy, nil
}

// DeleteGrpsIOMailingList deletes a mailing list
func (m *MockRepository) DeleteGrpsIOMailingList(ctx context.Context, uid string) error {
	slog.DebugContext(ctx, "mock mailing list: deleting mailing list", "mailing_list_uid", uid)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if mailing list exists
	if _, exists := m.mailingLists[uid]; !exists {
		return errors.NewNotFound("mailing list not found")
	}

	// Delete mailing list
	delete(m.mailingLists, uid)

	return nil
}

// UpdateSecondaryIndices updates secondary indices for a mailing list (mock implementation)
func (m *MockRepository) UpdateSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) error {
	slog.DebugContext(ctx, "mock mailing list: updating secondary indices", "mailing_list_uid", mailingList.UID)

	// Mock implementation - always succeeds
	return nil
}

// AddMailingList adds a mailing list to the mock data (useful for testing)
func (m *MockRepository) AddMailingList(mailingList *model.GrpsIOMailingList) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mailingLists[mailingList.UID] = mailingList
}
