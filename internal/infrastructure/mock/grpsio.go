// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/groupsio"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

// Global mock repository instance to share data between all repositories
var (
	globalMockRepo     *MockRepository
	globalMockRepoOnce = &sync.Once{}
)

// ErrorSimulationConfig configures error simulation for testing scenarios
type ErrorSimulationConfig struct {
	Enabled           bool             `json:"enabled"`
	ServiceErrors     map[string]error `json:"-"` // service_uid -> error to return
	MailingListErrors map[string]error `json:"-"` // mailing_list_uid -> error to return
	MemberErrors      map[string]error `json:"-"` // member_uid -> error to return
	GlobalError       error            `json:"-"` // error for all operations
	OperationErrors   map[string]error `json:"-"` // operation_name -> error
}

// MockRepository provides a mock implementation of all repository interfaces for testing
type MockRepository struct {
	services             map[string]*model.GrpsIOService
	serviceRevisions     map[string]uint64
	serviceIndexKeys     map[string]*model.GrpsIOService // indexKey -> service
	mailingLists         map[string]*model.GrpsIOMailingList
	mailingListRevisions map[string]uint64
	mailingListIndexKeys map[string]*model.GrpsIOMailingList // indexKey -> mailingList
	members              map[string]*model.GrpsIOMember      // UID -> member
	memberRevisions      map[string]uint64                   // UID -> revision
	memberIndexKeys      map[string]*model.GrpsIOMember      // indexKey -> member
	projectSlugs         map[string]string                   // projectUID -> slug
	projectNames         map[string]string                   // projectUID -> name
	committeeNames       map[string]string                   // committeeUID -> name
	errorSimulation      ErrorSimulationConfig               // Error simulation configuration
	errorSimulationMu    sync.RWMutex                        // Protect concurrent access to error config
	mu                   sync.RWMutex                        // Protect concurrent access to maps
}

// NewMockRepository creates a new mock repository with sample data
func NewMockRepository() *MockRepository {

	globalMockRepoOnce.Do(func() {
		now := time.Now()

		mock := &MockRepository{
			services:             make(map[string]*model.GrpsIOService),
			serviceRevisions:     make(map[string]uint64),
			serviceIndexKeys:     make(map[string]*model.GrpsIOService),
			mailingLists:         make(map[string]*model.GrpsIOMailingList),
			mailingListRevisions: make(map[string]uint64),
			mailingListIndexKeys: make(map[string]*model.GrpsIOMailingList),
			members:              make(map[string]*model.GrpsIOMember),
			memberRevisions:      make(map[string]uint64),
			memberIndexKeys:      make(map[string]*model.GrpsIOMember),
			projectSlugs:         make(map[string]string),
			projectNames:         make(map[string]string),
			committeeNames:       make(map[string]string),
			errorSimulation: ErrorSimulationConfig{
				Enabled:           false,
				ServiceErrors:     make(map[string]error),
				MailingListErrors: make(map[string]error),
				MemberErrors:      make(map[string]error),
				GlobalError:       nil,
				OperationErrors:   make(map[string]error),
			},
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
				UID:          "550e8400-e29b-41d4-a716-446655440001",
				Domain:       "lists.testproject.org",
				GroupID:      func() *int64 { id := int64(12345); return &id }(),
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
				UID:          "550e8400-e29b-41d4-a716-446655440002",
				Domain:       "lists.formation.testproject.org",
				GroupID:      func() *int64 { id := int64(12346); return &id }(),
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
				UID:          "550e8400-e29b-41d4-a716-446655440003",
				Domain:       "lists.example.org",
				GroupID:      func() *int64 { id := int64(12347); return &id }(),
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

		// Store services by ID and build indices
		for _, service := range sampleServices {
			mock.services[service.UID] = service
			mock.serviceRevisions[service.UID] = 1
			mock.serviceIndexKeys[service.BuildIndexKey(context.Background())] = service
		}

		// Add project mappings - using consistent naming
		mock.projectSlugs["7cad5a8d-19d0-41a4-81a6-043453daf9ee"] = "sample-project"
		mock.projectNames["7cad5a8d-19d0-41a4-81a6-043453daf9ee"] = "Cloud Native Computing Foundation"
		mock.projectSlugs["8dbc6b9e-20e1-42b5-92b7-154564eaf0ff"] = "example-project"
		mock.projectNames["8dbc6b9e-20e1-42b5-92b7-154564eaf0ff"] = "Example Project"

		// Add sample mailing list data
		sampleMailingLists := []*model.GrpsIOMailingList{
			{
				UID:              "mailing-list-1",
				GroupName:        "dev",
				Public:           true,
				Type:             "discussion_open",
				CommitteeUID:     "committee-1",
				CommitteeName:    "Technical Advisory Committee",
				CommitteeFilters: []string{"voting_rep", "observer"},
				Description:      "Development discussions and technical matters for the project",
				Title:            "Development List",
				SubjectTag:       "[DEV]",
				ServiceUID:       "550e8400-e29b-41d4-a716-446655440001",
				ProjectUID:       "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
				Writers:          []string{"dev-admin@testproject.org"},
				Auditors:         []string{"auditor@testproject.org"},
				CreatedAt:        now.Add(-18 * time.Hour),
				UpdatedAt:        now.Add(-2 * time.Hour),
			},
			{
				UID:         "mailing-list-2",
				GroupName:   "announce",
				Public:      true,
				Type:        "announcement",
				Description: "Official announcements and project news for all stakeholders",
				Title:       "Announcements",
				SubjectTag:  "[ANNOUNCE]",
				ServiceUID:  "550e8400-e29b-41d4-a716-446655440001",
				ProjectUID:  "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
				Writers:     []string{"admin@testproject.org"},
				Auditors:    []string{"auditor@testproject.org"},
				CreatedAt:   now.Add(-12 * time.Hour),
				UpdatedAt:   now.Add(-1 * time.Hour),
			},
			{
				UID:              "mailing-list-3",
				GroupName:        "formation-security",
				Public:           false,
				Type:             "discussion_moderated",
				CommitteeUID:     "committee-2",
				CommitteeName:    "Security Committee",
				CommitteeFilters: []string{"voting_rep"},
				Description:      "Private security discussions for committee members only",
				Title:            "Formation Security List",
				SubjectTag:       "[SECURITY]",
				ServiceUID:       "550e8400-e29b-41d4-a716-446655440002",
				ProjectUID:       "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
				Writers:          []string{"security@testproject.org"},
				Auditors:         []string{"security-audit@testproject.org"},
				CreatedAt:        now.Add(-6 * time.Hour),
				UpdatedAt:        now,
			},
		}

		// Store mailing lists by UID and build indices
		for _, mailingList := range sampleMailingLists {
			mock.mailingLists[mailingList.UID] = mailingList
			mock.mailingListRevisions[mailingList.UID] = 1
			mock.mailingListIndexKeys[mailingList.BuildIndexKey(context.Background())] = mailingList
		}

		globalMockRepo = mock
	})

	return globalMockRepo
}

// MockGrpsIOServiceWriter implements GrpsIOServiceWriter interface
type MockGrpsIOServiceWriter struct {
	mock *MockRepository
}

// MockGrpsIOMailingListWriter implements GrpsIOMailingListWriter interface
type MockGrpsIOMailingListWriter struct {
	mock *MockRepository
}

// ================== MockGrpsIOMailingListWriter implementation ==================

// CreateGrpsIOMailingList delegates to MockRepository
func (w *MockGrpsIOMailingListWriter) CreateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, uint64, error) {
	return w.mock.CreateGrpsIOMailingList(ctx, mailingList)
}

