package otel

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Provider wraps both TracerProvider and MeterProvider.
// Call Shutdown when the process exits to flush any buffered telemetry.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	Tracer         trace.Tracer
	Meter          metric.Meter
}

// ToolMetrics holds the OTel instruments emitted for every MCP tool call.
// The agent loop calls these directly — no global state.
type ToolMetrics struct {
	// mcp_tool_invocations_total{tool_name, status}
	Invocations metric.Int64Counter
	// mcp_tool_errors_total{tool_name, error_type}
	Errors metric.Int64Counter
	// mcp_tool_duration_seconds{tool_name}
	Duration metric.Float64Histogram
	// mcp_active_tool_calls{tool_name}
	ActiveCalls metric.Int64UpDownCounter
}

func NewProvider(ctx context.Context, endpoint, serviceName, serviceVersion string) (*Provider, error) {
	// Single gRPC connection reused for both trace and metric exporters.
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("otel: dial collector %s: %w", endpoint, err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			attribute.String("deployment.environment", "production"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel: build resource: %w", err)
	}

	// --- Traces ---
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("otel: trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // tune in prod
	)
	otel.SetTracerProvider(tp)

	// --- Metrics ---
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("otel: metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(metricExporter,
				sdkmetric.WithInterval(15*time.Second),
			),
		),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	tracer := tp.Tracer(serviceName)
	meter := mp.Meter(serviceName)

	return &Provider{
		tracerProvider: tp,
		meterProvider:  mp,
		Tracer:         tracer,
		Meter:          meter,
	}, nil
}

// NewToolMetrics creates the four standard instruments for MCP tool observability.
// Call once at startup and pass the result into each tool handler.
func NewToolMetrics(meter metric.Meter) (*ToolMetrics, error) {
	inv, err := meter.Int64Counter("mcp_tool_invocations_total",
		metric.WithDescription("Total MCP tool invocations"),
		metric.WithUnit("{call}"),
	)
	if err != nil {
		return nil, err
	}

	errs, err := meter.Int64Counter("mcp_tool_errors_total",
		metric.WithDescription("Total MCP tool errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	dur, err := meter.Float64Histogram("mcp_tool_duration_seconds",
		metric.WithDescription("MCP tool call duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30),
	)
	if err != nil {
		return nil, err
	}

	active, err := meter.Int64UpDownCounter("mcp_active_tool_calls",
		metric.WithDescription("Number of in-flight MCP tool calls"),
		metric.WithUnit("{call}"),
	)
	if err != nil {
		return nil, err
	}

	return &ToolMetrics{
		Invocations: inv,
		Errors:      errs,
		Duration:    dur,
		ActiveCalls: active,
	}, nil
}

func (p *Provider) Shutdown(ctx context.Context) {
	_ = p.tracerProvider.Shutdown(ctx)
	_ = p.meterProvider.Shutdown(ctx)
}
