package echobrick

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/demeero/bricks/errbrick"
)

func TestErrorHandler(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		fallback     FallbackFunc
		expectedBody string
		expectedCode int
	}{
		{
			name:         "HandleEchoError",
			err:          echo.NewHTTPError(http.StatusTeapot, "I'm a teapot"),
			expectedCode: http.StatusTeapot,
			expectedBody: `{"message":"I'm a teapot"}` + "\n",
		},
		{
			name:         "HandleInvalidDataError",
			err:          fmt.Errorf("%w: incorrect field", errbrick.ErrInvalidData),
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"message":"invalid data: incorrect field"}` + "\n",
		},
		{
			name:         "HandleNotFoundError",
			err:          errbrick.ErrNotFound,
			expectedCode: http.StatusNotFound,
			expectedBody: `{"message":"not found"}` + "\n",
		},
		{
			name:         "HandleForbiddenError",
			err:          errbrick.ErrForbidden,
			expectedCode: http.StatusForbidden,
			expectedBody: `{"message":"forbidden"}` + "\n",
		},
		{
			name:         "HandleConflictError",
			err:          errbrick.ErrConflict,
			expectedCode: http.StatusConflict,
			expectedBody: `{"message":"conflict"}` + "\n",
		},
		{
			name:         "HandleUnauthenticatedError",
			err:          errbrick.ErrUnauthenticated,
			expectedCode: http.StatusUnauthorized,
			expectedBody: `{"message":"unauthenticated"}` + "\n",
		},
		{
			name:         "HandleUnknownError",
			err:          assert.AnError,
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"message":"Internal Server Error"}` + "\n",
		},
		{
			name:         "HandleFallbackError",
			err:          assert.AnError,
			fallback:     func(err error) *echo.HTTPError { return echo.NewHTTPError(http.StatusTeapot, "I'm a teapot") },
			expectedCode: http.StatusTeapot,
			expectedBody: `{"message":"I'm a teapot"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := echo.New().NewContext(req, rec)
			ErrorHandler(tt.fallback)(tt.err, c)
			assert.Equal(t, tt.expectedCode, rec.Code)
			assert.Equal(t, tt.expectedBody, rec.Body.String())
		})
	}
}

func TestGRPCErrFallback(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected *echo.HTTPError
	}{
		{
			name:     "HandleInvalidArgumentError",
			err:      status.Error(codes.InvalidArgument, "invalid argument"),
			expected: echo.NewHTTPError(http.StatusBadRequest, "invalid argument"),
		},
		{
			name:     "HandleNotFoundError",
			err:      status.Error(codes.NotFound, "not found"),
			expected: echo.NewHTTPError(http.StatusNotFound, "not found"),
		},
		{
			name:     "HandlePermissionDeniedError",
			err:      status.Error(codes.PermissionDenied, "permission denied"),
			expected: echo.NewHTTPError(http.StatusForbidden, "permission denied"),
		},
		{
			name:     "HandleAlreadyExistsError",
			err:      status.Error(codes.AlreadyExists, "already exists"),
			expected: echo.NewHTTPError(http.StatusConflict, "already exists"),
		},
		{
			name:     "HandleUnauthenticatedError",
			err:      status.Error(codes.Unauthenticated, "unauthenticated"),
			expected: echo.NewHTTPError(http.StatusUnauthorized, "unauthenticated"),
		},
		{
			name:     "HandleUnknownError",
			err:      status.Error(codes.Unknown, "unknown error"),
			expected: nil,
		},
		{
			name:     "HandleNilError",
			err:      nil,
			expected: nil,
		},
		{
			name:     "NonGRPCError",
			err:      assert.AnError,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GRPCErrFallback(tt.err))
		})
	}
}