// UpdateGrpsIOMailingList delegates to MockRepository
func (w *MockGrpsIOMailingListWriter) UpdateGrpsIOMailingList(ctx context.Context, uid string, mailingList *model.GrpsIOMailingList, expectedRevision uint64) (*model.GrpsIOMailingList, uint64, error) {
	return w.mock.UpdateGrpsIOMailingListWithRevision(ctx, uid, mailingList, expectedRevision)
}

// DeleteGrpsIOMailingList delegates to MockRepository
func (w *MockGrpsIOMailingListWriter) DeleteGrpsIOMailingList(ctx context.Context, uid string, expectedRevision uint64, mailingList *model.GrpsIOMailingList) error {
	return w.mock.DeleteGrpsIOMailingListWithRevision(ctx, uid, expectedRevision)
}

// CreateSecondaryIndices delegates to MockRepository
func (w *MockGrpsIOMailingListWriter) CreateSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) ([]string, error) {
	return w.mock.CreateSecondaryIndices(ctx, mailingList)
}

// UniqueMailingListGroupName delegates to MockRepository
func (w *MockGrpsIOMailingListWriter) UniqueMailingListGroupName(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error) {
	return w.mock.UniqueMailingListGroupName(ctx, mailingList)
}

// GetKeyRevision delegates to MockRepository
func (w *MockGrpsIOMailingListWriter) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	slog.DebugContext(ctx, "mock get key revision", "key", key)
	return 1, nil
}

// Delete removes a key with the given revision
func (w *MockGrpsIOMailingListWriter) Delete(ctx context.Context, key string, revision uint64) error {
	slog.DebugContext(ctx, "mock delete key", "key", key, "revision", revision)
	return nil
}

// MockGrpsIOMemberWriter implements GrpsIOMemberWriter interface
type MockGrpsIOMemberWriter struct {
	mock *MockRepository
}

// ================== MockGrpsIOMemberWriter implementation ==================

// UniqueMember validates member email is unique within mailing list
func (w *MockGrpsIOMemberWriter) UniqueMember(ctx context.Context, member *model.GrpsIOMember) (string, error) {
	constraintKey := fmt.Sprintf("lookup:member:constraint:%s:%s", member.MailingListUID, member.Email)
	slog.DebugContext(ctx, "mock: validating unique member", "constraint_key", constraintKey)
	return constraintKey, nil
}

// CreateGrpsIOMember creates a new member and returns it with revision
func (w *MockGrpsIOMemberWriter) CreateGrpsIOMember(ctx context.Context, member *model.GrpsIOMember) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "mock member: creating member", "member_uid", member.UID, "email", member.Email)

	w.mock.mu.Lock()
	defer w.mock.mu.Unlock()

	// Check if member already exists
	if _, exists := w.mock.members[member.UID]; exists {
		return nil, 0, errors.NewConflict(fmt.Sprintf("member with ID %s already exists", member.UID))
	}

	// Set created/updated timestamps
	now := time.Now()
	member.CreatedAt = now
	member.UpdatedAt = now

	// Store member copy to avoid external modifications
	memberCopy := *member

	w.mock.members[member.UID] = &memberCopy
	w.mock.memberRevisions[member.UID] = 1
	w.mock.memberIndexKeys[member.BuildIndexKey(ctx)] = &memberCopy

	// Return member copy
	resultCopy := memberCopy

	return &resultCopy, 1, nil
}

// UpdateGrpsIOMember updates an existing member with optimistic concurrency control
func (w *MockGrpsIOMemberWriter) UpdateGrpsIOMember(ctx context.Context, uid string, member *model.GrpsIOMember, expectedRevision uint64) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "mock member: updating member", "member_uid", uid)

	w.mock.mu.Lock()
	defer w.mock.mu.Unlock()

	// Check if member exists
	existing, exists := w.mock.members[uid]
	if !exists {
		return nil, 0, errors.NewNotFound(fmt.Sprintf("member with UID %s not found", uid))
	}

	// Check revision for optimistic concurrency control
	currentRevision := w.mock.memberRevisions[uid]
	if expectedRevision != currentRevision {
		return nil, 0, errors.NewConflict(fmt.Sprintf("revision mismatch: expected %d, got %d", expectedRevision, currentRevision))
	}

	// Update member while preserving immutable fields
	memberCopy := *member
	memberCopy.UID = existing.UID                       // Preserve UID
	memberCopy.Email = existing.Email                   // Preserve email (immutable)
	memberCopy.MailingListUID = existing.MailingListUID // Preserve mailing list UID (immutable)
	memberCopy.CreatedAt = existing.CreatedAt           // Preserve created timestamp
	memberCopy.UpdatedAt = time.Now()                   // Update timestamp

	// Store updated member and increment revision
	w.mock.members[uid] = &memberCopy
	newRevision := currentRevision + 1
	w.mock.memberRevisions[uid] = newRevision

	// Update index if email or mailing list changed (though they shouldn't for immutable fields)
	if w.mock.memberIndexKeys != nil {
		w.mock.memberIndexKeys[memberCopy.BuildIndexKey(ctx)] = &memberCopy
	}

	// Return member copy
	resultCopy := memberCopy

	return &resultCopy, newRevision, nil
}

// DeleteGrpsIOMember deletes a member with optimistic concurrency control
func (w *MockGrpsIOMemberWriter) DeleteGrpsIOMember(ctx context.Context, uid string, expectedRevision uint64, member *model.GrpsIOMember) error {
	slog.DebugContext(ctx, "mock member: deleting member", "member_uid", uid)

	w.mock.mu.Lock()
	defer w.mock.mu.Unlock()

	// Check if member exists
	existing, exists := w.mock.members[uid]
	if !exists {
		return errors.NewNotFound(fmt.Sprintf("member with UID %s not found", uid))
	}

	// Check revision for optimistic concurrency control
	currentRevision := w.mock.memberRevisions[uid]
	if expectedRevision != currentRevision {
		return errors.NewConflict(fmt.Sprintf("revision mismatch: expected %d, got %d", expectedRevision, currentRevision))
	}

	// Remove from index
	if w.mock.memberIndexKeys != nil {
		delete(w.mock.memberIndexKeys, existing.BuildIndexKey(ctx))
	}

	// Delete member and revision
	delete(w.mock.members, uid)
	delete(w.mock.memberRevisions, uid)

	return nil
}

