package echobrick

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/demeero/bricks/httpbrick"
	"github.com/demeero/bricks/otelbrick"
	"github.com/demeero/bricks/slogbrick"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type LogCtxMWConfig struct {
	Trace     bool
	URI       bool
	Host      bool
	IP        bool
	UserAgent bool
	RequestID bool
}

// SlogCtxMW is a middleware that adds slog logger to request context.
// It adds http_route_path and http_method fields to the logger.
// Additional fields can be added by passing LogCtxMWConfig.
func SlogCtxMW(cfg LogCtxMWConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			attrs := []interface{}{
				slog.String("http_path", c.Path()),
				slog.String("http_method", req.Method),
			}
			if cfg.URI {
				attrs = append(attrs, slog.String("http_uri", req.RequestURI))
			}
			if cfg.Host {
				attrs = append(attrs, slog.String("http_host", req.Host))
			}
			if cfg.IP {
				attrs = append(attrs, slog.String("http_ip", c.RealIP()))
			}
			if cfg.UserAgent {
				attrs = append(attrs, slog.String("http_user_agent", req.UserAgent()))
			}
			if cfg.RequestID {
				attrs = append(attrs, slog.String("http_req_id", c.Response().Header().Get(echo.HeaderXRequestID)))
			}
			reqLogger := slog.Default().With(attrs...)
			ctx := req.Context()
			if cfg.Trace {
				reqLogger = slogbrick.WithOTELTrace(ctx, reqLogger)
			}
			c.SetRequest(req.WithContext(slogbrick.ToCtx(ctx, reqLogger)))
			return next(c)
		}
	}
}

// SlogLogMW is a middleware to provide logging for each request via slog.
func SlogLogMW(lvl slog.Level, skipper echomw.Skipper) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if skipper != nil && skipper(c) {
				return next(c)
			}
			req := c.Request()
			reqLogger := slogbrick.FromCtx(req.Context())

			reqLogger.Log(req.Context(), lvl, "incoming http req")

			start := time.Now().UTC()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			res := c.Response()

			reqLogger.Log(req.Context(), lvl, "outgoing http resp",
				slog.Int64("http_req_duration_ms", time.Since(start).Milliseconds()),
				slog.Int("http_resp_status", res.Status))
			return err
		}
	}
}

// RecoverSlogMW recovers from panics and logs via slog the stack trace.
// It returns a 500 status code.
func RecoverSlogMW() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if err := recover(); err != nil {
					c.Response().WriteHeader(http.StatusInternalServerError)
					slogbrick.FromCtx(c.Request().Context()).
						Error("http handler panicked",
							slog.Any("err", err),
							slog.String("stack", string(debug.Stack())))
				}
			}()
			return next(c)
		}
	}
}

type OTELMeterMWConfig struct {
	Attrs   *OTELMeterAttrsConfig
	Metrics *OTELMeterMetricsConfig
}

type OTELMeterAttrsConfig struct {
	Method       bool
	Route        bool
	Path         bool
	Status       bool
	AttrsFromCtx bool
	AttrsToCtx   bool
}

type OTELMeterMetricsConfig struct {
	ReqDuration       bool
	ReqCounter        bool
	ActiveReqsCounter bool
	ReqSize           bool
	RespSize          bool
}

type httpOTELMetrics struct {
	reqDurationHist   metric.Int64Histogram
	reqCounter        metric.Int64Counter
	reqSizeHist       metric.Int64Histogram
	respSizeHist      metric.Int64Histogram
	activeReqsCounter metric.Int64UpDownCounter
}

func newHTTPOTELMetrics(cfg *OTELMeterMetricsConfig) (*httpOTELMetrics, error) {
	httpMeter := otel.GetMeterProvider().Meter("brick/echobrick/OTELMeter")
	result := &httpOTELMetrics{}
	if cfg.ActiveReqsCounter {
		activeReqsCounter, err := httpMeter.Int64UpDownCounter("http.server.active_requests",
			metric.WithDescription("Number of active HTTP server requests."))
		if err != nil {
			return nil, fmt.Errorf("failed create http.server.active_requests metric: %w", err)
		}
		result.activeReqsCounter = activeReqsCounter
	}
	if cfg.ReqDuration {
		srvLatencyHist, err := httpMeter.Int64Histogram("http.server.request.duration",
			metric.WithDescription("Duration of HTTP server requests."),
			metric.WithUnit("ms"),
			metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10))
		if err != nil {
			return nil, fmt.Errorf("failed create http.server.request.duration metric: %w", err)
		}
		result.reqDurationHist = srvLatencyHist
	}
	if cfg.ReqCounter {
		srvReqCounter, err := httpMeter.Int64Counter("http.server.request.count",
			metric.WithDescription("The number of HTTP requests"))
		if err != nil {
			return nil, fmt.Errorf("failed create http.server.request.count metric: %w", err)
		}
		result.reqCounter = srvReqCounter
	}
	if cfg.ReqSize {
		srvReqSizeHist, err := httpMeter.Int64Histogram("http.server.request.body.size",
			metric.WithDescription("The size of the request payload body in bytes."), metric.WithUnit("By"))
		if err != nil {
			return nil, fmt.Errorf("failed create http.server.request.body.size metric: %w", err)
		}
		result.reqSizeHist = srvReqSizeHist
	}
	if cfg.RespSize {
		srvRespSizeHist, err := httpMeter.Int64Histogram("http.server.response.body.size",
			metric.WithDescription("Size of HTTP server response bodies."), metric.WithUnit("By"))
		if err != nil {
			return nil, fmt.Errorf("failed create http.server.response.body.size metric: %w", err)
		}
		result.respSizeHist = srvRespSizeHist
	}
	return result, nil
}

