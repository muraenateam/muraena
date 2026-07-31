package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/muraenateam/muraena/core/proxy"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

func TestPatchConfigLiveField(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()
	cfg := srv.sess.Config()
	cfg.Transform.Request.UserAgent = "old"
	srv.sess.SwapConfig(cfg)

	patch := map[string]interface{}{
		"Transform": map[string]interface{}{"Request": map[string]interface{}{"UserAgent": "newUA"}},
	}
	body, _ := json.Marshal(patch)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/v1/config", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body)
	}
	if srv.sess.Config().Transform.Request.UserAgent != "newUA" {
		t.Fatal("live field not applied")
	}
}

func TestPatchConfigRestartField(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()
	patch := map[string]interface{}{"Proxy": map[string]interface{}{"Port": 9999}}
	body, _ := json.Marshal(patch)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/v1/config", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("code = %d, want 409", rec.Code)
	}
	if srv.sess.Config().Proxy.Port == 9999 {
		t.Fatal("restart-required field was applied")
	}
}

func TestGetConfigRedactsSecrets(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()
	cfg := srv.sess.Config()
	cfg.TLS.KeyContent = "SECRETKEY"
	cfg.Redis.Password = "REDISPW"
	cfg.Telegram.BotToken = "TELEGRAMTOKEN"
	srv.sess.SwapConfig(cfg)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/config", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if bytes.Contains(rec.Body.Bytes(), []byte("SECRETKEY")) ||
		bytes.Contains(rec.Body.Bytes(), []byte("REDISPW")) ||
		bytes.Contains(rec.Body.Bytes(), []byte("TELEGRAMTOKEN")) {
		t.Fatal("secrets leaked in GET /config")
	}
	_ = session.Configuration{}
}

// TestPatchConfigRefreshesReplacerOnPhishingChange guards against a
// split-brain where PATCH /config swaps in a new live Configuration (e.g. a
// new Proxy.Phishing domain) but the running Replacer - which bakes in
// Proxy.Phishing/Proxy.Target/Transform.Response.CustomContent at Init time -
// keeps serving the stale value because RefreshReplacer was only triggered
// for patches touching "Origins".
func TestPatchConfigRefreshesReplacerOnPhishingChange(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()
	// Replacer.Init logs via log.Debug on a fresh (session-file-less) init;
	// the log package panics if no output has been configured yet.
	log.Init(srv.sess.Options, false, "")
	cfg := srv.sess.Config()
	cfg.Proxy.Target = "target.refresh-test.example"
	cfg.Proxy.Phishing = "old-phish.refresh-test.example"
	srv.sess.SwapConfig(cfg)

	sessionFile := cfg.Proxy.Target + ".session.json"
	t.Cleanup(func() { _ = os.Remove(sessionFile) })

	patch := map[string]interface{}{
		"Proxy": map[string]interface{}{"Phishing": "new-phish.refresh-test.example"},
	}
	body, _ := json.Marshal(patch)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/v1/config", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body)
	}
	if srv.sess.Config().Proxy.Phishing != "new-phish.refresh-test.example" {
		t.Fatal("live config not updated")
	}
	if got := proxy.CurrentReplacer().Phishing; got != "new-phish.refresh-test.example" {
		t.Fatalf("replacer not refreshed after Proxy.Phishing patch: got %q", got)
	}
}