// CreateMemberSecondaryIndices creates lookup indices for Groups.io IDs
func (w *MockGrpsIOMemberWriter) CreateMemberSecondaryIndices(ctx context.Context, member *model.GrpsIOMember) ([]string, error) {
	slog.DebugContext(ctx, "mock member: creating secondary indices", "member_uid", member.UID)

	// Mock implementation - return mock keys for testing
	var keys []string

	if member.GroupsIOMemberID != nil {
		keys = append(keys, fmt.Sprintf("lookup/groupsio-member-memberid/%d/%s", *member.GroupsIOMemberID, member.UID))
	}

	if member.GroupsIOGroupID != nil {
		keys = append(keys, fmt.Sprintf("lookup/groupsio-member-groupid/%d/%s", *member.GroupsIOGroupID, member.UID))
	}

	return keys, nil
}

// GetKeyRevision retrieves the revision for a given key
func (w *MockGrpsIOMemberWriter) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	slog.DebugContext(ctx, "mock member: get key revision", "key", key)
	return 1, nil
}

// Delete removes a key with the given revision
func (w *MockGrpsIOMemberWriter) Delete(ctx context.Context, key string, revision uint64) error {
	slog.DebugContext(ctx, "mock member: delete key", "key", key, "revision", revision)
	return nil
}

// MockGrpsIOWriter combines both service and mailing list writers
type MockGrpsIOWriter struct {
	mock              *MockRepository
	serviceWriter     *MockGrpsIOServiceWriter
	mailingListWriter *MockGrpsIOMailingListWriter
	memberWriter      *MockGrpsIOMemberWriter
}

// ================== MockGrpsIOWriter implementation (delegates to sub-writers) ==================

// Service writer methods
func (w *MockGrpsIOWriter) CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService) (*model.GrpsIOService, uint64, error) {
	return w.serviceWriter.CreateGrpsIOService(ctx, service)
}

func (w *MockGrpsIOWriter) UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error) {
	return w.serviceWriter.UpdateGrpsIOService(ctx, uid, service, expectedRevision)
}

func (w *MockGrpsIOWriter) DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64, service *model.GrpsIOService) error {
	return w.serviceWriter.DeleteGrpsIOService(ctx, uid, expectedRevision, service)
}

func (w *MockGrpsIOWriter) UniqueProjectType(ctx context.Context, service *model.GrpsIOService) (string, error) {
	return w.serviceWriter.UniqueProjectType(ctx, service)
}

func (w *MockGrpsIOWriter) UniqueProjectPrefix(ctx context.Context, service *model.GrpsIOService) (string, error) {
	return w.serviceWriter.UniqueProjectPrefix(ctx, service)
}

func (w *MockGrpsIOWriter) UniqueProjectGroupID(ctx context.Context, service *model.GrpsIOService) (string, error) {
	return w.serviceWriter.UniqueProjectGroupID(ctx, service)
}

// Mailing list writer methods
func (w *MockGrpsIOWriter) CreateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, uint64, error) {
	return w.mailingListWriter.CreateGrpsIOMailingList(ctx, mailingList)
}

func (w *MockGrpsIOWriter) UpdateGrpsIOMailingList(ctx context.Context, uid string, mailingList *model.GrpsIOMailingList, expectedRevision uint64) (*model.GrpsIOMailingList, uint64, error) {
	return w.mailingListWriter.UpdateGrpsIOMailingList(ctx, uid, mailingList, expectedRevision)
}

func (w *MockGrpsIOWriter) DeleteGrpsIOMailingList(ctx context.Context, uid string, expectedRevision uint64, mailingList *model.GrpsIOMailingList) error {
	return w.mailingListWriter.DeleteGrpsIOMailingList(ctx, uid, expectedRevision, mailingList)
}

func (w *MockGrpsIOWriter) CreateSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) ([]string, error) {
	return w.mailingListWriter.CreateSecondaryIndices(ctx, mailingList)
}

func (w *MockGrpsIOWriter) UniqueMailingListGroupName(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error) {
	return w.mailingListWriter.UniqueMailingListGroupName(ctx, mailingList)
}

func (w *MockGrpsIOWriter) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	return w.serviceWriter.GetKeyRevision(ctx, key)
}

// For cleanup operations
func (w *MockGrpsIOWriter) Delete(ctx context.Context, key string, revision uint64) error {
	return w.serviceWriter.Delete(ctx, key, revision)
}

// Member operations - delegate to member writer
func (w *MockGrpsIOWriter) UniqueMember(ctx context.Context, member *model.GrpsIOMember) (string, error) {
	return w.memberWriter.UniqueMember(ctx, member)
}

func (w *MockGrpsIOWriter) CreateGrpsIOMember(ctx context.Context, member *model.GrpsIOMember) (*model.GrpsIOMember, uint64, error) {
	return w.memberWriter.CreateGrpsIOMember(ctx, member)
}

func (w *MockGrpsIOWriter) UpdateGrpsIOMember(ctx context.Context, uid string, member *model.GrpsIOMember, expectedRevision uint64) (*model.GrpsIOMember, uint64, error) {
	return w.memberWriter.UpdateGrpsIOMember(ctx, uid, member, expectedRevision)
}

func (w *MockGrpsIOWriter) DeleteGrpsIOMember(ctx context.Context, uid string, expectedRevision uint64, member *model.GrpsIOMember) error {
	return w.memberWriter.DeleteGrpsIOMember(ctx, uid, expectedRevision, member)
}

