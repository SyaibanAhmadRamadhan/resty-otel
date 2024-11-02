package resty_otel

import (
	"fmt"
	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var defaultTracer = otel.Tracer("github.com/SyaibanAhmadRamadhan/resty-otel")

type spanNameFormatter func(resp *resty.Response, req *resty.Request) string

func defaultSpanNameFormatter(resp *resty.Response, req *resty.Request) string {
	if resp != nil {
		return fmt.Sprintf("http method %s, %d", req.Method, resp.StatusCode())
	}
	return "http method" + req.Method
}

type restyTracing struct {
	client                *resty.Client
	tracerProvider        trace.TracerProvider
	propagators           propagation.TextMapPropagator
	spanOptions           []trace.SpanStartOption
	tracer                trace.Tracer
	spanNameFormatter     spanNameFormatter
	attributeResponseBody bool
}

func New(client *resty.Client, opts ...options) {
	client = client.EnableTrace()
	tracing := &restyTracing{
		client:                client,
		tracerProvider:        otel.GetTracerProvider(),
		propagators:           otel.GetTextMapPropagator(),
		spanOptions:           []trace.SpanStartOption{},
		tracer:                defaultTracer,
		spanNameFormatter:     defaultSpanNameFormatter,
		attributeResponseBody: false,
	}

	for _, opt := range opts {
		opt(tracing)
	}

	client.OnBeforeRequest(tracing.onBeforeRequest())
	client.OnAfterResponse(tracing.onAfterResponse())
	client.OnError(tracing.onError())
}

func (r *restyTracing) onBeforeRequest() resty.RequestMiddleware {
	return func(client *resty.Client, request *resty.Request) error {
		ctx, _ := r.tracer.Start(request.Context(), r.spanNameFormatter(nil, request), r.spanOptions...)

		r.propagators.Inject(ctx, propagation.HeaderCarrier(request.Header))
		request.SetContext(ctx)
		return nil
	}
}

func (r *restyTracing) onAfterResponse() resty.ResponseMiddleware {
	return func(c *resty.Client, res *resty.Response) error {
		span := trace.SpanFromContext(res.Request.Context())
		if !span.IsRecording() {
			return nil
		}

		span.SetAttributes(
			attribute.Int("response.status_code", res.StatusCode()),
			attribute.String("response.status", res.Status()),
			attribute.String("response.proto", res.Proto()),
			attribute.String("response.time", res.Time().String()),
			attribute.String("response.received_at", res.ReceivedAt().String()),
		)

		if r.attributeResponseBody {
			span.SetAttributes(
				attribute.String("response.body", fmt.Sprintf("%v", res.String())),
			)
		}

		ti := res.Request.TraceInfo()
		span.SetAttributes(
			attribute.String("trace.dns_lookup", ti.DNSLookup.String()),
			attribute.String("trace.conn_time", ti.ConnTime.String()),
			attribute.String("trace.tcp_conn_time", ti.TCPConnTime.String()),
			attribute.String("trace.tls_handshake", ti.TLSHandshake.String()),
			attribute.String("trace.server_time", ti.ServerTime.String()),
			attribute.String("trace.response_time", ti.ResponseTime.String()),
			attribute.String("trace.total_time", ti.TotalTime.String()),
			attribute.Bool("trace.is_conn_reused", ti.IsConnReused),
			attribute.Bool("trace.is_conn_was_idle", ti.IsConnWasIdle),
			attribute.String("trace.conn_idle_time", ti.ConnIdleTime.String()),
			attribute.Int("trace.request_attempt", ti.RequestAttempt),
			attribute.String("trace.remote_addr", ti.RemoteAddr.String()),
		)
		span.SetName(r.spanNameFormatter(res, res.Request))

		span.End()
		return nil
	}
}

func (r *restyTracing) onError() resty.ErrorHook {
	return func(req *resty.Request, err error) {
		span := trace.SpanFromContext(req.Context())
		if !span.IsRecording() {
			return
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetName(r.spanNameFormatter(nil, req))
		span.End()
	}
}
