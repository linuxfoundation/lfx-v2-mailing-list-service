// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	"goa.design/goa/v3/dsl"
)

// BearerTokenAttribute is the DSL attribute for bearer token.
func BearerTokenAttribute() {
	dsl.Token("bearer_token", dsl.String, func() {
		dsl.Description("JWT token issued by Heimdall")
		dsl.Example("eyJhbGci...")
	})
}

// BadRequestError is the DSL type for a bad request error.
var BadRequestError = dsl.Type("bad-request-error", func() {
	dsl.Attribute("message", dsl.String, "Error message", func() {
		dsl.Example("The request was invalid.")
	})
	dsl.Required("message")
})

// NotFoundError is the DSL type for a not found error.
var NotFoundError = dsl.Type("not-found-error", func() {
	dsl.Attribute("message", dsl.String, "Error message", func() {
		dsl.Example("The resource was not found.")
	})
	dsl.Required("message")
})

// UnauthorizedError is the DSL type for an unauthorized error.
var UnauthorizedError = dsl.Type("unauthorized-error", func() {
	dsl.Attribute("message", dsl.String, "Error message", func() {
		dsl.Example("Unauthorized access.")
	})
	dsl.Required("message")
})

// ConflictError is the DSL type for a conflict error.
var ConflictError = dsl.Type("conflict-error", func() {
	dsl.Attribute("message", dsl.String, "Error message", func() {
		dsl.Example("The resource already exists.")
	})
	dsl.Required("message")
})

// InternalServerError is the DSL type for an internal server error.
var InternalServerError = dsl.Type("internal-server-error", func() {
	dsl.Attribute("message", dsl.String, "Error message", func() {
		dsl.Example("An internal server error occurred.")
	})
	dsl.Required("message")
})

// ServiceUnavailableError is the DSL type for a service unavailable error.
var ServiceUnavailableError = dsl.Type("service-unavailable-error", func() {
	dsl.Attribute("message", dsl.String, "Error message", func() {
		dsl.Example("The service is unavailable.")
	})
	dsl.Required("message")
})

// GroupsioServiceType represents an ITX GroupsIO service.
var GroupsioServiceType = dsl.Type("groupsio-service", func() {
	dsl.Description("A GroupsIO service managed via ITX")
	dsl.Attribute("id", dsl.String, "Service ID")
	dsl.Attribute("project_uid", dsl.String, "LFX v2 project UID", func() {
		dsl.Format(dsl.FormatUUID)
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})
	dsl.Attribute("type", dsl.String, "Service type", func() {
		dsl.Example("primary")
	})
	dsl.Attribute("group_id", dsl.Int64, "GroupsIO group ID")
	dsl.Attribute("domain", dsl.String, "Service domain")
	dsl.Attribute("prefix", dsl.String, "Email prefix")
	dsl.Attribute("status", dsl.String, "Service status")
	dsl.Attribute("created_at", dsl.String, "Creation timestamp")
	dsl.Attribute("updated_at", dsl.String, "Last update timestamp")
})

// GroupsioServiceRequestType represents a create/update request for a GroupsIO service.
var GroupsioServiceRequestType = dsl.Type("groupsio-service-request", func() {
	dsl.Description("Request body for creating or updating a GroupsIO service")
	dsl.Attribute("project_uid", dsl.String, "LFX v2 project UID", func() {
		dsl.Format(dsl.FormatUUID)
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})
	dsl.Attribute("type", dsl.String, "Service type", func() {
		dsl.Enum("v2_primary", "v2_formation", "v2_shared")
		dsl.Example("v2_primary")
	})
	dsl.Attribute("group_id", dsl.Int64, "GroupsIO group ID")
	dsl.Attribute("domain", dsl.String, "Service domain")
	dsl.Attribute("prefix", dsl.String, "Email prefix")
	dsl.Attribute("status", dsl.String, "Service status")
})

// GroupsioServiceListType represents a list of GroupsIO services.
var GroupsioServiceListType = dsl.Type("groupsio-service-list", func() {
	dsl.Description("List of GroupsIO services")
	dsl.Attribute("items", dsl.ArrayOf(GroupsioServiceType), "List of services")
	dsl.Attribute("total", dsl.Int, "Total count")
})

// GroupsioSubgroupType represents an ITX GroupsIO subgroup (mailing list).
var GroupsioSubgroupType = dsl.Type("groupsio-subgroup", func() {
	dsl.Description("A GroupsIO subgroup (mailing list) managed via ITX")
	dsl.Attribute("id", dsl.String, "Subgroup ID")
	dsl.Attribute("project_uid", dsl.String, "LFX v2 project UID", func() {
		dsl.Format(dsl.FormatUUID)
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})
	dsl.Attribute("committee_uid", dsl.String, "LFX v2 committee UID", func() {
		dsl.Format(dsl.FormatUUID)
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})
	dsl.Attribute("service_id", dsl.String, "Parent GroupsIO service ID")
	dsl.Attribute("group_id", dsl.Int64, "GroupsIO group ID")
	dsl.Attribute("name", dsl.String, "Subgroup name")
	dsl.Attribute("description", dsl.String, "Subgroup description")
	dsl.Attribute("type", dsl.String, "Subgroup type")
	dsl.Attribute("audience_access", dsl.String, "Audience access setting")
	dsl.Attribute("created_at", dsl.String, "Creation timestamp")
	dsl.Attribute("updated_at", dsl.String, "Last update timestamp")
})

