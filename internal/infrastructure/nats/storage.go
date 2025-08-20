// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"

	"github.com/nats-io/nats.go/jetstream"
)

type storage struct {
	client *NATSClient
}

// GetGrpsIOService retrieves a single service by ID and returns ETag revision
func (s *storage) GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "nats storage: getting service",
		"service_uid", uid)

	service := &model.GrpsIOService{}
	rev, err := s.get(ctx, constants.KVBucketNameGrpsIOServices, uid, service, false)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.DebugContext(ctx, "service not found", "service_uid", uid, "error", err)
			return nil, 0, errs.NewNotFound("service not found")
		}
		slog.ErrorContext(ctx, "failed to get service", "error", err, "service_uid", uid)
		return nil, 0, errs.NewServiceUnavailable("failed to get service")
	}

	slog.DebugContext(ctx, "nats storage: service retrieved",
		"service_uid", uid,
		"type", service.Type,
		"revision", rev)

	return service, rev, nil
}

// GetRevision retrieves only the revision for a given UID (reader interface)
func (s *storage) GetRevision(ctx context.Context, uid string) (uint64, error) {
	slog.DebugContext(ctx, "nats storage: getting service revision",
		"service_uid", uid)

	rev, err := s.get(ctx, constants.KVBucketNameGrpsIOServices, uid, &model.GrpsIOService{}, true)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.DebugContext(ctx, "service not found for revision", "service_uid", uid, "error", err)
			return 0, errs.NewNotFound("service not found")
		}
		slog.ErrorContext(ctx, "failed to get service revision", "error", err, "service_uid", uid)
		return 0, errs.NewServiceUnavailable("failed to get service revision")
	}

	slog.DebugContext(ctx, "nats storage: service revision retrieved",
		"service_uid", uid,
		"revision", rev)

	return rev, nil
}

// get retrieves a model from the NATS KV store by bucket and UID.
// It unmarshals the data into the provided model and returns the revision.
// If the UID is empty, it returns a validation error.
// It can be used for any model that has the similar need for fetching data by UID.
func (s *storage) get(ctx context.Context, bucket, uid string, model any, onlyRevision bool) (uint64, error) {
	if uid == "" {
		return 0, errs.NewValidation("UID cannot be empty")
	}

	kv, exists := s.client.kvStore[bucket]
	if !exists || kv == nil {
		return 0, errs.NewServiceUnavailable("KV bucket not available")
	}

	data, errGet := kv.Get(ctx, uid)
	if errGet != nil {
		return 0, errGet
	}

	if !onlyRevision {
		errUnmarshal := json.Unmarshal(data.Value(), model)
		if errUnmarshal != nil {
			return 0, errUnmarshal
		}
	}

	return data.Revision(), nil
}

// GetServiceRevision retrieves only the revision number for a given UID without unmarshaling the data
// This method will be used in future for conditional requests and caching scenarios
func (s *storage) GetServiceRevision(ctx context.Context, bucket, uid string) (uint64, error) {
	return s.get(ctx, bucket, uid, &model.GrpsIOService{}, true)
}

// CreateGrpsIOService creates a new service in NATS KV store
func (s *storage) CreateGrpsIOService(ctx context.Context, service *model.GrpsIOService) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "nats storage: creating service",
		"service_id", service.UID,
		"service_type", service.Type)

	rev, err := s.put(ctx, constants.KVBucketNameGrpsIOServices, service.UID, service)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create service", "error", err, "service_id", service.UID)
		return nil, 0, errs.NewServiceUnavailable("failed to create service")
	}

	slog.DebugContext(ctx, "nats storage: service created",
		"service_id", service.UID,
		"revision", rev)

	return service, rev, nil
}

