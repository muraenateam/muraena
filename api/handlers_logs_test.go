package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/muraenateam/muraena/api/logstream"
	"github.com/muraenateam/muraena/log"
)

func TestListLogsRequiresAuth(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/logs", nil)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}

func TestListLogsReturnsRingNewestFirst(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()

	_ = logstream.Append(log.LogLine{Level: "inf", Text: "first"}, 100)
	_ = logstream.Append(log.LogLine{Level: "inf", Text: "second"}, 100)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/logs?limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body)
	}
	var lines []log.LogLine
	if err := json.Unmarshal(rec.Body.Bytes(), &lines); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	if lines[0].Text != "second" || lines[1].Text != "first" {
		t.Fatalf("lines not newest-first: %+v", lines)
	}
}

func TestListLogsFiltersByLevel(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()

	_ = logstream.Append(log.LogLine{Level: "inf", Text: "info line"}, 100)
	_ = logstream.Append(log.LogLine{Level: "war", Text: "warn line"}, 100)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/logs?level=war", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body)
	}
	var lines []log.LogLine
	if err := json.Unmarshal(rec.Body.Bytes(), &lines); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || lines[0].Text != "warn line" {
		t.Fatalf("filtered lines = %+v, want just the warn line", lines)
	}
}

func TestLogsWSRejectsMissingToken(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/ws/logs", nil)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}

func TestLogsWSRejectsInvalidToken(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/ws/logs?token=not-a-real-token", nil)
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
}
