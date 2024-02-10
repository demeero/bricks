package httpbrick

import (
	"log/slog"
	"net/http"

	"github.com/felixge/httpsnoop"

	"github.com/demeero/bricks/slogbrick"
)

type accessLogMWOpts struct {
	Snoop   bool
	InMsg   string
	OutMsg  string
	Skipper Skipper
	Keys    AccessLogMWKeys
}

// AccessLogMWKeys is a set of keys for access log middleware. Use it to configure the default keys.
type AccessLogMWKeys struct {
	Duration string
	Code     string
	Written  string
}

// AccessLogMWOption is a function that configures access log middleware.
type AccessLogMWOption func(*accessLogMWOpts)

// SlogAccessLogMW is a middleware to provide logging for each request via slog.
// Use AccessLogMWOption to configure the middleware.
// The middleware extracts a logger instance from request's context and it's expected that the logger is already configured via SlogCtxMW middleware
// or somehow else and fields that expected to be logged (like traceID, path, etc.) are already set.
func SlogAccessLogMW(enabled bool, lvl slog.Level, options ...AccessLogMWOption) func(http.Handler) http.Handler {
	if !enabled {
		return NoopMW
	}
	opts := accessLogMWOpts{
		InMsg:  "incoming http req",
		OutMsg: "outgoing http resp",
		Keys: AccessLogMWKeys{
			Duration: "http.duration_ms",
			Code:     "http.code",
			Written:  "http.written",
		},
		Snoop: true,
	}
	for _, opt := range options {
		opt(&opts)
	}
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if opts.Skipper != nil && opts.Skipper(req) {
				h.ServeHTTP(w, req)
				return
			}
			reqLogger := slogbrick.FromCtx(req.Context())

			reqLogger.Log(req.Context(), lvl, opts.InMsg)

			if opts.Snoop {
				m := httpsnoop.CaptureMetrics(h, w, req)
				reqLogger.Log(req.Context(), lvl, opts.OutMsg,
					slog.Int64(opts.Keys.Duration, m.Duration.Milliseconds()),
					slog.Int(opts.Keys.Code, m.Code),
					slog.Int64(opts.Keys.Written, m.Written))
				return
			}

			h.ServeHTTP(w, req)
			reqLogger.Log(req.Context(), lvl, opts.OutMsg)
		})
	}
}

// WithAccessLogMWKeys configures the default logger keys.
func WithAccessLogMWKeys(keys AccessLogMWKeys) AccessLogMWOption {
	return func(opts *accessLogMWOpts) {
		if keys.Duration != "" {
			opts.Keys.Duration = keys.Duration
		}
		if keys.Code != "" {
			opts.Keys.Code = keys.Code
		}
		if keys.Written != "" {
			opts.Keys.Written = keys.Written
		}
	}
}

// WithAccessLogMWInMsg configures the incoming request message.
func WithAccessLogMWInMsg(msg string) AccessLogMWOption {
	return func(opts *accessLogMWOpts) {
		opts.InMsg = msg
	}
}

// WithAccessLogMWOutMsg configures the outgoing response message.
func WithAccessLogMWOutMsg(msg string) AccessLogMWOption {
	return func(opts *accessLogMWOpts) {
		opts.OutMsg = msg
	}
}

// WithAccessLogMWSkipper configures the skipper.
func WithAccessLogMWSkipper(skipper Skipper) AccessLogMWOption {
	return func(opts *accessLogMWOpts) {
		opts.Skipper = skipper
	}
}

// WithHTTPSnoopAccessLog configures the snoop mode,
// so the middleware will use httpsnoop.CaptureMetrics to log request duration, response code and written bytes.
func WithHTTPSnoopAccessLog(v bool) AccessLogMWOption {
	return func(opts *accessLogMWOpts) {
		opts.Snoop = v
	}
}
