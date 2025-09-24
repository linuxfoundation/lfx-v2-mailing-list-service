// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// Helper function for test pointer creation
func int64Ptr(v int64) *int64 { return &v }

func TestGrpsIOService_BuildIndexKey(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		service     *GrpsIOService
		description string
	}{
		{
			name: "primary service",
			service: &GrpsIOService{
				Type:       "primary",
				UID:        "service-123",
				ProjectUID: "project-456",
				Prefix:     "test-prefix",
				GroupID:    int64Ptr(12345),
			},
			description: "Primary service should use project_uid and type only",
		},
		{
			name: "formation service",
			service: &GrpsIOService{
				Type:       "formation",
				UID:        "service-456",
				ProjectUID: "project-789",
				Prefix:     "formation-prefix",
				GroupID:    int64Ptr(67890),
			},
			description: "Formation service should use project_uid, type, and prefix",
		},
		{
			name: "shared service",
			service: &GrpsIOService{
				Type:       "shared",
				UID:        "service-789",
				ProjectUID: "project-123",
				Prefix:     "shared-prefix",
				GroupID:    int64Ptr(54321),
			},
			description: "Shared service should use project_uid, type, and group_id",
		},
		{
			name: "unknown service type",
			service: &GrpsIOService{
				Type:       "unknown_type",
				UID:        "service-999",
				ProjectUID: "project-999",
				Prefix:     "unknown-prefix",
				GroupID:    int64Ptr(99999),
			},
			description: "Unknown service type should use project_uid, type, and uid as fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := tt.service.BuildIndexKey(ctx)
			key2 := tt.service.BuildIndexKey(ctx)

			// Keys should be consistent for same input
			assert.Equal(t, key1, key2, "Index keys should be consistent for same input")

			// Keys should be valid SHA-256 hex strings (64 characters)
			assert.Len(t, key1, 64, "Index key should be 64 characters (SHA-256 hex)")

			// Keys should only contain hex characters
			for _, char := range key1 {
				assert.True(t, (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f'),
					"Index key should only contain hex characters")
			}
		})
	}

	// Test that different services produce different keys
	t.Run("different services produce different keys", func(t *testing.T) {
		service1 := &GrpsIOService{
			Type:       "primary",
			ProjectUID: "project-123",
		}
		service2 := &GrpsIOService{
			Type:       "primary",
			ProjectUID: "project-456",
		}

		key1 := service1.BuildIndexKey(ctx)
		key2 := service2.BuildIndexKey(ctx)

		assert.NotEqual(t, key1, key2, "Different services should produce different keys")
	})

	// Test same project but different service types
	t.Run("same project different service types produce different keys", func(t *testing.T) {
		projectUID := "project-123"

		primaryService := &GrpsIOService{
			Type:       "primary",
			ProjectUID: projectUID,
		}
		formationService := &GrpsIOService{
			Type:       "formation",
			ProjectUID: projectUID,
			Prefix:     "test-prefix",
		}
		sharedService := &GrpsIOService{
			Type:       "shared",
			ProjectUID: projectUID,
			GroupID:    int64Ptr(12345),
		}

		primaryKey := primaryService.BuildIndexKey(ctx)
		formationKey := formationService.BuildIndexKey(ctx)
		sharedKey := sharedService.BuildIndexKey(ctx)

		assert.NotEqual(t, primaryKey, formationKey, "Primary and formation services should have different keys")
		assert.NotEqual(t, primaryKey, sharedKey, "Primary and shared services should have different keys")
		assert.NotEqual(t, formationKey, sharedKey, "Formation and shared services should have different keys")
	})

	// Test formation services with different prefixes
	t.Run("formation services with different prefixes produce different keys", func(t *testing.T) {
		projectUID := "project-123"

		formationService1 := &GrpsIOService{
			Type:       "formation",
			ProjectUID: projectUID,
			Prefix:     "prefix1",
		}
		formationService2 := &GrpsIOService{
			Type:       "formation",
			ProjectUID: projectUID,
			Prefix:     "prefix2",
		}

		key1 := formationService1.BuildIndexKey(ctx)
		key2 := formationService2.BuildIndexKey(ctx)

		assert.NotEqual(t, key1, key2, "Formation services with different prefixes should have different keys")
	})

	// Test shared services with different group IDs
	t.Run("shared services with different group IDs produce different keys", func(t *testing.T) {
		projectUID := "project-123"

		sharedService1 := &GrpsIOService{
			Type:       "shared",
			ProjectUID: projectUID,
			GroupID:    int64Ptr(12345),
		}
		sharedService2 := &GrpsIOService{
			Type:       "shared",
			ProjectUID: projectUID,
			GroupID:    int64Ptr(67890),
		}

		key1 := sharedService1.BuildIndexKey(ctx)
		key2 := sharedService2.BuildIndexKey(ctx)

		assert.NotEqual(t, key1, key2, "Shared services with different group IDs should have different keys")
	})
}

