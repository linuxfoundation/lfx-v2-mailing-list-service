// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package mock

import (
	"context"
	"errors"
	"testing"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	pkgerrors "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorSimulation(t *testing.T) {
	ctx := context.Background()

	t.Run("Service error simulation", func(t *testing.T) {
		repo := NewMockRepository()

		// Configure error for specific service
		serviceUID := "test-service-uid"
		expectedErr := pkgerrors.NewNotFound("simulated service not found")
		repo.SetErrorForService(serviceUID, expectedErr)

		// Try to get the service - should return configured error
		_, _, err := repo.GetGrpsIOService(ctx, serviceUID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, expectedErr))
	})

	t.Run("Mailing list error simulation", func(t *testing.T) {
		repo := NewMockRepository()

		// Configure error for specific mailing list
		mailingListUID := "test-mailinglist-uid"
		expectedErr := pkgerrors.NewConflict("simulated mailing list conflict")
		repo.SetErrorForMailingList(mailingListUID, expectedErr)

		// Try to get the mailing list - should return configured error
		_, _, err := repo.GetGrpsIOMailingList(ctx, mailingListUID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, expectedErr))
	})

	t.Run("Member error simulation", func(t *testing.T) {
		repo := NewMockRepository()

		// Configure error for specific member
		memberUID := "test-member-uid"
		expectedErr := pkgerrors.NewUnauthorized("simulated member unauthorized")
		repo.SetErrorForMember(memberUID, expectedErr)

		// Try to get the member - should return configured error
		_, _, err := repo.GetGrpsIOMember(ctx, memberUID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, expectedErr))
	})

	t.Run("Operation error simulation", func(t *testing.T) {
		repo := NewMockRepository()

		// Configure error for specific operation
		expectedErr := pkgerrors.NewServiceUnavailable("simulated service unavailable")
		repo.SetErrorForOperation("CreateGrpsIOMailingList", expectedErr)

		// Try to create a mailing list - should return configured error
		mailingList := &model.GrpsIOMailingList{
			UID:       "test-create-uid",
			GroupName: "test-list",
			Type:      "discussion_open",
		}
		_, _, err := repo.CreateGrpsIOMailingList(ctx, mailingList)
		require.Error(t, err)
		assert.True(t, errors.Is(err, expectedErr))
	})

	t.Run("Global error simulation", func(t *testing.T) {
		repo := NewMockRepository()

		// Configure global error for all operations
		expectedErr := pkgerrors.NewUnexpected("simulated global error")
		repo.SetGlobalError(expectedErr)

		// Try any operation - should return configured global error
		_, _, err := repo.GetGrpsIOService(ctx, "any-service")
		require.Error(t, err)
		assert.True(t, errors.Is(err, expectedErr))

		// Try another operation - should also return global error
		_, _, err = repo.GetGrpsIOMailingList(ctx, "any-mailinglist")
		require.Error(t, err)
		assert.True(t, errors.Is(err, expectedErr))
	})

	t.Run("Clear error simulation", func(t *testing.T) {
		repo := NewMockRepository()

		// Configure some errors
		repo.SetErrorForService("test-service", pkgerrors.NewNotFound("test error"))
		repo.SetGlobalError(pkgerrors.NewUnexpected("global error"))

		// Clear all error simulation
		repo.ClearErrorSimulation()

		// Operations should work normally now (return NotFound for non-existent resources)
		_, _, err := repo.GetGrpsIOService(ctx, "non-existent-service")
		require.Error(t, err)

		// Should be NotFound error from normal logic, not our simulated error
		var notFoundErr pkgerrors.NotFound
		assert.True(t, errors.As(err, &notFoundErr))
		assert.Contains(t, err.Error(), "service with UID non-existent-service not found")
	})

	t.Run("Error priority - global takes precedence", func(t *testing.T) {
		repo := NewMockRepository()

		// Set both specific and global errors
		serviceUID := "test-service"
		specificErr := pkgerrors.NewNotFound("specific error")
		globalErr := pkgerrors.NewUnexpected("global error")

		repo.SetErrorForService(serviceUID, specificErr)
		repo.SetGlobalError(globalErr)

		// Global error should take precedence
		_, _, err := repo.GetGrpsIOService(ctx, serviceUID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, globalErr))
		assert.False(t, errors.Is(err, specificErr))
	})

	t.Run("Error priority - operation over resource", func(t *testing.T) {
		repo := NewMockRepository()

		// Clear any existing error simulation first
		repo.ClearErrorSimulation()

		// Set both operation and resource-specific errors
		serviceUID := "test-service"
		resourceErr := pkgerrors.NewNotFound("resource error")
		operationErr := pkgerrors.NewConflict("operation error")

		repo.SetErrorForService(serviceUID, resourceErr)
		repo.SetErrorForOperation("GetGrpsIOService", operationErr)

		// Operation error should take precedence
		_, _, err := repo.GetGrpsIOService(ctx, serviceUID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, operationErr))
		assert.False(t, errors.Is(err, resourceErr))
	})
}
