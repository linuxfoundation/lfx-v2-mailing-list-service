// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/nats-io/nats.go"
)

const (
	committeeProjectLookupTimeout = 5 * time.Second
)

type committeeProjectLookupRequest struct {
	CommitteeUID string `json:"committee_uid"`
}

type committeeProjectLookupResponse struct {
	ProjectUID string `json:"project_uid,omitempty"`
	Error      string `json:"error,omitempty"`
}

// natsCommitteeProjectLookup implements port.CommitteeProjectLookup using NATS
// request/reply against lfx-v2-committee-service on CommitteeGetProjectSubject.
type natsCommitteeProjectLookup struct {
	conn    *nats.Conn
	timeout time.Duration
}

// GetCommitteeProject resolves committeeUID to its owning v2 project UID.
func (c *natsCommitteeProjectLookup) GetCommitteeProject(ctx context.Context, committeeUID string) (string, error) {
	if committeeUID == "" {
		return "", errs.NewValidation("committee UID is required")
	}

	reqBytes, err := json.Marshal(committeeProjectLookupRequest{CommitteeUID: committeeUID})
	if err != nil {
		return "", errs.NewUnexpected("failed to marshal committee project lookup request", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	msg, err := requestWithSpan(reqCtx, c.conn, constants.CommitteeGetProjectSubject, reqBytes)
	if err != nil {
		if err == context.DeadlineExceeded || err == nats.ErrTimeout {
			return "", errs.NewServiceUnavailable("committee project lookup timed out", err)
		}
		return "", errs.NewServiceUnavailable("committee project lookup failed", err)
	}

	var resp committeeProjectLookupResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return "", errs.NewServiceUnavailable("failed to parse committee project lookup response", err)
	}

	if resp.Error != "" {
		if resp.Error == "not found" {
			return "", errs.NewNotFound(resp.Error)
		}
		return "", errs.NewValidation(resp.Error)
	}

	return resp.ProjectUID, nil
}

// NewNATSCommitteeProjectLookup creates a CommitteeProjectLookup backed by the given NATSClient.
func NewNATSCommitteeProjectLookup(client *NATSClient) port.CommitteeProjectLookup {
	return &natsCommitteeProjectLookup{
		conn:    client.conn,
		timeout: committeeProjectLookupTimeout,
	}
}
