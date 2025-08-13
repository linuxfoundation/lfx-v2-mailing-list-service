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
			ID:           "service-1",
			Domain:       "lists.testproject.org",
			GroupID:      12345,
			Status:       "created",
			GlobalOwners: []string{"admin@testproject.org"},
			Prefix:       "",
			ProjectSlug:  "test-project",
			ProjectID:    "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
			URL:          "https://lists.testproject.org",
			GroupName:    "test-project",
			Public:       true,
			CreatedAt:    now.Add(-24 * time.Hour),
			UpdatedAt:    now,
		},
		{
			Type:         "v2_formation",
			ID:           "service-2",
			Domain:       "lists.formation.testproject.org",
			GroupID:      12346,
			Status:       "created",
			GlobalOwners: []string{"formation@testproject.org"},
			Prefix:       "formation",
			ProjectSlug:  "test-project",
			ProjectID:    "7cad5a8d-19d0-41a4-81a6-043453daf9ee",
			URL:          "https://lists.formation.testproject.org",
			GroupName:    "test-project-formation",
			Public:       false,
			CreatedAt:    now.Add(-12 * time.Hour),
			UpdatedAt:    now,
		},
		{
			Type:         "v2_primary",
			ID:           "service-3",
			Domain:       "lists.example.org",
			GroupID:      12347,
			Status:       "pending",
			GlobalOwners: []string{"owner@example.org", "admin@example.org"},
			Prefix:       "",
			ProjectSlug:  "example-project",
			ProjectID:    "8dbc6b9e-20e1-42b5-92b7-154564eaf0ff",
			URL:          "https://lists.example.org",
			GroupName:    "example-project",
			Public:       true,
			CreatedAt:    now.Add(-6 * time.Hour),
			UpdatedAt:    now.Add(-1 * time.Hour),
		},
	}

	// Store services by ID
	for _, service := range sampleServices {
		mock.services[service.ID] = service
		mock.serviceRevisions[service.ID] = 1
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

// IsReady checks if the service is ready (always returns nil for mocks)
func (m *MockGrpsIOService) IsReady(ctx context.Context) error {
	slog.DebugContext(ctx, "mock service ready check: always ready")
	return nil
}