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
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/redaction"

	"github.com/nats-io/nats.go/jetstream"
)

// bucketRoutingRules define routing rules as a slice for ordered evaluation
var bucketRoutingRules = []struct {
	prefix string
	bucket string
}{
	{constants.GroupsIOMailingListKeyPrefix, constants.KVBucketNameGroupsIOMailingLists},
	{constants.GroupsIOMemberLookupKeyPrefix, constants.KVBucketNameGroupsIOMembers},
	{constants.GroupsIOServiceLookupKeyPrefix, constants.KVBucketNameGroupsIOServices},
}

type storage struct {
	client *NATSClient
}

// GetGrpsIOService retrieves a single service by ID and returns ETag revision
func (s *storage) GetGrpsIOService(ctx context.Context, uid string) (*model.GrpsIOService, uint64, error) {
	slog.DebugContext(ctx, "nats storage: getting service",
		"service_uid", uid)

	service := &model.GrpsIOService{}
	rev, err := s.get(ctx, constants.KVBucketNameGroupsIOServices, uid, service, false)
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

	rev, err := s.get(ctx, constants.KVBucketNameGroupsIOServices, uid, &model.GrpsIOService{}, true)
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

// isRevisionMismatch checks if an error indicates a revision mismatch (CAS failure)
// This handles API error codes that JetStream returns for wrong last sequence
func (s *storage) isRevisionMismatch(err error) bool {
	var jsErr jetstream.JetStreamError
	if errors.As(err, &jsErr) && jsErr.APIError() != nil &&
		jsErr.APIError().ErrorCode == jetstream.JSErrCodeStreamWrongLastSequence {
		return true
	}
	return false
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

	rev, err := s.put(ctx, constants.KVBucketNameGroupsIOServices, service.UID, service)
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

	rev, err := s.putWithRevision(ctx, constants.KVBucketNameGroupsIOServices, uid, service, expectedRevision)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "service not found on update", "service_uid", uid)
			return nil, 0, errs.NewNotFound("service not found")
		}
		if s.isRevisionMismatch(err) {
			slog.WarnContext(ctx, "revision mismatch on update", "service_uid", uid, "expected_revision", expectedRevision)
			return nil, 0, errs.NewConflict("revision mismatch")
		}
		slog.ErrorContext(ctx, "failed to update service", "error", err, "service_uid", uid)
		return nil, 0, errs.NewServiceUnavailable("failed to update service")
	}

	slog.DebugContext(ctx, "nats storage: service updated",
		"service_uid", uid,
		"revision", rev)

	return service, rev, nil
}

// DeleteGrpsIOService deletes a service from NATS KV store with revision checking
func (s *storage) DeleteGrpsIOService(ctx context.Context, uid string, expectedRevision uint64, service *model.GrpsIOService) error {
	slog.DebugContext(ctx, "nats storage: deleting service",
		"service_uid", uid,
		"expected_revision", expectedRevision)

	// Delete the main service record
	err := s.delete(ctx, constants.KVBucketNameGroupsIOServices, uid, expectedRevision)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "service not found on delete", "service_uid", uid)
			return errs.NewNotFound("service not found")
		}
		if s.isRevisionMismatch(err) {
			slog.WarnContext(ctx, "revision mismatch on delete", "service_uid", uid, "expected_revision", expectedRevision)
			return errs.NewConflict("revision mismatch")
		}
		slog.ErrorContext(ctx, "failed to delete service", "error", err, "service_uid", uid)
		return errs.NewServiceUnavailable("failed to delete service")
	}

	// Clean up unique constraint.
	// Verification is necessary here to prevent deleting a constraint that might have been reused
	// by another service with the same parameters. Only delete if the constraint still points to this service's UID.
	constraintKey := fmt.Sprintf(constants.KVLookupGroupsIOServicePrefix, service.BuildIndexKey(ctx))
	kv, exists := s.client.kvStore[constants.KVBucketNameGroupsIOServices]
	if exists && kv != nil {
		entry, err := kv.Get(ctx, constraintKey)
		if err == nil && string(entry.Value()) == service.UID {
			// Only delete if it still points to our UID
			if delErr := kv.Delete(ctx, constraintKey, jetstream.LastRevision(entry.Revision())); delErr != nil {
				slog.DebugContext(ctx, "failed to delete constraint key during cleanup", "error", delErr, "key", constraintKey)
			} else {
				slog.DebugContext(ctx, "service constraint cleaned up successfully", "constraint_key", constraintKey)
			}
		}
		// Silently skip if not found or points to different UID (best effort cleanup)
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
	uniqueKey := fmt.Sprintf(constants.KVLookupGroupsIOServicePrefix, service.BuildIndexKey(ctx))

	slog.DebugContext(ctx, "validating unique project type constraint",
		"project_uid", service.ProjectUID,
		"service_type", service.Type,
		"constraint_key", uniqueKey,
	)

	return s.createUniqueConstraint(ctx, uniqueKey, service.UID)
}

