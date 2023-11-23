package otelbrick

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
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
	Headers               map[string]string
	ServiceName           string
	ServiceNamespace      string
	DeploymentEnvironment string
	OTELGRPCEndpoint      string
	OTELHTTPEndpoint      string
	OTELHTTPPathPrefix    string
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

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	opts = append([]sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(createRes(cfg)),
		sdktrace.WithSpanProcessor(bsp),
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