func (w *MockGrpsIOWriter) CreateMemberSecondaryIndices(ctx context.Context, member *model.GrpsIOMember) ([]string, error) {
	return w.memberWriter.CreateMemberSecondaryIndices(ctx, member)
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

// Error configuration methods for testing

// SetErrorForService configures the mock to return an error for a specific service UID
func (m *MockRepository) SetErrorForService(serviceUID string, err error) {
	m.errorSimulationMu.Lock()
	defer m.errorSimulationMu.Unlock()

	m.errorSimulation.Enabled = true
	m.errorSimulation.ServiceErrors[serviceUID] = err
}

// SetErrorForMailingList configures the mock to return an error for a specific mailing list UID
func (m *MockRepository) SetErrorForMailingList(mailingListUID string, err error) {
	m.errorSimulationMu.Lock()
	defer m.errorSimulationMu.Unlock()

	m.errorSimulation.Enabled = true
	m.errorSimulation.MailingListErrors[mailingListUID] = err
}

// SetErrorForMember configures the mock to return an error for a specific member UID
func (m *MockRepository) SetErrorForMember(memberUID string, err error) {
	m.errorSimulationMu.Lock()
	defer m.errorSimulationMu.Unlock()

	m.errorSimulation.Enabled = true
	m.errorSimulation.MemberErrors[memberUID] = err
}

// SetErrorForOperation configures the mock to return an error for a specific operation
func (m *MockRepository) SetErrorForOperation(operation string, err error) {
	m.errorSimulationMu.Lock()
	defer m.errorSimulationMu.Unlock()

	m.errorSimulation.Enabled = true
	m.errorSimulation.OperationErrors[operation] = err
}

// SetGlobalError configures the mock to return an error for all operations
func (m *MockRepository) SetGlobalError(err error) {
	m.errorSimulationMu.Lock()
	defer m.errorSimulationMu.Unlock()

	m.errorSimulation.Enabled = true
	m.errorSimulation.GlobalError = err
}

// ClearErrorSimulation disables all error simulation
func (m *MockRepository) ClearErrorSimulation() {
	m.errorSimulationMu.Lock()
	defer m.errorSimulationMu.Unlock()

	m.errorSimulation.Enabled = false
	m.errorSimulation.ServiceErrors = make(map[string]error)
	m.errorSimulation.MailingListErrors = make(map[string]error)
	m.errorSimulation.MemberErrors = make(map[string]error)
	m.errorSimulation.OperationErrors = make(map[string]error)
	m.errorSimulation.GlobalError = nil
}

// checkErrorSimulation checks if an error should be returned based on configuration
func (m *MockRepository) checkErrorSimulation(operation, resourceUID string) error {
	m.errorSimulationMu.RLock()
	defer m.errorSimulationMu.RUnlock()

	if !m.errorSimulation.Enabled {
		return nil
	}

	// Check global error first
	if m.errorSimulation.GlobalError != nil {
		return m.errorSimulation.GlobalError
	}

	// Check operation-specific error
	if err, exists := m.errorSimulation.OperationErrors[operation]; exists {
		return err
	}

	// Check resource-specific errors based on operation type
	if strings.Contains(operation, "Service") {
		if err, exists := m.errorSimulation.ServiceErrors[resourceUID]; exists {
			return err
		}
	} else if strings.Contains(operation, "MailingList") {
		if err, exists := m.errorSimulation.MailingListErrors[resourceUID]; exists {
			return err
		}
	} else if strings.Contains(operation, "Member") {
		if err, exists := m.errorSimulation.MemberErrors[resourceUID]; exists {
			return err
		}
	}

	return nil
}

// GetGrpsIOService retrieves a single service by ID and returns ETag revision
func (m *MockRepository) GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "mock service: getting service", "service_uid", uid)

	// Check error simulation first
	if err := m.checkErrorSimulation("GetGrpsIOService", uid); err != nil {
		return nil, 0, err
	}

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

// ================== MockGrpsIOServiceWriter implementation ==================

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
	w.mock.serviceIndexKeys[service.BuildIndexKey(ctx)] = &serviceCopy

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

	// Remove old index key and add new one
	oldIndexKey := existingService.BuildIndexKey(ctx)
	delete(w.mock.serviceIndexKeys, oldIndexKey)

	w.mock.services[uid] = &serviceCopy
	newRevision := currentRevision + 1
	w.mock.serviceRevisions[uid] = newRevision
	w.mock.serviceIndexKeys[service.BuildIndexKey(ctx)] = &serviceCopy

	// Return service copy
	resultCopy := serviceCopy
	resultCopy.GlobalOwners = make([]string, len(serviceCopy.GlobalOwners))
	copy(resultCopy.GlobalOwners, serviceCopy.GlobalOwners)
	resultCopy.Writers = append([]string(nil), serviceCopy.Writers...)
	resultCopy.Auditors = append([]string(nil), serviceCopy.Auditors...)

	return &resultCopy, newRevision, nil
}

// DeleteGrpsIOService deletes a service with revision checking
func (w *MockGrpsIOServiceWriter) DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64, service *model.GrpsIOService) error {
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

	// Use passed service for index key (same as original pattern)
	indexKey := service.BuildIndexKey(ctx)

	// Delete service and its indices
	delete(w.mock.services, uid)
	delete(w.mock.serviceRevisions, uid)
	delete(w.mock.serviceIndexKeys, indexKey)

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

// GetKeyRevision retrieves the revision for a given key (used for cleanup operations)
func (w *MockGrpsIOServiceWriter) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	slog.DebugContext(ctx, "mock get key revision", "key", key)
	return 1, nil
}