// UniqueProjectPrefix validates that the prefix is unique within the project for formation services
func (s *storage) UniqueProjectPrefix(ctx context.Context, service *model.GrpsIOService) (string, error) {
	uniqueKey := fmt.Sprintf(constants.KVLookupGroupsIOServicePrefix, service.BuildIndexKey(ctx))

	slog.DebugContext(ctx, "validating unique project prefix constraint",
		"project_uid", service.ProjectUID,
		"service_prefix", service.Prefix,
		"constraint_key", uniqueKey,
	)

	return s.createUniqueConstraint(ctx, uniqueKey, service.UID)
}

// UniqueProjectGroupID validates that the group_id is unique within the project for shared services
func (s *storage) UniqueProjectGroupID(ctx context.Context, service *model.GrpsIOService) (string, error) {
	uniqueKey := fmt.Sprintf(constants.KVLookupGroupsIOServicePrefix, service.BuildIndexKey(ctx))

	slog.DebugContext(ctx, "validating unique project group_id constraint",
		"project_uid", service.ProjectUID,
		"service_group_id", service.GroupID,
		"constraint_key", uniqueKey,
	)

	return s.createUniqueConstraint(ctx, uniqueKey, service.UID)
}

// UniqueMailingListGroupName validates that group name is unique within parent service
func (s *storage) UniqueMailingListGroupName(ctx context.Context, mailingList *model.GrpsIOMailingList) (string, error) {
	constraintKey := fmt.Sprintf(constants.KVLookupGroupsIOMailingListConstraintPrefix, mailingList.BuildIndexKey(ctx))

	slog.DebugContext(ctx, "validating unique mailing list group name constraint",
		"parent_uid", mailingList.ServiceUID,
		"group_name", mailingList.GroupName,
		"constraint_key", constraintKey)

	return s.createUniqueConstraintInBucket(ctx, constants.KVBucketNameGroupsIOMailingLists, constraintKey, mailingList.UID)
}

// createUniqueConstraint creates a unique constraint key in NATS KV (services bucket)
func (s *storage) createUniqueConstraint(ctx context.Context, uniqueKey, serviceID string) (string, error) {
	return s.createUniqueConstraintInBucket(ctx, constants.KVBucketNameGroupsIOServices, uniqueKey, serviceID)
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
	for _, rule := range bucketRoutingRules {
		if strings.HasPrefix(key, rule.prefix) {
			return rule.bucket
		}
	}
	// Default to services bucket for entity UIDs and backward compatibility
	return constants.KVBucketNameGroupsIOServices
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
func (s *storage) GetGrpsIOMailingList(ctx context.Context, uid string) (*model.GrpsIOMailingList, uint64, error) {
	slog.DebugContext(ctx, "nats storage: getting mailing list",
		"mailing_list_uid", uid)

	mailingList := &model.GrpsIOMailingList{}
	rev, err := s.get(ctx, constants.KVBucketNameGroupsIOMailingLists, uid, mailingList, false)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.DebugContext(ctx, "mailing list not found", "mailing_list_uid", uid, "error", err)
			return nil, 0, errs.NewNotFound("mailing list not found")
		}
		slog.ErrorContext(ctx, "failed to get mailing list", "error", err, "mailing_list_uid", uid)
		return nil, 0, errs.NewServiceUnavailable("failed to get mailing list")
	}

	slog.DebugContext(ctx, "nats storage: mailing list retrieved",
		"mailing_list_uid", uid,
		"group_name", mailingList.GroupName,
		"revision", rev)

	return mailingList, rev, nil
}

