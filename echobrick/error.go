package echobrick

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/demeero/bricks/errbrick"
	"github.com/demeero/bricks/slogbrick"
)

// FallbackFunc is a function that returns an error response for an unknown error.
// If the function returns nil, the error will be handled as an internal server error.
type FallbackFunc func(err error) *echo.HTTPError

// ErrorHandler returns an error handler for echo.
// If fallback is not nil, it will be called if the error is not recognized.
// If fallback returns nil, the error will be handled as an internal server error.
func ErrorHandler(fallback FallbackFunc) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		var (
			lg      = slogbrick.FromCtx(c.Request().Context())
			echoErr *echo.HTTPError
		)
		switch {
		case errors.As(err, &echoErr):
			handleEchoErr(echoErr, lg)
		case errors.Is(err, errbrick.ErrInvalidData):
			echoErr = echo.NewHTTPError(http.StatusBadRequest, err.Error())
		case errors.Is(err, errbrick.ErrNotFound):
			echoErr = echo.NewHTTPError(http.StatusNotFound, err.Error())
		case errors.Is(err, errbrick.ErrForbidden):
			echoErr = echo.NewHTTPError(http.StatusForbidden, err.Error())
		case errors.Is(err, errbrick.ErrConflict):
			echoErr = echo.NewHTTPError(http.StatusConflict, err.Error())
		case errors.Is(err, errbrick.ErrUnauthenticated):
			echoErr = echo.NewHTTPError(http.StatusUnauthorized, err.Error())
		default:
			if fallback != nil {
				if fallbackErr := fallback(err); fallbackErr != nil {
					echoErr = fallbackErr
					break
				}
			}
			lg.Error("internal server err", slog.Any("err", err))
			echoErr = echo.NewHTTPError(http.StatusInternalServerError)
		}
		if err = c.JSON(echoErr.Code, echoErr); err != nil {
			lg.Error("failed send err resp", slog.Any("err", err))
		}
	}
}

// GRPCErrFallback maps grpc errors to echo http errors.
func GRPCErrFallback(err error) *echo.HTTPError {
	grpcStatus := status.Convert(err)
	switch grpcStatus.Code() {
	case codes.InvalidArgument:
		return echo.NewHTTPError(http.StatusBadRequest, grpcStatus.Message())
	case codes.NotFound:
		return echo.NewHTTPError(http.StatusNotFound, grpcStatus.Message())
	case codes.PermissionDenied:
		return echo.NewHTTPError(http.StatusForbidden, grpcStatus.Message())
	case codes.AlreadyExists:
		return echo.NewHTTPError(http.StatusConflict, grpcStatus.Message())
	case codes.Unauthenticated:
		return echo.NewHTTPError(http.StatusUnauthorized, grpcStatus.Message())
	}
	return nil
}

func handleEchoErr(echoErr *echo.HTTPError, lg *slog.Logger) {
	if echoErr.Internal != nil {
		lg.Error("failed handle req", slog.Any("err", echoErr.Internal))
	}
	if msg, ok := echoErr.Message.(string); ok && msg == "" {
		echoErr.Message = http.StatusText(echoErr.Code)
	}
}
