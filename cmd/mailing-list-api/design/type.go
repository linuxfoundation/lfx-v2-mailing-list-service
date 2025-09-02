// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	"goa.design/goa/v3/dsl"
)

// ServiceBaseAttributes is the DSL attributes for a GroupsIO service base.
func ServiceBaseAttributes() {
	dsl.Attribute("type", dsl.String, "Service type", func() {
		dsl.Enum("primary", "formation", "shared")
		dsl.Example("primary")
	})
	dsl.Attribute("domain", dsl.String, "Service domain", func() {
		dsl.Example("lists.project.org")
	})
	dsl.Attribute("group_id", dsl.Int64, "GroupsIO group ID", func() {
		dsl.Example(12345)
	})
	dsl.Attribute("status", dsl.String, "Service status", func() {
		dsl.Example("created")
	})
	dsl.Attribute("global_owners", dsl.ArrayOf(dsl.String), "List of global owner email addresses (required for primary, forbidden for shared)", func() {
		dsl.Elem(func() {
			dsl.Format(dsl.FormatEmail)
		})
		dsl.Example([]string{"admin@example.com"})
	})
	dsl.Attribute("prefix", dsl.String, "Email prefix (required for formation and shared, forbidden for primary)", func() {
		dsl.Example("formation")
	})
	dsl.Attribute("project_slug", dsl.String, "Project slug identifier", func() {
		dsl.Format(dsl.FormatRegexp)
		dsl.Pattern(`^[a-z][a-z0-9_\-]*[a-z0-9]$`)
		dsl.Example("cncf")
	})
	dsl.Attribute("project_uid", dsl.String, "LFXv2 Project UID", func() {
		dsl.Format(dsl.FormatUUID)
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})
	dsl.Attribute("url", dsl.String, "Service URL", func() {
		dsl.Format(dsl.FormatURI)
		dsl.Example("https://lists.project.org")
	})
	dsl.Attribute("group_name", dsl.String, "GroupsIO group name", func() {
		dsl.Example("project-name")
	})
	dsl.Attribute("public", dsl.Boolean, "Whether the service is publicly accessible", func() {
		dsl.Default(false)
		dsl.Example(true)
	})

	// Base required fields for all service types
	dsl.Required("type", "project_uid")
}

// ServiceWithReadonlyAttributes is the DSL type for a GroupsIO service with readonly attributes.
var ServiceWithReadonlyAttributes = dsl.Type("service-with-readonly-attributes", func() {
	dsl.Description("A representation of GroupsIO services with readonly attributes.")

	ServiceUIDAttribute()
	ServiceBaseAttributes()
	ProjectNameAttribute()
	CreatedAtAttribute()
	UpdatedAtAttribute()
	LastReviewedAtAttribute()
	LastReviewedByAttribute()
	LastAuditedByAttribute()
	LastAuditedTimeAttribute()
	WritersAttribute()
	AuditorsAttribute()
})

// ServiceUIDAttribute is the DSL attribute for service UID.
func ServiceUIDAttribute() {
	dsl.Attribute("uid", dsl.String, "Service UID -- unique identifier for the service", func() {
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		dsl.Format(dsl.FormatUUID)
	})
}

// ServiceFull is the DSL type for a complete service representation with all attributes.
var ServiceFull = dsl.Type("service-full", func() {
	dsl.Description("A complete representation of GroupsIO services with all attributes including access control and audit trail.")

	ServiceUIDAttribute()
	ServiceBaseAttributes()
	ProjectNameAttribute()
	CreatedAtAttribute()
	UpdatedAtAttribute()
	LastReviewedAtAttribute()
	LastReviewedByAttribute()
	LastAuditedByAttribute()
	LastAuditedTimeAttribute()
	WritersAttribute()
	AuditorsAttribute()
})

// BearerTokenAttribute is the DSL attribute for bearer token.
func BearerTokenAttribute() {
	dsl.Token("bearer_token", dsl.String, func() {
		dsl.Description("JWT token issued by Heimdall")
		dsl.Example("eyJhbGci...")
	})
}

