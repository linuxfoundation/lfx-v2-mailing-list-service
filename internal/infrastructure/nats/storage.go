// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
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
			slog.DebugContext(ctx, "service not found", "service_uid", uid)
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

// get retrieves a model from the NATS KV store by bucket and UID.
// It unmarshals the data into the provided model and returns the revision.
// If the UID is empty, it returns a validation error.
// It can be used for any that has the similar need for fetching data by UID.
func (s *storage) get(ctx context.Context, bucket, uid string, model any, onlyRevision bool) (uint64, error) {
	if uid == "" {
		return 0, errs.NewValidation("UID cannot be empty")
	}

	data, errGet := s.client.kvStore[bucket].Get(ctx, uid)
	if errGet != nil {
		return 0, errGet
	}

	if !onlyRevision {
		errUnmarshal := json.Unmarshal(data.Value(), &model)
		if errUnmarshal != nil {
			return 0, errUnmarshal
		}
	}

	return data.Revision(), nil
}

// GetRevision retrieves only the revision number for a given UID without unmarshaling the data
func (s *storage) GetRevision(ctx context.Context, bucket, uid string) (uint64, error) {
	return s.get(ctx, bucket, uid, &model.GrpsIOService{}, true)
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