// UpdateGrpsIOService updates an existing service in NATS KV store with revision checking
func (s *storage) UpdateGrpsIOService(ctx context.Context, uid string, service *model.GrpsIOService, expectedRevision uint64) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "nats storage: updating service",
		"service_uid", uid,
		"expected_revision", expectedRevision)

	rev, err := s.putWithRevision(ctx, constants.KVBucketNameGrpsIOServices, uid, service, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update service", "error", err, "service_uid", uid)
		return nil, 0, errs.NewServiceUnavailable("failed to update service")
	}

	slog.DebugContext(ctx, "nats storage: service updated",
		"service_uid", uid,
		"revision", rev)

	return service, rev, nil
}

// DeleteGrpsIOService deletes a service from NATS KV store with revision checking
func (s *storage) DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64) error {
	slog.DebugContext(ctx, "nats storage: deleting service",
		"service_uid", uid,
		"expected_revision", expectedRevision)

	err := s.delete(ctx, constants.KVBucketNameGrpsIOServices, uid, expectedRevision)
	if err != nil {
		slog.ErrorContext(ctx, "failed to delete service", "error", err, "service_uid", uid)
		return errs.NewServiceUnavailable("failed to delete service")
	}

	slog.DebugContext(ctx, "nats storage: service deleted",
		"service_uid", uid)

	return nil
}

// put stores a model in the NATS KV store by bucket and UID.
// It marshals the model into JSON and stores it, returning the revision.
func (s *storage) put(ctx context.Context, bucket, uid string, model any) (uint64, error) {
	if uid == "" {
		return 0, errs.NewValidation("UID cannot be empty")
	}

	kv, exists := s.client.kvStore[bucket]
	if !exists || kv == nil {
		return 0, errs.NewServiceUnavailable("KV bucket not available")
	}

	data, err := json.Marshal(model)
	if err != nil {
		return 0, err
	}

	revision, err := kv.Put(ctx, uid, data)
	if err != nil {
		return 0, err
	}

	return revision, nil
}

// putWithRevision stores a model in the NATS KV store with expected revision checking.
// It performs conditional update based on the expected revision.
func (s *storage) putWithRevision(ctx context.Context, bucket, uid string, model any, expectedRevision uint64) (uint64, error) {
	if uid == "" {
		return 0, errs.NewValidation("UID cannot be empty")
	}

	kv, exists := s.client.kvStore[bucket]
	if !exists || kv == nil {
		return 0, errs.NewServiceUnavailable("KV bucket not available")
	}

	data, err := json.Marshal(model)
	if err != nil {
		return 0, err
	}

	revision, err := kv.Update(ctx, uid, data, expectedRevision)
	if err != nil {
		return 0, err
	}

	return revision, nil
}

// delete removes a model from the NATS KV store by bucket and UID with revision checking.
func (s *storage) delete(ctx context.Context, bucket, uid string, expectedRevision uint64) error {
	if uid == "" {
		return errs.NewValidation("UID cannot be empty")
	}

	kv, exists := s.client.kvStore[bucket]
	if !exists || kv == nil {
		return errs.NewServiceUnavailable("KV bucket not available")
	}

	err := kv.Delete(ctx, uid, jetstream.LastRevision(expectedRevision))
	if err != nil {
		return err
	}

	return nil
}

// UniqueProjectType validates that only one primary service exists per project
func (s *storage) UniqueProjectType(ctx context.Context, service *model.GrpsIOService) (string, error) {
	uniqueKey := fmt.Sprintf(constants.KVLookupGrpsIOServicePrefix, service.BuildIndexKey(ctx))

	slog.DebugContext(ctx, "validating unique project type constraint",
		"project_uid", service.ProjectUID,
		"service_type", service.Type,
		"constraint_key", uniqueKey,
	)

	return s.createUniqueConstraint(ctx, uniqueKey, service.UID)
}

