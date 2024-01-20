package httpbrick

import (
	"net/http"
)

// NoopMW is a middleware that does nothing.
func NoopMW(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		h.ServeHTTP(w, req)
	})
}
