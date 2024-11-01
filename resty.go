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
		ctx, span := r.tracer.Start(request.Context(), r.spanNameFormatter(nil, request), r.spanOptions...)
		span.SetAttributes(
			attribute.String("http.method", request.Method),
			attribute.String("http.url", request.URL),
			attribute.String("http.host", request.RawRequest.Host),
			attribute.String("http.path", request.RawRequest.URL.Path),
			attribute.String("http.scheme", request.RawRequest.URL.Scheme),
			attribute.Int("http.content_length", int(request.RawRequest.ContentLength)),
			attribute.String("http.query", request.RawRequest.URL.RawQuery),
			attribute.String("http.client_ip", request.RawRequest.RemoteAddr),
		)

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
			attribute.Int("http.status_code", res.StatusCode()),
			attribute.String("http.status_text", res.Status()),
			attribute.Int64("http.response_content_length", res.Size()),
			attribute.String("http.response_body", string(res.Body())),
			attribute.Float64("http.response_time_ms", res.Time().Seconds()*1000),
			attribute.String("http.protocol", res.Request.RawRequest.Proto),
			attribute.String("http.connection_reused", fmt.Sprintf("%v", res.Request.RawRequest.Response.Close)),
			attribute.String("http.remote_address", res.Request.RawRequest.RemoteAddr),
			attribute.Int("http.request_content_length", int(res.Request.RawRequest.ContentLength)),
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
