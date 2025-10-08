// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestEtagValidator(t *testing.T) {
	tests := []struct {
		name      string
		etag      *string
		expected  uint64
		expectErr bool
	}{
		{
			name:      "valid numeric etag",
			etag:      stringPtr("123"),
			expected:  123,
			expectErr: false,
		},
		{
			name:      "valid quoted etag",
			etag:      stringPtr(`"456"`),
			expected:  456,
			expectErr: false,
		},
		{
			name:      "valid weak etag",
			etag:      stringPtr(`W/"789"`),
			expected:  789,
			expectErr: false,
		},
		{
			name:      "valid weak etag lowercase",
			etag:      stringPtr(`w/"101"`),
			expected:  101,
			expectErr: false,
		},
		{
			name:      "nil etag",
			etag:      nil,
			expected:  0,
			expectErr: true,
		},
		{
			name:      "empty etag",
			etag:      stringPtr(""),
			expected:  0,
			expectErr: true,
		},
		{
			name:      "invalid etag format",
			etag:      stringPtr("invalid"),
			expected:  0,
			expectErr: true,
		},
		{
			name:      "etag with spaces",
			etag:      stringPtr("  123  "),
			expected:  123,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := etagValidator(tt.etag)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestValidateServiceCreationRules(t *testing.T) {
	tests := []struct {
		name      string
		payload   *mailinglistservice.CreateGrpsioServicePayload
		expectErr bool
	}{
		{
			name: "valid primary service",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:         "primary",
				GlobalOwners: []string{"owner@example.com"},
			},
			expectErr: false,
		},
		{
			name: "valid formation service",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:   "formation",
				Prefix: stringPtr("test-prefix"),
			},
			expectErr: false,
		},
		{
			name: "valid shared service",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:    "shared",
				Prefix:  stringPtr("shared-prefix"),
				GroupID: int64Ptr(12345),
			},
			expectErr: false,
		},
		{
			name: "invalid service type",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type: "invalid-type",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServiceCreationRules(tt.payload)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePrimaryRules(t *testing.T) {
	tests := []struct {
		name      string
		payload   *mailinglistservice.CreateGrpsioServicePayload
		expectErr bool
	}{
		{
			name: "valid primary service",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:         "primary",
				GlobalOwners: []string{"owner@example.com"},
			},
			expectErr: false,
		},
		{
			name: "primary service with prefix should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:         "primary",
				Prefix:       stringPtr("test-prefix"),
				GlobalOwners: []string{"owner@example.com"},
			},
			expectErr: true,
		},
		{
			name: "primary service without global owners should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type: "primary",
			},
			expectErr: true,
		},
		{
			name: "primary service with invalid email should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:         "primary",
				GlobalOwners: []string{"invalid-email"},
			},
			expectErr: true,
		},
		{
			name: "primary service with too many emails should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:         "primary",
				GlobalOwners: []string{"owner1@example.com", "owner2@example.com", "owner3@example.com", "owner4@example.com", "owner5@example.com", "owner6@example.com", "owner7@example.com", "owner8@example.com", "owner9@example.com", "owner10@example.com", "owner11@example.com"},
			},
			expectErr: true,
		},
		{
			name: "primary service with empty prefix string is valid",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:         "primary",
				Prefix:       stringPtr(""),
				GlobalOwners: []string{"owner@example.com"},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrimaryRules(tt.payload)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateFormationRules(t *testing.T) {
	tests := []struct {
		name      string
		payload   *mailinglistservice.CreateGrpsioServicePayload
		expectErr bool
	}{
		{
			name: "valid formation service",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:   "formation",
				Prefix: stringPtr("test-prefix"),
			},
			expectErr: false,
		},
		{
			name: "formation service with global owners",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:         "formation",
				Prefix:       stringPtr("test-prefix"),
				GlobalOwners: []string{"owner@example.com"},
			},
			expectErr: false,
		},
		{
			name: "formation service without prefix should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type: "formation",
			},
			expectErr: true,
		},
		{
			name: "formation service with empty prefix should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:   "formation",
				Prefix: stringPtr(""),
			},
			expectErr: true,
		},
		{
			name: "formation service with whitespace prefix should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:   "formation",
				Prefix: stringPtr("   "),
			},
			expectErr: true,
		},
		{
			name: "formation service with invalid email should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:         "formation",
				Prefix:       stringPtr("test-prefix"),
				GlobalOwners: []string{"invalid-email"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFormationRules(tt.payload)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateSharedRules(t *testing.T) {
	tests := []struct {
		name      string
		payload   *mailinglistservice.CreateGrpsioServicePayload
		expectErr bool
	}{
		{
			name: "valid shared service",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:    "shared",
				Prefix:  stringPtr("shared-prefix"),
				GroupID: int64Ptr(12345),
			},
			expectErr: false,
		},
		{
			name: "shared service without prefix should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:    "shared",
				GroupID: int64Ptr(12345),
			},
			expectErr: true,
		},
		{
			name: "shared service with empty prefix should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:    "shared",
				Prefix:  stringPtr(""),
				GroupID: int64Ptr(12345),
			},
			expectErr: true,
		},
		{
			name: "shared service without group_id should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:   "shared",
				Prefix: stringPtr("shared-prefix"),
			},
			expectErr: true,
		},
		{
			name: "shared service with invalid group_id should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:    "shared",
				Prefix:  stringPtr("shared-prefix"),
				GroupID: int64Ptr(0),
			},
			expectErr: true,
		},
		{
			name: "shared service with negative group_id should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:    "shared",
				Prefix:  stringPtr("shared-prefix"),
				GroupID: int64Ptr(-1),
			},
			expectErr: true,
		},
		{
			name: "shared service with global owners should fail",
			payload: &mailinglistservice.CreateGrpsioServicePayload{
				Type:         "shared",
				Prefix:       stringPtr("shared-prefix"),
				GroupID:      int64Ptr(12345),
				GlobalOwners: []string{"owner@example.com"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSharedRules(tt.payload)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateUpdateImmutabilityConstraints(t *testing.T) {
	existing := &model.GrpsIOService{
		Type:       "primary",
		ProjectUID: "project-123",
		Prefix:     "",
		Domain:     "example.groups.io",
		GroupID:    int64Ptr(12345),
		URL:        "https://example.groups.io/g/test",
		GroupName:  "test-group",
	}

	tests := []struct {
		name      string
		payload   *mailinglistservice.UpdateGrpsioServicePayload
		expectErr bool
	}{
		{
			name: "valid update with mutable fields only",
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				Type:         "primary",
				ProjectUID:   "project-123",
				Status:       stringPtr("active"),
				GlobalOwners: []string{"newowner@example.com"},
				Public:       true,
			},
			expectErr: false,
		},
		{
			name: "attempt to change type should fail",
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				Type:       "formation",
				ProjectUID: "project-123",
			},
			expectErr: true,
		},
		{
			name: "attempt to change project_uid should fail",
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				Type:       "primary",
				ProjectUID: "different-project",
			},
			expectErr: true,
		},
		{
			name: "attempt to change prefix should fail",
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				Type:       "primary",
				ProjectUID: "project-123",
				Prefix:     stringPtr("new-prefix"),
			},
			expectErr: true,
		},
		{
			name: "attempt to change domain should fail",
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				Type:       "primary",
				ProjectUID: "project-123",
				Domain:     stringPtr("different.groups.io"),
			},
			expectErr: true,
		},
		{
			name: "attempt to change group_id should fail",
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				Type:       "primary",
				ProjectUID: "project-123",
				GroupID:    int64Ptr(99999),
			},
			expectErr: true,
		},
		{
			name: "attempt to change url should fail",
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				Type:       "primary",
				ProjectUID: "project-123",
				URL:        stringPtr("https://different.groups.io/g/test"),
			},
			expectErr: true,
		},
		{
			name: "attempt to change group_name should fail",
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				Type:       "primary",
				ProjectUID: "project-123",
				GroupName:  stringPtr("different-group"),
			},
			expectErr: true,
		},
		{
			name: "update with invalid email should fail",
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				Type:         "primary",
				ProjectUID:   "project-123",
				GlobalOwners: []string{"invalid-email"},
			},
			expectErr: true,
		},
		{
			name: "update with too many global owners should fail",
			payload: &mailinglistservice.UpdateGrpsioServicePayload{
				Type:         "primary",
				ProjectUID:   "project-123",
				GlobalOwners: []string{"owner1@example.com", "owner2@example.com", "owner3@example.com", "owner4@example.com", "owner5@example.com", "owner6@example.com", "owner7@example.com", "owner8@example.com", "owner9@example.com", "owner10@example.com", "owner11@example.com"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUpdateImmutabilityConstraints(existing, tt.payload)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDeleteProtectionRules(t *testing.T) {
	tests := []struct {
		name      string
		service   *model.GrpsIOService
		expectErr bool
	}{
		{
			name: "primary service deletion should fail",
			service: &model.GrpsIOService{
				UID:  "service-123",
				Type: "primary",
			},
			expectErr: true,
		},
		{
			name: "formation service deletion should succeed",
			service: &model.GrpsIOService{
				UID:  "service-456",
				Type: "formation",
			},
			expectErr: false,
		},
		{
			name: "shared service deletion should succeed",
			service: &model.GrpsIOService{
				UID:  "service-789",
				Type: "shared",
			},
			expectErr: false,
		},
		{
			name: "unknown service type deletion should fail",
			service: &model.GrpsIOService{
				UID:  "service-unknown",
				Type: "unknown",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDeleteProtectionRules(tt.service)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEmailAddresses(t *testing.T) {
	tests := []struct {
		name      string
		emails    []string
		fieldName string
		expectErr bool
	}{
		{
			name:      "valid email addresses",
			emails:    []string{"test@example.com", "user@domain.org"},
			fieldName: "global_owners",
			expectErr: false,
		},
		{
			name:      "single valid email",
			emails:    []string{"valid@email.com"},
			fieldName: "global_owners",
			expectErr: false,
		},
		{
			name:      "invalid email address",
			emails:    []string{"invalid-email"},
			fieldName: "global_owners",
			expectErr: true,
		},
		{
			name:      "mixed valid and invalid emails",
			emails:    []string{"valid@email.com", "invalid-email"},
			fieldName: "global_owners",
			expectErr: true,
		},
		{
			name:      "nil email slice",
			emails:    nil,
			fieldName: "global_owners",
			expectErr: false,
		},
		{
			name:      "empty email slice",
			emails:    []string{},
			fieldName: "global_owners",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmailAddresses(tt.emails, tt.fieldName)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMailingListCreation(t *testing.T) {
	tests := []struct {
		name      string
		payload   *mailinglistservice.CreateGrpsioMailingListPayload
		expectErr bool
	}{
		{
			name: "valid mailing list payload",
			payload: &mailinglistservice.CreateGrpsioMailingListPayload{
				GroupName:   "test-list",
				Type:        "discussion_open",
				Description: "This is a test mailing list description",
				Title:       "Test List",
				ServiceUID:  "parent-123",
			},
			expectErr: false,
		},
		{
			name: "valid mailing list with committee",
			payload: &mailinglistservice.CreateGrpsioMailingListPayload{
				GroupName:        "committee-list",
				Type:             "discussion_moderated",
				CommitteeUID:     stringPtr("committee-123"),
				CommitteeFilters: []string{"Voting Rep", "Observer"},
				Description:      "Committee-based mailing list",
				Title:            "Committee List",
				ServiceUID:       "parent-456",
			},
			expectErr: false,
		},
		{
			name:      "nil payload should fail",
			payload:   nil,
			expectErr: true,
		},
		// Group name length test removed - now handled by GOA MaxLength validation
		{
			name: "committee filters without committee should fail",
			payload: &mailinglistservice.CreateGrpsioMailingListPayload{
				GroupName:        "invalid-list",
				Type:             "discussion_open",
				CommitteeFilters: []string{"Voting Rep"},
				Description:      "Invalid committee setup",
				Title:            "Invalid List",
				ServiceUID:       "parent-123",
			},
			expectErr: true,
		},
		// Committee filter enum test removed - now handled by GOA enum validation
		// Group name length test removed - now handled by GOA MinLength validation
		// Group name pattern test removed - now handled by GOA Pattern validation
		{
			name: "reserved word group name should fail",
			payload: &mailinglistservice.CreateGrpsioMailingListPayload{
				GroupName:   "admin",
				Type:        "announcement",
				Description: "Test description that meets minimum length",
				Title:       "Test Title",
				ServiceUID:  "parent-123",
			},
			expectErr: true,
		},
		// Title length test removed - now handled by GOA MinLength validation
		// Title length test removed - now handled by GOA MaxLength validation
		// Description length test removed - now handled by GOA MaxLength validation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMailingListCreation(tt.payload)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{
			name:     "item exists in slice",
			slice:    []string{"a", "b", "c"},
			item:     "b",
			expected: true,
		},
		{
			name:     "item does not exist in slice",
			slice:    []string{"a", "b", "c"},
			item:     "d",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			item:     "a",
			expected: false,
		},
		{
			name:     "nil slice",
			slice:    nil,
			item:     "a",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// MockServiceReader is a mock implementation of the GrpsIOServiceReader interface
type MockServiceReader struct {
	mock.Mock
}

func (m *MockServiceReader) GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error) {
	args := m.Called(ctx, uid)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).(*model.GrpsIOService), args.Get(1).(uint64), args.Error(2)
}

func (m *MockServiceReader) GetRevision(ctx context.Context, uid string) (uint64, error) {
	args := m.Called(ctx, uid)
	return args.Get(0).(uint64), args.Error(1)
}

func (m *MockServiceReader) GetServicesByGroupID(ctx context.Context, groupID uint64) ([]*model.GrpsIOService, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.GrpsIOService), args.Error(1)
}

func TestValidateMailingListUpdateParentServiceChange(t *testing.T) {
	ctx := context.Background()

	existingMailingList := &model.GrpsIOMailingList{
		UID:         "mailing-list-123",
		GroupName:   "sub-list", // Different from parent service to avoid main group rules
		Public:      false,
		Type:        "discussion_open",
		ServiceUID:  "service-123",
		ProjectUID:  "project-123",
		ProjectName: "Test Project",
		Description: "Test description for mailing list",
	}

	parentService := &model.GrpsIOService{
		UID:        "service-123",
		Type:       "primary",
		ProjectUID: "project-123",
		GroupName:  "main-group", // Different from mailing list to avoid main group rules
	}

	tests := []struct {
		name           string
		payload        *mailinglistservice.UpdateGrpsioMailingListPayload
		setupMock      func(*MockServiceReader)
		expectErr      bool
		expectedErrMsg string
	}{
		{
			name: "valid update without parent service change",
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				GroupName:   "sub-list",    // unchanged
				ServiceUID:  "service-123", // unchanged
				Description: "Updated description that is long enough",
				Type:        "discussion_moderated",
				Public:      false,
			},
			setupMock: func(m *MockServiceReader) {
				// No mock calls expected since ServiceUID hasn't changed
			},
			expectErr: false,
		},
		{
			name: "valid parent service change within same project",
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				GroupName:   "sub-list",
				ServiceUID:  "service-456", // different service
				Description: "Updated description that is long enough",
				Type:        "discussion_open",
				Public:      false,
			},
			setupMock: func(m *MockServiceReader) {
				newService := &model.GrpsIOService{
					UID:        "service-456",
					Type:       "formation",
					ProjectUID: "project-123", // same project
					Prefix:     "formation",
				}
				m.On("GetGrpsIOService", ctx, "service-456").Return(newService, uint64(1), nil)
			},
			expectErr: false,
		},
		{
			name: "blocked cross-project parent service change",
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				GroupName:   "sub-list",
				ServiceUID:  "service-different-project", // different project
				Description: "Updated description that is long enough",
				Type:        "discussion_open",
				Public:      false,
			},
			setupMock: func(m *MockServiceReader) {
				differentProjectService := &model.GrpsIOService{
					UID:        "service-different-project",
					Type:       "primary",
					ProjectUID: "different-project-456", // different project
				}
				m.On("GetGrpsIOService", ctx, "service-different-project").Return(differentProjectService, uint64(1), nil)
			},
			expectErr:      true,
			expectedErrMsg: "cannot move mailing list to service in different project",
		},
		{
			name: "parent service change with non-existent service",
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				GroupName:   "sub-list",
				ServiceUID:  "non-existent-service",
				Description: "Updated description that is long enough",
				Type:        "discussion_open",
				Public:      false,
			},
			setupMock: func(m *MockServiceReader) {
				m.On("GetGrpsIOService", ctx, "non-existent-service").Return(nil, uint64(0), errors.NewServiceUnavailable("service not found"))
			},
			expectErr:      true,
			expectedErrMsg: "new parent service not found",
		},
		{
			name: "immutable group_name change should fail",
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				GroupName:   "different-list-name", // changed
				ServiceUID:  "service-123",
				Description: "Updated description that is long enough",
				Type:        "discussion_open",
				Public:      false,
			},
			setupMock: func(m *MockServiceReader) {
				// No mock calls expected since validation should fail before that
			},
			expectErr:      true,
			expectedErrMsg: "field 'group_name' is immutable",
		},
		{
			name: "private to public visibility change should fail",
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				GroupName:   "sub-list",
				ServiceUID:  "service-123",
				Description: "Updated description that is long enough",
				Type:        "discussion_open",
				Public:      true, // changing from private to public
			},
			setupMock: func(m *MockServiceReader) {
				// No mock calls expected since validation should fail before that
			},
			expectErr:      true,
			expectedErrMsg: "cannot change visibility from private to public",
		},
		// Description length test removed - now handled by GOA MinLength validation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockReader := new(MockServiceReader)
			tt.setupMock(mockReader)

			err := validateMailingListUpdate(ctx, existingMailingList, parentService, tt.payload, mockReader)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			mockReader.AssertExpectations(t)
		})
	}
}

// ==================================================================================
// Enhanced Business Rule Validation Tests (POST-PUT Conversion)
// ==================================================================================

func TestValidateServiceBusinessRules(t *testing.T) {
	tests := []struct {
		name      string
		service   *model.GrpsIOService
		expectErr bool
		errMsg    string
	}{
		{
			name: "primary service with GlobalOwners should pass",
			service: &model.GrpsIOService{
				Type:         "primary",
				GlobalOwners: []string{"owner@example.com"},
			},
			expectErr: false,
		},
		{
			name: "primary service without GlobalOwners should fail",
			service: &model.GrpsIOService{
				Type:         "primary",
				GlobalOwners: []string{}, // Empty - should fail
			},
			expectErr: true,
			errMsg:    "primary services must have at least one global owner",
		},
		{
			name: "primary service with nil GlobalOwners should fail",
			service: &model.GrpsIOService{
				Type:         "primary",
				GlobalOwners: nil, // Nil - should fail
			},
			expectErr: true,
			errMsg:    "primary services must have at least one global owner",
		},
		{
			name: "formation service without GlobalOwners should pass",
			service: &model.GrpsIOService{
				Type:         "formation",
				GlobalOwners: []string{}, // Empty is OK for formation
			},
			expectErr: false,
		},
		{
			name: "shared service without GlobalOwners should pass",
			service: &model.GrpsIOService{
				Type:         "shared",
				GlobalOwners: nil, // Nil is OK for shared
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServiceBusinessRules(tt.service)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
