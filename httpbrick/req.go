package httpbrick

import "net/http"

func ComputeApproximateRequestSize(req *http.Request) int64 {
	size := 0
	if req.URL != nil {
		size = len(req.URL.Path)
	}

	size += len(req.Method)
	size += len(req.Proto)
	for name, values := range req.Header {
		size += len(name)
		for _, value := range values {
			size += len(value)
		}
	}
	size += len(req.Host)

	// N.B. req.Form and req.MultipartForm are assumed to be included in req.URL.

	if req.ContentLength != -1 {
		size += int(req.ContentLength)
	}
	return int64(size)
}
