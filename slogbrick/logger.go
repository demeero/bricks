package slogbrick

import (
	"context"
	"log/slog"
	"os"

	"github.com/demeero/bricks/configbrick"
	"go.opentelemetry.io/otel/trace"
)

type logCtxKey struct{}

var logKey = logCtxKey{}

// Configure configures slog logger.
func Configure(cfg configbrick.Log, fields map[string]string) {
	opts := &slog.HandlerOptions{
		Level:     ParseLevel(cfg.Level, slog.LevelInfo),
		AddSource: cfg.AddSource,
	}
	var h slog.Handler
	if cfg.JSON {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	logger := slog.New(h)
	if len(fields) > 0 {
		for k, v := range fields {
			logger = logger.With(slog.String(k, v))
		}
	}
	slog.SetDefault(logger)
	slog.Info("log configured")
}

func ParseLevel(level string, fallback slog.Level) slog.Level {
	logLvl := &slog.LevelVar{}
	if err := logLvl.UnmarshalText([]byte(level)); err != nil {
		slog.Error("failed parse log level - use fallback",
			slog.Any("err", err), slog.String("level", level), slog.String("fallback", level))
		logLvl.Set(fallback)
	}
	return logLvl.Level()
}

// FromCtx returns slog logger from context.
func FromCtx(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(logKey).(*slog.Logger)
	if !ok {
		slog.Debug("no slog instance in context - using default")
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
