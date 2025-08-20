// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package design

import (
	"goa.design/goa/v3/dsl"
)

// ServiceBase is the DSL type for a GroupsIO service base.
var ServiceBase = dsl.Type("service-base", func() {
	dsl.Description("A base representation of GroupsIO services without readonly attributes.")

	ServiceBaseAttributes()
})

// ServiceCreatePayload is a specialized type for service creation with validation
var ServiceCreatePayload = dsl.Type("service-create-payload", func() {
	dsl.Description("Payload for creating GroupsIO services with type-specific validation.")

	ServiceBaseAttributes()

	// Add validation rules that will be enforced in the service layer
	dsl.Meta("validation:type-specific", "true")
})

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
	CreatedAtAttribute()
	UpdatedAtAttribute()
})

// ServiceUIDAttribute is the DSL attribute for service UID.
func ServiceUIDAttribute() {
	dsl.Attribute("uid", dsl.String, "Service UID -- unique identifier for the service", func() {
		dsl.Example("7cad5a8d-19d0-41a4-81a6-043453daf9ee")
		dsl.Format(dsl.FormatUUID)
	})
}

// ServiceModel represents a GroupsIO service (legacy - keeping for compatibility)
var ServiceModel = ServiceWithReadonlyAttributes

// BearerTokenAttribute is the DSL attribute for bearer token.
func BearerTokenAttribute() {
	dsl.Token("bearer_token", dsl.String, func() {
		dsl.Description("JWT token issued by Heimdall")
		dsl.Example("eyJhbGci...")
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