// GetMailingListRevision retrieves only the revision for a given UID
func (s *storage) GetMailingListRevision(ctx context.Context, uid string) (uint64, error) {
	slog.DebugContext(ctx, "nats storage: getting mailing list revision", "mailing_list_uid", uid)

	kv, exists := s.client.kvStore[constants.KVBucketNameGroupsIOMailingLists]
	if !exists || kv == nil {
		return 0, errs.NewServiceUnavailable("KV bucket not available")
	}

	entry, err := kv.Get(ctx, uid)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.DebugContext(ctx, "mailing list not found", "mailing_list_uid", uid, "error", err)
			return 0, errs.NewNotFound("mailing list not found")
		}
		slog.ErrorContext(ctx, "failed to get mailing list revision", "error", err, "mailing_list_uid", uid)
		return 0, errs.NewServiceUnavailable("failed to get mailing list revision")
	}

	slog.DebugContext(ctx, "nats storage: mailing list revision retrieved", "mailing_list_uid", uid, "revision", entry.Revision())
	return entry.Revision(), nil
}

// CreateGrpsIOMailingList creates a new mailing list in NATS KV store (following service pattern)
func (s *storage) CreateGrpsIOMailingList(ctx context.Context, mailingList *model.GrpsIOMailingList) (*model.GrpsIOMailingList, uint64, error) {
	slog.DebugContext(ctx, "nats storage: creating mailing list",
		"mailing_list_id", mailingList.UID,
		"group_name", mailingList.GroupName)

	rev, err := s.put(ctx, constants.KVBucketNameGroupsIOMailingLists, mailingList.UID, mailingList)
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
	kv, exists := s.client.kvStore[constants.KVBucketNameGroupsIOMailingLists]
	if !exists || kv == nil {
		return nil, errs.NewServiceUnavailable("KV bucket not available")
	}

	var createdKeys []string
	// Define indices to create
	indices := []struct {
		name   string
		prefix string
		id     string
		skip   bool
	}{
		{"service", constants.KVLookupGroupsIOMailingListServicePrefix, mailingList.ServiceUID, false},
		{"project", constants.KVLookupGroupsIOMailingListProjectPrefix, mailingList.ProjectUID, false},
		{"committee", constants.KVLookupGroupsIOMailingListCommitteePrefix, mailingList.CommitteeUID, mailingList.CommitteeUID == ""},
	}
	// Create each index
	for _, idx := range indices {
		if idx.skip {
			continue
		}
		key := fmt.Sprintf(idx.prefix, idx.id) + "/" + mailingList.UID
		created, err := s.createOrSkipIndex(ctx, kv, key, mailingList.UID, idx.name)
		if err != nil {
			return createdKeys, err
		}
		if created {
			createdKeys = append(createdKeys, key)
		}
	}
	slog.DebugContext(ctx, "secondary indices created successfully",
		"mailing_list_uid", mailingList.UID,
		"indices_created", createdKeys)
	return createdKeys, nil
}

// createOrSkipIndex creates an index or skips if it already exists
func (s *storage) createOrSkipIndex(ctx context.Context, kv jetstream.KeyValue, key, value, indexName string) (bool, error) {
	_, err := kv.Create(ctx, key, []byte(value))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) {
			slog.DebugContext(ctx, "index already exists, skipping",
				"index", indexName,
				"key", key)
			return false, nil
		}
		slog.ErrorContext(ctx, "failed to create index",
			"index", indexName,
			"error", err,
			"key", key)
		return false, errs.NewServiceUnavailable(fmt.Sprintf("failed to create %s index", indexName))
	}
	return true, nil
}

