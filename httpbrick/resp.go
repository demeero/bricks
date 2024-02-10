package httpbrick

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func JSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode JSON response", slog.Any("err", err))
	}
}

func JSONResponseMsg(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": msg,
	})
	if err != nil {
		slog.Error("failed to encode JSON response", slog.Any("err", err))
	}
}