// Delete removes a key with the given revision (used for cleanup and rollback)
func (w *MockGrpsIOServiceWriter) Delete(ctx context.Context, key string, revision uint64) error {
	slog.DebugContext(ctx, "mock delete key", "key", key, "revision", revision)
	return nil
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

// GetServicesByGroupID retrieves all services for a given GroupsIO parent group ID
// Returns empty slice if no services found (not an error)
func (m *MockRepository) GetServicesByGroupID(ctx context.Context, groupID uint64) ([]*model.GrpsIOService, error) {
	slog.DebugContext(ctx, "mock service: getting services by group_id", "group_id", groupID)

	// Check error simulation first
	if err := m.checkErrorSimulation("GetServicesByGroupID", fmt.Sprintf("%d", groupID)); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var services []*model.GrpsIOService

	// Iterate through all services to find matches
	for _, service := range m.services {
		if service.GroupID != nil && uint64(*service.GroupID) == groupID {
			// Return deep copy to avoid data races
			serviceCopy := *service
			serviceCopy.GlobalOwners = make([]string, len(service.GlobalOwners))
			copy(serviceCopy.GlobalOwners, service.GlobalOwners)
			serviceCopy.Writers = append([]string(nil), service.Writers...)
			serviceCopy.Auditors = append([]string(nil), service.Auditors...)
			services = append(services, &serviceCopy)
		}
	}

	slog.DebugContext(ctx, "mock service: services retrieved by group_id",
		"group_id", groupID,
		"count", len(services))

	return services, nil
}

// MockGrpsIOReaderWriter combines reader and writer functionality
type MockGrpsIOReaderWriter struct {
	port.GrpsIOReader
	port.GrpsIOWriter
}

// IsReady checks if the service is ready (always returns nil for mocks)
func (m *MockGrpsIOReaderWriter) IsReady(ctx context.Context) error {
	slog.DebugContext(ctx, "mock storage ready check: always ready")
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

// NewMockGrpsIOMailingListWriter creates a mock grpsio mailing list writer
func NewMockGrpsIOMailingListWriter(mock *MockRepository) port.GrpsIOMailingListWriter {
	return &MockGrpsIOMailingListWriter{mock: mock}
}

// NewMockGrpsIOMemberWriter creates a mock grpsio member writer
func NewMockGrpsIOMemberWriter(mock *MockRepository) port.GrpsIOMemberWriter {
	return &MockGrpsIOMemberWriter{mock: mock}
}

// NewMockGrpsIOReader creates a mock grpsio reader
func NewMockGrpsIOReader(mock *MockRepository) port.GrpsIOReader {
	return mock
}

// NewMockGrpsIOWriter creates a mock grpsio writer
func NewMockGrpsIOWriter(mock *MockRepository) port.GrpsIOWriter {
	return &MockGrpsIOWriter{
		mock:              mock,
		serviceWriter:     &MockGrpsIOServiceWriter{mock: mock},
		mailingListWriter: &MockGrpsIOMailingListWriter{mock: mock},
		memberWriter:      &MockGrpsIOMemberWriter{mock: mock},
	}
}

// NewMockGrpsIOReaderWriter creates a mock grpsio reader writer
func NewMockGrpsIOReaderWriter(mock *MockRepository) port.GrpsIOReaderWriter {
	return &MockGrpsIOReaderWriter{
		GrpsIOReader: mock,
		GrpsIOWriter: &MockGrpsIOWriter{
			mock:              mock,
			serviceWriter:     &MockGrpsIOServiceWriter{mock: mock},
			mailingListWriter: &MockGrpsIOMailingListWriter{mock: mock},
			memberWriter:      &MockGrpsIOMemberWriter{mock: mock},
		},
	}
}

// NewMockGrpsIOMemberReader creates a mock grpsio member reader
func NewMockGrpsIOMemberReader(mock *MockRepository) port.GrpsIOMemberReader {
	return mock
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
	m.serviceIndexKeys = make(map[string]*model.GrpsIOService)
	m.mailingLists = make(map[string]*model.GrpsIOMailingList)
	m.mailingListRevisions = make(map[string]uint64)
	m.mailingListIndexKeys = make(map[string]*model.GrpsIOMailingList)
	m.members = make(map[string]*model.GrpsIOMember)
	m.memberRevisions = make(map[string]uint64)
	m.memberIndexKeys = make(map[string]*model.GrpsIOMember)
	m.projectSlugs = make(map[string]string)
	m.projectNames = make(map[string]string)
	m.committeeNames = make(map[string]string)
}

// ==================== MOCK MESSAGE PUBLISHER ====================

// MockGrpsIOMessagePublisher implements MessagePublisher interface for testing
type MockGrpsIOMessagePublisher struct{}

// Indexer simulates publishing an indexer message
func (p *MockGrpsIOMessagePublisher) Indexer(ctx context.Context, subject string, message any) error {
	slog.InfoContext(ctx, "mock publisher: indexer message published",
		"subject", subject,
		"message_type", "indexer",
	)
	return nil
}

// Access simulates publishing an access control message
func (p *MockGrpsIOMessagePublisher) Access(ctx context.Context, subject string, message any) error {
	slog.InfoContext(ctx, "mock publisher: access message published",
		"subject", subject,
		"message_type", "access",
	)
	return nil
}

// NewMockGrpsIOMessagePublisher creates a mock message publisher
func NewMockGrpsIOMessagePublisher() port.MessagePublisher {
	return &MockGrpsIOMessagePublisher{}
}

// MockGroupsIOClient provides a mock implementation of the GroupsIO HTTP client
type MockGroupsIOClient struct {
	CallLog []string // Track method calls for testing
}

// Verify MockGroupsIOClient implements ClientInterface
var _ groupsio.ClientInterface = (*MockGroupsIOClient)(nil)

// NewMockGroupsIOClient creates a new mock GroupsIO client
func NewMockGroupsIOClient() *MockGroupsIOClient {
	return &MockGroupsIOClient{
		CallLog: make([]string, 0),
	}
}

// CreateGroup mocks the Groups.io group creation API
func (m *MockGroupsIOClient) CreateGroup(ctx context.Context, domain string, options groupsio.GroupCreateOptions) (*groupsio.GroupObject, error) {
	m.CallLog = append(m.CallLog, fmt.Sprintf("CreateGroup(domain=%s, group_name=%s)", domain, options.GroupName))

	// Return mock result
	return &groupsio.GroupObject{
		ID: 12345, // Mock group ID
	}, nil
}

// UpdateGroup mocks the Groups.io group update API
func (m *MockGroupsIOClient) UpdateGroup(ctx context.Context, domain string, groupID uint64, updates groupsio.GroupUpdateOptions) error {
	m.CallLog = append(m.CallLog, fmt.Sprintf("UpdateGroup(domain=%s, group_id=%d, owners=%v)", domain, groupID, updates.GlobalOwners))

	slog.InfoContext(ctx, "[MOCK] Groups.io group update simulated",
		"domain", domain, "group_id", groupID, "global_owners", updates.GlobalOwners)

	return nil
}

// UpdateMember mocks the Groups.io member update API
func (m *MockGroupsIOClient) UpdateMember(ctx context.Context, domain string, memberID uint64, updates groupsio.MemberUpdateOptions) error {
	m.CallLog = append(m.CallLog, fmt.Sprintf("UpdateMember(domain=%s, member_id=%d, mod_status=%s)", domain, memberID, updates.ModStatus))

	slog.InfoContext(ctx, "[MOCK] Groups.io member update simulated",
		"domain", domain, "member_id", memberID, "mod_status", updates.ModStatus, "delivery", updates.DeliveryMode)

	return nil
}

// UpdateSubgroup mocks the Groups.io subgroup update API
func (m *MockGroupsIOClient) UpdateSubgroup(ctx context.Context, domain string, subgroupID uint64, updates groupsio.SubgroupUpdateOptions) error {
	m.CallLog = append(m.CallLog, fmt.Sprintf("UpdateSubgroup(domain=%s, subgroup_id=%d, title=%s)", domain, subgroupID, updates.Title))

	slog.InfoContext(ctx, "[MOCK] Groups.io subgroup update simulated",
		"domain", domain, "subgroup_id", subgroupID, "title", updates.Title, "description", updates.Description)

	return nil
}

// DeleteGroup mocks the Groups.io group deletion API
func (m *MockGroupsIOClient) DeleteGroup(ctx context.Context, domain string, groupID uint64) error {
	m.CallLog = append(m.CallLog, fmt.Sprintf("DeleteGroup(domain=%s, group_id=%d)", domain, groupID))

	slog.InfoContext(ctx, "[MOCK] Groups.io group deletion simulated",
		"domain", domain, "group_id", groupID)

	return nil
}

// CreateSubgroup mocks the Groups.io subgroup creation API
func (m *MockGroupsIOClient) CreateSubgroup(ctx context.Context, domain string, parentGroupID uint64, options groupsio.SubgroupCreateOptions) (*groupsio.SubgroupObject, error) {
	m.CallLog = append(m.CallLog, fmt.Sprintf("CreateSubgroup(domain=%s, parent_id=%d, name=%s)", domain, parentGroupID, options.GroupName))

	// Return mock result
	return &groupsio.SubgroupObject{
		ID: 67890, // Mock subgroup ID
	}, nil
}

// DeleteSubgroup mocks the Groups.io subgroup deletion API
func (m *MockGroupsIOClient) DeleteSubgroup(ctx context.Context, domain string, subgroupID uint64) error {
	m.CallLog = append(m.CallLog, fmt.Sprintf("DeleteSubgroup(domain=%s, subgroup_id=%d)", domain, subgroupID))

	slog.InfoContext(ctx, "[MOCK] Groups.io subgroup deletion simulated",
		"domain", domain, "subgroup_id", subgroupID)

	return nil
}

// AddMember mocks the Groups.io member addition API
func (m *MockGroupsIOClient) AddMember(ctx context.Context, domain string, subgroupID uint64, email, name string) (*groupsio.MemberObject, error) {
	m.CallLog = append(m.CallLog, fmt.Sprintf("AddMember(domain=%s, subgroup_id=%d, email=%s)", domain, subgroupID, email))

	// Return mock result
	return &groupsio.MemberObject{
		ID: 11111, // Mock member ID
	}, nil
}

// RemoveMember mocks the Groups.io member removal API
func (m *MockGroupsIOClient) RemoveMember(ctx context.Context, domain string, memberID uint64) error {
	m.CallLog = append(m.CallLog, fmt.Sprintf("RemoveMember(domain=%s, member_id=%d)", domain, memberID))

	slog.InfoContext(ctx, "[MOCK] Groups.io member removal simulated",
		"domain", domain, "member_id", memberID)

	return nil
}

// IsReady mocks the readiness check
func (m *MockGroupsIOClient) IsReady(ctx context.Context) error {
	return nil // Mock client is always ready
}

// GetCallLog returns the log of method calls for testing
func (m *MockGroupsIOClient) GetCallLog() []string {
	return m.CallLog
}

// ClearCallLog clears the call log
func (m *MockGroupsIOClient) ClearCallLog() {
	m.CallLog = make([]string, 0)
}

// GetServiceCount returns the total number of services
func (m *MockRepository) GetServiceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.services)
}

