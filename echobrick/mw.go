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
	Path         bool
	URI          bool
	Status       bool
	AttrsFromCtx bool
	AttrsToCtx   bool
}

type OTELMeterMetricsConfig struct {
	LatencyHist  bool
	ReqCounter   bool
	ReqSizeHist  bool
	RespSizeHist bool
}

type httpOTELMetrics struct {
	srvLatencyHist  metric.Int64Histogram
	srvReqCounter   metric.Int64Counter
	srvReqSizeHist  metric.Int64Histogram
	srvRespSizeHist metric.Int64Histogram
}

func newHTTPOTELMetrics(cfg *OTELMeterMetricsConfig) (*httpOTELMetrics, error) {
	httpMeter := otel.GetMeterProvider().Meter("brick/echobrick/OTELMeter")
	result := &httpOTELMetrics{}
	if cfg.LatencyHist {
		srvLatencyHist, err := httpMeter.Int64Histogram("http_server_latency",
			metric.WithDescription("The latency of HTTP requests"), metric.WithUnit("ms"))
		if err != nil {
			return nil, fmt.Errorf("failed create http_server_latency metric: %w", err)
		}
		result.srvLatencyHist = srvLatencyHist
	}
	if cfg.ReqCounter {
		srvReqCounter, err := httpMeter.Int64Counter("http_server_request_count",
			metric.WithDescription("The number of HTTP requests"))
		if err != nil {
			return nil, fmt.Errorf("failed create http_server_request_count metric: %w", err)
		}
		result.srvReqCounter = srvReqCounter
	}
	if cfg.ReqSizeHist {
		srvReqSizeHist, err := httpMeter.Int64Histogram("http_server_request_size",
			metric.WithDescription("The size of HTTP requests"), metric.WithUnit("B"))
		if err != nil {
			return nil, fmt.Errorf("failed create http_server_request_size metric: %w", err)
		}
		result.srvReqSizeHist = srvReqSizeHist
	}
	if cfg.RespSizeHist {
		srvRespSizeHist, err := httpMeter.Int64Histogram("http_server_req_size",
			metric.WithDescription("The size of HTTP responses"), metric.WithUnit("B"))
		if err != nil {
			return nil, fmt.Errorf("failed create http_server_response_size metric: %w", err)
		}
		result.srvRespSizeHist = srvRespSizeHist
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
			Path:       true,
			Status:     true,
			AttrsToCtx: true,
		}
	}
	if cfg.Metrics == nil {
		cfg.Metrics = &OTELMeterMetricsConfig{
			LatencyHist:  true,
			ReqCounter:   true,
			ReqSizeHist:  true,
			RespSizeHist: true,
		}
	}
	httpMeter, err := newHTTPOTELMetrics(cfg.Metrics)
	if err != nil {
		return nil, fmt.Errorf("failed create http metrics: %w", err)
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now().UTC()

			var reqSz int64
			if cfg.Metrics.ReqSizeHist {
				reqSz = httpbrick.ComputeApproximateRequestSize(c.Request())
			}

			var attrs []attribute.KeyValue
			if cfg.Attrs.Method {
				attrs = append(attrs, attribute.String("method", c.Request().Method))
			}
			if cfg.Attrs.Path {
				attrs = append(attrs, attribute.String("path", c.Path()))
			}
			if cfg.Attrs.URI {
				attrs = append(attrs, attribute.String("uri", c.Request().RequestURI))
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
				attrs = append(attrs, attribute.Int("status", c.Response().Status))
			}

			ctx := c.Request().Context()

			if cfg.Metrics.ReqCounter {
				httpMeter.srvReqCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
			}
			if cfg.Metrics.LatencyHist {
				httpMeter.srvLatencyHist.Record(ctx, time.Since(start).Milliseconds(), metric.WithAttributes(attrs...))
			}
			if cfg.Metrics.ReqSizeHist {
				httpMeter.srvReqSizeHist.Record(ctx, reqSz, metric.WithAttributes(attrs...))
			}
			if cfg.Metrics.RespSizeHist {
				httpMeter.srvRespSizeHist.Record(ctx, c.Response().Size, metric.WithAttributes(attrs...))
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
