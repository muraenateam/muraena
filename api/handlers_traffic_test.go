package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/muraenateam/muraena/api/traffic"
	"github.com/muraenateam/muraena/core/capture"
)

func TestListTrafficEndpoint(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()
	_ = traffic.Save(capture.Flow{ID: "f1", Host: "h", Path: "/a", Status: 200}, 100, 60)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/traffic?limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code = %d", rec.Code)
	}
	var list []map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("traffic = %d, want 1", len(list))
	}
}

func TestGetTrafficDetail(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()
	_ = traffic.Save(capture.Flow{ID: "f1", ResBodyOriginal: "o", ResBodyModified: "m"}, 100, 60)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/traffic/f1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code = %d", rec.Code)
	}
	var f map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &f)
	if f["resBodyOriginal"] != "o" || f["resBodyModified"] != "m" {
		t.Fatalf("bodies = %+v", f)
	}
}