// ==================== MAILING LIST READER OPERATIONS ====================

// GetGrpsIOMailingList retrieves a mailing list by UID (interface implementation)
func (m *MockRepository) GetGrpsIOMailingList(ctx context.Context, uid string) (*model.GrpsIOMailingList, uint64, error) {
	return m.GetGrpsIOMailingListWithRevision(ctx, uid)
}

// GetMailingListRevision retrieves only the revision for a given UID (interface implementation)
func (m *MockRepository) GetMailingListRevision(ctx context.Context, uid string) (uint64, error) {
	return m.GetGrpsIOMailingListRevision(ctx, uid)
}

// GetGrpsIOMailingListWithRevision retrieves a mailing list by UID with revision (internal helper)
func (m *MockRepository) GetGrpsIOMailingListWithRevision(ctx context.Context, uid string) (*model.GrpsIOMailingList, uint64, error) {
	slog.DebugContext(ctx, "mock mailing list: getting mailing list with revision", "mailing_list_uid", uid)

	// Check error simulation first
	if err := m.checkErrorSimulation("GetGrpsIOMailingList", uid); err != nil {
		return nil, 0, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	mailingList, exists := m.mailingLists[uid]
	if !exists {
		return nil, 0, errors.NewNotFound("mailing list not found")
	}

	// Return a deep copy to avoid data races
	mailingListCopy := *mailingList
	mailingListCopy.CommitteeFilters = make([]string, len(mailingList.CommitteeFilters))
	copy(mailingListCopy.CommitteeFilters, mailingList.CommitteeFilters)
	mailingListCopy.Writers = append([]string(nil), mailingList.Writers...)
	mailingListCopy.Auditors = append([]string(nil), mailingList.Auditors...)

	revision := m.mailingListRevisions[uid]
	if revision == 0 {
		revision = 1
	}

	return &mailingListCopy, revision, nil
}

// GetGrpsIOMailingListRevision retrieves only the revision for a mailing list
func (m *MockRepository) GetGrpsIOMailingListRevision(ctx context.Context, uid string) (uint64, error) {
	slog.DebugContext(ctx, "mock mailing list: getting revision", "mailing_list_uid", uid)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.mailingLists[uid]; !exists {
		return 0, errors.NewNotFound("mailing list not found")
	}

	revision := m.mailingListRevisions[uid]
	if revision == 0 {
		revision = 1
	}

	return revision, nil
}

// GetMailingListByGroupID retrieves a mailing list by GroupsIO subgroup ID
// Returns NotFound error if mailing list doesn't exist
func (m *MockRepository) GetMailingListByGroupID(ctx context.Context, groupID uint64) (*model.GrpsIOMailingList, uint64, error) {
	slog.DebugContext(ctx, "mock mailing list: getting mailing list by group_id", "group_id", groupID)

	// Check error simulation first
	if err := m.checkErrorSimulation("GetMailingListByGroupID", fmt.Sprintf("%d", groupID)); err != nil {
		return nil, 0, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Iterate through all mailing lists to find match
	for uid, mailingList := range m.mailingLists {
		if mailingList.SubgroupID != nil && uint64(*mailingList.SubgroupID) == groupID {
			// Return deep copy to avoid data races
			mailingListCopy := *mailingList
			revision := m.mailingListRevisions[uid]
			if revision == 0 {
				revision = 1
			}

			slog.DebugContext(ctx, "found mailing list with matching group_id",
				"mailing_list_uid", mailingListCopy.UID,
				"group_name", mailingListCopy.GroupName,
				"group_id", groupID,
				"revision", revision)

			return &mailingListCopy, revision, nil
		}
	}

	slog.DebugContext(ctx, "mailing list not found by group_id", "group_id", groupID)
	return nil, 0, errors.NewNotFound("mailing list not found")
}

// CheckMailingListExists checks if a mailing list with the given name exists in parent service
func (m *MockRepository) CheckMailingListExists(ctx context.Context, parentID, groupName string) (bool, error) {
	slog.DebugContext(ctx, "mock mailing list: checking mailing list existence", "parent_id", parentID, "group_name", groupName)

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, mailingList := range m.mailingLists {
		if mailingList.ServiceUID == parentID && mailingList.GroupName == groupName {
			return true, nil
		}
	}

	return false, nil
}

// ==================== MAILING LIST WRITER OPERATIONS ====================

// CreateGrpsIOMailingList creates a new mailing list in the mock storage (interface implementation)
func (m *MockRepository) CreateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, uint64, error) {
	slog.DebugContext(ctx, "mock mailing list: creating mailing list", "mailing_list_id", mailingList.UID)

	// Check error simulation first
	if err := m.checkErrorSimulation("CreateGrpsIOMailingList", mailingList.UID); err != nil {
		return nil, 0, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if mailing list already exists
	if _, exists := m.mailingLists[mailingList.UID]; exists {
		return nil, 0, errors.NewConflict("mailing list already exists")
	}

	// Set created/updated timestamps
	now := time.Now()
	mailingList.CreatedAt = now
	mailingList.UpdatedAt = now

	// Store mailing list copy to avoid external modifications
	mailingListCopy := *mailingList
	mailingListCopy.CommitteeFilters = make([]string, len(mailingList.CommitteeFilters))
	copy(mailingListCopy.CommitteeFilters, mailingList.CommitteeFilters)
	mailingListCopy.Writers = append([]string(nil), mailingList.Writers...)
	mailingListCopy.Auditors = append([]string(nil), mailingList.Auditors...)

	m.mailingLists[mailingList.UID] = &mailingListCopy
	m.mailingListRevisions[mailingList.UID] = 1
	m.mailingListIndexKeys[mailingList.BuildIndexKey(ctx)] = &mailingListCopy

	// Return mailing list copy
	resultCopy := mailingListCopy
	resultCopy.CommitteeFilters = make([]string, len(mailingListCopy.CommitteeFilters))
	copy(resultCopy.CommitteeFilters, mailingListCopy.CommitteeFilters)
	resultCopy.Writers = append([]string(nil), mailingListCopy.Writers...)
	resultCopy.Auditors = append([]string(nil), mailingListCopy.Auditors...)

	return &resultCopy, 1, nil
}

// UpdateGrpsIOMailingList updates an existing mailing list (interface implementation)
func (m *MockRepository) UpdateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "mock mailing list: updating mailing list", "mailing_list_uid", mailingList.UID)

	// Check error simulation first
	if err := m.checkErrorSimulation("UpdateGrpsIOMailingList", mailingList.UID); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if mailing list exists
	existingMailingList, exists := m.mailingLists[mailingList.UID]
	if !exists {
		return nil, errors.NewNotFound("mailing list not found")
	}

	// Remove old index key
	oldIndexKey := existingMailingList.BuildIndexKey(ctx)
	delete(m.mailingListIndexKeys, oldIndexKey)

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
	currentRevision := m.mailingListRevisions[mailingList.UID]
	m.mailingListRevisions[mailingList.UID] = currentRevision + 1
	m.mailingListIndexKeys[mailingList.BuildIndexKey(ctx)] = &mailingListCopy

	// Return mailing list copy
	resultCopy := mailingListCopy
	resultCopy.CommitteeFilters = make([]string, len(mailingListCopy.CommitteeFilters))
	copy(resultCopy.CommitteeFilters, mailingListCopy.CommitteeFilters)
	resultCopy.Writers = append([]string(nil), mailingListCopy.Writers...)
	resultCopy.Auditors = append([]string(nil), mailingListCopy.Auditors...)

	return &resultCopy, nil
}

