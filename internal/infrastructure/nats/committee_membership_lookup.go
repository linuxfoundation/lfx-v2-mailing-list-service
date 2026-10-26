// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/nats-io/nats.go"
)

const (
	committeeMembershipLookupTimeout = 5 * time.Second
)

// committeeMembershipLookupErrorResponse matches the {"error":"<msg>"} envelope
// that lfx-v2-committee-service's generic NATS handler sends on failure.
type committeeMembershipLookupErrorResponse struct {
	Error string `json:"error,omitempty"`
}

// natsCommitteeMembershipLookup implements port.CommitteeMembershipLookup using NATS
// request/reply against lfx-v2-committee-service on CommitteeListMembersSubject.
//
// The request body is the raw committee UID (not JSON-wrapped) and the success
// response is a JSON array of committee member objects, matching
// lfx-v2-committee-service's HandleCommitteeListMembers contract.
type natsCommitteeMembershipLookup struct {
	conn    *nats.Conn
	timeout time.Duration
}

// GetCommitteeMembers returns the current members of committeeUID.
func (c *natsCommitteeMembershipLookup) GetCommitteeMembers(ctx context.Context, committeeUID string) ([]port.CommitteeMember, error) {
	if committeeUID == "" {
		return nil, errs.NewValidation("committee UID is required")
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	msg, err := requestWithSpan(reqCtx, c.conn, constants.CommitteeListMembersSubject, []byte(committeeUID))
	if err != nil {
		if err == context.DeadlineExceeded || err == nats.ErrTimeout {
			return nil, errs.NewServiceUnavailable("committee membership lookup timed out", err)
		}
		return nil, errs.NewServiceUnavailable("committee membership lookup failed", err)
	}

	var errResp committeeMembershipLookupErrorResponse
	if err := json.Unmarshal(msg.Data, &errResp); err == nil && errResp.Error != "" {
		if strings.Contains(strings.ToLower(errResp.Error), "not found") {
			return nil, errs.NewNotFound(errResp.Error)
		}
		return nil, errs.NewValidation(errResp.Error)
	}

	var members []port.CommitteeMember
	if err := json.Unmarshal(msg.Data, &members); err != nil {
		return nil, errs.NewServiceUnavailable("failed to parse committee membership lookup response", err)
	}

	return members, nil
}

// NewNATSCommitteeMembershipLookup creates a CommitteeMembershipLookup backed by the given NATSClient.
func NewNATSCommitteeMembershipLookup(client *NATSClient) port.CommitteeMembershipLookup {
	return &natsCommitteeMembershipLookup{
		conn:    client.conn,
		timeout: committeeMembershipLookupTimeout,
	}
}