// UpdateGrpsIOMailingList updates an existing mailing list with optimistic concurrency control
func (s *storage) UpdateGrpsIOMailingList(ctx context.Context, uid string, mailingList *model.GrpsIOMailingList, expectedRevision uint64) (*model.GrpsIOMailingList, uint64, error) {
	slog.DebugContext(ctx, "nats storage: updating mailing list",
		"mailing_list_uid", uid,
		"expected_revision", expectedRevision)

	rev, err := s.putWithRevision(ctx, constants.KVBucketNameGroupsIOMailingLists, uid, mailingList, expectedRevision)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "mailing list not found on update", "mailing_list_uid", uid)
			return nil, 0, errs.NewNotFound("mailing list not found")
		}
		if s.isRevisionMismatch(err) {
			slog.WarnContext(ctx, "revision mismatch on update", "mailing_list_uid", uid, "expected_revision", expectedRevision)
			return nil, 0, errs.NewConflict("revision mismatch")
		}
		slog.ErrorContext(ctx, "failed to update mailing list", "error", err, "mailing_list_uid", uid)
		return nil, 0, errs.NewServiceUnavailable("failed to update mailing list")
	}

	slog.DebugContext(ctx, "nats storage: mailing list updated",
		"mailing_list_uid", uid,
		"revision", rev)

	return mailingList, rev, nil
}

// DeleteGrpsIOMailingList deletes a mailing list with optimistic concurrency control
func (s *storage) DeleteGrpsIOMailingList(ctx context.Context, uid string, expectedRevision uint64, mailingList *model.GrpsIOMailingList) error {
	slog.DebugContext(ctx, "nats storage: deleting mailing list",
		"mailing_list_uid", uid,
		"expected_revision", expectedRevision)

	// Use the passed mailing list data - no need to fetch again

	// Delete the main record with optimistic concurrency control using helper
	err := s.delete(ctx, constants.KVBucketNameGroupsIOMailingLists, uid, expectedRevision)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "mailing list not found on delete", "mailing_list_uid", uid)
			return errs.NewNotFound("mailing list not found")
		}
		if s.isRevisionMismatch(err) {
			slog.WarnContext(ctx, "revision mismatch on delete", "mailing_list_uid", uid, "expected_revision", expectedRevision)
			return errs.NewConflict("revision mismatch")
		}
		slog.ErrorContext(ctx, "failed to delete mailing list", "error", err, "mailing_list_uid", uid)
		return errs.NewServiceUnavailable("failed to delete mailing list")
	}

	// Clean up secondary indices (best effort - don't fail if they don't exist)
	s.deleteMailingListSecondaryIndices(ctx, mailingList)

	// Clean up unique constraint (verify it belongs to this list)
	constraintKey := fmt.Sprintf(constants.KVLookupGroupsIOMailingListConstraintPrefix, mailingList.BuildIndexKey(ctx))
	kv, exists := s.client.kvStore[constants.KVBucketNameGroupsIOMailingLists]
	if exists && kv != nil {
		entry, err := kv.Get(ctx, constraintKey)
		if err == nil && string(entry.Value()) == mailingList.UID {
			// Only delete if it still points to our UID
			if delErr := kv.Delete(ctx, constraintKey, jetstream.LastRevision(entry.Revision())); delErr != nil {
				slog.DebugContext(ctx, "failed to delete constraint key during cleanup", "error", delErr, "key", constraintKey)
			}
		}
		// Silently skip if not found or points to different UID (best effort cleanup)
	}

	slog.DebugContext(ctx, "nats storage: mailing list deleted", "mailing_list_uid", uid)
	return nil
}

