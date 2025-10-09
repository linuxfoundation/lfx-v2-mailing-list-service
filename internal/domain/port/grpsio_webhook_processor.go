// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// GrpsIOWebhookProcessor handles GroupsIO webhook event processing
type GrpsIOWebhookProcessor interface {
	ProcessEvent(ctx context.Context, event *model.GrpsIOWebhookEvent) error
}
