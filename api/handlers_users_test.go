package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/muraenateam/muraena/api/auth"
)

func adminToken(t *testing.T) string {
	t.Helper()
	a, _, err := auth.Issue("admin", "admin", 15, 7)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestCreateAndListUsers(t *testing.T) {
	setupTestRedis(t)
	seedAdmin(t)
	srv := newTestServer()
	tok := adminToken(t)

	body, _ := json.Marshal(map[string]string{"username": "op1", "password": "pw", "role": "admin"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("create code = %d body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	srv.Router().ServeHTTP(rec, req)
	var out []map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out) != 2 {
		t.Fatalf("users = %d, want 2", len(out))
	}
}

func TestCannotDeleteLastAdmin(t *testing.T) {
	setupTestRedis(t)
	seedAdmin(t)
	srv := newTestServer()
	tok := adminToken(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/v1/users/admin", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("code = %d, want 409", rec.Code)
	}
}
