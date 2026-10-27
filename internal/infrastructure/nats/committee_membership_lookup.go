// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"errors"
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
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, nats.ErrTimeout) {
			return nil, errs.NewServiceUnavailable("committee membership lookup timed out", err)
		}
		return nil, errs.NewServiceUnavailable("committee membership lookup failed", err)
	}

	return decodeCommitteeMembers(msg.Data)
}

// committeeNotFoundPhrase is the exact substring lfx-v2-committee-service's GetBase use case
// produces for a missing committee (see rewrapWithUID in that repo's committee_reader.go,
// wrapping storage.go's `errs.NewNotFound("committee not found", ...)`): the wire message is
// `committee "<uid>": committee not found: committee UID: <uid>`. The NATS transport only ever
// sends the error's message string — no type or stable code travels over the wire (see that
// repo's cmd/committee-api/service/committee_handler.go, which marshals `errHandler.Error()`
// verbatim) — so this is the most precise match available short of a wire-contract change
// upstream. A bare "not found" substring would also match unrelated failures such as
// "committee settings not found" or an attribute/member lookup miss, misclassifying a
// transient storage error as a terminal NotFound.
const committeeNotFoundPhrase = "committee not found"

// decodeCommitteeMembers decodes a lfx-v2-committee-service list_members reply: either a
// JSON array of committee members, or a {"error":"<msg>"} envelope. It classifies the
// envelope's message into the port error type callers use to decide retry behavior:
//   - "committee not found"                            -> NotFound (the committee doesn't exist)
//   - "invalid committee uid in list_members request" -> Validation (the known malformed-UID
//     response from committee_member_handler.go's uuid.Parse check)
//   - anything else                           -> ServiceUnavailable (storage/internal failures
//     inside committee-service use the same envelope; treat unrecognized messages as a
//     transient upstream failure, not a client-input error, so retry/backoff logic applies)
func decodeCommitteeMembers(data []byte) ([]port.CommitteeMember, error) {
	var errResp committeeMembershipLookupErrorResponse
	if err := json.Unmarshal(data, &errResp); err == nil && errResp.Error != "" {
		switch msg := strings.ToLower(errResp.Error); {
		case strings.Contains(msg, committeeNotFoundPhrase):
			return nil, errs.NewNotFound(errResp.Error)
		case strings.Contains(msg, "invalid committee uid in list_members request"):
			return nil, errs.NewValidation(errResp.Error)
		default:
			return nil, errs.NewServiceUnavailable("committee membership lookup failed: " + errResp.Error)
		}
	}

	var members []port.CommitteeMember
	if err := json.Unmarshal(data, &members); err != nil {
		return nil, errs.NewServiceUnavailable("failed to parse committee membership lookup response", err)
	}
	// The success contract is always a JSON array, using [] for no members. A JSON `null`
	// unmarshals into a nil slice without error, which would otherwise look identical to a
	// legitimate empty result — reject it so a malformed/unexpected reply surfaces as a
	// lookup failure instead of silently reporting zero members.
	if members == nil {
		return nil, errs.NewServiceUnavailable("committee membership lookup returned null instead of an array")
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
