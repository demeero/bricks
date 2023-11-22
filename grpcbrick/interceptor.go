package grpcbrick

import (
	"context"
	"log/slog"
	"time"

	"github.com/demeero/bricks/slogbrick"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type Skipper func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo) bool

func SlogCtxUnaryServerInterceptor(trace bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		reqLogger := slog.Default().With(slog.String("grpc_method", info.FullMethod))
		if trace {
			reqLogger = slogbrick.WithOTELTrace(ctx, reqLogger)
		}
		return handler(slogbrick.ToCtx(ctx, reqLogger), req)
	}
}

func SlogUnaryServerInterceptor(lvl slog.Level, skipper Skipper) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if skipper != nil && skipper(ctx, req, info) {
			return handler(ctx, req)
		}
		reqLogger := slogbrick.FromCtx(ctx)
		reqLogger.Log(ctx, lvl, "incoming grpc req")

		startTime := time.Now().UTC()
		resp, err := handler(ctx, req)
		if err != nil {
			reqLogger = reqLogger.With(slog.Any("err", err))
		}

		reqLogger.Log(ctx, lvl, "outgoing grpc resp",
			slog.Int64("grpc_req_duration_ms", time.Since(startTime).Milliseconds()),
			slog.String("grpc_resp_code", status.Code(err).String()))
		return resp, err
	}
}
