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
	dsl.Description("Service for proxying GroupsIO operations to the ITX API")
})

// JWTAuth defines the JWT security scheme for authenticated endpoints
var JWTAuth = dsl.JWTSecurity("jwt", func() {
	dsl.Description("Heimdall authorization")
})

// MailingListService defines the mailing list service.
var _ = dsl.Service("mailing-list", func() {
	dsl.Description("The mailing list service proxies GroupsIO operations to the ITX API")

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

	// ---- GroupsIO Service endpoints ----

	dsl.Method("list-groupsio-services", func() {
		dsl.Description("List GroupsIO services, optionally filtered by project UID")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("project_uid", dsl.String, "LFX v2 project UID filter", func() {
				dsl.Format(dsl.FormatUUID)
			})
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioServiceListType)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/services")
			dsl.Param("project_uid")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("create-groupsio-service", func() {
		dsl.Description("Create a GroupsIO service")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Extend(GroupsioServiceRequestType)
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioServiceType)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("Conflict", ConflictError, "Conflict")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.POST("/groupsio/services")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusCreated)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("Conflict", dsl.StatusConflict)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("get-groupsio-service", func() {
		dsl.Description("Get a GroupsIO service by ID")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("service_id", dsl.String, "Service ID")
			dsl.Required("service_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioServiceType)
		dsl.Error("NotFound", NotFoundError, "Service not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/services/{service_id}")
			dsl.Param("service_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("update-groupsio-service", func() {
		dsl.Description("Update a GroupsIO service")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("service_id", dsl.String, "Service ID")
			dsl.Extend(GroupsioServiceRequestType)
			dsl.Required("service_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioServiceType)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("NotFound", NotFoundError, "Service not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.PUT("/groupsio/services/{service_id}")
			dsl.Param("service_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("delete-groupsio-service", func() {
		dsl.Description("Delete a GroupsIO service")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("service_id", dsl.String, "Service ID")
			dsl.Required("service_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Error("NotFound", NotFoundError, "Service not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.DELETE("/groupsio/services/{service_id}")
			dsl.Param("service_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusNoContent)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("get-groupsio-service-projects", func() {
		dsl.Description("Get projects that have GroupsIO services")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioProjectsResponseType)
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/services/_projects")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("find-parent-groupsio-service", func() {
		dsl.Description("Find the parent GroupsIO service for a project")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("project_uid", dsl.String, "LFX v2 project UID", func() {
				dsl.Format(dsl.FormatUUID)
			})
			dsl.Required("project_uid")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioServiceType)
		dsl.Error("NotFound", NotFoundError, "Parent service not found")
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/services/find_parent")
			dsl.Param("project_uid")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	// ---- GroupsIO Subgroup endpoints ----

	dsl.Method("list-groupsio-subgroups", func() {
		dsl.Description("List GroupsIO subgroups, optionally filtered by project UID and/or committee UID")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("project_uid", dsl.String, "LFX v2 project UID filter", func() {
				dsl.Format(dsl.FormatUUID)
			})
			dsl.Attribute("committee_uid", dsl.String, "LFX v2 committee UID filter", func() {
				dsl.Format(dsl.FormatUUID)
			})
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioSubgroupListType)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/subgroups")
			dsl.Param("project_uid")
			dsl.Param("committee_uid")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("create-groupsio-subgroup", func() {
		dsl.Description("Create a GroupsIO subgroup")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Extend(GroupsioSubgroupRequestType)
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioSubgroupType)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("Conflict", ConflictError, "Conflict")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.POST("/groupsio/subgroups")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusCreated)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("Conflict", dsl.StatusConflict)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("get-groupsio-subgroup", func() {
		dsl.Description("Get a GroupsIO subgroup by ID")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("subgroup_id", dsl.String, "Subgroup ID")
			dsl.Required("subgroup_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioSubgroupType)
		dsl.Error("NotFound", NotFoundError, "Subgroup not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/subgroups/{subgroup_id}")
			dsl.Param("subgroup_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("update-groupsio-subgroup", func() {
		dsl.Description("Update a GroupsIO subgroup")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("subgroup_id", dsl.String, "Subgroup ID")
			dsl.Extend(GroupsioSubgroupRequestType)
			dsl.Required("subgroup_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioSubgroupType)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("NotFound", NotFoundError, "Subgroup not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.PUT("/groupsio/subgroups/{subgroup_id}")
			dsl.Param("subgroup_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("delete-groupsio-subgroup", func() {
		dsl.Description("Delete a GroupsIO subgroup")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("subgroup_id", dsl.String, "Subgroup ID")
			dsl.Required("subgroup_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Error("NotFound", NotFoundError, "Subgroup not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.DELETE("/groupsio/subgroups/{subgroup_id}")
			dsl.Param("subgroup_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusNoContent)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("get-groupsio-subgroup-count", func() {
		dsl.Description("Get count of GroupsIO subgroups for a project")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("project_uid", dsl.String, "LFX v2 project UID", func() {
				dsl.Format(dsl.FormatUUID)
			})
			dsl.Required("project_uid")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioCountType)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/subgroups/count")
			dsl.Param("project_uid")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("get-groupsio-subgroup-member-count", func() {
		dsl.Description("Get count of members in a GroupsIO subgroup")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("subgroup_id", dsl.String, "Subgroup ID")
			dsl.Required("subgroup_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioCountType)
		dsl.Error("NotFound", NotFoundError, "Subgroup not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/subgroups/{subgroup_id}/member_count")
			dsl.Param("subgroup_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	// ---- GroupsIO Member endpoints ----

	dsl.Method("list-groupsio-members", func() {
		dsl.Description("List members of a GroupsIO subgroup")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("subgroup_id", dsl.String, "Subgroup ID")
			dsl.Required("subgroup_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioMemberListType)
		dsl.Error("NotFound", NotFoundError, "Subgroup not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/subgroups/{subgroup_id}/members")
			dsl.Param("subgroup_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("add-groupsio-member", func() {
		dsl.Description("Add a member to a GroupsIO subgroup")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("subgroup_id", dsl.String, "Subgroup ID")
			dsl.Extend(GroupsioMemberRequestType)
			dsl.Required("subgroup_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioMemberType)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("NotFound", NotFoundError, "Subgroup not found")
		dsl.Error("Conflict", ConflictError, "Member already exists")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.POST("/groupsio/subgroups/{subgroup_id}/members")
			dsl.Param("subgroup_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusCreated)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("Conflict", dsl.StatusConflict)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("get-groupsio-member", func() {
		dsl.Description("Get a member of a GroupsIO subgroup by ID")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("subgroup_id", dsl.String, "Subgroup ID")
			dsl.Attribute("member_id", dsl.String, "Member ID")
			dsl.Required("subgroup_id", "member_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioMemberType)
		dsl.Error("NotFound", NotFoundError, "Member not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.GET("/groupsio/subgroups/{subgroup_id}/members/{member_id}")
			dsl.Param("subgroup_id")
			dsl.Param("member_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("update-groupsio-member", func() {
		dsl.Description("Update a member of a GroupsIO subgroup")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("subgroup_id", dsl.String, "Subgroup ID")
			dsl.Attribute("member_id", dsl.String, "Member ID")
			dsl.Extend(GroupsioMemberRequestType)
			dsl.Required("subgroup_id", "member_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioMemberType)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("NotFound", NotFoundError, "Member not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.PUT("/groupsio/subgroups/{subgroup_id}/members/{member_id}")
			dsl.Param("subgroup_id")
			dsl.Param("member_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("delete-groupsio-member", func() {
		dsl.Description("Delete a member from a GroupsIO subgroup")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("subgroup_id", dsl.String, "Subgroup ID")
			dsl.Attribute("member_id", dsl.String, "Member ID")
			dsl.Required("subgroup_id", "member_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Error("NotFound", NotFoundError, "Member not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.DELETE("/groupsio/subgroups/{subgroup_id}/members/{member_id}")
			dsl.Param("subgroup_id")
			dsl.Param("member_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusNoContent)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	dsl.Method("invite-groupsio-members", func() {
		dsl.Description("Invite members to a GroupsIO subgroup by email")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Attribute("subgroup_id", dsl.String, "Subgroup ID")
			dsl.Extend(GroupsioInviteMembersRequestType)
			dsl.Required("subgroup_id", "emails")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("NotFound", NotFoundError, "Subgroup not found")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.POST("/groupsio/subgroups/{subgroup_id}/invitemembers")
			dsl.Param("subgroup_id")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusNoContent)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("NotFound", dsl.StatusNotFound)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	// ---- Other endpoints ----

	dsl.Method("check-groupsio-subscriber", func() {
		dsl.Description("Check if an email address is subscribed to a GroupsIO subgroup")
		dsl.Security(JWTAuth)
		dsl.Payload(func() {
			BearerTokenAttribute()
			dsl.Extend(GroupsioCheckSubscriberRequestType)
			dsl.Required("email", "subgroup_id")
			dsl.Token("bearer_token", dsl.String)
		})
		dsl.Result(GroupsioCheckSubscriberResponseType)
		dsl.Error("BadRequest", BadRequestError, "Bad request")
		dsl.Error("InternalServerError", InternalServerError, "Internal server error")
		dsl.Error("ServiceUnavailable", ServiceUnavailableError, "Service unavailable")
		dsl.HTTP(func() {
			dsl.POST("/groupsio/checksubscriber")
			dsl.Header("bearer_token:Authorization")
			dsl.Response(dsl.StatusOK)
			dsl.Response("BadRequest", dsl.StatusBadRequest)
			dsl.Response("InternalServerError", dsl.StatusInternalServerError)
			dsl.Response("ServiceUnavailable", dsl.StatusServiceUnavailable)
		})
	})

	// Serve the file gen/http/openapi3.json for requests sent to /openapi.json.
	dsl.Files("/openapi.json", "gen/http/openapi3.json")
})
