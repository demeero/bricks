package httpbrick

import (
	"fmt"
	"net/http"

	"github.com/felixge/httpsnoop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/demeero/bricks/otelbrick"
)

type httpOTELMetrics struct {
	reqDurationHist   metric.Int64Histogram
	reqCounter        metric.Int64Counter
	reqSizeHist       metric.Int64Histogram
	respSizeHist      metric.Int64Histogram
	activeReqsCounter metric.Int64UpDownCounter
}

func newHTTPOTelMetrics(opts otelMeterMetricsOpts) (*httpOTELMetrics, error) {
	httpMeter := otel.GetMeterProvider().Meter("brick/echobrick/OTELMeter")
	result := &httpOTELMetrics{}
	if opts.ActiveReqsCounter {
		activeReqsCounter, err := httpMeter.Int64UpDownCounter(opts.Names.ActiveReqsCounter,
			metric.WithDescription("Number of active HTTP server requests."))
		if err != nil {
			return nil, fmt.Errorf("failed create http.server.active_requests metric: %w", err)
		}
		result.activeReqsCounter = activeReqsCounter
	}
	if opts.ReqDuration {
		srvLatencyHist, err := httpMeter.Int64Histogram(opts.Names.ReqDurationHist,
			metric.WithDescription("Duration of HTTP server requests."),
			metric.WithUnit("ms"),
			metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10))
		if err != nil {
			return nil, fmt.Errorf("failed create http.server.request.duration metric: %w", err)
		}
		result.reqDurationHist = srvLatencyHist
	}
	if opts.ReqCounter {
		srvReqCounter, err := httpMeter.Int64Counter(opts.Names.ReqCounter,
			metric.WithDescription("The number of HTTP requests"))
		if err != nil {
			return nil, fmt.Errorf("failed create http.server.request.count metric: %w", err)
		}
		result.reqCounter = srvReqCounter
	}
	if opts.ReqSize {
		srvReqSizeHist, err := httpMeter.Int64Histogram(opts.Names.ReqSizeHist,
			metric.WithDescription("The size of the request payload body in bytes."), metric.WithUnit("By"))
		if err != nil {
			return nil, fmt.Errorf("failed create http.server.request.body.size metric: %w", err)
		}
		result.reqSizeHist = srvReqSizeHist
	}
	if opts.RespSize {
		srvRespSizeHist, err := httpMeter.Int64Histogram(opts.Names.RespSizeHist,
			metric.WithDescription("Size of HTTP server response bodies."), metric.WithUnit("By"))
		if err != nil {
			return nil, fmt.Errorf("failed create http.server.response.body.size metric: %w", err)
		}
		result.respSizeHist = srvRespSizeHist
	}
	return result, nil
}

// OTelMeterMW is a middleware that records metrics for each request.
//
//nolint:gocognit,gocyclo,cyclop // it's ok to be cyclomatic for this func
func OTelMeterMW(options ...OTelMeterMWOption) (func(h http.Handler) http.Handler, error) {
	opts := otelMeterOpts{
		Attrs: otelMeterAttrsOpts{
			AttrsFromCtx: true,
			AttrsToCtx:   true,
		},
		Metrics: otelMeterMetricsOpts{
			Names: otelMetricNames{
				ReqDurationHist:   "http.server.request.duration",
				ReqCounter:        "http.server.request.count",
				ReqSizeHist:       "http.server.request.body.size",
				RespSizeHist:      "http.server.response.body.size",
				ActiveReqsCounter: "http.server.active_requests",
			},
			ReqDuration:       true,
			ReqCounter:        true,
			ReqSize:           true,
			RespSize:          true,
			ActiveReqsCounter: true,
		},
	}
	for _, opt := range options {
		opt(&opts)
	}
	httpMeter, err := newHTTPOTelMetrics(opts.Metrics)
	if err != nil {
		return nil, fmt.Errorf("failed create http metrics: %w", err)
	}

	var snoop bool
	if opts.Metrics.ReqDuration || opts.Metrics.RespSize || opts.Attrs.Code {
		snoop = true
	}
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if opts.Skipper != nil && opts.Skipper(req) {
				h.ServeHTTP(w, req)
				return
			}
			if opts.Metrics.ActiveReqsCounter {
				httpMeter.activeReqsCounter.Add(req.Context(), 1)
				defer httpMeter.activeReqsCounter.Add(req.Context(), -1)
			}

			var reqSz int64
			if opts.Metrics.ReqSize {
				reqSz = ComputeApproximateRequestSize(req)
			}

			var attrs []attribute.KeyValue
			attrs = append(attrs, semconv.HTTPRequestMethodKey.String(req.Method))

			if opts.Attrs.URI {
				reqURI := req.RequestURI
				if reqURI == "" {
					reqURI = req.URL.RequestURI()
				}
				attrs = append(attrs, semconv.URLPathKey.String(reqURI))
			}
			if opts.Attrs.AttrsFromCtx {
				attrs = append(attrs, otelbrick.AttrsFromCtx(req.Context())...)
			}

			if opts.Attrs.AttrsToCtx {
				req = req.WithContext(otelbrick.AttrsToCtx(req.Context(), attrs))
			}

			if snoop {
				m := httpsnoop.CaptureMetrics(h, w, req)
				if opts.Attrs.Code {
					attrs = append(attrs, semconv.HTTPResponseStatusCode(m.Code))
				}
				ctx := req.Context()
				if opts.Metrics.ReqDuration {
					httpMeter.reqDurationHist.Record(ctx, m.Duration.Milliseconds(), metric.WithAttributes(attrs...))
				}
				if opts.Metrics.RespSize {
					httpMeter.respSizeHist.Record(ctx, m.Written, metric.WithAttributes(attrs...))
				}
				return
			}

			h.ServeHTTP(w, req)

			ctx := req.Context()

			if opts.Metrics.ReqCounter {
				httpMeter.reqCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
			}
			if opts.Metrics.ReqSize {
				httpMeter.reqSizeHist.Record(ctx, reqSz, metric.WithAttributes(attrs...))
			}
		})
	}, nil
}
