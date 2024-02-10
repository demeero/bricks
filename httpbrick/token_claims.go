package httpbrick

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

type TokenClaimsMWOpts func(*tokenClaimsMWOpts)

type tokenClaimsMWOpts struct {
	RespWriter    func(w http.ResponseWriter, status int, msg string)
	KeyFuncOpts   keyfunc.Options
	JWKSURL       string
	Header        string
	HeaderPrefix  string
	ErrStatusCode int
}

// WithTokenClaimsRespWriter sets the response writer function for the middleware in the case of error.
// JSONResponseMsg is used by default.
func WithTokenClaimsRespWriter(w func(w http.ResponseWriter, status int, msg string)) TokenClaimsMWOpts {
	return func(opts *tokenClaimsMWOpts) {
		opts.RespWriter = w
	}
}

// WithTokenClaimsErrStatusCode sets the status code for the middleware in the case of error.
// http.StatusUnauthorized is used by default.
func WithTokenClaimsErrStatusCode(code int) TokenClaimsMWOpts {
	return func(opts *tokenClaimsMWOpts) {
		opts.ErrStatusCode = code
	}
}

// WithTokenClaimsJWKSURL sets the URL to retrieve JWKS from.
func WithTokenClaimsJWKSURL(url string) TokenClaimsMWOpts {
	return func(opts *tokenClaimsMWOpts) {
		opts.JWKSURL = url
	}
}

// WithTokenClaimsHeader sets the header name to retrieve the token from.
// "Authorization" is used by default.
func WithTokenClaimsHeader(header string) TokenClaimsMWOpts {
	return func(opts *tokenClaimsMWOpts) {
		opts.Header = header
	}
}

// WithTokenClaimsHeaderPrefix sets the prefix for the token in the header.
// "bearer" is used by default.
func WithTokenClaimsHeaderPrefix(prefix string) TokenClaimsMWOpts {
	return func(opts *tokenClaimsMWOpts) {
		opts.HeaderPrefix = prefix
	}
}

// WithTokenClaimsKeyFuncOpts sets the options for the keyfunc.
func WithTokenClaimsKeyFuncOpts(opts keyfunc.Options) TokenClaimsMWOpts {
	return func(o *tokenClaimsMWOpts) {
		o.KeyFuncOpts = opts
	}
}

var defaultTokenClaimsMWOpts = tokenClaimsMWOpts{
	RespWriter:    JSONResponseMsg,
	ErrStatusCode: http.StatusUnauthorized,
	Header:        "Authorization",
	HeaderPrefix:  "bearer",
}

// TokenClaimsMW is a middleware that extracts the JWT token from the request and puts the claims to the request context.
func TokenClaimsMW(options ...TokenClaimsMWOpts) func(http.Handler) http.Handler {
	opts := defaultTokenClaimsMWOpts
	for _, opt := range options {
		opt(&opts)
	}
	var jwtKeyFunc jwt.Keyfunc = func(token *jwt.Token) (interface{}, error) {
		return jwt.UnsafeAllowNoneSignatureType, nil
	}
	if opts.JWKSURL != "" {
		jwks, err := keyfunc.Get(opts.JWKSURL, opts.KeyFuncOpts)
		if err != nil {
			slog.Error("failed create JWKS from resource at the given URL",
				slog.Any("err", err), slog.String("jwks_url", opts.JWKSURL))
		} else {
			jwtKeyFunc = jwks.Keyfunc
		}
	}
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			jwtToken, err := retrieveJWT(opts, req)
			if err != nil {
				opts.RespWriter(w, opts.ErrStatusCode, err.Error())
				return
			}
			claims := jwt.MapClaims{}
			tkn, err := jwt.ParseWithClaims(jwtToken, &claims, jwtKeyFunc)
			if err != nil {
				opts.RespWriter(w, opts.ErrStatusCode, "failed parse token")
				return
			}
			if !tkn.Valid {
				opts.RespWriter(w, opts.ErrStatusCode, "invalid token")
				return
			}
			req = req.WithContext(tokenClaimsToCtx(req.Context(), claims))
			h.ServeHTTP(w, req)
		})
	}
}

type jwtTokenClaimsKey struct{}

var tknClaimsKey = jwtTokenClaimsKey{}

func tokenClaimsToCtx(ctx context.Context, claims jwt.MapClaims) context.Context {
	return context.WithValue(ctx, tknClaimsKey, claims)
}

// TokenClaimsFromCtx returns the token claims from the context.
func TokenClaimsFromCtx(ctx context.Context) jwt.MapClaims {
	claims, ok := ctx.Value(tknClaimsKey).(jwt.MapClaims)
	if !ok {
		return jwt.MapClaims{}
	}
	return claims
}

// retrieveJWT returns the token string from the request.
func retrieveJWT(opts tokenClaimsMWOpts, request *http.Request) (string, error) {
	header := request.Header.Get(opts.Header)
	if header == "" {
		return "", errors.New("authorization header is empty")
	}
	if !strings.HasPrefix(header, opts.HeaderPrefix) {
		return "", errors.New("invalid authorization header format")
	}
	return strings.TrimSpace(strings.TrimPrefix(header, opts.HeaderPrefix)), nil
}
