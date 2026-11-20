// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package eventing

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/domain/model"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

type fakeConsumerManager struct{ consumer jetstream.Consumer }

func (f fakeConsumerManager) CreateOrUpdateConsumer(context.Context, string, jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	return f.consumer, nil
}

type shutdownConsumeContext struct {
	jetstream.ConsumeContext
	stopped chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func (s *shutdownConsumeContext) Stop()                   { s.once.Do(func() { close(s.stopped); close(s.closed) }) }
func (s *shutdownConsumeContext) Closed() <-chan struct{} { return s.closed }

type shutdownConsumer struct {
	jetstream.Consumer
	ctx *shutdownConsumeContext
}

func (s shutdownConsumer) Consume(handler jetstream.MessageHandler, _ ...jetstream.PullConsumeOpt) (jetstream.ConsumeContext, error) {
	go handler(shutdownMessage{})
	return s.ctx, nil
}

type shutdownMessage struct{ jetstream.Msg }

func (shutdownMessage) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{NumDelivered: 1}, nil
}
func (shutdownMessage) Subject() string                  { return "$KV.v1-objects.itx-groupsio-v2-member.test" }
func (shutdownMessage) Data() []byte                     { return []byte("{}") }
func (shutdownMessage) Headers() nats.Header             { return nil }
func (shutdownMessage) Ack() error                       { return nil }
func (shutdownMessage) NakWithDelay(time.Duration) error { return nil }

type callbackProcessor struct {
	process func(context.Context, model.StreamMessage)
}

func (p callbackProcessor) Process(ctx context.Context, msg model.StreamMessage) { p.process(ctx, msg) }

func TestEventProcessorStart(t *testing.T) {
	t.Run("shutdown preserves handler context and closes storage only after callback completion", func(t *testing.T) {
		consume := &shutdownConsumeContext{stopped: make(chan struct{}), closed: make(chan struct{})}
		ep := &natsEventProcessor{natsClient: fakeConsumerManager{consumer: shutdownConsumer{ctx: consume}}, callbacksDone: make(chan struct{})}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered := make(chan context.Context, 1)
		release := make(chan struct{})
		completed := make(chan struct{})
		storeClosed := make(chan error, 1)
		var releaseOnce sync.Once
		releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }
		t.Cleanup(releaseHandler)
		processor := callbackProcessor{process: func(handlerCtx context.Context, _ model.StreamMessage) {
			entered <- handlerCtx
			select {
			case <-release:
			case <-handlerCtx.Done():
			}
			if handlerCtx.Err() == nil {
				close(completed)
			}
		}}
		go func() { err := ep.Start(ctx, processor); storeClosed <- err }()
		handlerCtx := <-entered
		cancel()
		select {
		case <-consume.stopped:
		case <-time.After(time.Second):
			t.Fatal("consumer not stopped")
		}
		require.NoError(t, handlerCtx.Err(), "lifecycle cancellation must not cancel the in-flight completion write")
		select {
		case err := <-storeClosed:
			t.Fatalf("storage closed while callback was active: %v", err)
		default:
		}
		releaseHandler()
		select {
		case err := <-storeClosed:
			require.NoError(t, err)
		case <-time.After(time.Second):
			t.Fatal("shutdown did not complete")
		}
		select {
		case <-completed:
		default:
			t.Fatal("callback did not complete before store closure")
		}
	})
}

func TestEventProcessorStop(t *testing.T) {
	t.Run("deadline cancels processing but completion barrier remains available", func(t *testing.T) {
		consume := &shutdownConsumeContext{stopped: make(chan struct{}), closed: make(chan struct{})}
		processingCtx, cancelProcessing := context.WithCancel(context.Background())
		defer cancelProcessing()
		ready := make(chan struct{})
		close(ready)
		ep := &natsEventProcessor{started: true, ready: ready, consumeCtx: consume, processingCancel: cancelProcessing, callbacksDone: make(chan struct{})}
		ep.active.Add(1)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		require.ErrorIs(t, ep.Stop(ctx), context.DeadlineExceeded)
		require.ErrorIs(t, processingCtx.Err(), context.Canceled)
		ep.active.Done()
		select {
		case <-ep.callbacksDone:
		case <-time.After(time.Second):
			t.Fatal("callback barrier did not finish")
		}
	})
}
