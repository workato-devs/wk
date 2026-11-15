// Package telemetry implements anonymous usage collection for the wk CLI.
//
// Design decisions are recorded in docs/adrs/ADR-010-telemetry.md.
// The short version:
//   - Collects command names, flags (not values), OS/arch, CLI version, duration.
//   - Identifies devices by HMAC-SHA256 of the OS machine ID (not the raw ID).
//   - Opt-out via DO_NOT_TRACK=1, WK_TELEMETRY_DISABLED=1, or `wk telemetry disable`.
//   - WK_TELEMETRY_DEBUG=1 prints events to stderr without sending them.
//   - Telemetry failure is always silent — it must never surface to the user.
package telemetry

import (
	"context"
	"os"
	"runtime"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	wkversion "github.com/workato-devs/wk/internal/version"
)
const (
	// ServiceName is the OTEL service name emitted on every span resource.
	ServiceName = "wk"

	// SourceHeader and SourceValue are sent on every OTLP request to identify
	// events as coming from the wk CLI. Not a secret — it is an identifier for
	// routing and rate limiting. See ADR-010 Decision 2 and ADR-001 Decision 3.
	SourceHeader = "X-Wk-Source"
	SourceValue  = "wk-cli-prod"

	// DefaultEndpoint is the production OTLP ingest URL.
	DefaultEndpoint = "https://telemetry.wk.workato.com"

	// EnvEndpoint overrides the ingest endpoint. Use for local test server:
	//   WK_TELEMETRY_ENDPOINT=http://localhost:4318 wk status
	EnvEndpoint = "WK_TELEMETRY_ENDPOINT"

	// EnvDebug enables debug mode: spans are printed to stderr, not sent.
	EnvDebug = "WK_TELEMETRY_DEBUG"

	// ShutdownTimeout is the maximum time to flush pending spans on exit.
	// A CLI command is typically done in <1s; 2s is generous.
	ShutdownTimeout = 2 * time.Second
)

// Provider holds the OTEL trace provider and the active command span.
// A zero Provider is valid and no-op — all methods are safe to call on it.
type Provider struct {
	tp         *sdktrace.TracerProvider
	activeSpan trace.Span
}

// global is the package-level provider set by Init. Cobra hooks call
// GlobalProvider() to access it.
var global *Provider

// Init initialises the telemetry provider. Must be called once at startup,
// before any cobra command runs. The returned Provider's Shutdown must be
// called before process exit to flush pending spans.
//
// Returns a no-op Provider if telemetry is disabled or if any initialisation
// step fails. Never returns an error.
func Init(ctx context.Context) *Provider {
	p := &Provider{}
	global = p

	if IsDisabled() {
		return p
	}

	if os.Getenv(EnvDebug) == "1" {
		dp := initDebugProvider(ctx)
		global = dp
		return dp
	}

	endpoint := DefaultEndpoint
	if e := os.Getenv(EnvEndpoint); e != "" {
		endpoint = e
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithHeaders(map[string]string{SourceHeader: SourceValue}),
		otlptracehttp.WithTimeout(2 * time.Second),
	}
	// Allow plain HTTP for local test server (http:// prefix).
	if len(endpoint) >= 7 && endpoint[:7] == "http://" {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return p // silent — telemetry failure must not surface
	}

	res := buildResource(ctx)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			// A CLI process is short-lived. Use a very short batch timeout so
			// spans are exported before Shutdown is called.
			sdktrace.WithBatchTimeout(500*time.Millisecond),
			sdktrace.WithMaxExportBatchSize(1),
		),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	ShowFirstRunNotice()

	p.tp = tp
	global = p
	return p
}

// GlobalProvider returns the package-level Provider set by Init.
// Returns a no-op Provider if Init has not been called.
func GlobalProvider() *Provider {
	if global != nil {
		return global
	}
	return &Provider{}
}

// StartCommand creates a root span for a CLI command invocation. Called from
// the cobra PersistentPreRunE hook after cobra has resolved the target command.
// The commandPath is the cobra command path (e.g. "wk push", "wk agentic genies list").
func (p *Provider) StartCommand(ctx context.Context, commandPath string) context.Context {
	if p.tp == nil {
		return ctx
	}
	tracer := otel.Tracer(ServiceName)
	ctx, span := tracer.Start(ctx, "wk.command",
		trace.WithAttributes(attribute.String("wk.command", commandPath)),
	)
	p.activeSpan = span
	return ctx
}

// EndCommand finalises the active span with outcome attributes and ends it.
// Called from Execute after the command returns, before Shutdown.
//
// flagsPresent contains the names of flags that were explicitly set (not their
// values — values may contain sensitive data such as paths or project names).
func (p *Provider) EndCommand(flagsPresent []string, exitCode int) {
	if p.activeSpan == nil {
		return
	}
	p.activeSpan.SetAttributes(
		attribute.StringSlice("wk.flags", flagsPresent),
		attribute.Int("wk.exit_code", exitCode),
		attribute.String("wk.os", runtime.GOOS),
		attribute.String("wk.arch", runtime.GOARCH),
		attribute.String("wk.go_version", runtime.Version()),
		attribute.Bool("wk.ci", IsCI()),
		attribute.String("wk.version", wkversion.Version()),
	)
	p.activeSpan.End()
	p.activeSpan = nil
}

// Shutdown flushes all pending spans and stops the provider. Must be called
// before process exit. Blocks until the flush completes or ctx expires.
// Safe to call on a zero Provider.
func (p *Provider) Shutdown(ctx context.Context) {
	if p.tp == nil {
		return
	}
	_ = p.tp.Shutdown(ctx)
}

// buildResource constructs the OTEL Resource for all spans. The resource
// carries attributes that are constant for the lifetime of the process.
func buildResource(ctx context.Context) *resource.Resource {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(ServiceName),
			semconv.ServiceVersion(wkversion.Version()),
			attribute.String("wk.device_id", DeviceID()),
		),
	)
	if err != nil {
		// resource.New can fail if the context is cancelled; return an
		// empty resource rather than propagating.
		return resource.Empty()
	}
	return res
}
