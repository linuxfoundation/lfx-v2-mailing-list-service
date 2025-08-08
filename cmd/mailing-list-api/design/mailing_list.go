// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

// Package design defines the Goa API design for the mailing list service.
package design

import (
	. "goa.design/goa/v3/dsl" //nolint:revive,staticcheck // GOA design requires dot imports
)

// API describes the global properties of the API server.
var _ = API("mailing-list", func() {
	Title("Mailing List Service")
	Description("Service for managing mailing lists in LFX")
	Version("1.0")
	Server("mailing-list-api", func() {
		Host("localhost", func() {
			URI("http://localhost:8080")
		})
	})
})

// Note: JWT security not included in base PR as health endpoints are unauthenticated
// JWT security will be added when implementing authenticated CRUD operations:
//
// var JWTAuth = JWTSecurity("jwt", func() {
//     Description("Heimdall authorization")
// })

// MailingListService defines the mailing list service.
var _ = Service("mailing-list", func() {
	Description("The mailing list service manages mailing lists")

	// Health check endpoints
	Method("livez", func() {
		Description("Liveness probe endpoint")
		HTTP(func() {
			GET("/livez")
			Response(StatusOK)
		})
	})

	Method("readyz", func() {
		Description("Readiness probe endpoint")
		HTTP(func() {
			GET("/readyz")
			Response(StatusOK)
			Response(StatusServiceUnavailable)
		})
	})
})
