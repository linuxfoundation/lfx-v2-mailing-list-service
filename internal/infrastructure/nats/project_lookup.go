// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/port"
	errs "github.com/linuxfoundation/lfx-v2-mailing-list-service/pkg/errors"
	"github.com/nats-io/nats.go"
)

const (
	projectGetSlugSubject    = "lfx.projects-api.get_slug"
	projectLookupTimeout     = 5 * time.Second
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

	msg, err := p.conn.RequestWithContext(reqCtx, projectGetSlugSubject, []byte(projectUID))
	if err != nil {
		if err == context.DeadlineExceeded || err == nats.ErrTimeout {
			return "", errs.NewServiceUnavailable("project slug lookup timed out", err)
		}
		return "", errs.NewServiceUnavailable("project slug lookup failed", err)
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
