package httpbrick

import (
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/trace"

	"github.com/demeero/bricks/slogbrick"
)

const (
	otelSpanIDLogKey  = "otel.span_id"
	otelTraceIDLogKey = "otel.trace_id"
	uriLogKey         = "http.uri"
	hostLogKey        = "http.host"
	peerLogKey        = "http.peer"
	userAgentLogKey   = "http.user_agent"
	methodLogKey      = "http.method"
)

var defaultLogCtxMWKeys = LogCtxMWKeys{
	OTelSpanID:  otelSpanIDLogKey,
	OTelTraceID: otelTraceIDLogKey,
	URI:         uriLogKey,
	Host:        hostLogKey,
	Peer:        peerLogKey,
	UserAgent:   userAgentLogKey,
	Method:      methodLogKey,
}

// LogCtxMWKeys is a set of keys for request attributes in logger.
type LogCtxMWKeys struct {
	OTelSpanID  string
	OTelTraceID string
	URI         string
	Host        string
	Peer        string
	UserAgent   string
	Method      string
}

type logCtxMWOpts struct {
	Keys        LogCtxMWKeys
	attrsSize   uint8
	Method      bool
	OTelSpanCtx bool
	URI         bool
	Host        bool
	Peer        bool
	UserAgent   bool
}

type LogCtxMWOption func(*logCtxMWOpts)

// SlogCtxMW is a middleware that adds slog logger to request context.
// It also adds some request attributes to the logger.
// Use LogCtxMWOption to configure the middleware.
// The middleware uses slogbrick.FromCtx to get logger from context, so it can extend an existing logger.
// The middleware uses slogbrick.ToCtx to put logger to context, so it can be used in underlying handlers.
func SlogCtxMW(options ...LogCtxMWOption) func(http.Handler) http.Handler {
	opts := logCtxMWOpts{
		Keys:      defaultLogCtxMWKeys,
		Method:    true,
		attrsSize: uint8(1),
	}
	for _, opt := range options {
		opt(&opts)
	}
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			attrs := logAttrsFromReq(req, opts)
			reqLogger := slogbrick.FromCtx(ctx).With(attrs...)
			ctx = slogbrick.ToCtx(ctx, reqLogger)
			h.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

func logAttrsFromReq(req *http.Request, opts logCtxMWOpts) []interface{} {
	attrs := make([]interface{}, 0, opts.attrsSize)

	if opts.Method {
		attrs = append(attrs, slog.String(opts.Keys.Method, req.Method))
	}

	if opts.Host {
		attrs = append(attrs, slog.String(opts.Keys.Host, req.Host))
	}

	if opts.Peer {
		attrs = append(attrs, slog.String(opts.Keys.Peer, req.RemoteAddr))
	}

	if opts.UserAgent {
		attrs = append(attrs, slog.String(opts.Keys.UserAgent, req.UserAgent()))
	}

	if opts.URI {
		reqURI := req.RequestURI
		if reqURI == "" {
			reqURI = req.URL.RequestURI()
		}
		attrs = append(attrs, slog.String(opts.Keys.URI, reqURI))
	}

	ctx := req.Context()

	if opts.OTelSpanCtx {
		spanCtx := trace.SpanContextFromContext(ctx)
		if spanCtx.HasSpanID() {
			attrs = append(attrs, slog.String(opts.Keys.OTelSpanID, spanCtx.SpanID().String()))
		}
		if spanCtx.HasTraceID() {
			attrs = append(attrs, slog.String(opts.Keys.OTelTraceID, spanCtx.TraceID().String()))
		}
	}

	return attrs
}

// WithOTelSpanCtxLogAttr adds OTEL span_id and trace_id attributes to logger.
func WithOTelSpanCtxLogAttr() LogCtxMWOption {
	return func(opts *logCtxMWOpts) {
		opts.OTelSpanCtx = true
		opts.attrsSize += 2
	}
}

// WithURILogAttr adds URI attribute to logger.
func WithURILogAttr() LogCtxMWOption {
	return func(opts *logCtxMWOpts) {
		opts.URI = true
		opts.attrsSize++
	}
}

// WithHostLogAttr adds host attribute to logger.
func WithHostLogAttr() LogCtxMWOption {
	return func(opts *logCtxMWOpts) {
		opts.Host = true
		opts.attrsSize++
	}
}

// WithPeerLogAttr adds peer attribute to logger.
func WithPeerLogAttr() LogCtxMWOption {
	return func(opts *logCtxMWOpts) {
		opts.Peer = true
		opts.attrsSize++
	}
}

// WithUserAgentLogAttr adds user agent attribute to logger.
func WithUserAgentLogAttr() LogCtxMWOption {
	return func(opts *logCtxMWOpts) {
		opts.UserAgent = true
		opts.attrsSize++
	}
}

// WithLogCtxMWKeys allows to configure keys for request attributes.
// Use this option if you want to use different keys for request attributes in your logger.
func WithLogCtxMWKeys(keys LogCtxMWKeys) LogCtxMWOption {
	return func(opts *logCtxMWOpts) {
		if keys.OTelSpanID != "" {
			opts.Keys.OTelSpanID = keys.OTelSpanID
		}
		if keys.OTelTraceID != "" {
			opts.Keys.OTelTraceID = keys.OTelTraceID
		}
		if keys.URI != "" {
			opts.Keys.URI = keys.URI
		}
		if keys.Host != "" {
			opts.Keys.Host = keys.Host
		}
		if keys.Peer != "" {
			opts.Keys.Peer = keys.Peer
		}
		if keys.UserAgent != "" {
			opts.Keys.UserAgent = keys.UserAgent
		}
		if keys.Method != "" {
			opts.Keys.Method = keys.Method
		}
	}
}
