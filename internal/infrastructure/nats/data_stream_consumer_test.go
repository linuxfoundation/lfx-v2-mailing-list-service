// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/stretchr/testify/assert"
)

type blockingEventHandler struct {
	entered chan struct{}
	release chan struct{}
}

func (h *blockingEventHandler) HandleChange(context.Context, string, map[string]any) bool {
	close(h.entered)
	<-h.release
	return false
}
func (h *blockingEventHandler) HandleRemoval(context.Context, string) bool { return false }

func TestDataStreamConsumerInProgressStopsBeforeAck(t *testing.T) {
	h := &blockingEventHandler{entered: make(chan struct{}), release: make(chan struct{})}
	var acked, progress atomic.Int32
	var atAck atomic.Int32
	beat := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		consumer := &dataStreamConsumer{handler: h, heartbeatInterval: time.Millisecond}
		consumer.Process(context.Background(), model.StreamMessage{
			Key: "service.svc-1", Data: []byte(`{}`),
			Ack: func() error { atAck.Store(progress.Load()); acked.Store(1); return nil },
			InProgress: func() error {
				progress.Add(1)
				select {
				case beat <- struct{}{}:
				default:
				}
				return nil
			},
		})
	}()
	<-h.entered
	assert.Equal(t, int32(0), acked.Load())
	select {
	case <-beat:
	case <-time.After(time.Second):
		t.Fatal("in-progress heartbeat did not run during handler processing")
	}
	close(h.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("consumer did not finish")
	}
	assert.Equal(t, int32(1), acked.Load())
	assert.Greater(t, atAck.Load(), int32(0))
	time.Sleep(5 * time.Millisecond)
	assert.Equal(t, atAck.Load(), progress.Load(), "heartbeat must stop before ACK")
}

func TestDataStreamConsumerHeartbeatUsesConfiguredAckWait(t *testing.T) {
	c := NewDataStreamConsumer(&blockingEventHandler{}, 5*time.Second).(*dataStreamConsumer)
	assert.Greater(t, c.heartbeatInterval, time.Duration(0))
	assert.Less(t, c.heartbeatInterval, 5*time.Second)
}

func TestDataStreamConsumerZeroHeartbeatIntervalDoesNotPanic(t *testing.T) {
	var acked bool
	consumer := &dataStreamConsumer{handler: &cancelEventHandler{cancel: func() {}}}
	consumer.Process(context.Background(), model.StreamMessage{
		Key: "member.mem-1", IsRemoval: true,
		InProgress: func() error { return nil },
		Ack:        func() error { acked = true; return nil },
	})
	assert.True(t, acked)
}

func TestDataStreamConsumerCancelledWorkNAKsInsteadOfACKing(t *testing.T) {
	for _, removal := range []bool{false, true} {
		t.Run(map[bool]string{false: "update", true: "removal"}[removal], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var acked, nacked bool
			h := &cancelEventHandler{cancel: cancel}
			NewDataStreamConsumer(h).Process(ctx, model.StreamMessage{
				Key: "member.mem-1", Data: []byte(`{}`), IsRemoval: removal,
				Ack: func() error { acked = true; return nil },
				Nak: func(time.Duration) error { nacked = true; return nil },
			})
			assert.False(t, acked, "cancelled work must not ACK unpublished changes")
			assert.True(t, nacked)
		})
	}
}

type cancelEventHandler struct{ cancel context.CancelFunc }

func (h *cancelEventHandler) HandleChange(context.Context, string, map[string]any) bool {
	h.cancel()
	return false
}

func (h *cancelEventHandler) HandleRemoval(context.Context, string) bool {
	h.cancel()
	return false
}
