// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGrpsIOService_Tags(t *testing.T) {
	tests := []struct {
		name         string
		service      *GroupsIOService
		expectedTags []string
	}{
		{
			name:         "nil service",
			service:      nil,
			expectedTags: nil,
		},
		{
			name: "complete service",
			service: &GroupsIOService{
				Type:        "primary",
				UID:         "service-123",
				ProjectUID:  "project-456",
				ProjectSlug: "test-project",
			},
			expectedTags: []string{
				"project_uid:project-456",
				"project_slug:test-project",
				"service-123",
				"service_uid:service-123",
				"service_type:primary",
			},
		},
		{
			name: "minimal service - empty fields",
			service: &GroupsIOService{
				Type:        "",
				UID:         "",
				ProjectUID:  "",
				ProjectSlug: "",
			},
			expectedTags: nil,
		},
		{
			name: "service with partial fields",
			service: &GroupsIOService{
				Type:       "formation",
				UID:        "service-789",
				ProjectUID: "project-999",
			},
			expectedTags: []string{
				"project_uid:project-999",
				"service-789",
				"service_uid:service-789",
				"service_type:formation",
			},
		},
		{
			name: "service with only project slug",
			service: &GroupsIOService{
				ProjectSlug: "awesome-project",
			},
			expectedTags: []string{
				"project_slug:awesome-project",
			},
		},
		{
			name: "service with only service type",
			service: &GroupsIOService{
				Type: "shared",
			},
			expectedTags: []string{
				"service_type:shared",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := tt.service.Tags()
			assert.Equal(t, tt.expectedTags, tags)
		})
	}
}

func BenchmarkGrpsIOService_Tags(b *testing.B) {
	service := &GroupsIOService{
		Type:        "primary",
		UID:         "service-" + uuid.New().String(),
		ProjectUID:  "project-" + uuid.New().String(),
		ProjectSlug: "benchmark-project",
	}

	for b.Loop() {
		_ = service.Tags()
	}
}