// UpdateGrpsIOMailingListWithRevision updates an existing mailing list with revision checking (internal helper)
func (m *MockRepository) UpdateGrpsIOMailingListWithRevision(ctx context.Context, uid string, mailingList *model.GrpsIOMailingList, expectedRevision uint64) (*model.GrpsIOMailingList, uint64, error) {
	slog.DebugContext(ctx, "mock mailing list: updating mailing list with revision", "mailing_list_uid", uid, "expected_revision", expectedRevision)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if mailing list exists
	existingMailingList, exists := m.mailingLists[uid]
	if !exists {
		return nil, 0, errors.NewNotFound("mailing list not found")
	}

	// Check revision
	currentRevision := m.mailingListRevisions[uid]
	if currentRevision != expectedRevision {
		return nil, 0, errors.NewConflict(fmt.Sprintf("revision mismatch: expected %d, got %d", expectedRevision, currentRevision))
	}

	// Remove old index key
	oldIndexKey := existingMailingList.BuildIndexKey(ctx)
	delete(m.mailingListIndexKeys, oldIndexKey)

	// Preserve created timestamp, update updated timestamp
	mailingList.CreatedAt = existingMailingList.CreatedAt
	mailingList.UpdatedAt = time.Now()
	mailingList.UID = uid // Ensure UID matches

	// Store mailing list copy
	mailingListCopy := *mailingList
	mailingListCopy.CommitteeFilters = make([]string, len(mailingList.CommitteeFilters))
	copy(mailingListCopy.CommitteeFilters, mailingList.CommitteeFilters)
	mailingListCopy.Writers = append([]string(nil), mailingList.Writers...)
	mailingListCopy.Auditors = append([]string(nil), mailingList.Auditors...)

	m.mailingLists[uid] = &mailingListCopy
	newRevision := currentRevision + 1
	m.mailingListRevisions[uid] = newRevision
	m.mailingListIndexKeys[mailingList.BuildIndexKey(ctx)] = &mailingListCopy

	// Return mailing list copy
	resultCopy := mailingListCopy
	resultCopy.CommitteeFilters = make([]string, len(mailingListCopy.CommitteeFilters))
	copy(resultCopy.CommitteeFilters, mailingListCopy.CommitteeFilters)
	resultCopy.Writers = append([]string(nil), mailingListCopy.Writers...)
	resultCopy.Auditors = append([]string(nil), mailingListCopy.Auditors...)

	return &resultCopy, newRevision, nil
}

// DeleteGrpsIOMailingList deletes a mailing list (interface implementation)
func (m *MockRepository) DeleteGrpsIOMailingList(ctx context.Context, uid string) error {
	slog.DebugContext(ctx, "mock mailing list: deleting mailing list", "mailing_list_uid", uid)

	// Check error simulation first
	if err := m.checkErrorSimulation("DeleteGrpsIOMailingList", uid); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if mailing list exists
	mailingList, exists := m.mailingLists[uid]
	if !exists {
		return errors.NewNotFound("mailing list not found")
	}

	// Get the index key before deleting
	indexKey := mailingList.BuildIndexKey(ctx)

	// Delete mailing list and its indices
	delete(m.mailingLists, uid)
	delete(m.mailingListRevisions, uid)
	delete(m.mailingListIndexKeys, indexKey)

	return nil
}