// deleteMailingListSecondaryIndices removes all secondary indices for a mailing list (best effort)
func (s *storage) deleteMailingListSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) {
	kv, exists := s.client.kvStore[constants.KVBucketNameGroupsIOMailingLists]
	if !exists || kv == nil {
		return
	}

	// Service index
	serviceKey := fmt.Sprintf(constants.KVLookupGroupsIOMailingListServicePrefix, mailingList.ServiceUID) + "/" + mailingList.UID
	err := kv.Delete(ctx, serviceKey)
	if err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
		slog.WarnContext(ctx, "failed to delete service index", "error", err, "key", serviceKey)
	}

	// Project index
	projectKey := fmt.Sprintf(constants.KVLookupGroupsIOMailingListProjectPrefix, mailingList.ProjectUID) + "/" + mailingList.UID
	err = kv.Delete(ctx, projectKey)
	if err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
		slog.WarnContext(ctx, "failed to delete project index", "error", err, "key", projectKey)
	}

	// Committee index (only if committee-based)
	if mailingList.CommitteeUID != "" {
		committeeKey := fmt.Sprintf(constants.KVLookupGroupsIOMailingListCommitteePrefix, mailingList.CommitteeUID) + "/" + mailingList.UID
		err = kv.Delete(ctx, committeeKey)
		if err != nil && !errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "failed to delete committee index", "error", err, "key", committeeKey)
		}
	}
}

// CreateSecondaryIndices creates secondary indices for a mailing list (used by orchestrator)
func (s *storage) CreateSecondaryIndices(ctx context.Context, mailingList *model.GrpsIOMailingList) ([]string, error) {
	// Reuse the existing secondary index creation method and return the created keys
	return s.createMailingListSecondaryIndices(ctx, mailingList)
}

// CheckMailingListExists checks if a mailing list with the given name exists in parent service
func (s *storage) CheckMailingListExists(ctx context.Context, parentID, groupName string) (bool, error) {
	// Create temporary mailing list to generate consistent constraint key
	tempMailingList := &model.GrpsIOMailingList{
		ServiceUID: parentID,
		GroupName:  groupName,
	}
	constraintKey := fmt.Sprintf(constants.KVLookupGroupsIOMailingListConstraintPrefix, tempMailingList.BuildIndexKey(ctx))

	slog.DebugContext(ctx, "nats storage: checking mailing list existence",
		"parent_id", parentID,
		"group_name", groupName,
		"constraint_key", constraintKey)

	kv, exists := s.client.kvStore[constants.KVBucketNameGroupsIOMailingLists]
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

// ================== GrpsIOMember operations ==================

// UniqueMember creates a unique constraint for member email within mailing list
func (s *storage) UniqueMember(ctx context.Context, member *model.GrpsIOMember) (string, error) {
	constraintKey := fmt.Sprintf(constants.KVLookupGroupsIOMemberConstraintPrefix, member.BuildIndexKey(ctx))

	slog.DebugContext(ctx, "validating unique member constraint",
		"mailing_list_uid", member.MailingListUID,
		"email", redaction.RedactEmail(member.Email),
		"constraint_key", constraintKey)

	return s.createUniqueConstraintInBucket(ctx, constants.KVBucketNameGroupsIOMembers, constraintKey, member.UID)
}

// CreateGrpsIOMember stores a new member in NATS KV (following mailing list pattern)
func (s *storage) CreateGrpsIOMember(ctx context.Context, member *model.GrpsIOMember) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "nats storage: creating member",
		"member_id", member.UID,
		"email", redaction.RedactEmail(member.Email),
		"mailing_list_uid", member.MailingListUID)

	rev, err := s.put(ctx, constants.KVBucketNameGroupsIOMembers, member.UID, member)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create member", "error", err, "member_id", member.UID)
		return nil, 0, errs.NewServiceUnavailable("failed to create member")
	}

	slog.DebugContext(ctx, "nats storage: member created",
		"member_id", member.UID,
		"revision", rev)

	return member, rev, nil
}

// GetGrpsIOMember retrieves a member by UID
func (s *storage) GetGrpsIOMember(ctx context.Context, uid string) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "nats storage: getting member",
		"member_uid", uid)

	member := &model.GrpsIOMember{}
	rev, err := s.get(ctx, constants.KVBucketNameGroupsIOMembers, uid, member, false)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.DebugContext(ctx, "member not found", "member_uid", uid, "error", err)
			return nil, 0, errs.NewNotFound("member not found")
		}
		slog.ErrorContext(ctx, "failed to get member", "error", err, "member_uid", uid)
		return nil, 0, errs.NewServiceUnavailable("failed to get member")
	}

	slog.DebugContext(ctx, "nats storage: member retrieved",
		"member_uid", uid,
		"email", redaction.RedactEmail(member.Email),
		"revision", rev)

	return member, rev, nil
}

