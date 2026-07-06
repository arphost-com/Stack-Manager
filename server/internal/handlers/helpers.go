package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// APIResponse is the standard JSON envelope for all API responses.
type APIResponse struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp string      `json:"timestamp"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Status:    "ok",
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Status:    "error",
		Error:     msg,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// writeErrorWithData is the failure counterpart to writeJSON: it emits an
// error envelope with a message the frontend can display AND the structured
// result (compose down output, exit code, etc.) so the operator can see
// what actually happened. Prevents the "HTTP 400" fallback the client
// shows when the JSON body is shaped like a success envelope but carries
// a 4xx/5xx status.
func writeErrorWithData(w http.ResponseWriter, status int, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{
		Status:    "error",
		Error:     msg,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
