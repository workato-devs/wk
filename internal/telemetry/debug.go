package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// initDebugProvider returns a Provider that dumps spans to stderr as formatted
// JSON without making any network calls. Used when WK_TELEMETRY_DEBUG=1.
func initDebugProvider(ctx context.Context) *Provider {
	exp := &stderrExporter{}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exp), // synchronous: no batching, no network
		sdktrace.WithResource(buildResource(ctx)),
	)
	otel.SetTracerProvider(tp)
	ShowFirstRunNotice()
	return &Provider{tp: tp}
}

// stderrExporter implements sdktrace.SpanExporter. It marshals each finished
// span as indented JSON and writes it to stderr with a [telemetry] prefix.
type stderrExporter struct{}

func (e *stderrExporter) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	for _, s := range spans {
		evt := debugEvent(s)
		data, err := json.MarshalIndent(evt, "", "  ")
		if err != nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "[telemetry] %s\n", data)
	}
	return nil
}

func (e *stderrExporter) Shutdown(_ context.Context) error { return nil }

// debugEvent converts a ReadOnlySpan into a plain map for clean JSON output.
func debugEvent(s sdktrace.ReadOnlySpan) map[string]any {
	attrs := make(map[string]any, len(s.Attributes()))
	for _, kv := range s.Attributes() {
		attrs[string(kv.Key)] = kv.Value.AsInterface()
	}
	res := make(map[string]any)
	for _, kv := range s.Resource().Attributes() {
		res[string(kv.Key)] = kv.Value.AsInterface()
	}
	var durationMS float64
	if !s.StartTime().IsZero() && !s.EndTime().IsZero() {
		durationMS = float64(s.EndTime().Sub(s.StartTime())) / float64(time.Millisecond)
	}
	return map[string]any{
		"name":        s.Name(),
		"trace_id":    s.SpanContext().TraceID().String(),
		"span_id":     s.SpanContext().SpanID().String(),
		"start_time":  s.StartTime().UTC().Format(time.RFC3339Nano),
		"duration_ms": durationMS,
		"attributes":  attrs,
		"resource":    res,
	}
}

// Ensure tracetest is imported — used in tests to create in-memory spans.
var _ = tracetest.NewInMemoryExporter
var _ attribute.Key
