package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/muraenateam/muraena/webui"
)

func TestWebUIServesIndex(t *testing.T) {
	h := webui.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/ code = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Muraena") {
		t.Fatalf("index missing expected content")
	}

	// unknown client route falls back to index.html
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/victims", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/victims fallback code = %d", rec.Code)
	}
}
