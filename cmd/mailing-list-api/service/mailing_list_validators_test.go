// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"testing"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// TestValidateDescriptionLength was removed - description length validation
// is now handled by GOA design layer (MinLength/MaxLength attributes)

func TestValidateMailingListUpdate(t *testing.T) {
	tests := []struct {
		name          string
		existing      *model.GrpsIOMailingList
		parentService *model.GrpsIOService
		payload       *mailinglistservice.UpdateGrpsioMailingListPayload
		wantErr       bool
		errMsg        string
	}{
		{
			name: "valid update - same visibility",
			existing: &model.GrpsIOMailingList{
				UID:        "ml-123",
				GroupName:  "test-group",
				Public:     true,
				ServiceUID: "svc-123",
			},
			parentService: &model.GrpsIOService{
				UID:       "svc-123",
				GroupName: "different-group",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				GroupName:   "test-group",
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description with enough characters",
			},
			wantErr: false,
		},
		{
			name: "valid update - public to public",
			existing: &model.GrpsIOMailingList{
				UID:        "ml-123",
				Public:     true,
				ServiceUID: "svc-123",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description with enough characters",
			},
			wantErr: false,
		},
		{
			name: "invalid update - private to public",
			existing: &model.GrpsIOMailingList{
				UID:        "ml-123",
				Public:     false,
				ServiceUID: "svc-123",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description with enough characters",
			},
			wantErr: true,
			errMsg:  "cannot change visibility from private to public",
		},
		{
			name: "invalid update - change service uid",
			existing: &model.GrpsIOMailingList{
				UID:        "ml-123",
				Public:     true,
				ServiceUID: "svc-123",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				Public:      true,
				ServiceUID:  "svc-456",
				Description: "Valid description with enough characters",
			},
			wantErr: true,
			errMsg:  "cannot change parent service",
		},
		// Description length validation test removed - now handled by GOA design layer
		{
			name: "valid update - valid subject tag",
			existing: &model.GrpsIOMailingList{
				UID:        "ml-123",
				Public:     true,
				ServiceUID: "svc-123",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description with enough characters",
				SubjectTag:  stringPtr("VALID-TAG"),
			},
			wantErr: false,
		},
		{
			name: "invalid update - invalid subject tag",
			existing: &model.GrpsIOMailingList{
				UID:        "ml-123",
				Public:     true,
				ServiceUID: "svc-123",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description with enough characters",
				SubjectTag:  stringPtr("[INVALID]"),
			},
			wantErr: true,
			errMsg:  "invalid subject tag format",
		},
		// Committee filter validation test removed - now handled by GOA design layer
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Provide nil parent service for tests that don't need it
			// Pass nil service reader since these tests don't test parent service changes
			err := validateMailingListUpdate(context.Background(), tt.existing, tt.parentService, tt.payload, nil)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateMailingListUpdate() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("validateMailingListUpdate() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateMailingListUpdate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

// TestValidateMailingListUpdateNewRules tests the new validation rules added for Groups.io compatibility
func TestValidateMailingListUpdateNewRules(t *testing.T) {
	tests := []struct {
		name          string
		existing      *model.GrpsIOMailingList
		parentService *model.GrpsIOService
		payload       *mailinglistservice.UpdateGrpsioMailingListPayload
		wantErr       bool
		errMsg        string
	}{
		{
			name: "invalid update - group name change (immutability rule)",
			existing: &model.GrpsIOMailingList{
				UID:         "ml-123",
				GroupName:   "original-group",
				Type:        "announcement",
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description",
			},
			parentService: &model.GrpsIOService{
				UID:       "svc-123",
				GroupName: "different-group",
				Type:      "formation",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				GroupName:   "changed-group",
				Type:        "announcement",
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description with enough characters",
			},
			wantErr: true,
			errMsg:  "field 'group_name' is immutable",
		},
		{
			name: "invalid update - main group type change",
			existing: &model.GrpsIOMailingList{
				UID:         "ml-123",
				GroupName:   "main-group",
				Type:        "announcement",
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description",
			},
			parentService: &model.GrpsIOService{
				UID:       "svc-123",
				GroupName: "main-group", // Same as mailing list - makes it a main group
				Type:      "primary",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				GroupName:   "main-group",
				Type:        "discussion_open",
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description with enough characters",
			},
			wantErr: true,
			errMsg:  "main group must be an announcement list",
		},
		{
			name: "invalid update - main group visibility change",
			existing: &model.GrpsIOMailingList{
				UID:         "ml-123",
				GroupName:   "main-group",
				Type:        "announcement",
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description",
			},
			parentService: &model.GrpsIOService{
				UID:       "svc-123",
				GroupName: "main-group", // Same as mailing list - makes it a main group
				Type:      "primary",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				GroupName:   "main-group",
				Type:        "announcement",
				Public:      false,
				ServiceUID:  "svc-123",
				Description: "Valid description with enough characters",
			},
			wantErr: true,
			errMsg:  "main group must remain public",
		},
		{
			name: "invalid update - set type to custom when not already custom",
			existing: &model.GrpsIOMailingList{
				UID:         "ml-123",
				GroupName:   "test-group",
				Type:        "announcement",
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description",
			},
			parentService: &model.GrpsIOService{
				UID:       "svc-123",
				GroupName: "different-group",
				Type:      "formation",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				GroupName:   "test-group",
				Type:        "custom",
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description with enough characters",
			},
			wantErr: true,
			errMsg:  "cannot set type to \"custom\"",
		},
		{
			name: "valid update - main group keeps valid values",
			existing: &model.GrpsIOMailingList{
				UID:         "ml-123",
				GroupName:   "main-group",
				Type:        "announcement",
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Valid description",
			},
			parentService: &model.GrpsIOService{
				UID:       "svc-123",
				GroupName: "main-group", // Same as mailing list - makes it a main group
				Type:      "primary",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				GroupName:   "main-group",
				Type:        "announcement",
				Public:      true,
				ServiceUID:  "svc-123",
				Description: "Updated description with enough characters",
			},
			wantErr: false,
		},
		{
			name: "valid update - non-main group can change type and visibility",
			existing: &model.GrpsIOMailingList{
				UID:         "ml-123",
				GroupName:   "sub-group",
				Type:        "announcement",
				Public:      false,
				ServiceUID:  "svc-123",
				Description: "Valid description",
			},
			parentService: &model.GrpsIOService{
				UID:       "svc-123",
				GroupName: "main-group", // Different from mailing list - not a main group
				Type:      "primary",
			},
			payload: &mailinglistservice.UpdateGrpsioMailingListPayload{
				UID:         stringPtr("ml-123"),
				GroupName:   "sub-group",
				Type:        "discussion_moderated",
				Public:      false,
				ServiceUID:  "svc-123",
				Description: "Updated description with enough characters",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMailingListUpdate(context.Background(), tt.existing, tt.parentService, tt.payload, nil)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateMailingListUpdate() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("validateMailingListUpdate() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateMailingListUpdate() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestValidateMailingListDeleteProtection(t *testing.T) {
	tests := []struct {
		name          string
		mailingList   *model.GrpsIOMailingList
		parentService *model.GrpsIOService
		wantErr       bool
		errMsg        string
	}{
		{
			name: "valid delete - non-primary service",
			mailingList: &model.GrpsIOMailingList{
				UID:       "ml-123",
				GroupName: "test-group",
			},
			parentService: &model.GrpsIOService{
				Type:      "formation",
				GroupName: "parent-group",
			},
			wantErr: false,
		},
		{
			name: "valid delete - primary service but different group name",
			mailingList: &model.GrpsIOMailingList{
				UID:       "ml-123",
				GroupName: "subgroup",
			},
			parentService: &model.GrpsIOService{
				Type:      "primary",
				GroupName: "main-group",
			},
			wantErr: false,
		},
		{
			name: "invalid delete - main group of primary service",
			mailingList: &model.GrpsIOMailingList{
				UID:       "ml-123",
				GroupName: "main-group",
			},
			parentService: &model.GrpsIOService{
				Type:      "primary",
				GroupName: "main-group",
			},
			wantErr: true,
			errMsg:  "cannot delete the main group of a primary service",
		},
		{
			name: "valid delete - no parent service",
			mailingList: &model.GrpsIOMailingList{
				UID:       "ml-123",
				GroupName: "test-group",
			},
			parentService: nil,
			wantErr:       false,
		},
		{
			name: "valid delete - committee-based list (with debug log)",
			mailingList: &model.GrpsIOMailingList{
				UID:       "ml-123",
				GroupName: "committee-group",
				Committees: []model.Committee{
					{UID: "committee-456"},
				},
			},
			parentService: &model.GrpsIOService{
				Type:      "formation",
				GroupName: "parent-group",
			},
			wantErr: false,
		},
		{
			name: "invalid delete - main group of formation service",
			mailingList: &model.GrpsIOMailingList{
				UID:       "ml-123",
				GroupName: "form-prefix",
			},
			parentService: &model.GrpsIOService{
				Type:   "formation",
				Prefix: "form-prefix",
			},
			wantErr: true,
			errMsg:  "cannot delete the main group of a formation service",
		},
		{
			name: "invalid delete - main group of shared service",
			mailingList: &model.GrpsIOMailingList{
				UID:       "ml-123",
				GroupName: "shared-prefix",
			},
			parentService: &model.GrpsIOService{
				Type:   "shared",
				Prefix: "shared-prefix",
			},
			wantErr: true,
			errMsg:  "cannot delete the main group of a shared service",
		},
		{
			name: "invalid delete - announcement list",
			mailingList: &model.GrpsIOMailingList{
				UID:       "ml-123",
				GroupName: "announcements",
				Type:      "announcement",
			},
			parentService: &model.GrpsIOService{
				Type:      "primary",
				GroupName: "main-group",
			},
			wantErr: true,
			errMsg:  "announcement lists require special handling for deletion",
		},
		{
			name: "valid delete - formation service different prefix",
			mailingList: &model.GrpsIOMailingList{
				UID:       "ml-123",
				GroupName: "sub-list",
			},
			parentService: &model.GrpsIOService{
				Type:   "formation",
				Prefix: "form-prefix",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMailingListDeleteProtection(tt.mailingList, tt.parentService)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateMailingListDeleteProtection() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("validateMailingListDeleteProtection() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateMailingListDeleteProtection() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

func TestIsValidSubjectTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want bool
	}{
		{
			name: "valid tag",
			tag:  "VALID-TAG",
			want: true,
		},
		{
			name: "empty tag",
			tag:  "",
			want: false,
		},
		{
			name: "whitespace only",
			tag:  "   ",
			want: false,
		},
		// Length validation removed - now handled by GOA MaxLength validation
		{
			name: "tag with newline",
			tag:  "BAD\nTAG",
			want: false,
		},
		{
			name: "tag with carriage return",
			tag:  "BAD\rTAG",
			want: false,
		},
		{
			name: "tag with tab",
			tag:  "BAD\tTAG",
			want: false,
		},
		{
			name: "tag with square brackets",
			tag:  "[INVALID]",
			want: false,
		},
		{
			name: "valid tag with spaces",
			tag:  "  VALID TAG  ",
			want: true,
		},
		{
			name: "maximum length tag",
			tag:  "THIS-IS-EXACTLY-FIFTY-CHARS-1234567890123456789",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidSubjectTag(tt.tag)
			if got != tt.want {
				t.Errorf("isValidSubjectTag() = %v, want %v", got, tt.want)
			}
		})
	}
}
