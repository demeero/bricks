package httpbrick

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/demeero/bricks/slogbrick"
)

// RecoverMWOption is a function that configures recover middleware.
type RecoverMWOption func(*recoverMWOpts)

type recoverMWOpts struct {
	printStackWriter io.Writer
	LogStackField    bool
	PrintStack       bool
}

// WithRecoverLogStackField allows to add stack trace to logger "stack" attribute.
func WithRecoverLogStackField(v bool) RecoverMWOption {
	return func(opts *recoverMWOpts) {
		opts.LogStackField = v
	}
}

// WithRecoverPrintStack allows to print stack trace to writer (e.g. os.Stderr).
func WithRecoverPrintStack(w io.Writer) RecoverMWOption {
	return func(opts *recoverMWOpts) {
		opts.PrintStack = true
		opts.printStackWriter = w
	}
}

// RecoverMW is a middleware that recovers from panics and logs them.
//
//nolint:errorlint // it's ok to not use errors.Is for http.ErrAbortHandler
func RecoverMW(options ...RecoverMWOption) func(http.Handler) http.Handler {
	opts := recoverMWOpts{LogStackField: true}
	for _, opt := range options {
		opt(&opts)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					if rvr == http.ErrAbortHandler {
						// we don't recover http.ErrAbortHandler so the response
						// to the client is aborted, this should not be logged
						panic(rvr)
					}
					stackTrace := string(debug.Stack())
					reqLogger := slogbrick.FromCtx(req.Context())
					logPanic(opts, stackTrace, rvr, reqLogger)
					stackTracePrint(opts, stackTrace, rvr, reqLogger)
					if req.Header.Get("Connection") != "Upgrade" {
						w.WriteHeader(http.StatusInternalServerError)
					}
				}
			}()

			next.ServeHTTP(w, req)
		})
	}
}

func logPanic(opts recoverMWOpts, stack string, rvr interface{}, reqLogger *slog.Logger) {
	attrs := []interface{}{
		slog.Any("err", rvr),
	}
	if opts.LogStackField {
		attrs = append(attrs, slog.String("stack", stack))
	}
	reqLogger.With(attrs...).Error("http handler panicked and recovered")
}

func stackTracePrint(opts recoverMWOpts, stack string, rvr interface{}, reqLogger *slog.Logger) {
	if !opts.PrintStack {
		return
	}
	if _, err := fmt.Fprintf(opts.printStackWriter, "%v:\n%s\n", rvr, stack); err != nil {
		reqLogger.Error("failed print stack trace", slog.Any("err", err))
	}
}
