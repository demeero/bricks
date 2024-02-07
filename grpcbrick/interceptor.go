package grpcbrick

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/demeero/bricks/errbrick"
	"github.com/demeero/bricks/slogbrick"
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

func ErrUnaryServerInterceptor(skipper Skipper) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if skipper != nil && skipper(ctx, req, info) {
			return handler(ctx, req)
		}
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		// if it's a valid GRPC error - skip the handling
		_, ok := status.FromError(err)
		if ok {
			return resp, err
		}

		switch {
		case errors.Is(err, errbrick.ErrInvalidData):
			return resp, status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, errbrick.ErrNotFound):
			return resp, status.Error(codes.NotFound, err.Error())
		case errors.Is(err, errbrick.ErrForbidden):
			return nil, status.Error(codes.PermissionDenied, err.Error())
		case errors.Is(err, errbrick.ErrConflict):
			return nil, status.Error(codes.AlreadyExists, err.Error())
		case errors.Is(err, errbrick.ErrUnauthenticated):
			return nil, status.Error(codes.Unauthenticated, err.Error())
		default:
			slogbrick.FromCtx(ctx).Error("internal err", slog.Any("err", err))
			return resp, status.Error(codes.Internal, "internal error")
		}
	}
}
