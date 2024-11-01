package resty_otel

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type options func(tracing *restyTracing)

func WithCustomFormatter(fn spanNameFormatter) options {
	return func(tracing *restyTracing) {
		if fn != nil {
			tracing.spanNameFormatter = fn
		}
	}
}

func WithTraceResponseBody() options {
	return func(tracing *restyTracing) {
		tracing.attributeResponseBody = true
	}
}

func WithCustomTracerName(s string) options {
	return func(tracing *restyTracing) {
		if s != "" {
			tracing.tracer = otel.Tracer(s)
		}
	}
}

func WithSpanStartOptions(opts []trace.SpanStartOption) options {
	return func(tracing *restyTracing) {
		if opts != nil {
			tracing.spanOptions = opts
		}
	}
}
