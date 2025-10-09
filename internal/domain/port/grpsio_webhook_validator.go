// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

// GrpsIOWebhookValidator defines the contract for GroupsIO webhook signature validation
type GrpsIOWebhookValidator interface {
	// ValidateSignature validates the webhook signature against the raw body
	ValidateSignature(body []byte, signature string) error

	// IsValidEvent checks if the event type is supported
	IsValidEvent(eventType string) bool
}
