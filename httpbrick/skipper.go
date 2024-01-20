package httpbrick

import (
	"net/http"
)

// Skipper defines a function to skip middleware.
// Returning true skips processing the middleware.
type Skipper func(req *http.Request) bool
