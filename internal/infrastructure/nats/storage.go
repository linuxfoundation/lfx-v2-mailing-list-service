// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

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

// UniqueMailingListGroupName validates that group name is unique within parent service
func (s *storage) UniqueMailingListGroupName(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error) {
	constraintKey := fmt.Sprintf(constants.KVLookupMailingListConstraintPrefix, mailingList.ServiceUID, mailingList.GroupName)

	slog.DebugContext(ctx, "validating unique mailing list group name constraint",
		"parent_uid", mailingList.ServiceUID,
		"group_name", mailingList.GroupName,
		"constraint_key", constraintKey)

	return s.createUniqueConstraintInBucket(ctx, constants.KVBucketNameGrpsIOMailingLists, constraintKey, mailingList.UID)
}

// createUniqueConstraint creates a unique constraint key in NATS KV (services bucket)
func (s *storage) createUniqueConstraint(ctx context.Context, uniqueKey, serviceID string) (string, error) {
	return s.createUniqueConstraintInBucket(ctx, constants.KVBucketNameGrpsIOServices, uniqueKey, serviceID)
}

// createUniqueConstraintInBucket creates a unique constraint key in a specific NATS KV bucket
func (s *storage) createUniqueConstraintInBucket(ctx context.Context, bucket, uniqueKey, entityID string) (string, error) {
	kv, exists := s.client.kvStore[bucket]
	if !exists || kv == nil {
		return uniqueKey, errs.NewServiceUnavailable("KV bucket not available")
	}

	// Try to create the constraint key - this will fail if it already exists
	_, err := kv.Create(ctx, uniqueKey, []byte(entityID))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) {
			slog.WarnContext(ctx, "constraint violation - key already exists",
				"constraint_key", uniqueKey,
				"entity_id", entityID,
				"bucket", bucket,
			)
			return uniqueKey, errs.NewConflict("entity with same constraints already exists")
		}
		slog.ErrorContext(ctx, "failed to create unique constraint",
			"error", err,
			"constraint_key", uniqueKey,
			"entity_id", entityID,
			"bucket", bucket,
		)
		return uniqueKey, errs.NewUnexpected("failed to create unique constraint", err)
	}

	slog.DebugContext(ctx, "unique constraint created successfully",
		"constraint_key", uniqueKey,
		"entity_id", entityID,
		"bucket", bucket,
	)

	return uniqueKey, nil
}

// detectBucketForKey determines which bucket to use based on key prefix patterns
func (s *storage) detectBucketForKey(key string) string {
	// Check if key is a mailing list related key (secondary indices or constraint keys)
	if strings.HasPrefix(key, constants.MailingListKeyPrefix) {
		return constants.KVBucketNameGrpsIOMailingLists
	}

	// Service constraint keys
	if strings.HasPrefix(key, constants.ServiceLookupKeyPrefix) {
		return constants.KVBucketNameGrpsIOServices
	}

	// Default to services bucket for entity UIDs and other keys
	// This covers service UIDs and maintains backward compatibility
	return constants.KVBucketNameGrpsIOServices
}

// GetKeyRevision retrieves the revision for a given key (used for cleanup operations)
func (s *storage) GetKeyRevision(ctx context.Context, key string) (uint64, error) {
	bucket := s.detectBucketForKey(key)
	return s.getKeyRevisionFromBucket(ctx, bucket, key)
}

// getKeyRevisionFromBucket retrieves the revision for a given key from a specific bucket
func (s *storage) getKeyRevisionFromBucket(ctx context.Context, bucket, key string) (uint64, error) {
	if key == "" {
		return 0, errs.NewValidation("key cannot be empty")
	}

	kv, exists := s.client.kvStore[bucket]
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
	bucket := s.detectBucketForKey(key)
	return s.deleteFromBucket(ctx, bucket, key, revision)
}