// LastReviewedAtAttribute is the DSL attribute for last review timestamp.
func LastReviewedAtAttribute() {
	dsl.Attribute("last_reviewed_at", dsl.String, "The timestamp when the service was last reviewed in RFC3339 format", func() {
		dsl.Format(dsl.FormatDateTime)
		dsl.Example("2025-08-04T09:00:00Z")
	})
}

// LastReviewedByAttribute is the DSL attribute for last review user.
func LastReviewedByAttribute() {
	dsl.Attribute("last_reviewed_by", dsl.String, "The user ID who last reviewed this service", func() {
		dsl.Example("user_id_12345")
	})
}

// ProjectNameAttribute is the DSL attribute for project name (read-only).
func ProjectNameAttribute() {
	dsl.Attribute("project_name", dsl.String, "Project name (read-only)", func() {
		dsl.Example("Cloud Native Computing Foundation")
	})
}

// ProjectSlugAttribute is the DSL attribute for project slug (read-only).
func ProjectSlugAttribute() {
	dsl.Attribute("project_slug", dsl.String, "Project slug identifier (read-only)", func() {
		dsl.Format(dsl.FormatRegexp)
		dsl.Pattern(`^[a-z][a-z0-9_\-]*[a-z0-9]$`)
		dsl.Example("cncf")
	})
}

// WritersAttribute is the DSL attribute for service writers.
func WritersAttribute() {
	dsl.Attribute("writers", dsl.ArrayOf(dsl.String), "Manager user IDs who can edit/modify this service", func() {
		dsl.Example([]string{"manager_user_id1", "manager_user_id2"})
	})
}

// AuditorsAttribute is the DSL attribute for service auditors.
func AuditorsAttribute() {
	dsl.Attribute("auditors", dsl.ArrayOf(dsl.String), "Auditor user IDs who can audit this service", func() {
		dsl.Example([]string{"auditor_user_id1", "auditor_user_id2"})
	})
}

// LastAuditedByAttribute is the DSL attribute for last audited by user.
func LastAuditedByAttribute() {
	dsl.Attribute("last_audited_by", dsl.String, "The user ID who last audited the service", func() {
		dsl.Example("user_id_12345")
	})
}

// LastAuditedTimeAttribute is the DSL attribute for last audit timestamp.
func LastAuditedTimeAttribute() {
	dsl.Attribute("last_audited_time", dsl.String, "The timestamp when the service was last audited", func() {
		dsl.Format(dsl.FormatDateTime)
		dsl.Example("2023-05-10T09:15:00Z")
	})
}

// IfMatchAttribute is the DSL attribute for If-Match header (for conditional requests).
func IfMatchAttribute() {
	dsl.Attribute("if_match", dsl.String, "If-Match header value for conditional requests", func() {
		dsl.Example("123")
	})
}

// VersionAttribute is the DSL attribute for API version.
func VersionAttribute() {
	dsl.Attribute("version", dsl.String, "Version of the API", func() {
		dsl.Example("1")
		dsl.Enum("1")
	})
}

// ETagAttribute is the DSL attribute for ETag header.
func ETagAttribute() {
	dsl.Attribute("etag", dsl.String, "ETag header value", func() {
		dsl.Example("123")
	})
}

// CreatedAtAttribute is the DSL attribute for creation timestamp.
func CreatedAtAttribute() {
	dsl.Attribute("created_at", dsl.String, "The timestamp when the service was created (read-only)", func() {
		dsl.Format(dsl.FormatDateTime)
		dsl.Example("2023-01-15T10:30:00Z")
	})
}

