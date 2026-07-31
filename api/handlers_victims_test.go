package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/muraenateam/muraena/core/db"
)

func TestListAndDeleteVictim(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()
	v := &db.Victim{ID: "v1", IP: "1.1.1.1", UA: "ua"}
	if err := v.Store(); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/victims", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("list code = %d", rec.Code)
	}
	var list []map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("victims = %d, want 1", len(list))
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("DELETE", "/api/v1/victims/v1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("delete code = %d", rec.Code)
	}
	all, _ := db.GetAllVictims()
	if len(all) != 0 {
		t.Fatal("victim not deleted")
	}
}

func TestGetVictimNotFound(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/victims/does-not-exist", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("get nonexistent victim code = %d, want 404", rec.Code)
	}
}

func TestForceInstrumentVictimNotFound(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/victims/does-not-exist/instrument", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("instrument nonexistent victim code = %d, want 404", rec.Code)
	}
}
