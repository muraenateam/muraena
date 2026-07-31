package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/muraenateam/muraena/api/auth"
	"github.com/muraenateam/muraena/api/store"
)

func seedAdmin(t *testing.T) {
	t.Helper()
	h, _ := auth.HashPassword("pw")
	if err := store.CreateUser(store.User{Name: "admin", PassHash: h, Role: "admin", Created: "t"}); err != nil {
		t.Fatal(err)
	}
}

func TestLoginAndMe(t *testing.T) {
	setupTestRedis(t)
	seedAdmin(t)
	srv := newTestServer()

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "pw"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("login code = %d body=%s", rec.Code, rec.Body)
	}
	var tok struct{ Access, Refresh string }
	_ = json.Unmarshal(rec.Body.Bytes(), &tok)
	if tok.Access == "" {
		t.Fatal("no access token")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok.Access)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("me code = %d", rec.Code)
	}
}

func TestLoginBadPassword(t *testing.T) {
	setupTestRedis(t)
	seedAdmin(t)
	srv := newTestServer()
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "nope"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}
