package httpbrick

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/demeero/bricks/slogbrick"
)

// RecoverSlogMW recovers from panics and logs via slog the stack trace.
// It responds with a 500 status code.
func RecoverSlogMW(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				slogbrick.FromCtx(req.Context()).
					Error("http handler panicked",
						slog.Any("err", err),
						slog.String("stack", string(debug.Stack())))
			}
		}()
		h.ServeHTTP(w, req)
	})
}
