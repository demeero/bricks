package httpbrick

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONResponse(t *testing.T) {
	tests := []struct {
		name   string
		status int
		data   interface{}
	}{
		{
			name:   "JSONResponse_WithValidData_ReturnsCorrectResponse",
			status: http.StatusOK,
			data:   map[string]interface{}{"message": "success"},
		},
		{
			name:   "JSONResponse_WithNilData_ReturnsEmptyResponse",
			status: http.StatusOK,
			data:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			JSONResponse(rr, tt.status, tt.data)

			assert.Equal(t, tt.status, rr.Code)

			var resp map[string]interface{}
			err := json.NewDecoder(rr.Body).Decode(&resp)
			if tt.data == nil {
				assert.ErrorIs(t, err, io.EOF)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.data, resp)
		})
	}
}

func TestJSONResponseMsg(t *testing.T) {
	tests := []struct {
		name   string
		status int
		msg    string
	}{
		{
			name:   "JSONResponseMsg_WithValidMessage_ReturnsCorrectResponse",
			status: http.StatusOK,
			msg:    "success",
		},
		{
			name:   "JSONResponseMsg_WithEmptyMessage_ReturnsEmptyResponse",
			status: http.StatusOK,
			msg:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			JSONResponseMsg(rr, tt.status, tt.msg)

			assert.Equal(t, tt.status, rr.Code)

			var resp map[string]interface{}
			require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
			assert.Equal(t, tt.msg, resp["message"])
		})
	}
}