func TestGrpsIOService_Tags(t *testing.T) {
	tests := []struct {
		name         string
		service      *GrpsIOService
		expectedTags []string
	}{
		{
			name:         "nil service",
			service:      nil,
			expectedTags: nil,
		},
		{
			name: "complete service",
			service: &GrpsIOService{
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
			service: &GrpsIOService{
				Type:        "",
				UID:         "",
				ProjectUID:  "",
				ProjectSlug: "",
			},
			expectedTags: nil,
		},
		{
			name: "service with partial fields",
			service: &GrpsIOService{
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
			service: &GrpsIOService{
				ProjectSlug: "awesome-project",
			},
			expectedTags: []string{
				"project_slug:awesome-project",
			},
		},
		{
			name: "service with only service type",
			service: &GrpsIOService{
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

// Test edge cases and boundary conditions
func TestGrpsIOService_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("empty service", func(t *testing.T) {
		service := &GrpsIOService{}

		// BuildIndexKey should still work with empty fields
		key := service.BuildIndexKey(ctx)
		assert.Len(t, key, 64, "Should produce valid key even with empty fields")

		// Tags should return empty slice
		tags := service.Tags()
		assert.Nil(t, tags, "Should return nil for service with no fields")
	})

	t.Run("service with special characters", func(t *testing.T) {
		service := &GrpsIOService{
			Type:        "primary",
			UID:         "service-with-special-chars-@#$%",
			ProjectUID:  "project-with-spaces and symbols!",
			ProjectSlug: "project-slug-with-unicode-🚀",
		}

		// BuildIndexKey should handle special characters
		key := service.BuildIndexKey(ctx)
		assert.Len(t, key, 64, "Should handle special characters in hash")

		// Tags should include special characters as-is
		tags := service.Tags()
		assert.Contains(t, tags, "project_slug:project-slug-with-unicode-🚀")
		assert.Contains(t, tags, "service-with-special-chars-@#$%")
		assert.Contains(t, tags, "service_uid:service-with-special-chars-@#$%")
	})

	t.Run("service with very long fields", func(t *testing.T) {
		longString := ""
		for i := 0; i < 1000; i++ {
			longString += "a"
		}

		service := &GrpsIOService{
			Type:       "primary",
			ProjectUID: longString,
		}

		// BuildIndexKey should handle long strings (hash is fixed length)
		key := service.BuildIndexKey(ctx)
		assert.Len(t, key, 64, "Should produce fixed-length key regardless of input length")
	})
}

// Benchmark tests for performance-critical functions
func BenchmarkGrpsIOService_BuildIndexKey(b *testing.B) {
	ctx := context.Background()

	tests := []struct {
		name    string
		service *GrpsIOService
	}{
		{
			name: "primary",
			service: &GrpsIOService{
				Type:       "primary",
				ProjectUID: "project-" + uuid.New().String(),
			},
		},
		{
			name: "formation",
			service: &GrpsIOService{
				Type:       "formation",
				ProjectUID: "project-" + uuid.New().String(),
				Prefix:     "formation-prefix",
			},
		},
		{
			name: "shared",
			service: &GrpsIOService{
				Type:       "shared",
				ProjectUID: "project-" + uuid.New().String(),
				GroupID:    int64Ptr(12345),
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = tt.service.BuildIndexKey(ctx)
			}
		})
	}
}

func BenchmarkGrpsIOService_Tags(b *testing.B) {
	service := &GrpsIOService{
		Type:        "primary",
		UID:         "service-" + uuid.New().String(),
		ProjectUID:  "project-" + uuid.New().String(),
		ProjectSlug: "benchmark-project",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.Tags()
	}
}
