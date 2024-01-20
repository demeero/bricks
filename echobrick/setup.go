package echobrick

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/demeero/bricks/configbrick"
	"github.com/demeero/bricks/httpbrick"
	"github.com/demeero/bricks/slogbrick"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	echolog "github.com/labstack/gommon/log"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

type EchoConfig struct {
	configbrick.HTTP
	// ServiceName is used otelecho.Middleware
	ServiceName string
}

func Setup(cfg EchoConfig, globalSkipper httpbrick.Skipper, hook func(e *echo.Echo)) func(ctx context.Context) {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Logger.SetLevel(echolog.OFF)
	e.Server.ReadTimeout = cfg.ReadTimeout
	e.Server.ReadHeaderTimeout = cfg.ReadHeaderTimeout
	e.Server.WriteTimeout = cfg.WriteTimeout
	e.HTTPErrorHandler = ErrorHandler

	middlewares(cfg, globalSkipper, e)
	hook(e)
	go func() {
		slog.Info("initializing HTTP server")
		err := e.Start(fmt.Sprintf(":%d", cfg.Port))
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("failed HTTP serve: %s", err)
		}
	}()
	for _, r := range e.Routes() {
		if r != nil {
			slog.Info("HTTP registered routes",
				slog.String("method", r.Method), slog.String("path", r.Path), slog.String("name", r.Name))
		}
	}
	return func(ctx context.Context) {
		slog.Info("HTTP server shutdown started")
		if err := e.Shutdown(ctx); err != nil {
			slog.Error("HTTP server shutdown failed", slog.Any("err", err))
			return
		}
		slog.Info("HTTP server shutdown finished")
	}
}

func middlewares(cfg EchoConfig, globalSkipper httpbrick.Skipper, e *echo.Echo) {
	meterMW, err := httpbrick.OTelMeterMW()
	if err != nil {
		log.Fatalf("failed create meter middleware: %s", err)
	}
	echoSkipper := func(c echo.Context) bool {
		return globalSkipper(c.Request())
	}
	e.Pre(echomw.RemoveTrailingSlash())
	e.Use(echo.WrapMiddleware(httpbrick.RecoverSlogMW))
	e.Use(RouteCtxMW())
	e.Use(otelecho.Middleware(cfg.ServiceName, otelecho.WithSkipper(echoSkipper)))
	e.Use(echo.WrapMiddleware(meterMW))
	e.Use(echo.WrapMiddleware(httpbrick.SlogCtxMW()))
	e.Use(echo.WrapMiddleware(httpbrick.SlogAccessLogMW(cfg.AccessLog,
		slogbrick.ParseLevel(cfg.AccessLogLevel, slog.LevelDebug),
		httpbrick.WithAccessLogMWSkipper(globalSkipper))))
}
