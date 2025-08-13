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

	// Service Management endpoint
	dsl.Method("get-grpsio-service", func() {
		dsl.Description("Get groupsIO service details by ID")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			VersionAttribute()
			dsl.Attribute("uid", dsl.String, "Service unique identifier")
		})
		dsl.Result(func() {
			dsl.Attribute("service", ServiceModel)
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

	// Serve the file gen/http/openapi3.json for requests sent to /openapi.json.
	dsl.Files("/openapi.json", "gen/http/openapi3.json")
})
