package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/healthz", nil)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}
