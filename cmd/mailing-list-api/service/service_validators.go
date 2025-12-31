// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"strconv"
	"strings"

	mailinglistservice "github.com/linuxfoundation/lfx-v2-mailing-list-service/gen/mailing_list"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
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

// Reserved words that cannot be used for group names
var reservedWords = []string{
	"admin", "api", "www", "mail", "support", "noreply", "postmaster",
	"no-reply", "webmaster", "root", "administrator", "moderator",
}

// Group name pattern validation is now handled by GOA design layer

// validateGroupName validates business rules for group names (reserved words only)
// Format and length validations are now handled by GOA design layer
func validateGroupName(groupName, fieldName string) error {
	// Reserved words validation (business logic that cannot be in GOA)
	lowerName := strings.ToLower(groupName)
	for _, reserved := range reservedWords {
		if lowerName == reserved {
			return errors.NewValidation(fmt.Sprintf("%s cannot use reserved word '%s'", fieldName, reserved))
		}
	}

	return nil
}

// validateServiceCreationRules validates type-specific business rules for service creation
// TODO: Future PR - Service limits per project when NATS List/Watch available:
// - Max 1 primary per project (unique constraint already enforced)
// - Max 5 formation per project
// - Max 10 shared per project
func validateServiceCreationRules(payload *mailinglistservice.CreateGrpsioServicePayload) error {
	serviceType := payload.Type

	switch serviceType {
	case constants.ServiceTypePrimary:
		return validatePrimaryRules(payload)
	case constants.ServiceTypeFormation:
		return validateFormationRules(payload)
	case constants.ServiceTypeShared:
		return validateSharedRules(payload)
	default:
		return errors.NewValidation(fmt.Sprintf("invalid service type: %s. Must be one of: %s, %s, %s",
			serviceType, constants.ServiceTypePrimary, constants.ServiceTypeFormation, constants.ServiceTypeShared))
	}
}

