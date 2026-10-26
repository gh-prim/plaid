package httpserver

import (
	"encoding/json"
	"net/http"
)

// writeError writes the shared API error shape: {"error": {"code", "message"}}.
// code is a stable key clients translate; message is English, for developers.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
