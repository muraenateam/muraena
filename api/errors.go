package api

import (
	"encoding/json"
	"net/http"
	"os"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]string{"code": code, "message": msg},
	})
}

func writeFile(path string, b []byte) error { return os.WriteFile(path, b, 0644) }

func jsonMarshal(v interface{}) ([]byte, error) { return json.Marshal(v) }