// validatePrimaryRules validates rules for primary service type
func validatePrimaryRules(payload *mailinglistservice.CreateGrpsioServicePayload) error {
	// primary rules:
	// - prefix must NOT be provided (will return 400 error)
	// - global_owners must be provided and contain at least one valid email
	// - Unique constraint validation handled by storage layer (UniqueProjectType)

	if payload.Prefix != nil && *payload.Prefix != "" {
		return errors.NewValidation("prefix must not be provided for primary service type")
	}

	// global_owners is required for primary services and must be 1-10 emails
	if len(payload.GlobalOwners) == 0 {
		return errors.NewValidation("global_owners is required and must contain at least one email address for primary service type")
	}
	if len(payload.GlobalOwners) > 10 {
		return errors.NewValidation("global_owners must not exceed 10 email addresses for primary service type")
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
	// - global_owners must NOT be provided (will return 400 error)

	if payload.Prefix == nil || strings.TrimSpace(*payload.Prefix) == "" {
		return errors.NewValidation("prefix is required and must be non-empty for shared service type")
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

	// Check prefix immutability - only validate if explicitly provided
	if payload.Prefix != nil && *payload.Prefix != existing.Prefix {
		return errors.NewValidation(fmt.Sprintf("field 'prefix' is immutable. Cannot change from '%s' to '%s'", existing.Prefix, *payload.Prefix))
	}

	// Check domain immutability - only validate if explicitly provided
	if payload.Domain != nil && *payload.Domain != existing.Domain {
		return errors.NewValidation(fmt.Sprintf("field 'domain' is immutable. Cannot change from '%s' to '%s'", existing.Domain, *payload.Domain))
	}

	// Check group_id immutability - only validate if explicitly provided
	if payload.GroupID != nil {
		// Compare nullable pointers properly
		if existing.GroupID == nil || *payload.GroupID != *existing.GroupID {
			existingVal := "null"
			if existing.GroupID != nil {
				existingVal = fmt.Sprintf("%d", *existing.GroupID)
			}
			return errors.NewValidation(fmt.Sprintf("field 'group_id' is immutable. Cannot change from '%s' to '%d'", existingVal, *payload.GroupID))
		}
	}

	// Check url immutability - only validate if explicitly provided
	if payload.URL != nil && *payload.URL != existing.URL {
		return errors.NewValidation(fmt.Sprintf("field 'url' is immutable. Cannot change from '%s' to '%s'", existing.URL, *payload.URL))
	}

	// Check group_name immutability - only validate if explicitly provided
	if payload.GroupName != nil && *payload.GroupName != existing.GroupName {
		return errors.NewValidation(fmt.Sprintf("field 'group_name' is immutable. Cannot change from '%s' to '%s'", existing.GroupName, *payload.GroupName))
	}

	// Validate global_owners email addresses if being updated
	// Primary services MUST always have at least one owner - critical business rule
	if existing.Type == constants.ServiceTypePrimary {
		if len(payload.GlobalOwners) == 0 {
			return errors.NewValidation("global_owners must contain at least one email address for primary service type")
		}
	}
	if len(payload.GlobalOwners) > 10 {
		return errors.NewValidation("global_owners must not exceed 10 email addresses")
	}
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
	case constants.ServiceTypePrimary:
		return errors.NewValidation("Primary services cannot be deleted as they are critical infrastructure components")
	case constants.ServiceTypeFormation:
		// TODO: Add owner permission check when OpenFGA integration is complete
		// For now, allow deletion of formation services
		slog.Debug("Allowing deletion of formation service", "service_id", service.UID, "type", service.Type)
		return nil
	case constants.ServiceTypeShared:
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

	// Group name validation: length, pattern, and reserved words
	if err := validateGroupName(payload.GroupName, "group_name"); err != nil {
		return err
	}

	// Title, description, and committee filter format validations now handled by GOA

	// Committees validation - each committee must have a UID
	for i, committee := range payload.Committees {
		if committee == nil {
			return errors.NewValidation(fmt.Sprintf("committees[%d] cannot be nil", i))
		}
		if committee.UID == "" {
			return errors.NewValidation(fmt.Sprintf("committees[%d].uid is required", i))
		}
	}

	return nil
}

// Description length validation is now handled by GOA design layer

// validateMailingListUpdate validates update constraints for mailing lists
func validateMailingListUpdate(ctx context.Context, existing *model.GrpsIOMailingList, parentService *model.GrpsIOService, payload *mailinglistservice.UpdateGrpsioMailingListPayload, serviceReader port.GrpsIOServiceReader) error {
	// Validate group_name immutability (critical business rule)
	if payload.GroupName != existing.GroupName {
		return errors.NewValidation("field 'group_name' is immutable")
	}

	// Validate main group restrictions (critical business rule from Groups.io)
	if parentService != nil && isMainGroupForService(existing, parentService) {
		// Main groups must remain public announcement lists
		if payload.Type != "announcement" {
			return errors.NewValidation("main group must be an announcement list")
		}
		if !payload.Public {
			return errors.NewValidation("main group must remain public")
		}
	}

	// Cannot set type to "custom" unless already "custom" (Groups.io business rule)
	if payload.Type == "custom" && existing.Type != "custom" {
		return errors.NewValidation("cannot set type to \"custom\"")
	}

	// Cannot change visibility from private to public
	// TODO: LFXV2-479 - Migrate from boolean 'public' field to string 'visibility' field
	// for full Groups.io API compatibility (supporting "public", "private", "custom" values)
	if !existing.Public && payload.Public {
		return errors.NewValidation("cannot change visibility from private to public")
	}

	// Parent service change validation (allow within same project only)
	if payload.ServiceUID != existing.ServiceUID {
		// Check if service reader is available for validation
		if serviceReader == nil {
			// Fallback to old restrictive behavior if no service reader provided
			slog.WarnContext(ctx, "service reader not available for parent service validation - blocking change",
				"mailing_list_uid", existing.UID,
				"old_service_uid", existing.ServiceUID,
				"new_service_uid", payload.ServiceUID)
			return errors.NewValidation("cannot change parent service")
		}

		// Fetch the new parent service to validate the project ownership
		newParentService, _, err := serviceReader.GetGrpsIOService(ctx, payload.ServiceUID)
		if err != nil {
			slog.ErrorContext(ctx, "failed to retrieve new parent service for validation",
				"error", err,
				"new_service_uid", payload.ServiceUID,
				"mailing_list_uid", existing.UID)
			return errors.NewValidation("new parent service not found")
		}

		// Allow parent service changes only within the same project
		if newParentService.ProjectUID != existing.ProjectUID {
			slog.WarnContext(ctx, "blocked cross-project parent service change",
				"mailing_list_uid", existing.UID,
				"current_project_uid", existing.ProjectUID,
				"new_project_uid", newParentService.ProjectUID,
				"current_service_uid", existing.ServiceUID,
				"new_service_uid", payload.ServiceUID)
			return errors.NewValidation("cannot move mailing list to service in different project")
		}

		slog.InfoContext(ctx, "allowing parent service change within same project",
			"mailing_list_uid", existing.UID,
			"project_uid", existing.ProjectUID,
			"old_service_uid", existing.ServiceUID,
			"new_service_uid", payload.ServiceUID)
	}

	// Detect committee changes for member sync
	if committeesChanged(existing.Committees, payload.Committees) {
		// TODO: LFXV2-478 - Trigger committee member sync
		slog.Debug("committees change detected - member sync required", "mailing_list_uid", existing.UID)
	}

	// Description and title length validations now handled by GOA

	// Validate subject tag format if provided
	subjectTagValue := payloadStringValue(payload.SubjectTag)
	if subjectTagValue != "" {
		if !isValidSubjectTag(subjectTagValue) {
			return errors.NewValidation("invalid subject tag format")
		}
	}

	// Committee filter enum validations now handled by GOA

	return nil
}

// validateMailingListDeleteProtection validates deletion protection rules
func validateMailingListDeleteProtection(mailingList *model.GrpsIOMailingList, parentService *model.GrpsIOService) error {
	// Check if it's a main group (any service type)
	if parentService != nil && isMainGroupForService(mailingList, parentService) {
		return errors.NewValidation(fmt.Sprintf("cannot delete the main group of a %s service", parentService.Type))
	}

	// Protect announcement lists (typically used for critical communications)
	if mailingList.Type == "announcement" {
		return errors.NewValidation("announcement lists require special handling for deletion")
	}

	// Check for active committee associations
	if len(mailingList.Committees) > 0 {
		// TODO: LFXV2-478 - When committee sync is implemented, validate:
		// - Check if committee sync is active
		// - Verify no pending sync operations
		// - Ensure committee members are notified
		committeeUIDs := make([]string, 0, len(mailingList.Committees))
		for _, c := range mailingList.Committees {
			committeeUIDs = append(committeeUIDs, c.UID)
		}
		slog.Debug("committee-based list deletion - cleanup may be required",
			"mailing_list_uid", mailingList.UID,
			"committee_uids", committeeUIDs)
	}

	// TODO: LFXV2-353 - Groups.io API integration for:
	// - Actual group/subgroup creation and validation
	// - Validate subscriber count (block if >50 active subscribers)
	// - Check for recent activity (block if activity within 7 days)
	// - Verify no pending messages in moderation queue
	// - DNS delegation checks for primary services

	// TODO: LFXV2-478 - Committee service integration for:
	// - Member synchronization when committee changes
	// - Committee association event handling
	// - Automatic member updates based on committee filters

	return nil
}

// isValidSubjectTag validates subject tag format (business logic only)
// Length validation is now handled by GOA design layer
func isValidSubjectTag(tag string) bool {
	trimmed := strings.TrimSpace(tag)
	if len(trimmed) == 0 {
		return false
	}

	// Check for characters that would break email subject formatting
	invalidChars := []string{"\n", "\r", "\t", "[", "]"}
	for _, char := range invalidChars {
		if strings.Contains(trimmed, char) {
			return false
		}
	}

	return true
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

// isMainGroupForService determines if a list is the "main" group for its parent.
func isMainGroupForService(ml *model.GrpsIOMailingList, svc *model.GrpsIOService) bool {
	switch svc.Type {
	case constants.ServiceTypePrimary:
		return ml.GroupName == svc.GroupName
	case constants.ServiceTypeFormation, constants.ServiceTypeShared:
		return ml.GroupName == svc.Prefix
	default:
		return false
	}
}

// committeesChanged detects if committees have changed between existing domain model and payload
// This includes changes to UIDs and AllowedVotingStatuses to ensure proper validation
func committeesChanged(existing []model.Committee, payload []*mailinglistservice.Committee) bool {
	// Different number of committees means changed
	if len(existing) != len(payload) {
		return true
	}

	// Build a map of existing committees by UID for detailed comparison
	existingMap := make(map[string]model.Committee)
	for _, c := range existing {
		existingMap[c.UID] = c
	}

	// Check if any payload committee has changed UID or AllowedVotingStatuses
	for _, payloadCommittee := range payload {
		if payloadCommittee == nil {
			continue
		}

		existingCommittee, exists := existingMap[payloadCommittee.UID]
		if !exists {
			// Committee UID not found in existing
			return true
		}

		// Compare allowed voting statuses - both length and content
		if len(existingCommittee.AllowedVotingStatuses) != len(payloadCommittee.AllowedVotingStatuses) {
			return true
		}

		// Build status set for comparison
		existingStatuses := make(map[string]bool)
		for _, s := range existingCommittee.AllowedVotingStatuses {
			existingStatuses[s] = true
		}

		// Check if any payload status is missing from existing
		for _, s := range payloadCommittee.AllowedVotingStatuses {
			if !existingStatuses[s] {
				return true
			}
		}
	}

	return false
}

// validateMemberUpdate validates that immutable fields are not changed during updates
func validateMemberUpdate(existing, updated *model.GrpsIOMember) error {
	if existing == nil || updated == nil {
		return errors.NewValidation("invalid member data for validation")
	}

	// Check immutable fields
	if existing.Email != updated.Email {
		return errors.NewValidation("email cannot be changed")
	}

	if existing.UID != updated.UID {
		return errors.NewValidation("member UID cannot be changed")
	}

	if existing.MailingListUID != updated.MailingListUID {
		return errors.NewValidation("mailing list UID cannot be changed")
	}

	// TODO: LFXV2-353 - Add validation for Groups.io sync requirements
	// if existing.GroupsIOMemberID != updated.GroupsIOMemberID {
	//     return errors.NewBadRequest("Groups.io member ID cannot be changed")
	// }

	return nil
}

// validateMemberCreation validates business logic for member creation beyond GOA's basic validations
func validateMemberCreation(ctx context.Context, payload *mailinglistservice.CreateGrpsioMailingListMemberPayload, reader port.GrpsIOServiceReader) error {
	slog.DebugContext(ctx, "validating member creation payload")
	if payload == nil {
		return errors.NewValidation("payload is required")
	}

	// Validate mailing list exists - this will be checked by the orchestrator as well,
	// but we validate early to provide better error messages
	if payload.UID == "" {
		return errors.NewValidation("mailing list UID is required")
	}

	// Check for valid email format - GOA already validates this, but we can add additional business rules here
	if payload.Email == "" {
		return errors.NewValidation("email is required")
	}

	// TODO: LFXV2-480 - Add business logic validations:
	// - Validate mailing list capacity limits
	// - Check member permissions based on who's adding them

	// TODO: LFXV2-353 - Groups.io API integration:
	// - Validate against Groups.io API constraints
	// - Auto-adopt members from Groups.io if they exist there but not in our database

	return nil
}

// validateMemberDeleteProtection validates that a member can be safely deleted
func validateMemberDeleteProtection(member *model.GrpsIOMember) error {
	if member == nil {
		return errors.NewValidation("member is required for deletion validation")
	}

	// Basic validation - member must be in a valid state for deletion
	if member.UID == "" {
		return errors.NewValidation("member UID is required")
	}

	// Check if member is an owner or moderator - log warning for now
	if member.ModStatus == "owner" {
		slog.Warn("Deleting an owner - ensure this is not the sole owner",
			"member_uid", member.UID,
			"email", redaction.RedactEmail(member.Email),
			"mailing_list_uid", member.MailingListUID)
	}

	if member.ModStatus == "moderator" {
		slog.Info("Deleting a moderator",
			"member_uid", member.UID,
			"email", redaction.RedactEmail(member.Email),
			"mailing_list_uid", member.MailingListUID)
	}

	// TODO: LFXV2-353 - Add sole owner/moderator protection via Groups.io API
	// This is already noted in the delete endpoint with a TODO comment
	// When Groups.io API integration is added, we will:
	// - Check if this member is the only owner/moderator of the mailing list
	// - Prevent deletion if it would orphan the mailing list (return error if sole owner)
	// - Validate member status allows deletion
	// - Check cascading impacts of member deletion
	// - Handle Groups.io API error "sole group owner" as seen in old implementation

	return nil
}

// ==================================================================================
// Enhanced Business Rule Validation (POST-PUT Conversion)
// ==================================================================================
// These functions validate business rules AFTER PUT payload conversion to prevent
// violations of critical constraints while maintaining pure PUT semantics.

// validateServiceBusinessRules validates business rules after PUT conversion
// This prevents PUT semantics from violating mandatory business constraints
func validateServiceBusinessRules(service *model.GrpsIOService) error {
	// Primary services MUST have GlobalOwners (critical business rule)
	// This prevents clearing GlobalOwners via PUT from making primary services invalid
	if service.Type == constants.ServiceTypePrimary && len(service.GlobalOwners) == 0 {
		return errors.NewValidation("primary services must have at least one global owner")
	}

	// Additional service business rules can be added here
	// Example: specific service types requiring certain fields

	return nil
}
