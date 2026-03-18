package otel

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ToolHandler is the signature every SRE tool implements.
type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

// Wrap returns a new ToolHandler that emits an OTel span and tool metrics
// around every call to inner. This is the single place where all
// observability for tool calls is wired — individual tools stay clean.
func Wrap(
	tracer trace.Tracer,
	metrics *ToolMetrics,
	toolName string,
	inner ToolHandler,
) ToolHandler {
	return func(ctx context.Context, args map[string]any) (any, error) {
		start := time.Now()
		attrs := []attribute.KeyValue{
			attribute.String("mcp.tool.name", toolName),
		}

		// Start span — this gets correlated to Tempo.
		ctx, span := tracer.Start(ctx, "mcp.tool/"+toolName,
			trace.WithAttributes(attrs...),
			trace.WithSpanKind(trace.SpanKindInternal),
		)
		defer span.End()

		// Track in-flight calls (gauge).
		metrics.ActiveCalls.Add(ctx, 1, metric.WithAttributes(attrs...))
		defer metrics.ActiveCalls.Add(ctx, -1, metric.WithAttributes(attrs...))

		result, err := inner(ctx, args)

		elapsed := time.Since(start).Seconds()
		status := "success"
		if err != nil {
			status = "error"
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			metrics.Errors.Add(ctx, 1, metric.WithAttributes(
				append(attrs, attribute.String("error.type", errorType(err)))...,
			))
		} else {
			span.SetStatus(codes.Ok, "")
		}

		statusAttr := append(attrs, attribute.String("status", status))
		metrics.Invocations.Add(ctx, 1, metric.WithAttributes(statusAttr...))
		metrics.Duration.Record(ctx, elapsed, metric.WithAttributes(statusAttr...))

		return result, err
	}
}

// errorType returns a coarse error category suitable for a metric label.
// Keep cardinality low — don't include dynamic values like error messages.
func errorType(err error) string {
	if err == nil {
		return ""
	}
	// Callers can wrap errors with typed sentinels; for now coarse categories.
	switch err.(type) {
	default:
		return "internal"
	}
}
