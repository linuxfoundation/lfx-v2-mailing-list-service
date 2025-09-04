// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package design defines the Goa API design for the mailing list service.
package design

import (
	"goa.design/goa/v3/dsl"
)

// API describes the global properties of the API server.
var _ = dsl.API("mailing-list", func() {
	dsl.Title("Mailing List Service")
	dsl.Description("Service for managing mailing lists in LFX")
})

// JWTAuth defines the JWT security scheme for authenticated endpoints
var JWTAuth = dsl.JWTSecurity("jwt", func() {
	dsl.Description("Heimdall authorization")
})

// MailingListService defines the mailing list service.
var _ = dsl.Service("mailing-list", func() {
	dsl.Description("The mailing list service manages mailing lists and services")

	// Health check endpoints
	dsl.Method("livez", func() {
		dsl.Description("Check if the service is alive.")
		dsl.Result(dsl.Bytes, func() {
			dsl.Example("OK")
		})
		dsl.HTTP(func() {
			dsl.GET("/livez")
			dsl.Response(dsl.StatusOK, func() {
				dsl.ContentType("text/plain")
			})
		})
	})

	dsl.Method("readyz", func() {
		dsl.Description("Check if the service is able to take inbound requests.")
		dsl.Result(dsl.Bytes, func() {
			dsl.Example("OK")
		})
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/readyz")
			dsl.Response(dsl.StatusOK, func() {
				dsl.ContentType("text/plain")
			})
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	// Service Management endpoints
	dsl.Method("create-grpsio-service", func() {
		dsl.Description("Create GroupsIO service with type-specific validation rules")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()

			ServiceBaseAttributes()

			WritersAttribute()
			AuditorsAttribute()

			// Only common required fields - type-specific validation handled in service layer
			dsl.Required("type", "project_uid")
		})
		dsl.Result(ServiceFull)
		dsl.Error("BadRequest", BadRequestError, "Bad request - Invalid type, missing required fields, or validation failures")
		dsl.Error("NotFound", NotFoundError, "Resource not found")
		dsl.Error("Conflict", ConflictError, "Conflict")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.POST("/groupsio/services")
			dsl.Param("version:v")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusCreated)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("Conflict", dsl.StatusConflict)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("get-grpsio-service", func() {
		dsl.Description("Get groupsIO service details by ID")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			ServiceUIDAttribute()
		})
		dsl.Result(func() {
			dsl.Attribute("service", ServiceWithReadonlyAttributes)
			ETagAttribute()
			dsl.Required("service")
		})
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("NotFound", NotFoundError, "Resource not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/services/{uid}")
			dsl.Param("version:v")
			dsl.Param("uid")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK, func() {
				dsl.Body("service")
				dsl.Header("etag:ETag")
			})
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("update-grpsio-service", func() {
		dsl.Description("Update GroupsIO service")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			IfMatchAttribute()

			ServiceUIDAttribute()
			ServiceBaseAttributes()

			WritersAttribute()
			AuditorsAttribute()

			dsl.Required("type", "project_uid")
		})
		dsl.Result(ServiceWithReadonlyAttributes)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("NotFound", NotFoundError, "Resource not found")
		dsl.Error("Conflict", ConflictError, "Conflict")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.PUT("/groupsio/services/{uid}")
			dsl.Param("version:v")
			dsl.Param("uid")
			dsl.Header("bearer_token:Authorization")
			dsl.Header("if_match:If-Match")
			dsl.Response(dsl.StatusOK)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("Conflict", dsl.StatusConflict)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("delete-grpsio-service", func() {
		dsl.Description("Delete GroupsIO service")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			IfMatchAttribute()
			ServiceUIDAttribute()
		})
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("NotFound", NotFoundError, "Resource not found")
		dsl.Error("Conflict", ConflictError, "Conflict")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.DELETE("/groupsio/services/{uid}")
			dsl.Param("version:v")
			dsl.Param("uid")
			dsl.Header("bearer_token:Authorization")
			dsl.Header("if_match:If-Match")
			dsl.Response(dsl.StatusNoContent)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("Conflict", dsl.StatusConflict)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	// Mailing List Management endpoints
	dsl.Method("create-grpsio-mailing-list", func() {
		dsl.Description("Create GroupsIO mailing list/subgroup with comprehensive validation")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()

			MailingListBaseAttributes()

			WritersAttribute()
			AuditorsAttribute()

			// Required fields for mailing list creation
			dsl.Required("group_name", "public", "type", "description", "title", "service_uid")
		})
		dsl.Result(MailingListFull)
		dsl.Error("BadRequest", BadRequestError, "Bad request - Invalid data, missing required fields, or validation failures")
		dsl.Error("NotFound", NotFoundError, "Parent service not found or committee not found")
		dsl.Error("Conflict", ConflictError, "Mailing list with same name already exists")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.POST("/groupsio/mailing-lists")
			dsl.Param("version:v")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusCreated)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("Conflict", dsl.StatusConflict)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("get-grpsio-mailing-list", func() {
		dsl.Description("Get GroupsIO mailing list details by UID")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			MailingListUIDAttribute()
		})
		dsl.Result(func() {
			dsl.Attribute("mailing_list", MailingListWithReadonlyAttributes)
			ETagAttribute()
			dsl.Required("mailing_list")
		})
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("NotFound", NotFoundError, "Mailing list not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/mailing-lists/{uid}")
			dsl.Param("version:v")
			dsl.Param("uid")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK, func() {
				dsl.Body("mailing_list")
				dsl.Header("etag:ETag")
			})
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("update-grpsio-mailing-list", func() {
		dsl.Description("Update GroupsIO mailing list")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			IfMatchAttribute()

			MailingListUIDAttribute()
			MailingListBaseAttributes()

			WritersAttribute()
			AuditorsAttribute()

			dsl.Required("group_name", "public", "type", "description", "title", "service_uid")
		})
		dsl.Result(MailingListWithReadonlyAttributes)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("NotFound", NotFoundError, "Mailing list not found")
		dsl.Error("Conflict", ConflictError, "Conflict - ETag mismatch or validation failure")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.PUT("/groupsio/mailing-lists/{uid}")
			dsl.Param("version:v")
			dsl.Param("uid")
			dsl.Header("bearer_token:Authorization")
			dsl.Header("if_match:If-Match")
			dsl.Response(dsl.StatusOK)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("Conflict", dsl.StatusConflict)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("delete-grpsio-mailing-list", func() {
		dsl.Description("Delete GroupsIO mailing list")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			IfMatchAttribute()
			MailingListUIDAttribute()
		})
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("NotFound", NotFoundError, "Mailing list not found")
		dsl.Error("Conflict", ConflictError, "Conflict - ETag mismatch or deletion not allowed")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.DELETE("/groupsio/mailing-lists/{uid}")
			dsl.Param("version:v")
			dsl.Param("uid")
			dsl.Header("bearer_token:Authorization")
			dsl.Header("if_match:If-Match")
			dsl.Response(dsl.StatusNoContent)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("Conflict", dsl.StatusConflict)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	// Serve the file gen/http/openapi3.json for requests sent to /openapi.json.
	dsl.Files("/openapi.json", "gen/http/openapi3.json")
})