// deleteFromBucket removes a key with the given revision from a specific bucket
func (s *storage) deleteFromBucket(ctx context.Context, bucket, key string, revision uint64) error {
	if key == "" {
		return errs.NewValidation("key cannot be empty")
	}

	kv, exists := s.client.kvStore[bucket]
	if !exists || kv == nil {
		return errs.NewServiceUnavailable("KV bucket not available")
	}

	err := kv.Delete(ctx, key, jetstream.LastRevision(revision))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			// Key not found, consider it a success for idempotency
			slog.WarnContext(ctx, "key not found during deletion", "key", key, "revision", revision, "bucket", bucket)
			return nil
		}
		slog.ErrorContext(ctx, "failed to delete key", "error", err, "key", key, "revision", revision, "bucket", bucket)
		return errs.NewServiceUnavailable("failed to delete key", err)
	}

	slog.DebugContext(ctx, "key deleted successfully", "key", key, "revision", revision, "bucket", bucket)
	return nil
}

// IsReady checks if the storage is ready by verifying the client connection
func (s *storage) IsReady(ctx context.Context) error {
	return s.client.IsReady(ctx)
}

// GetGrpsIOMailingList retrieves a single mailing list by UID
func (s *storage) GetGrpsIOMailingList(ctx context.Context, uid string) (*model.GrpsIOMailingList, error) {
	slog.DebugContext(ctx, "nats storage: getting mailing list",
		"mailing_list_uid", uid)

	mailingList := &model.GrpsIOMailingList{}
	rev, err := s.get(ctx, constants.KVBucketNameGrpsIOMailingLists, uid, mailingList, false)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.DebugContext(ctx, "mailing list not found", "mailing_list_uid", uid, "error", err)
			return nil, errs.NewNotFound("mailing list not found")
		}
		slog.ErrorContext(ctx, "failed to get mailing list", "error", err, "mailing_list_uid", uid)
		return nil, errs.NewServiceUnavailable("failed to get mailing list")
	}

	slog.DebugContext(ctx, "nats storage: mailing list retrieved",
		"mailing_list_uid", uid,
		"group_name", mailingList.GroupName,
		"revision", rev)

	return mailingList, nil
}

// CreateGrpsIOMailingList creates a new mailing list in NATS KV store (following service pattern)
func (s *storage) CreateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, uint64, error) {
	slog.DebugContext(ctx, "nats storage: creating mailing list",
		"mailing_list_id", mailingList.UID,
		"group_name", mailingList.GroupName)

	rev, err := s.put(ctx, constants.KVBucketNameGrpsIOMailingLists, mailingList.UID, mailingList)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create mailing list", "error", err, "mailing_list_id", mailingList.UID)
		return nil, 0, errs.NewServiceUnavailable("failed to create mailing list")
	}

	slog.DebugContext(ctx, "nats storage: mailing list created",
		"mailing_list_id", mailingList.UID,
		"revision", rev)

	return mailingList, rev, nil
}

// createMailingListSecondaryIndices creates all secondary indices for the mailing list
func (s *storage) createMailingListSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) ([]string, error) {
	kv, exists := s.client.kvStore[constants.KVBucketNameGrpsIOMailingLists]
	if !exists || kv == nil {
		return nil, errs.NewServiceUnavailable("KV bucket not available")
	}

	var createdKeys []string

	// TODO: When implementing GetGrpsIOMailingListsByParent/Project/Committee methods,
	// use kv.Keys(ctx, prefix) to scan for keys matching the pattern, then extract
	// UIDs from key suffixes and batch fetch the mailing lists

	// Parent index
	parentKey := fmt.Sprintf(constants.KVLookupMailingListParentPrefix, mailingList.ServiceUID) + "/" + mailingList.UID
	_, err := kv.Create(ctx, parentKey, []byte(mailingList.UID))
	if err != nil {
		slog.ErrorContext(ctx, "failed to create parent index", "error", err, "key", parentKey)
		return createdKeys, errs.NewServiceUnavailable("failed to create parent index")
	}
	createdKeys = append(createdKeys, parentKey)

	// Project index
	projectKey := fmt.Sprintf(constants.KVLookupMailingListProjectPrefix, mailingList.ProjectUID) + "/" + mailingList.UID
	_, err = kv.Create(ctx, projectKey, []byte(mailingList.UID))
	if err != nil {
		slog.ErrorContext(ctx, "failed to create project index", "error", err, "key", projectKey)
		return createdKeys, errs.NewServiceUnavailable("failed to create project index")
	}
	createdKeys = append(createdKeys, projectKey)

	// Committee index (only if committee-based)
	if mailingList.CommitteeUID != "" {
		committeeKey := fmt.Sprintf(constants.KVLookupMailingListCommitteePrefix, mailingList.CommitteeUID) + "/" + mailingList.UID
		_, err = kv.Create(ctx, committeeKey, []byte(mailingList.UID))
		if err != nil {
			slog.ErrorContext(ctx, "failed to create committee index", "error", err, "key", committeeKey)
			return createdKeys, errs.NewServiceUnavailable("failed to create committee index")
		}
		createdKeys = append(createdKeys, committeeKey)
	}

	slog.DebugContext(ctx, "secondary indices created successfully",
		"mailing_list_uid", mailingList.UID,
		"indices_created", createdKeys)

	return createdKeys, nil
}