// UpdatedAtAttribute is the DSL attribute for update timestamp.
func UpdatedAtAttribute() {
	dsl.Attribute("updated_at", dsl.String, "The timestamp when the service was last updated (read-only)", func() {
		dsl.Format(dsl.FormatDateTime)
		dsl.Example("2023-06-20T14:45:30Z")
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

// MailingListBaseAttributes defines attributes for mailing list requests (CREATE/UPDATE) - excludes project_uid.
func MailingListBaseAttributes() {
	dsl.Attribute("group_name", dsl.String, "Mailing list group name", func() {
		dsl.Example("technical-steering-committee")
		dsl.Pattern(`^[a-zA-Z0-9][a-zA-Z0-9_-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$`)
		dsl.MinLength(3)
		dsl.MaxLength(34)
	})
	dsl.Attribute("public", dsl.Boolean, "Whether the mailing list is publicly accessible", func() {
		dsl.Default(false)
		dsl.Example(false)
	})
	dsl.Attribute("type", dsl.String, "Mailing list type", func() {
		dsl.Enum("announcement", "discussion_moderated", "discussion_open")
		dsl.Example("discussion_moderated")
	})
	dsl.Attribute("committee_uid", dsl.String, "Committee UUID for committee-based mailing lists", func() {
		dsl.Format(dsl.FormatUUID)
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})
	dsl.Attribute("committee_filters", dsl.ArrayOf(dsl.String), "Committee member filters", func() {
		dsl.Elem(func() {
			dsl.Enum("Voting Rep", "Alternate Voting Rep", "Observer", "Emeritus", "None")
		})
		dsl.Example([]string{"Voting Rep", "Alternate Voting Rep"})
	})
	dsl.Attribute("description", dsl.String, "Mailing list description (11-500 characters)", func() {
		dsl.MinLength(11)
		dsl.MaxLength(500)
		dsl.Example("Technical steering committee discussions")
	})
	dsl.Attribute("title", dsl.String, "Mailing list title", func() {
		dsl.Example("Technical Steering Committee")
		dsl.MinLength(5)
		dsl.MaxLength(100)
	})
	dsl.Attribute("subject_tag", dsl.String, "Subject tag prefix", func() {
		dsl.Example("[TSC]")
		dsl.MaxLength(50)
	})
	dsl.Attribute("service_uid", dsl.String, "Service UUID", func() {
		dsl.Format(dsl.FormatUUID)
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})

}

// MailingListUIDAttribute is the DSL attribute for mailing list UID.
func MailingListUIDAttribute() {
	dsl.Attribute("uid", dsl.String, "Mailing list UID -- unique identifier for the mailing list", func() {
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		dsl.Format(dsl.FormatUUID)
	})
}

// MailingListFull is the DSL type for a complete mailing list representation with all attributes.
var MailingListFull = dsl.Type("mailing-list-full", func() {
	dsl.Description("A complete representation of GroupsIO mailing lists with all attributes including access control and audit trail.")

	MailingListUIDAttribute()
	MailingListBaseAttributes()

	// project_uid only appears in responses (inherited from parent service)
	dsl.Attribute("project_uid", dsl.String, "LFXv2 Project UID (inherited from parent service)", func() {
		dsl.Format(dsl.FormatUUID)
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})

	ProjectNameAttribute()
	ProjectSlugAttribute()
	CreatedAtAttribute()
	UpdatedAtAttribute()
	LastReviewedAtAttribute()
	LastReviewedByAttribute()
	WritersAttribute()
	AuditorsAttribute()
})

// MailingListWithReadonlyAttributes is the DSL type for a mailing list with readonly attributes.
var MailingListWithReadonlyAttributes = dsl.Type("mailing-list-with-readonly-attributes", func() {
	dsl.Description("A representation of GroupsIO mailing lists with readonly attributes.")

	MailingListUIDAttribute()
	MailingListBaseAttributes()

	// project_uid only appears in responses (inherited from parent service)
	dsl.Attribute("project_uid", dsl.String, "LFXv2 Project UID (inherited from parent service)", func() {
		dsl.Format(dsl.FormatUUID)
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
	})

	ProjectNameAttribute()
	ProjectSlugAttribute()
	CreatedAtAttribute()
	UpdatedAtAttribute()
	WritersAttribute()
	AuditorsAttribute()
})
