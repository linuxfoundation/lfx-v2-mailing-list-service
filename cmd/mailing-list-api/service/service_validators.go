// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"fmt"
	"log/slog"
	"net/mail"
	"strconv"
	"strings"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/redaction"
)

// etagValidator validates ETag format and converts to uint64 for optimistic locking
// Supports standard HTTP ETag formats: "123", W/"123", and plain numeric "123"
func etagValidator(etag *string) (uint64, error) {
	// Parse ETag to get revision for optimistic locking
	if etag == nil || *etag == "" {
		return 0, errors.NewValidation("ETag is required for update operations")
	}

	raw := strings.TrimSpace(*etag)

	// Handle weak ETags: W/"123" -> "123"
	if strings.HasPrefix(raw, "W/") || strings.HasPrefix(raw, "w/") {
		raw = strings.TrimSpace(raw[2:])
	}

	// Strip surrounding quotes if present: "123" -> 123
	raw = strings.Trim(raw, `"`)

	parsedRevision, errParse := strconv.ParseUint(raw, 10, 64)
	if errParse != nil {
		return 0, errors.NewValidation("invalid ETag format", errParse)
	}

	return parsedRevision, nil
}

// validateServiceCreationRules validates type-specific business rules for service creation
func validateServiceCreationRules(payload *mailinglistservice.CreateGrpsioServicePayload) error {
	serviceType := payload.Type

	switch serviceType {
	case "primary":
		return validatePrimaryRules(payload)
	case "formation":
		return validateFormationRules(payload)
	case "shared":
		return validateSharedRules(payload)
	default:
		return errors.NewValidation(fmt.Sprintf("invalid service type: %s. Must be one of: primary, formation, shared", serviceType))
	}
}

// validatePrimaryRules validates rules for primary service type
func validatePrimaryRules(payload *mailinglistservice.CreateGrpsioServicePayload) error {
	// primary rules:
	// - prefix must NOT be provided (will return 400 error)
	// - global_owners must be provided and contain at least one valid email
	// - No existing non-formation service for the project (TODO: implement project validation)

	if payload.Prefix != nil && *payload.Prefix != "" {
		return errors.NewValidation("prefix must not be provided for primary service type")
	}

	// global_owners is required for primary services
	if len(payload.GlobalOwners) == 0 {
		return errors.NewValidation("global_owners is required and must contain at least one email address for primary service type")
	}

	// Validate global_owners email addresses
	if err := validateEmailAddresses(payload.GlobalOwners, "global_owners"); err != nil {
		return err
	}

	return nil
}

// validateFormationRules validates rules for formation service type
func validateFormationRules(payload *mailinglistservice.CreateGrpsioServicePayload) error {
	// formation rules:
	// - prefix must be non-empty string

	if payload.Prefix == nil || strings.TrimSpace(*payload.Prefix) == "" {
		return errors.NewValidation("prefix is required and must be non-empty for formation service type")
	}

	// Validate global_owners email addresses if provided
	if err := validateEmailAddresses(payload.GlobalOwners, "global_owners"); err != nil {
		return err
	}

	return nil
}

// validateSharedRules validates rules for shared service type
func validateSharedRules(payload *mailinglistservice.CreateGrpsioServicePayload) error {
	// shared rules:
	// - prefix must be non-empty string
	// - group_id must be valid Groups.io group ID
	// - global_owners must NOT be provided (will return 400 error)

	if payload.Prefix == nil || strings.TrimSpace(*payload.Prefix) == "" {
		return errors.NewValidation("prefix is required and must be non-empty for shared service type")
	}

	if payload.GroupID == nil || *payload.GroupID <= 0 {
		return errors.NewValidation("group_id is required and must be a valid Groups.io group ID for shared service type")
	}

	if len(payload.GlobalOwners) > 0 {
		return errors.NewValidation("global_owners must not be provided for shared service type")
	}

	return nil
}

