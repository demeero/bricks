package echobrick

import (
	"context"

	"github.com/demeero/bricks/otelbrick"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type routeCtxKey struct{}

var routeCtxKeyInstance = routeCtxKey{}

// RouteCtxMW is a middleware that adds the current route to the context.
// It also adds the current route as metric attributes that can be fetched by otelbrick.AttrsFromCtx.
func RouteCtxMW() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			route := c.Path()
			ctx = context.WithValue(ctx, routeCtxKeyInstance, route)
			ctx = otelbrick.AttrsToCtx(ctx, []attribute.KeyValue{semconv.HTTPRoute(route)})
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// RouteFromCtx returns the current route from the context.
func RouteFromCtx(ctx context.Context) string {
	route, ok := ctx.Value(routeCtxKeyInstance).(string)
	if !ok {
		return ""
	}
	return route
}
