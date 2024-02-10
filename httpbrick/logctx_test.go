package httpbrick

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/demeero/bricks/slogbrick"
)

func TestSlogCtxMW(t *testing.T) {
	tests := []struct {
		name     string
		options  []LogCtxMWOption
		checkLog func(*testing.T, *bytes.Buffer)
	}{
		{
			name: "WithURILogAttr_AddsURIAttribute",
			options: []LogCtxMWOption{
				WithURILogAttr(),
			},
			checkLog: func(t *testing.T, buf *bytes.Buffer) {
				assert.Contains(t, buf.String(), uriLogKey)
			},
		},
		{
			name: "WithHostLogAttr_AddsHostAttribute",
			options: []LogCtxMWOption{
				WithHostLogAttr(),
			},
			checkLog: func(t *testing.T, buf *bytes.Buffer) {
				assert.Contains(t, buf.String(), hostLogKey)
			},
		},
		{
			name: "WithPeerLogAttr_AddsPeerAttribute",
			options: []LogCtxMWOption{
				WithPeerLogAttr(),
			},
			checkLog: func(t *testing.T, buf *bytes.Buffer) {
				assert.Contains(t, buf.String(), peerLogKey)
			},
		},
		{
			name: "WithUserAgentLogAttr_AddsUserAgentAttribute",
			options: []LogCtxMWOption{
				WithUserAgentLogAttr(),
			},
			checkLog: func(t *testing.T, buf *bytes.Buffer) {
				assert.Contains(t, buf.String(), userAgentLogKey)
			},
		},
		// Add more test cases here
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/", nil)
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			h := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo})
			slog.SetDefault(slog.New(h))

			rr := httptest.NewRecorder()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				logger := slogbrick.FromCtx(r.Context())
				logger.Info("test")
			})

			SlogCtxMW(tt.options...)(handler).ServeHTTP(rr, req)
		})
	}
}
