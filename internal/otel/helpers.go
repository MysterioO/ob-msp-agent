package otel

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// WithStringAttr is a helper for trace.Start to add a string attribute at span creation.
func WithStringAttr(key, val string) trace.SpanStartOption {
	return trace.WithAttributes(attribute.String(key, val))
}
