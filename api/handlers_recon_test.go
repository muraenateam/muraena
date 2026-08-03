//go:build !windows

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/muraenateam/muraena/log"
)

func fakeNode(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	node := filepath.Join(dir, "node")
	reconJSON := `{"target":"example.com","origins":["cdn.example.com"],"loginPages":[],"secretsPaths":["/login"],"secretsPatterns":[]}`
	body := "#!/bin/sh\ncat <<'JSON'\n" + reconJSON + "\nJSON\n"
	if err := os.WriteFile(node, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return node
}

func TestStartReconAppliesConfig(t *testing.T) {
	setupTestRedis(t)
	srv := newTestServer()
	// Auto-apply refreshes the Replacer, whose Init logs via log.Debug; the log
	// package panics if no output has been configured yet.
	log.Init(srv.sess.Options, false, "")
	cfg := srv.sess.Config()
	cfg.Recon.NodePath = fakeNode(t)
	cfg.Recon.Script = "ignored.js"
	srv.sess.SwapConfig(cfg)

	body, _ := json.Marshal(map[string]interface{}{"target": "https://example.com", "depth": 1, "maxPages": 5})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/recon", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	srv.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body)
	}
	var out struct{ ReconID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)

	// wait for the job, then verify origins merged
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if contains(srv.sess.Config().Origins.ExternalOrigins, "*.example.com") ||
			contains(srv.sess.Config().Origins.ExternalOrigins, "cdn.example.com") {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("origins not merged: %v", srv.sess.Config().Origins.ExternalOrigins)
}

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}
