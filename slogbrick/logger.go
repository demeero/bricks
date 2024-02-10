package slogbrick

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/demeero/bricks/configbrick"

	"github.com/lmittmann/tint"
)

type loggerOpts struct {
	W     io.Writer
	Attrs []slog.Attr
}

type LoggerOpt func(*loggerOpts)

// WithWriter allows to configure slog logger with custom writer.
// By default, slog logger writes to os.Stdout.
func WithWriter(w io.Writer) LoggerOpt {
	return func(o *loggerOpts) {
		o.W = w
	}
}

// WithAttrs allows to configure slog logger with custom attributes.
func WithAttrs(attrs ...slog.Attr) LoggerOpt {
	return func(o *loggerOpts) {
		o.Attrs = attrs
	}
}

type logCtxKey struct{}

var logKey = logCtxKey{}

// Configure configures slog logger.
func Configure(cfg configbrick.Log, options ...LoggerOpt) {
	level := ParseLevel(cfg.Level, slog.LevelInfo)
	handlerOpts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	opts := loggerOpts{
		W: os.Stdout,
	}
	for _, opt := range options {
		opt(&opts)
	}

	var h slog.Handler
	switch {
	case cfg.JSON:
		h = slog.NewJSONHandler(opts.W, handlerOpts)
	case cfg.Pretty:
		h = tint.NewHandler(opts.W, &tint.Options{
			Level:      level,
			AddSource:  cfg.AddSource,
			TimeFormat: time.Kitchen,
		})
	default:
		h = slog.NewTextHandler(opts.W, handlerOpts)
	}

	logger := slog.New(h.WithAttrs(opts.Attrs))

	slog.SetDefault(logger)
	slog.Info("log configured")
}

func ParseLevel(level string, fallback slog.Level) slog.Level {
	logLvl := &slog.LevelVar{}
	if err := logLvl.UnmarshalText([]byte(level)); err != nil {
		slog.Error("failed parse log level - use fallback",
			slog.Any("err", err), slog.String("level", level), slog.String("fallback", fallback.String()))
		logLvl.Set(fallback)
	}
	return logLvl.Level()
}

// FromCtx returns slog logger from context.
func FromCtx(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(logKey).(*slog.Logger)
	if !ok {
		// no slog instance in context - using default
		return slog.Default()
	}
	return logger
}

// ToCtx adds slog logger to context.
func ToCtx(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, logKey, logger)
}

// WithOTELTrace adds OTEL trace info to slog logger.
// This is useful when you want to add trace info to log output.
// ctx has to be a context with OTEL trace info.
func WithOTELTrace(ctx context.Context, logger *slog.Logger) *slog.Logger {
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		logger = logger.With(slog.String("otel.span_id", spanCtx.SpanID().String()), slog.String("otel.trace_id", spanCtx.TraceID().String()))
	}
	return logger
}
