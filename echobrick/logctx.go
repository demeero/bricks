package echobrick

import (
	"log/slog"

	"github.com/labstack/echo/v4"

	"github.com/demeero/bricks/httpbrick"
	"github.com/demeero/bricks/slogbrick"
)

const (
	clientIPLogKey = "http.client_ip"
	routeLogKey    = "http.route"
)

// LogCtxMWKeys is a set of keys for request attributes in logger.
type LogCtxMWKeys struct {
	ClientIP string
	Route    string
}

var defaultLogCtxMWKeys = LogCtxMWKeys{
	ClientIP: clientIPLogKey,
	Route:    routeLogKey,
}

// LogCtxMWOption is a function that configures log context middleware.
type LogCtxMWOption func(*logCtxMWOpts)

type logCtxMWOpts struct {
	attrsSize uint8
	Opts      []httpbrick.LogCtxMWOption
	Route     bool
	IP        bool
	Keys      LogCtxMWKeys
}

// WithIPLogAttr allows to add client IP to request attributes in logger.
func WithIPLogAttr() LogCtxMWOption {
	return func(opts *logCtxMWOpts) {
		opts.IP = true
		opts.attrsSize++
	}
}

// WithHTTPBrickOpts allows to configure httpbrick options for log context middleware.
func WithHTTPBrickOpts(opts ...httpbrick.LogCtxMWOption) LogCtxMWOption {
	return func(o *logCtxMWOpts) {
		o.Opts = append(o.Opts, opts...)
	}
}

// WithLogCtxMWKeys allows to configure keys for request attributes.
// Use this option if you want to use different keys for request attributes in your logger.
func WithLogCtxMWKeys(keys LogCtxMWKeys) LogCtxMWOption {
	return func(opts *logCtxMWOpts) {
		if keys.ClientIP != "" {
			opts.Keys.ClientIP = keys.ClientIP
		}
		if keys.Route != "" {
			opts.Keys.Route = keys.Route
		}
	}
}

// SlogCtxMW is an extension of httpbrick.SlogCtxMW middleware.
// It adds route to request attributes in logger.
// Also, it can add other stuff to logger attributes that can be easily obtained from echo.Context.
func SlogCtxMW(options ...LogCtxMWOption) echo.MiddlewareFunc {
	opts := logCtxMWOpts{
		Route:     true,
		attrsSize: uint8(1),
	}
	for _, opt := range options {
		opt(&opts)
	}
	mw := echo.WrapMiddleware(httpbrick.SlogCtxMW(opts.Opts...))
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			attrs := make([]interface{}, 0, opts.attrsSize)
			if opts.Route {
				attrs = append(attrs, slog.String(routeLogKey, c.Path()))
			}
			if opts.IP {
				attrs = append(attrs, slog.String(clientIPLogKey, c.RealIP()))
			}
			ctx := c.Request().Context()
			reqLogger := slogbrick.FromCtx(ctx).With(attrs...)
			ctx = slogbrick.ToCtx(ctx, reqLogger)
			c.SetRequest(c.Request().WithContext(ctx))
			return mw(next)(c)
		}
	}
}
