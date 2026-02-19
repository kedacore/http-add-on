package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "keda-http-interceptor"

// setupTracing initialises the OpenTelemetry TracerProvider and text-map
// propagator. It returns a Tracer for creating spans and a shutdown function
// that must be called on exit. The OTLP endpoint and other SDK settings are
// read from standard OTEL_* environment variables by the exporter.
func setupTracing(ctx context.Context, exporter string) (trace.Tracer, func(context.Context) error, error) {
	res, err := resource.Merge(resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(tracerName),
		))
	if err != nil {
		return nil, nil, fmt.Errorf("creating OTel resource: %w", err)
	}

	spanExporter, err := newSpanExporter(ctx, exporter)
	if err != nil {
		return nil, nil, fmt.Errorf("creating OTel exporter (%s): %w", exporter, err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(spanExporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
		b3.New(),
	))

	tracer := tp.Tracer(tracerName)
	shutdown := func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), spanExporter.Shutdown(ctx))
	}
	return tracer, shutdown, nil
}

func newSpanExporter(ctx context.Context, kind string) (sdktrace.SpanExporter, error) {
	switch strings.ToLower(kind) {
	case "http/protobuf":
		return otlptracehttp.New(ctx)
	case "grpc":
		return otlptracegrpc.New(ctx)
	case "console":
		return stdouttrace.New()
	default:
		return nil, fmt.Errorf("unsupported tracing exporter: %q (use http/protobuf, grpc, or console)", kind)
	}
}
