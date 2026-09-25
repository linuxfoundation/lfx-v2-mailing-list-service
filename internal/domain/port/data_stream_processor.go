// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package port

import (
	"context"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
)

// DataStreamProcessor processes individual messages dispatched by an EventProcessor.
type DataStreamProcessor interface {
	Process(ctx context.Context, msg model.StreamMessage)
}
