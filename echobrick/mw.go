package echobrick

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

func TokenClaimsMW(jwksURL string, opts keyfunc.Options) echo.MiddlewareFunc {
	var jwtKeyFunc jwt.Keyfunc = func(token *jwt.Token) (interface{}, error) {
		return jwt.UnsafeAllowNoneSignatureType, nil
	}
	if jwksURL != "" {
		jwks, err := keyfunc.Get(jwksURL, opts)
		if err != nil {
			slog.Error("failed create JWKS from resource at the given URL",
				slog.Any("err", err), slog.String("jwks_url", jwksURL))
		} else {
			jwtKeyFunc = jwks.Keyfunc
		}
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			jwtToken, err := retrieveJWT(c.Request())
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, err)
			}
			claims := jwt.MapClaims{}
			tkn, err := jwt.ParseWithClaims(jwtToken, &claims, jwtKeyFunc)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "failed parse token")
			}
			if !tkn.Valid {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}
			c.SetRequest(c.Request().WithContext(tokenClaimsToCtx(c.Request().Context(), claims)))
			return next(c)
		}
	}
}

type jwtTokenClaimsKey struct{}

var tknClaimsKey = jwtTokenClaimsKey{}

func tokenClaimsToCtx(ctx context.Context, claims jwt.MapClaims) context.Context {
	return context.WithValue(ctx, tknClaimsKey, claims)
}

func TokenClaimsFromCtx(ctx context.Context) jwt.MapClaims {
	claims, ok := ctx.Value(tknClaimsKey).(jwt.MapClaims)
	if !ok {
		return jwt.MapClaims{}
	}
	return claims
}

// retrieveJWT returns the token string from the request.
func retrieveJWT(request *http.Request) (string, error) {
	header := request.Header.Get("Authorization")
	if header == "" {
		return "", errors.New("authorization header is empty")
	}
	h := strings.Split(header, " ")
	if len(h) != 2 || !strings.EqualFold(h[0], "bearer") {
		return "", errors.New("invalid authorization header format")
	}
	return h[1], nil
}
