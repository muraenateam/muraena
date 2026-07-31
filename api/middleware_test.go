package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/muraenateam/muraena/api/auth"
)

func TestRequireAuthRejectsNoToken(t *testing.T) {
	h := RequireAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}

func TestRequireAuthAcceptsValidToken(t *testing.T) {
	setupTestRedis(t)
	access, _, err := auth.Issue("admin", "admin", 15, 7)
	if err != nil {
		t.Fatal(err)
	}
	h := RequireAuth("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if claimsFrom(r).Subject != "admin" {
			t.Error("claims not in context")
		}
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
}
