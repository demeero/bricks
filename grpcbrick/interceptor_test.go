package grpcbrick

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/demeero/bricks/errbrick"
)

func TestErrUnaryServerInterceptor(t *testing.T) {
	var tests = []struct {
		name    string
		skipper Skipper
		err     error
		wantErr error
	}{
		{
			name:    "with-skipper",
			skipper: func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo) bool { return true },
			err:     errors.New("test"),
			wantErr: errors.New("test"),
		},
		{
			name:    "unknown-error",
			err:     errors.New("test"),
			wantErr: status.Error(codes.Internal, "internal error"),
		},
		{
			name:    "ErrInvalidData",
			err:     errbrick.ErrInvalidData,
			wantErr: status.Error(codes.InvalidArgument, errbrick.ErrInvalidData.Error()),
		},
		{
			name:    "ErrNotFound",
			err:     errbrick.ErrNotFound,
			wantErr: status.Error(codes.NotFound, errbrick.ErrNotFound.Error()),
		},
		{
			name:    "ErrForbidden",
			err:     errbrick.ErrForbidden,
			wantErr: status.Error(codes.PermissionDenied, errbrick.ErrForbidden.Error()),
		},
		{
			name:    "ErrConflict",
			err:     errbrick.ErrConflict,
			wantErr: status.Error(codes.AlreadyExists, errbrick.ErrConflict.Error()),
		},
		{
			name:    "ErrUnauthenticated",
			err:     errbrick.ErrUnauthenticated,
			wantErr: status.Error(codes.Unauthenticated, errbrick.ErrUnauthenticated.Error()),
		},
		{
			name: "no-error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intercept := ErrUnaryServerInterceptor(tt.skipper)
			_, err := intercept(context.Background(), nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, tt.err
			})
			assert.Equal(t, err, tt.wantErr)
		})
	}
}