// validateUpdateImmutabilityConstraints validates that only mutable fields are being modified
func validateUpdateImmutabilityConstraints(existing *model.GrpsIOService, payload *mailinglistservice.UpdateGrpsioServicePayload) error {
	// Immutable Fields: type, project_uid, prefix, domain, group_id, url, group_name
	// Mutable Fields: global_owners, status, public only

	if payload.Type != existing.Type {
		return errors.NewValidation(fmt.Sprintf("field 'type' is immutable. Cannot change from '%s' to '%s'", existing.Type, payload.Type))
	}

	if payload.ProjectUID != existing.ProjectUID {
		return errors.NewValidation(fmt.Sprintf("field 'project_uid' is immutable. Cannot change from '%s' to '%s'", existing.ProjectUID, payload.ProjectUID))
	}

	// Check prefix immutability
	if payload.Prefix != nil && *payload.Prefix != existing.Prefix {
		return errors.NewValidation(fmt.Sprintf("field 'prefix' is immutable. Cannot change from '%s' to '%s'", existing.Prefix, *payload.Prefix))
	}

	// Check domain immutability
	if payload.Domain != nil && *payload.Domain != existing.Domain {
		return errors.NewValidation(fmt.Sprintf("field 'domain' is immutable. Cannot change from '%s' to '%s'", existing.Domain, *payload.Domain))
	}

	// Check group_id immutability
	if payload.GroupID != nil && *payload.GroupID != existing.GroupID {
		return errors.NewValidation(fmt.Sprintf("field 'group_id' is immutable. Cannot change from '%d' to '%d'", existing.GroupID, *payload.GroupID))
	}

	// Check url immutability
	if payload.URL != nil && *payload.URL != existing.URL {
		return errors.NewValidation(fmt.Sprintf("field 'url' is immutable. Cannot change from '%s' to '%s'", existing.URL, *payload.URL))
	}

	// Check group_name immutability
	if payload.GroupName != nil && *payload.GroupName != existing.GroupName {
		return errors.NewValidation(fmt.Sprintf("field 'group_name' is immutable. Cannot change from '%s' to '%s'", existing.GroupName, *payload.GroupName))
	}

	// Validate global_owners email addresses if being updated
	if err := validateEmailAddresses(payload.GlobalOwners, "global_owners"); err != nil {
		return err
	}

	return nil
}

// validateDeleteProtectionRules validates deletion protection rules based on service type
func validateDeleteProtectionRules(service *model.GrpsIOService) error {
	// Delete Protection Rules:
	// - primary services: Cannot be deleted (critical infrastructure protection)
	// - formation/shared services: Can be deleted by owner only (TODO: implement owner check)

	switch service.Type {
	case "primary":
		return errors.NewValidation("Primary services cannot be deleted as they are critical infrastructure components")
	case "formation":
		// TODO: Add owner permission check when OpenFGA integration is complete
		// For now, allow deletion of formation services
		slog.Debug("Allowing deletion of formation service", "service_id", service.UID, "type", service.Type)
		return nil
	case "shared":
		// TODO: Add owner permission check when OpenFGA integration is complete
		// For now, allow deletion of shared services
		slog.Debug("Allowing deletion of shared service", "service_id", service.UID, "type", service.Type)
		return nil
	default:
		return errors.NewValidation(fmt.Sprintf("Unknown service type '%s' - deletion not permitted", service.Type))
	}
}

// validateEmailAddresses validates a slice of email addresses
func validateEmailAddresses(emails []string, fieldName string) error {
	if emails == nil {
		return nil
	}
	for _, email := range emails {
		if _, err := mail.ParseAddress(email); err != nil {
			return errors.NewValidation(fmt.Sprintf("invalid email address in %s: %s", fieldName, redaction.RedactEmail(email)))
		}
	}
	return nil
}

// validateMailingListCreation validates mailing list creation payload
func validateMailingListCreation(payload *mailinglistservice.CreateGrpsioMailingListPayload) error {
	if payload == nil {
		return errors.NewValidation("payload is required")
	}

	// Group name length validation (like old ITX service - max 34 chars)
	if len(payload.GroupName) > 34 {
		return errors.NewValidation("group name is too long (maximum 34 characters)")
	}

	// Committee filters validation
	if len(payload.CommitteeFilters) > 0 && (payload.CommitteeUID == nil || *payload.CommitteeUID == "") {
		return errors.NewValidation("committee must not be empty if committee_filters is non-empty")
	}

	// Validate committee filter values
	validFilters := []string{"Voting Rep", "Alternate Voting Rep", "Observer", "Emeritus", "None"}
	for _, filter := range payload.CommitteeFilters {
		if !contains(validFilters, filter) {
			return errors.NewValidation(fmt.Sprintf("invalid committee_filter: %s. Valid values: %v", filter, validFilters))
		}
	}

	return nil
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
