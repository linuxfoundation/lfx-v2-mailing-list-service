// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
)

type messageRequest struct {
	client *NATSClient
}

func (m *messageRequest) get(ctx context.Context, subject, uid string) (string, error) {

	data := []byte(uid)
	msg, err := m.client.conn.RequestWithContext(ctx, subject, data)
	if err != nil {
		return "", err
	}

	// Try to parse as JSON error response first
	var errorResponse struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(msg.Data, &errorResponse); err == nil && errorResponse.Error != "" {
		slog.WarnContext(ctx, "message responded with an error", "subject", subject, "uid", uid, "error", errorResponse.Error)
		return "", errors.NewUnexpected(errorResponse.Error)
	}

	attribute := string(msg.Data)
	if attribute == "" {
		return "", errors.NewNotFound(fmt.Sprintf("project attribute %s not found for uid: %s", subject, uid))
	}

	return attribute, nil

}

func (m *messageRequest) ProjectSlug(ctx context.Context, uid string) (string, error) {
	return m.get(ctx, constants.ProjectGetSlugSubject, uid)
}

func (m *messageRequest) ProjectName(ctx context.Context, uid string) (string, error) {
	return m.get(ctx, constants.ProjectGetNameSubject, uid)
}

func (m *messageRequest) ProjectParentUID(ctx context.Context, uid string) (string, error) {
	return m.get(ctx, constants.ProjectGetParentUIDSubject, uid)
}

func (m *messageRequest) CommitteeName(ctx context.Context, uid string) (string, error) {
	return m.get(ctx, constants.CommitteeGetNameSubject, uid)
}

// ListMembers retrieves all members for a given committee via NATS request/reply
func (m *messageRequest) ListMembers(ctx context.Context, committeeUID string) ([]model.CommitteeMember, error) {
	slog.DebugContext(ctx, "requesting committee members via NATS",
		"committee_uid", committeeUID,
		"subject", constants.CommitteeListMembersSubject)

	// Send committee UID as request
	data := []byte(committeeUID)
	msg, err := m.client.conn.RequestWithContext(ctx, constants.CommitteeListMembersSubject, data)
	if err != nil {
		slog.ErrorContext(ctx, "failed to request committee members",
			"error", err,
			"committee_uid", committeeUID)
		return nil, errors.NewServiceUnavailable(fmt.Sprintf("committee-api unavailable: %v", err))
	}

	// Response should be empty for not found
	if len(msg.Data) == 0 {
		slog.WarnContext(ctx, "committee not found or has no members",
			"committee_uid", committeeUID)
		return []model.CommitteeMember{}, nil
	}

	// Unmarshal response
	var members []model.CommitteeMember
	if err := json.Unmarshal(msg.Data, &members); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal committee members response",
			"error", err,
			"committee_uid", committeeUID)
		return nil, fmt.Errorf("failed to unmarshal committee members: %w", err)
	}

	slog.InfoContext(ctx, "successfully retrieved committee members",
		"committee_uid", committeeUID,
		"member_count", len(members))

	return members, nil
}

// NewEntityAttributeReader creates a new entity attribute reader implementation using NATS messaging.
func NewEntityAttributeReader(client *NATSClient) port.EntityAttributeReader {
	return &messageRequest{
		client: client,
	}
}
