package echobrick

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/demeero/bricks/errbrick"
	"github.com/demeero/bricks/slogbrick"
)

func ErrorHandler(err error, c echo.Context) {
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
		lg.Error("internal server err", slog.Any("err", err))
		echoErr = echo.NewHTTPError(http.StatusInternalServerError)
	}
	if err = c.JSON(echoErr.Code, echoErr); err != nil {
		lg.Error("failed send err resp", slog.Any("err", err))
	}
}

func handleEchoErr(echoErr *echo.HTTPError, lg *slog.Logger) {
	if echoErr.Internal != nil {
		lg.Error("failed handle req", slog.Any("err", echoErr.Internal))
	}
	if msg, ok := echoErr.Message.(string); ok && msg == "" {
		echoErr.Message = http.StatusText(echoErr.Code)
	}
}
