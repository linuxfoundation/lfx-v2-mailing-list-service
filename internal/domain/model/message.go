// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package model

import (
	"context"
	"encoding/json"
	"log/slog"

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
}

// Build constructs an indexer message with proper context extraction and data marshaling
func (g *IndexerMessage) Build(ctx context.Context, input any) (*IndexerMessage, error) {
	// Extract headers from context for authorization propagation
	headers := make(map[string]string)
	if authorization, ok := ctx.Value(constants.AuthorizationContextID).(string); ok {
		headers[constants.AuthorizationHeader] = authorization
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

// AccessMessage is the schema for the data in the message sent to the fga-sync service
// These are the fields that the fga-sync service needs in order to update the OpenFGA permissions
type AccessMessage struct {
	UID string `json:"uid"`
	// ObjectType is the type of the object that the message is about, e.g. "groupsio_service"
	ObjectType string `json:"object_type"`
	// Public is the public flag for the object
	Public bool `json:"public"`
	// Relations is reserved for future use and is intentionally left empty
	Relations map[string][]string `json:"relations"`
	// References are used to store the references of the object,
	// e.g. "project" and its value is the project UID for inheritance
	References map[string][]string `json:"references"`
}