// UniqueProjectPrefix validates that the prefix is unique within the project for formation services
func (s *storage) UniqueProjectPrefix(ctx context.Context, service *model.GrpsIOService) (string, error) {
	uniqueKey := fmt.Sprintf(constants.KVLookupGrpsIOServicePrefix, service.BuildIndexKey(ctx))

	slog.DebugContext(ctx, "validating unique project prefix constraint",
		"project_uid", service.ProjectUID,
		"service_prefix", service.Prefix,
		"constraint_key", uniqueKey,
	)

	return s.createUniqueConstraint(ctx, uniqueKey, service.UID)
}

// UniqueProjectGroupID validates that the group_id is unique within the project for shared services
func (s *storage) UniqueProjectGroupID(ctx context.Context, service *model.GrpsIOService) (string, error) {
	uniqueKey := fmt.Sprintf(constants.KVLookupGrpsIOServicePrefix, service.BuildIndexKey(ctx))

	slog.DebugContext(ctx, "validating unique project group_id constraint",
		"project_uid", service.ProjectUID,
		"service_group_id", service.GroupID,
		"constraint_key", uniqueKey,
	)

	return s.createUniqueConstraint(ctx, uniqueKey, service.UID)
}

// createUniqueConstraint creates a unique constraint key in NATS KV
func (s *storage) createUniqueConstraint(ctx context.Context, uniqueKey, serviceID string) (string, error) {
	kv, exists := s.client.kvStore[constants.KVBucketNameGrpsIOServices]
	if !exists || kv == nil {
		return uniqueKey, errs.NewServiceUnavailable("KV bucket not available")
	}

	// Try to create the constraint key - this will fail if it already exists
	_, err := kv.Create(ctx, uniqueKey, []byte(serviceID))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) {
			slog.WarnContext(ctx, "constraint violation - key already exists",
				"constraint_key", uniqueKey,
				"service_id", serviceID,
			)
			return uniqueKey, errs.NewConflict("service with same constraints already exists")
		}
		slog.ErrorContext(ctx, "failed to create unique constraint",
			"error", err,
			"constraint_key", uniqueKey,
			"service_id", serviceID,
		)
		return uniqueKey, errs.NewUnexpected("failed to create unique constraint", err)
	}

	slog.DebugContext(ctx, "unique constraint created successfully",
		"constraint_key", uniqueKey,
		"service_id", serviceID,
	)

	return uniqueKey, nil
}

// GetKeyRevision retrieves the revision for a given key (used for cleanup operations)
func (s *storage) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	if key == "" {
		return 0, errs.NewValidation("key cannot be empty")
	}

	kv, exists := s.client.kvStore[constants.KVBucketNameGrpsIOServices]
	if !exists || kv == nil {
		return 0, errs.NewServiceUnavailable("KV bucket not available")
	}

	entry, err := kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return 0, errs.NewNotFound("key not found")
		}
		return 0, errs.NewServiceUnavailable("failed to get key revision", err)
	}

	return entry.Revision(), nil
}

// Delete removes a key with the given revision (used for cleanup and rollback)
func (s *storage) Delete(ctx context.Context, key string, revision uint64) error {
	if key == "" {
		return errs.NewValidation("key cannot be empty")
	}

	kv, exists := s.client.kvStore[constants.KVBucketNameGrpsIOServices]
	if !exists || kv == nil {
		return errs.NewServiceUnavailable("KV bucket not available")
	}

	err := kv.Delete(ctx, key, jetstream.LastRevision(revision))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "key not found during deletion", "key", key, "revision", revision)
			return nil // Key already gone, consider it a success
		}
		slog.ErrorContext(ctx, "failed to delete key", "error", err, "key", key, "revision", revision)
		return errs.NewServiceUnavailable("failed to delete key", err)
	}

	slog.DebugContext(ctx, "key deleted successfully", "key", key, "revision", revision)
	return nil
}

// IsReady checks if the storage is ready by verifying the client connection
func (s *storage) IsReady(ctx context.Context) error {
	return s.client.IsReady(ctx)
}

func NewStorage(client *NATSClient) port.GrpsIOServiceReaderWriter {
	return &storage{
		client: client,
	}
}
