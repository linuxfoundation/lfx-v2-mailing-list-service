// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/nats-io/nats.go"
)

const (
	lookupSubject  = "lfx.lookup_v1_mapping"
	defaultTimeout = 5 * time.Second
)

// NATSTranslator implements port.Translator using NATS request/reply to the v1-sync-helper.
//
// Key format sent to the v1-sync-helper:
//   - V2ToV1: "{subject}.uid.{fromID}"   e.g. "project.uid.abc-123"
//   - V1ToV2: "{subject}.sfid.{fromID}"  e.g. "project.sfid.a0B000000001abc"
type NATSTranslator struct {
	conn    *nats.Conn
	timeout time.Duration
}

// MapID translates fromID according to subject and direction.
// Returns the translated ID or a domain error if the mapping is not found.
func (t *NATSTranslator) MapID(ctx context.Context, subject, direction, fromID string) (string, error) {
	if fromID == "" {
		return "", errs.NewValidation(fmt.Sprintf("%s ID is required", subject))
	}

	key, err := buildKey(subject, direction, fromID)
	if err != nil {
		return "", err
	}

	response, err := t.lookup(ctx, key)
	if err != nil {
		return "", err
	}

	// Committee V2→V1 responses use compound format "projectSFID:committeeSFID".
	if subject == constants.TranslationSubjectCommittee && direction == constants.TranslationDirectionV2ToV1 {
		return parseCommitteeV2ToV1Response(response)
	}

	return response, nil
}

// buildKey constructs the lookup key from subject and direction.
func buildKey(subject, direction, fromID string) (string, error) {
	switch direction {
	case constants.TranslationDirectionV2ToV1:
		return fmt.Sprintf("%s.uid.%s", subject, fromID), nil
	case constants.TranslationDirectionV1ToV2:
		return fmt.Sprintf("%s.sfid.%s", subject, fromID), nil
	default:
		return "", errs.NewValidation(fmt.Sprintf("unknown translation direction: %s", direction))
	}
}

// lookup performs the NATS request/reply and validates the response.
func (t *NATSTranslator) lookup(ctx context.Context, key string) (string, error) {
	msg, err := t.conn.RequestWithContext(ctx, lookupSubject, []byte(key))
	if err != nil {
		if err == context.DeadlineExceeded || err == nats.ErrTimeout {
			return "", errs.NewServiceUnavailable("v1-sync-helper lookup timed out", err)
		}
		return "", errs.NewServiceUnavailable("failed to lookup ID mapping", err)
	}

	response := string(msg.Data)

	if strings.HasPrefix(response, "error: ") {
		errMsg := strings.TrimPrefix(response, "error: ")
		return "", errs.NewServiceUnavailable(fmt.Sprintf("v1-sync-helper error: %s", errMsg))
	}

	if response == "" {
		return "", errs.NewValidation(fmt.Sprintf("mapping not found for %s", key))
	}

	return response, nil
}

// parseCommitteeV2ToV1Response extracts the committee SFID from a compound
// "projectSFID:committeeSFID" response returned by the v1-sync-helper.
func parseCommitteeV2ToV1Response(response string) (string, error) {
	parts := strings.Split(response, ":")
	if len(parts) == 1 {
		return response, nil
	}
	if len(parts) != 2 || parts[1] == "" {
		return "", errs.NewServiceUnavailable(fmt.Sprintf("unexpected committee mapping format: %s", response))
	}
	return parts[1], nil
}

// NewNATSTranslator creates a NATSTranslator using the provided NATS connection.
func NewNATSTranslator(conn *nats.Conn, timeout time.Duration) *NATSTranslator {
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return &NATSTranslator{conn: conn, timeout: timeout}
}

// NewNATSTranslatorFromClient creates a NATSTranslator from an existing NATSClient.
func NewNATSTranslatorFromClient(client *NATSClient, timeout time.Duration) *NATSTranslator {
	return NewNATSTranslator(client.conn, timeout)
}