// UpdateGrpsIOMember updates an existing member with optimistic concurrency control
func (s *storage) UpdateGrpsIOMember(ctx context.Context, uid string, member *model.GrpsIOMember, expectedRevision uint64) (*model.GrpsIOMember, uint64, error) {
	slog.DebugContext(ctx, "nats storage: updating member",
		"member_uid", uid,
		"email", redaction.RedactEmail(member.Email),
		"expected_revision", expectedRevision)

	rev, err := s.putWithRevision(ctx, constants.KVBucketNameGroupsIOMembers, uid, member, expectedRevision)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "member not found on update", "member_uid", uid)
			return nil, 0, errs.NewNotFound("member not found")
		}
		if s.isRevisionMismatch(err) {
			slog.WarnContext(ctx, "revision mismatch on update", "member_uid", uid, "expected_revision", expectedRevision)
			return nil, 0, errs.NewConflict("revision mismatch")
		}
		slog.ErrorContext(ctx, "failed to update member", "error", err, "member_uid", uid)
		return nil, 0, errs.NewServiceUnavailable("failed to update member")
	}

	slog.DebugContext(ctx, "nats storage: member updated",
		"member_uid", uid,
		"email", redaction.RedactEmail(member.Email),
		"revision", rev)

	return member, rev, nil
}

// DeleteGrpsIOMember deletes a member with optimistic concurrency control
func (s *storage) DeleteGrpsIOMember(ctx context.Context, uid string, expectedRevision uint64, member *model.GrpsIOMember) error {
	slog.DebugContext(ctx, "nats storage: deleting member",
		"member_uid", uid,
		"expected_revision", expectedRevision)

	// Use the passed member data - no need to fetch again

	err := s.delete(ctx, constants.KVBucketNameGroupsIOMembers, uid, expectedRevision)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			slog.WarnContext(ctx, "member not found on delete", "member_uid", uid)
			return errs.NewNotFound("member not found")
		}
		if s.isRevisionMismatch(err) {
			slog.WarnContext(ctx, "revision mismatch on delete", "member_uid", uid, "expected_revision", expectedRevision)
			return errs.NewConflict("revision mismatch")
		}
		slog.ErrorContext(ctx, "failed to delete member", "error", err, "member_uid", uid)
		return errs.NewServiceUnavailable("failed to delete member")
	}

	// Clean up unique constraint (verify it belongs to this member)
	constraintKey := fmt.Sprintf(constants.KVLookupGroupsIOMemberConstraintPrefix, member.BuildIndexKey(ctx))
	kv, exists := s.client.kvStore[constants.KVBucketNameGroupsIOMembers]
	if exists && kv != nil {
		entry, err := kv.Get(ctx, constraintKey)
		if err == nil && string(entry.Value()) == member.UID {
			// Only delete if it still points to our UID
			if delErr := kv.Delete(ctx, constraintKey, jetstream.LastRevision(entry.Revision())); delErr != nil {
				slog.DebugContext(ctx, "failed to delete constraint key during cleanup", "error", delErr, "key", constraintKey)
			}
		}
		// Silently skip if not found or points to different UID (best effort cleanup)
	}

	slog.DebugContext(ctx, "nats storage: member deleted",
		"member_uid", uid)

	return nil
}

// GetMemberRevision retrieves only the revision for a given UID
func (s *storage) GetMemberRevision(ctx context.Context, uid string) (uint64, error) {
	return s.get(ctx, constants.KVBucketNameGroupsIOMembers, uid, &model.GrpsIOMember{}, true)
}

func NewStorage(client *NATSClient) port.GrpsIOReaderWriter {
	return &storage{
		client: client,
	}
}