// GroupsioSubgroupRequestType represents a create/update request for a GroupsIO subgroup.
var GroupsioSubgroupRequestType = dsl.Type("groupsio-subgroup-request", func() {
	dsl.Description("Request body for creating or updating a GroupsIO subgroup")
	dsl.Attribute("project_uid", dsl.String, "LFX v2 project UID", func() {
		dsl.Format(dsl.FormatUUID)
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})
	dsl.Attribute("committee_uid", dsl.String, "LFX v2 committee UID", func() {
		dsl.Format(dsl.FormatUUID)
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})
	dsl.Attribute("service_id", dsl.String, "Parent GroupsIO service ID")
	dsl.Attribute("group_id", dsl.Int64, "GroupsIO group ID")
	dsl.Attribute("name", dsl.String, "Subgroup name")
	dsl.Attribute("description", dsl.String, "Subgroup description")
	dsl.Attribute("type", dsl.String, "Subgroup type")
	dsl.Attribute("audience_access", dsl.String, "Audience access setting")
})

// GroupsioSubgroupListType represents a list of GroupsIO subgroups.
var GroupsioSubgroupListType = dsl.Type("groupsio-subgroup-list", func() {
	dsl.Description("List of GroupsIO subgroups")
	dsl.Attribute("items", dsl.ArrayOf(GroupsioSubgroupType), "List of subgroups")
	dsl.Attribute("total", dsl.Int, "Total count")
})

// GroupsioCountType represents a count response.
var GroupsioCountType = dsl.Type("groupsio-count", func() {
	dsl.Description("Count response")
	dsl.Attribute("count", dsl.Int, "Count value")
	dsl.Required("count")
})

// GroupsioMemberType represents an ITX GroupsIO member.
var GroupsioMemberType = dsl.Type("groupsio-member", func() {
	dsl.Description("A member of a GroupsIO subgroup")
	dsl.Attribute("id", dsl.String, "Member ID")
	dsl.Attribute("email", dsl.String, "Member email address", func() {
		dsl.Format(dsl.FormatEmail)
	})
	dsl.Attribute("name", dsl.String, "Member display name")
	dsl.Attribute("member_type", dsl.String, "Member type")
	dsl.Attribute("delivery_mode", dsl.String, "Email delivery mode")
	dsl.Attribute("mod_status", dsl.String, "Moderation status")
	dsl.Attribute("status", dsl.String, "Member status")
	dsl.Attribute("user_id", dsl.String, "User ID")
	dsl.Attribute("organization", dsl.String, "Member organization")
	dsl.Attribute("job_title", dsl.String, "Member job title")
	dsl.Attribute("username", dsl.String, "Groups.io username")
	dsl.Attribute("role", dsl.String, "Member role")
	dsl.Attribute("voting_status", dsl.String, "Voting status")
	dsl.Attribute("created_at", dsl.String, "Creation timestamp")
	dsl.Attribute("updated_at", dsl.String, "Last update timestamp")
})

// GroupsioMemberRequestType represents a create/update request for a GroupsIO member.
var GroupsioMemberRequestType = dsl.Type("groupsio-member-request", func() {
	dsl.Description("Request body for adding or updating a GroupsIO member")
	dsl.Attribute("email", dsl.String, "Member email address", func() {
		dsl.Format(dsl.FormatEmail)
	})
	dsl.Attribute("name", dsl.String, "Member display name")
	dsl.Attribute("member_type", dsl.String, "Member type")
	dsl.Attribute("mod_status", dsl.String, "Moderation status", func() {
		dsl.Enum("none", "moderator", "owner")
	})
	dsl.Attribute("delivery_mode", dsl.String, "Email delivery mode", func() {
		dsl.Enum("email_delivery_single", "email_delivery_digest", "email_delivery_none", "email_delivery_special", "email_delivery_html_digest", "email_delivery_summary")
	})
	dsl.Attribute("user_id", dsl.String, "User ID")
	dsl.Attribute("organization", dsl.String, "Member organization")
	dsl.Attribute("job_title", dsl.String, "Member job title")
})

// GroupsioMemberListType represents a list of GroupsIO members.
var GroupsioMemberListType = dsl.Type("groupsio-member-list", func() {
	dsl.Description("List of GroupsIO members")
	dsl.Attribute("items", dsl.ArrayOf(GroupsioMemberType), "List of members")
	dsl.Attribute("total", dsl.Int, "Total count")
})

// GroupsioInviteMembersRequestType represents an invite members request.
var GroupsioInviteMembersRequestType = dsl.Type("groupsio-invite-members-request", func() {
	dsl.Description("Request body for inviting members to a GroupsIO subgroup")
	dsl.Attribute("emails", dsl.ArrayOf(dsl.String), "Email addresses to invite")
	dsl.Required("emails")
})

// GroupsioCheckSubscriberRequestType represents a check subscriber request.
var GroupsioCheckSubscriberRequestType = dsl.Type("groupsio-check-subscriber-request", func() {
	dsl.Description("Request body for checking if an email is subscribed")
	dsl.Attribute("email", dsl.String, "Email address to check", func() {
		dsl.Format(dsl.FormatEmail)
	})
	dsl.Attribute("subgroup_id", dsl.String, "Subgroup ID")
	dsl.Required("email", "subgroup_id")
})

// GroupsioCheckSubscriberResponseType represents a check subscriber response.
var GroupsioCheckSubscriberResponseType = dsl.Type("groupsio-check-subscriber-response", func() {
	dsl.Description("Response for check subscriber request")
	dsl.Attribute("subscribed", dsl.Boolean, "Whether the email is subscribed")
	dsl.Required("subscribed")
})

// GroupsioProjectsResponseType represents a list of projects with services.
var GroupsioProjectsResponseType = dsl.Type("groupsio-projects-response", func() {
	dsl.Description("Projects that have GroupsIO services")
	dsl.Attribute("projects", dsl.ArrayOf(dsl.String), "List of project identifiers")
})
