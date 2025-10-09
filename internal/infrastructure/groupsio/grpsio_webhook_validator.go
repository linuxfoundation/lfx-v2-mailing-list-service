// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package groupsio

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"log/slog"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// GrpsIOWebhookValidator handles validation of GroupsIO webhook signatures
type GrpsIOWebhookValidator struct {
	secret string
}

// NewGrpsIOWebhookValidator creates a new GroupsIO webhook validator
func NewGrpsIOWebhookValidator(secret string) port.GrpsIOWebhookValidator {
	return &GrpsIOWebhookValidator{secret: secret}
}

// ValidateSignature validates the GroupsIO HMAC-SHA1 signature
func (v *GrpsIOWebhookValidator) ValidateSignature(body []byte, signature string) error {
	if v.secret == "" {
		return fmt.Errorf("webhook secret not configured")
	}

	if signature == "" {
		return fmt.Errorf("missing webhook signature")
	}

	// Calculate HMAC-SHA1 with base64 encoding (GroupsIO algorithm)
	mac := hmac.New(sha1.New, []byte(v.secret))
	mac.Write(body)
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		slog.Error("invalid webhook signature")
		return fmt.Errorf("invalid webhook signature")
	}

	return nil
}

// IsValidEvent checks if the event type is supported by GroupsIO
func (v *GrpsIOWebhookValidator) IsValidEvent(eventType string) bool {
	validEvents := map[string]bool{
		constants.SubGroupCreatedEvent:       true,
		constants.SubGroupDeletedEvent:       true,
		constants.SubGroupMemberAddedEvent:   true,
		constants.SubGroupMemberRemovedEvent: true,
		constants.SubGroupMemberBannedEvent:  true,
	}
	return validEvents[eventType]
}
