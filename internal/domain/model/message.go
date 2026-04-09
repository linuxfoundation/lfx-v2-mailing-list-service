// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"context"
	"encoding/json"
	"log/slog"

	indexertypes "github.com/linuxfoundation/lfx-v2-indexer-service/pkg/types"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
)

// MessageAction is a type for the action of a GroupsIO service message
type MessageAction string

// MessageAction constants for the action of a GroupsIO service message
const (
	// ActionCreated is the action for a resource creation message
	ActionCreated MessageAction = "created"
	// ActionUpdated is the action for a resource update message
	ActionUpdated MessageAction = "updated"
	// ActionDeleted is the action for a resource deletion message
	ActionDeleted MessageAction = "deleted"
)

// IndexerMessage is a NATS message schema for sending messages related to GroupsIO service CRUD operations
// This message is consumed by indexing services to maintain search indexes
type IndexerMessage struct {
	Action  MessageAction     `json:"action"`
	Headers map[string]string `json:"headers"`
	Data    any               `json:"data"`
	// Tags is a list of tags to be set on the indexed resource for search
	Tags []string `json:"tags"`
	// IndexingConfig carries optional indexer-specific configuration.
	// When non-nil it bypasses server-side enrichers and uses the supplied values directly.
	IndexingConfig *indexertypes.IndexingConfig `json:"indexing_config,omitempty"`
}

// Build constructs an indexer message with proper context extraction and data marshaling
func (g *IndexerMessage) Build(ctx context.Context, input any) (*IndexerMessage, error) {
	// Extract headers from context for authorization propagation
	headers := make(map[string]string)
	if authorization, ok := ctx.Value(constants.AuthorizationContextID).(string); ok {
		headers[constants.AuthorizationHeader] = authorization
	} else {
		// Fallback for system-generated events (webhooks, etc.) that don't have user auth context
		// This is just a dummy value so that the indexer service can still process the message,
		// given that it requires an authorization header.
		headers[constants.AuthorizationHeader] = "Bearer mailing-list-service"
	}
	if principal, ok := ctx.Value(constants.PrincipalContextID).(string); ok {
		headers[constants.XOnBehalfOfHeader] = principal
	}
	g.Headers = headers

	var payload any

	switch g.Action {
	case ActionCreated, ActionUpdated:
		// For create/update actions, marshal and unmarshal to get a map[string]any
		// that the indexer expects
		data, err := json.Marshal(input)
		if err != nil {
			slog.ErrorContext(ctx, "error marshalling data into JSON", "error", err)
			return nil, err
		}
		var jsonData map[string]any
		if err := json.Unmarshal(data, &jsonData); err != nil {
			slog.ErrorContext(ctx, "error unmarshalling data into JSON", "error", err)
			return nil, err
		}
		payload = jsonData
	case ActionDeleted:
		// For delete actions, the data should just be a string of the UID being deleted
		payload = input
	}

	g.Data = payload
	return g, nil
}

// BuildWithIndexingConfig is like Build but also sets IndexingConfig on the message.
// Use this when the indexer requires additional routing or configuration beyond the default.
func (g *IndexerMessage) BuildWithIndexingConfig(ctx context.Context, input any, indexingConfig *indexertypes.IndexingConfig) (*IndexerMessage, error) {
	msg, err := g.Build(ctx, input)
	if err != nil {
		return nil, err
	}
	msg.IndexingConfig = indexingConfig
	return msg, nil
}

// GenericFGAMessage is the envelope for all FGA sync operations.
// It uses the generic, resource-agnostic FGA sync handlers.
type GenericFGAMessage struct {
	ObjectType string `json:"object_type"` // Resource type, e.g. "groupsio_service"
	Operation  string `json:"operation"`   // Operation name, e.g. "update_access"
	Data       any    `json:"data"`        // Operation-specific payload
}

// FGAUpdateAccessData is the data payload for update_access operations.
// This is a full sync — any relations not listed (and not excluded) will be removed.
type FGAUpdateAccessData struct {
	UID              string              `json:"uid"`
	Public           bool                `json:"public"`
	Relations        map[string][]string `json:"relations,omitempty"`
	References       map[string][]string `json:"references,omitempty"`
	ExcludeRelations []string            `json:"exclude_relations,omitempty"`
}

// FGADeleteAccessData is the data payload for delete_access operations.
type FGADeleteAccessData struct {
	UID string `json:"uid"`
}

// FGAMemberPutData is the data payload for member_put and member_remove operations.
type FGAMemberPutData struct {
	UID                   string   `json:"uid"`
	Username              string   `json:"username"`
	Relations             []string `json:"relations"`
	MutuallyExclusiveWith []string `json:"mutually_exclusive_with,omitempty"`
}
