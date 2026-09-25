// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package nats

import (
	"context"

	natsgo "github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// tracer is safe to initialize at package level — otel.Tracer() returns a
// delegating tracer that forwards to whatever TracerProvider is registered at
// call time, so otel.SetTracerProvider() updates it regardless of init order.
var tracer = otel.Tracer("github.com/linuxfoundation/lfx-v2-mailing-list-service/internal/infrastructure/nats")

// natsHeaderCarrier adapts nats.Header to the OTel TextMapCarrier interface
// so trace context can be injected/extracted from NATS message headers.
type natsHeaderCarrier natsgo.Header

func (c natsHeaderCarrier) Get(key string) string {
	vals := c[key]
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func (c natsHeaderCarrier) Set(key string, value string) {
	if c == nil {
		return
	}
	c[key] = []string{value}
}

func (c natsHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

var _ propagation.TextMapCarrier = natsHeaderCarrier{}

// requestWithSpan performs a NATS RequestMsgWithContext with tracing.
func requestWithSpan(ctx context.Context, conn *natsgo.Conn, subject string, data []byte) (*natsgo.Msg, error) {
	ctx, span := tracer.Start(ctx, "nats.request",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", subject),
			attribute.Int("messaging.message.body.size", len(data)),
		),
	)
	defer span.End()

	natsMsg := natsgo.NewMsg(subject)
	natsMsg.Data = data
	otel.GetTextMapPropagator().Inject(ctx, natsHeaderCarrier(natsMsg.Header))

	msg, err := conn.RequestMsgWithContext(ctx, natsMsg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return msg, err
}

// publishWithSpan performs a NATS PublishMsg with tracing.
func publishWithSpan(ctx context.Context, conn *natsgo.Conn, subject string, data []byte) error {
	ctx, span := tracer.Start(ctx, "nats.publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", subject),
			attribute.Int("messaging.message.body.size", len(data)),
		),
	)
	defer span.End()

	natsMsg := natsgo.NewMsg(subject)
	natsMsg.Data = data
	otel.GetTextMapPropagator().Inject(ctx, natsHeaderCarrier(natsMsg.Header))

	err := conn.PublishMsg(natsMsg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}
