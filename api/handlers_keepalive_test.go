package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/muraenateam/muraena/core/db"
)

func TestCreateAndListKeepalive(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()
	(&db.Victim{ID: "v1"}).Store()

	body, _ := json.Marshal(map[string]interface{}{"victimID": "v1", "intervalMin": 5})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/keepalives", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("create code = %d body=%s", rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api/v1/keepalives", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	var list []map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("keepalives = %d, want 1", len(list))
	}
}