// DeleteGrpsIOMailingListWithRevision deletes a mailing list with revision checking (internal helper)
func (m *MockRepository) DeleteGrpsIOMailingListWithRevision(ctx context.Context, uid string, expectedRevision uint64) error {
	slog.DebugContext(ctx, "mock mailing list: deleting mailing list with revision", "mailing_list_uid", uid, "expected_revision", expectedRevision)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if mailing list exists
	mailingList, exists := m.mailingLists[uid]
	if !exists {
		return errors.NewNotFound("mailing list not found")
	}

	// Check revision
	currentRevision := m.mailingListRevisions[uid]
	if currentRevision != expectedRevision {
		return errors.NewConflict(fmt.Sprintf("revision mismatch: expected %d, got %d", expectedRevision, currentRevision))
	}

	// Get the index key before deleting
	indexKey := mailingList.BuildIndexKey(ctx)

	// Delete mailing list and its indices
	delete(m.mailingLists, uid)
	delete(m.mailingListRevisions, uid)
	delete(m.mailingListIndexKeys, indexKey)

	return nil
}

// CreateSecondaryIndices creates secondary indices for a mailing list (mock implementation)
func (m *MockRepository) CreateSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) ([]string, error) {
	slog.DebugContext(ctx, "mock mailing list: creating secondary indices", "mailing_list_uid", mailingList.UID)

	// Mock implementation - return mock keys that would be created
	createdKeys := []string{
		fmt.Sprintf(constants.KVLookupGroupsIOMailingListServicePrefix, mailingList.ServiceUID),
		fmt.Sprintf(constants.KVLookupGroupsIOMailingListProjectPrefix, mailingList.ProjectUID),
	}

	if mailingList.CommitteeUID != "" {
		createdKeys = append(createdKeys, fmt.Sprintf(constants.KVLookupGroupsIOMailingListCommitteePrefix, mailingList.CommitteeUID))
	}

	return createdKeys, nil
}

// UniqueMailingListGroupName validates that group name is unique within parent service
func (m *MockRepository) UniqueMailingListGroupName(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error) {
	constraintKey := fmt.Sprintf("lookup:mailing_list:constraint:%s:%s", mailingList.ServiceUID, mailingList.GroupName)
	slog.DebugContext(ctx, "mock: validating unique mailing list group name", "constraint_key", constraintKey)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if there's already a mailing list with the same group name and parent
	for _, existingList := range m.mailingLists {
		if existingList.ServiceUID == mailingList.ServiceUID && existingList.GroupName == mailingList.GroupName {
			// Skip if it's the same mailing list (during updates)
			if mailingList.UID != "" && existingList.UID == mailingList.UID {
				continue
			}
			return existingList.UID, errors.NewConflict(fmt.Sprintf("mailing list with group name '%s' already exists in service '%s'", mailingList.GroupName, mailingList.ServiceUID))
		}
	}

	return constraintKey, nil
}

// AddMailingList adds a mailing list to the mock data (useful for testing)
func (m *MockRepository) AddMailingList(mailingList *model.GrpsIOMailingList) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mailingLists[mailingList.UID] = mailingList
	m.mailingListRevisions[mailingList.UID] = 1
}

// GetMailingListCount returns the number of mailing lists in mock data (useful for testing)
func (m *MockRepository) GetMailingListCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.mailingLists)
}

// ==================== MEMBER READER OPERATIONS ====================

// GetGrpsIOMember retrieves a member by UID (interface implementation)
func (m *MockRepository) GetGrpsIOMember(ctx context.Context, uid string) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "mock member: getting member", "member_uid", uid)

	// Check error simulation first
	if err := m.checkErrorSimulation("GetGrpsIOMember", uid); err != nil {
		return nil, 0, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	member, exists := m.members[uid]
	if !exists {
		return nil, 0, errors.NewNotFound(fmt.Sprintf("member with UID %s not found", uid))
	}

	// Return a deep copy of the member to avoid data races
	memberCopy := *member
	revision := m.memberRevisions[uid]
	return &memberCopy, revision, nil
}

// GetMemberRevision retrieves only the revision for a given member UID (interface implementation)
func (m *MockRepository) GetMemberRevision(ctx context.Context, uid string) (uint64, error) {
	slog.DebugContext(ctx, "mock member: getting member revision", "member_uid", uid)

	m.mu.RLock()
	defer m.mu.RUnlock()

	if rev, exists := m.memberRevisions[uid]; exists {
		return rev, nil
	}

	return 0, errors.NewNotFound("member not found")
}

// GetMemberByGroupsIOMemberID retrieves member by Groups.io member ID (interface implementation)
func (m *MockRepository) GetMemberByGroupsIOMemberID(ctx context.Context, memberID uint64) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "mock member: getting member by Groups.io member ID", "member_id", memberID)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Search through all members for matching Groups.io member ID
	for uid, member := range m.members {
		if member.GroupsIOMemberID != nil && uint64(*member.GroupsIOMemberID) == memberID {
			memberCopy := *member
			revision := m.memberRevisions[uid]
			return &memberCopy, revision, nil
		}
	}

	return nil, 0, errors.NewNotFound(fmt.Sprintf("member with Groups.io member ID %d not found", memberID))
}

// GetMemberByEmail retrieves member by email within mailing list (interface implementation)
func (m *MockRepository) GetMemberByEmail(ctx context.Context, mailingListUID, email string) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "mock member: getting member by email",
		"mailing_list_uid", mailingListUID,
		"email", email)

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Search through all members for matching mailing list and email
	for uid, member := range m.members {
		if member.MailingListUID == mailingListUID && member.Email == email {
			memberCopy := *member
			revision := m.memberRevisions[uid]
			return &memberCopy, revision, nil
		}
	}

	return nil, 0, errors.NewNotFound(fmt.Sprintf("member with email %s not found in mailing list %s", email, mailingListUID))
}

// AddMember adds a member to the mock repository for testing
func (m *MockRepository) AddMember(member *model.GrpsIOMember) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Store member copy to avoid external modifications
	memberCopy := *member

	m.members[member.UID] = &memberCopy
	m.memberRevisions[member.UID] = 1
	// Generate index key for the member
	ctx := context.Background()
	m.memberIndexKeys[member.BuildIndexKey(ctx)] = &memberCopy
}

// GetMemberCount returns the number of members in the mock repository
func (m *MockRepository) GetMemberCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.members)
}

// ClearMembers clears all member data from the mock repository
func (m *MockRepository) ClearMembers() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.members = make(map[string]*model.GrpsIOMember)
	m.memberRevisions = make(map[string]uint64)
	m.memberIndexKeys = make(map[string]*model.GrpsIOMember)
}