// DeleteGrpsIOMailingList deletes a mailing list and all its secondary indices (TODO: implement in future PR)
func (s *storage) DeleteGrpsIOMailingList(ctx context.Context, uid string) error {
	// TODO: Implement in future PR for DELETE endpoint
	return errs.NewServiceUnavailable("delete mailing list not implemented yet")
}

// UpdateGrpsIOMailingList updates an existing mailing list (TODO: implement in future PR)
func (s *storage) UpdateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, error) {
	// TODO: Implement in future PR for PUT endpoint
	return nil, errs.NewServiceUnavailable("update mailing list not implemented yet")
}

// CreateSecondaryIndices creates secondary indices for a mailing list (used by orchestrator)
func (s *storage) CreateSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) ([]string, error) {
	// Reuse the existing secondary index creation method and return the created keys
	return s.createMailingListSecondaryIndices(ctx, mailingList)
}

// GetGrpsIOMailingListsByParent retrieves mailing lists by parent service ID (TODO: implement in future PR)
func (s *storage) GetGrpsIOMailingListsByParent(ctx context.Context, parentID string) ([]*model.GrpsIOMailingList, error) {
	// TODO: Implement in future PR for GET list endpoints
	return nil, errs.NewServiceUnavailable("get mailing lists by parent not implemented yet")
}

// GetGrpsIOMailingListsByCommittee retrieves mailing lists by committee ID (TODO: implement in future PR)
func (s *storage) GetGrpsIOMailingListsByCommittee(ctx context.Context, committeeID string) ([]*model.GrpsIOMailingList, error) {
	// TODO: Implement in future PR for GET list endpoints
	return nil, errs.NewServiceUnavailable("get mailing lists by committee not implemented yet")
}

// GetGrpsIOMailingListsByProject retrieves mailing lists by project ID (TODO: implement in future PR)
func (s *storage) GetGrpsIOMailingListsByProject(ctx context.Context, projectID string) ([]*model.GrpsIOMailingList, error) {
	// TODO: Implement in future PR for GET list endpoints
	return nil, errs.NewServiceUnavailable("get mailing lists by project not implemented yet")
}

// CheckMailingListExists checks if a mailing list with the given name exists in parent service
func (s *storage) CheckMailingListExists(ctx context.Context, parentID, groupName string) (bool, error) {
	constraintKey := fmt.Sprintf(constants.KVLookupMailingListConstraintPrefix, parentID, groupName)

	slog.DebugContext(ctx, "nats storage: checking mailing list existence",
		"parent_id", parentID,
		"group_name", groupName,
		"constraint_key", constraintKey)

	kv, exists := s.client.kvStore[constants.KVBucketNameGrpsIOMailingLists]
	if !exists || kv == nil {
		return false, errs.NewServiceUnavailable("KV bucket not available")
	}

	_, err := kv.Get(ctx, constraintKey)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return false, nil // Doesn't exist
		}
		return false, errs.NewServiceUnavailable("failed to check mailing list existence")
	}

	return true, nil // Exists
}

func NewStorage(client *NATSClient) port.GrpsIOReaderWriter {
	return &storage{
		client: client,
	}
}
