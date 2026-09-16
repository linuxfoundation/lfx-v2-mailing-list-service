// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	"github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/constants"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/nats-io/nats.go"
)

// projectServiceErrorCode returns the error code from a project-service error
// envelope ({"error":"not_found",...} or {"error":"internal",...}), or "" if
// data is a normal success payload.
func projectServiceErrorCode(data []byte) string {
	var env struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &env) != nil {
		return ""
	}
	return env.Error
}

const (
	projectLookupTimeout = 5 * time.Second
)

// natsProjectLookup implements port.ProjectLookup using NATS request/reply
// against the project service's lfx.projects-api.get_slug subject.
type natsProjectLookup struct {
	conn    *nats.Conn
	timeout time.Duration
}

// GetProjectSlug returns the URL slug for the given project UID.
func (p *natsProjectLookup) GetProjectSlug(ctx context.Context, projectUID string) (string, error) {
	if projectUID == "" {
		return "", nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	msg, err := requestWithSpan(reqCtx, p.conn, constants.ProjectGetSlugSubject, []byte(projectUID))
	if err != nil {
		if err == context.DeadlineExceeded || err == nats.ErrTimeout {
			return "", errs.NewServiceUnavailable("project slug lookup timed out", err)
		}
		return "", errs.NewServiceUnavailable("project slug lookup failed", err)
	}

	// Project-service returns {"error":"<code>",...} on errors; any other response
	// (plain slug string or empty body for a project with no slug) is a success value.
	if code := projectServiceErrorCode(msg.Data); code != "" {
		if code == "not_found" {
			// A confirmed absent project is treated the same as a project with
			// no slug: return ("", nil) so callers proceed without a slug
			// rather than retrying a permanent absence as a transient failure.
			return "", nil
		}
		return "", errs.NewUnexpected(fmt.Sprintf("project-service error for %s (code=%s)", projectUID, code))
	}
	return string(msg.Data), nil
}

// NewNATSProjectLookup creates a ProjectLookup backed by the given NATSClient.
func NewNATSProjectLookup(client *NATSClient) port.ProjectLookup {
	return &natsProjectLookup{
		conn:    client.conn,
		timeout: projectLookupTimeout,
	}
}
