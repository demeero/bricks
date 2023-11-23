package otelbrick

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type MeterConfig struct {
	Exclusions            map[attribute.Key]*regexp.Regexp
	Headers               map[string]string
	ServiceName           string
	ServiceNamespace      string
	DeploymentEnvironment string
	OTELHTTPEndpoint      string
	OTELHTTPPathPrefix    string
	Insecure              bool
	RuntimeMetrics        bool
	HostMetrics           bool
}

func InitMeter(ctx context.Context, cfg MeterConfig) (func(ctx context.Context) error, error) {
	if cfg.OTELHTTPEndpoint == "" {
		slog.Info("otel meter disabled")
		otel.SetMeterProvider(noopmetric.NewMeterProvider())
		return func(context.Context) error { return nil }, nil
	}
	otlpOpts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(cfg.OTELHTTPEndpoint)}
	if cfg.OTELHTTPPathPrefix != "" {
		otlpOpts = append(otlpOpts, otlpmetrichttp.WithURLPath(fmt.Sprintf("/%s/v1/metrics", cfg.OTELHTTPPathPrefix)))
	}
	if cfg.Insecure {
		otlpOpts = append(otlpOpts, otlpmetrichttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		otlpOpts = append(otlpOpts, otlpmetrichttp.WithHeaders(cfg.Headers))
	}
	exp, err := otlpmetrichttp.New(ctx, otlpOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed init metrics exporter: %w", err)
	}
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceNamespace(cfg.ServiceNamespace),
		semconv.DeploymentEnvironment(cfg.DeploymentEnvironment),
	)
	var reader metric.Reader = metric.NewPeriodicReader(exp)
	if len(cfg.Exclusions) > 0 {
		reader = newExclusionReader(reader, cfg.Exclusions)
	}
	meterProvider := metric.NewMeterProvider(metric.WithResource(res), metric.WithReader(reader))
	if cfg.HostMetrics {
		if err := host.Start(host.WithMeterProvider(meterProvider)); err != nil {
			return nil, fmt.Errorf("failed start host metrics: %w", err)
		}
	}
	if cfg.RuntimeMetrics {
		if err := runtime.Start(runtime.WithMeterProvider(meterProvider)); err != nil {
			return nil, fmt.Errorf("failed start runtime metrics: %w", err)
		}
	}
	otel.SetMeterProvider(meterProvider)
	return meterProvider.Shutdown, nil
}

type exclusionReader struct {
	metric.Reader
	exclusions map[attribute.Key]*regexp.Regexp
}

func newExclusionReader(r metric.Reader, exclusions map[attribute.Key]*regexp.Regexp) *exclusionReader {
	return &exclusionReader{
		Reader:     r,
		exclusions: exclusions,
	}
}

func (r *exclusionReader) Collect(ctx context.Context, rm *metricdata.ResourceMetrics) error {
	if r.exclude(rm.Resource) {
		return nil
	}
	return r.Reader.Collect(ctx, rm)
}

func (r *exclusionReader) exclude(res *resource.Resource) bool {
	for key, matcher := range r.exclusions {
		for _, keyValue := range res.Attributes() {
			if key == keyValue.Key && matcher.MatchString(keyValue.Value.AsString()) {
				return true
			}
		}
	}
	return false
}