// OTELMeterMW is a middleware that records metrics for each request.
//
//nolint:gocognit,gocyclo,cyclop // it's ok to be cyclomatic for this func
func OTELMeterMW(cfg OTELMeterMWConfig) (echo.MiddlewareFunc, error) {
	if cfg.Attrs == nil {
		cfg.Attrs = &OTELMeterAttrsConfig{
			Method:     true,
			Route:      true,
			Status:     true,
			AttrsToCtx: true,
		}
	}
	if cfg.Metrics == nil {
		cfg.Metrics = &OTELMeterMetricsConfig{
			ReqDuration:       true,
			ReqCounter:        true,
			ReqSize:           true,
			RespSize:          true,
			ActiveReqsCounter: true,
		}
	}
	httpMeter, err := newHTTPOTELMetrics(cfg.Metrics)
	if err != nil {
		return nil, fmt.Errorf("failed create http metrics: %w", err)
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if cfg.Metrics.ActiveReqsCounter {
				httpMeter.activeReqsCounter.Add(c.Request().Context(), 1)
				defer httpMeter.activeReqsCounter.Add(c.Request().Context(), -1)
			}

			start := time.Now().UTC()
			var reqSz int64
			if cfg.Metrics.ReqSize {
				reqSz = httpbrick.ComputeApproximateRequestSize(c.Request())
			}

			var attrs []attribute.KeyValue
			if cfg.Attrs.Method {
				attrs = append(attrs, semconv.HTTPRequestMethodKey.String(c.Request().Method))
			}
			if cfg.Attrs.Route {
				attrs = append(attrs, semconv.HTTPRoute(c.Path()))
			}
			if cfg.Attrs.Path {
				attrs = append(attrs, semconv.URLPathKey.String(c.Request().RequestURI))
			}
			if cfg.Attrs.AttrsFromCtx {
				attrs = append(attrs, otelbrick.AttrsFromCtx(c.Request().Context())...)
			}

			if cfg.Attrs.AttrsToCtx {
				c.SetRequest(c.Request().WithContext(otelbrick.AttrsToCtx(c.Request().Context(), attrs)))
			}

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			if cfg.Attrs.Status {
				attrs = append(attrs, semconv.HTTPResponseStatusCode(c.Response().Status))
			}

			ctx := c.Request().Context()

			if cfg.Metrics.ReqCounter {
				httpMeter.reqCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
			}
			if cfg.Metrics.ReqDuration {
				httpMeter.reqDurationHist.Record(ctx, time.Since(start).Milliseconds(), metric.WithAttributes(attrs...))
			}
			if cfg.Metrics.ReqSize {
				httpMeter.reqSizeHist.Record(ctx, reqSz, metric.WithAttributes(attrs...))
			}
			if cfg.Metrics.RespSize {
				httpMeter.respSizeHist.Record(ctx, c.Response().Size, metric.WithAttributes(attrs...))
			}

			return err
		}
	}, nil
}

func TokenClaimsMW(jwksURL string, opts keyfunc.Options) echo.MiddlewareFunc {
	var jwtKeyFunc jwt.Keyfunc = func(token *jwt.Token) (interface{}, error) {
		return jwt.UnsafeAllowNoneSignatureType, nil
	}
	if jwksURL != "" {
		jwks, err := keyfunc.Get(jwksURL, opts)
		if err != nil {
			slog.Error("failed create JWKS from resource at the given URL",
				slog.Any("err", err), slog.String("jwks_url", jwksURL))
		} else {
			jwtKeyFunc = jwks.Keyfunc
		}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			jwtToken, err := retrieveJWT(c.Request())
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, err)
			}
			claims := jwt.MapClaims{}
			tkn, err := jwt.ParseWithClaims(jwtToken, &claims, jwtKeyFunc)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "failed parse token")
			}
			if !tkn.Valid {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}
			c.SetRequest(c.Request().WithContext(tokenClaimsToCtx(c.Request().Context(), claims)))
			return next(c)
		}
	}
}

type jwtTokenClaimsKey struct{}

var tknClaimsKey = jwtTokenClaimsKey{}

func tokenClaimsToCtx(ctx context.Context, claims jwt.MapClaims) context.Context {
	return context.WithValue(ctx, tknClaimsKey, claims)
}

func TokenClaimsFromCtx(ctx context.Context) jwt.MapClaims {
	claims, ok := ctx.Value(tknClaimsKey).(jwt.MapClaims)
	if !ok {
		return jwt.MapClaims{}
	}
	return claims
}

// retrieveJWT returns the token string from the request.
func retrieveJWT(request *http.Request) (string, error) {
	header := request.Header.Get("Authorization")
	if header == "" {
		return "", errors.New("authorization header is empty")
	}
	h := strings.Split(header, " ")
	if len(h) != 2 || !strings.EqualFold(h[0], "bearer") {
		return "", errors.New("invalid authorization header format")
	}
	return h[1], nil
}
