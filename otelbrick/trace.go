package otelbrick

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

type TraceConfig struct {
	SpanExclusions        map[attribute.Key]*regexp.Regexp
	Headers               map[string]string
	ServiceName           string
	ServiceNamespace      string
	DeploymentEnvironment string
	OTELGRPCEndpoint      string
	OTELHTTPEndpoint      string
	OTELHTTPPathPrefix    string
	SamplingRate          float64
	Insecure              bool
}

func InitTrace(ctx context.Context, cfg TraceConfig, opts ...sdktrace.TracerProviderOption) (func(context.Context) error, error) {
	if cfg.OTELHTTPEndpoint == "" && cfg.OTELGRPCEndpoint == "" {
		slog.Info("otel trace disabled")
		otel.SetTracerProvider(nooptrace.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	traceExporter, err := createExporter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed create trace exporter: %w", err)
	}

	spanProcessor := sdktrace.NewBatchSpanProcessor(traceExporter)
	if len(cfg.SpanExclusions) > 0 {
		slog.Info("span exclusions enabled")
		spanProcessor = newExclusionSpanProcessor(spanProcessor, cfg.SpanExclusions)
	}
	sampler := sdktrace.AlwaysSample()
	if cfg.SamplingRate > 0 {
		slog.Info("span sampling enabled")
		sampler = sdktrace.TraceIDRatioBased(cfg.SamplingRate)
	}
	opts = append([]sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.ParentBased(sampler)),
		sdktrace.WithResource(createRes(cfg)),
		sdktrace.WithSpanProcessor(spanProcessor),
	}, opts...)
	tracerProvider := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tracerProvider)

	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tracerProvider.Shutdown, nil
}

func createRes(cfg TraceConfig) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceNamespace(cfg.ServiceNamespace),
		semconv.DeploymentEnvironment(cfg.DeploymentEnvironment),
	)
}

func createExporter(ctx context.Context, cfg TraceConfig) (*otlptrace.Exporter, error) {
	if cfg.OTELHTTPEndpoint != "" {
		return createHTTPExporter(ctx, cfg)
	}
	traceOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.OTELGRPCEndpoint)}
	if cfg.Insecure {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	}
	return otlptracegrpc.New(ctx, traceOpts...)
}

func createHTTPExporter(ctx context.Context, cfg TraceConfig) (*otlptrace.Exporter, error) {
	traceOpts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.OTELHTTPEndpoint)}
	if cfg.OTELHTTPPathPrefix != "" {
		traceOpts = append(traceOpts, otlptracehttp.WithURLPath(fmt.Sprintf("/%s/v1/traces", cfg.OTELHTTPPathPrefix)))
	}
	if cfg.Insecure {
		traceOpts = append(traceOpts, otlptracehttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		traceOpts = append(traceOpts, otlptracehttp.WithHeaders(cfg.Headers))
	}
	return otlptracehttp.New(ctx, traceOpts...)
}

type exclusionSpanProcessor struct {
	sdktrace.SpanProcessor
	exclusions map[attribute.Key]*regexp.Regexp
}

func newExclusionSpanProcessor(next sdktrace.SpanProcessor, exclusions map[attribute.Key]*regexp.Regexp) *exclusionSpanProcessor {
	return &exclusionSpanProcessor{SpanProcessor: next, exclusions: exclusions}
}

func (sp *exclusionSpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	if !sp.exclude(s) {
		sp.SpanProcessor.OnEnd(s)
	}
}

func (sp *exclusionSpanProcessor) exclude(s sdktrace.ReadOnlySpan) bool {
	for key, matcher := range sp.exclusions {
		for _, keyValue := range s.Attributes() {
			if key == keyValue.Key && matcher.MatchString(keyValue.Value.AsString()) {
				return true
			}
		}
	}
	return false
}
