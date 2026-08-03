package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	toml "github.com/pelletier/go-toml"

	"github.com/muraenateam/muraena/session"
)

// scoglioReconTarget is the REAL Scoglio origin (not the proxy) that an operator
// would point recon at to build a config. dnsmasq maps *.muraena.anti to
// 127.0.0.1 and Scoglio listens on :9443 with the mkcert wildcard cert.
const scoglioReconTarget = "https://scoglio.muraena.anti:9443"

// The cross-origin subresource hosts seeded into Scoglio's base.html so recon
// has real external origins to discover (see scoglio/templates/base.html).
var scoglioSeededOrigins = []string{"cdn.muraena.anti", "assets.muraena.anti"}

// reconResult mirrors api/recon.Result (the JSON under a job's "result").
type reconResult struct {
	Target     string   `json:"target"`
	Origins    []string `json:"origins"`
	LoginPages []struct {
		URL              string `json:"url"`
		Action           string `json:"action"`
		UsernameSelector string `json:"usernameSelector"`
		PasswordSelector string `json:"passwordSelector"`
	} `json:"loginPages"`
	SecretsPaths    []string `json:"secretsPaths"`
	SecretsPatterns []struct {
		Label    string `json:"label"`
		Matching string `json:"matching"`
	} `json:"secretsPatterns"`
}

// reconJob mirrors api/recon.Job.
type reconJob struct {
	ID     string       `json:"id"`
	Status string       `json:"status"`
	Result *reconResult `json:"result"`
	Err    string       `json:"error"`
}

// skipIfNotRecon skips unless the Scoglio API env is up AND puppeteer is
// available (MURAENA_RECON=1, set by run_scoglio_test.sh when node + puppeteer
// installed).
func skipIfNotRecon(t *testing.T) {
	t.Helper()
	skipIfNotScoglioAPI(t)
	if os.Getenv("MURAENA_RECON") == "" {
		t.Skip("Skipping recon integration test: set MURAENA_RECON=1 (node + puppeteer required)")
	}
}

// reconStart POSTs /recon and returns the async reconID (asserts 202).
func reconStart(t *testing.T, access, target string, depth, maxPages int, apply bool) string {
	t.Helper()
	resp, data := apiDo(t, "POST", "/recon", access, map[string]interface{}{
		"target": target, "depth": depth, "maxPages": maxPages, "apply": apply,
	})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST /recon (%s) status %d body %s", target, resp.StatusCode, string(data))
	}
	var out struct {
		ReconID string `json:"reconID"`
	}
	decodeJSON(t, data, &out)
	if out.ReconID == "" {
		t.Fatalf("POST /recon returned empty reconID: %s", string(data))
	}
	return out.ReconID
}

// reconWait polls GET /recon/{id} until the job is done (fails on failed/timeout).
func reconWait(t *testing.T, access, id string, timeout time.Duration) *reconResult {
	t.Helper()
	var job reconJob
	ok := pollUntil(t, timeout, func() bool {
		resp, data := apiDo(t, "GET", "/recon/"+id, access, nil)
		if resp.StatusCode != 200 {
			return false
		}
		job = reconJob{}
		if json.Unmarshal(data, &job) != nil {
			return false
		}
		if job.Status == "failed" {
			t.Fatalf("recon job %s failed: %s", id, job.Err)
		}
		return job.Status == "done" && job.Result != nil
	})
	if !ok {
		t.Fatalf("recon job %s did not complete within %s (last status %q)", id, timeout, job.Status)
	}
	return job.Result
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// TestAPIReconScoglio drives real puppeteer recon against the local Scoglio
// target and verifies it produces a valid Muraena config: the seeded external
// origins are discovered, the login form's credential fields are captured as
// secrets, the merge applies to the live config, and the persisted TOML parses
// as a valid configuration containing the discovered origins.
func TestAPIReconScoglio(t *testing.T) {
	skipIfNotRecon(t)
	access, _ := apiLogin(t, adminUser, adminPass)

	// Preview first (apply=false) so we can inspect the raw discovered result.
	id := reconStart(t, access, scoglioReconTarget, 1, 10, false)
	res := reconWait(t, access, id, 90*time.Second)

	// --- Origins derived ---
	for _, want := range scoglioSeededOrigins {
		if !containsStr(res.Origins, want) {
			t.Errorf("recon origins %v missing seeded origin %q", res.Origins, want)
		}
	}

	// --- Login form credential fields ---
	var login *struct {
		URL              string `json:"url"`
		Action           string `json:"action"`
		UsernameSelector string `json:"usernameSelector"`
		PasswordSelector string `json:"passwordSelector"`
	}
	for i := range res.LoginPages {
		if res.LoginPages[i].PasswordSelector != "" {
			login = &res.LoginPages[i]
			break
		}
	}
	if login == nil {
		t.Fatalf("recon found no login page with a password field: %+v", res.LoginPages)
	}
	if login.PasswordSelector != "#password" {
		t.Errorf("passwordSelector = %q, want #password", login.PasswordSelector)
	}
	if login.UsernameSelector != "#email" {
		t.Errorf("usernameSelector = %q, want #email", login.UsernameSelector)
	}
	if login.Action != "/login" {
		t.Errorf("login action = %q, want /login", login.Action)
	}
	if !containsStr(res.SecretsPaths, "/login") {
		t.Errorf("secretsPaths %v missing /login", res.SecretsPaths)
	}
	var hasPassword, hasUsername bool
	for _, p := range res.SecretsPatterns {
		if p.Matching == "password" {
			hasPassword = true
		}
		if p.Matching == "email" || p.Label == "username" {
			hasUsername = true
		}
	}
	if !hasPassword || !hasUsername {
		t.Errorf("secretsPatterns missing password/username: %+v", res.SecretsPatterns)
	}

	// --- Apply the previewed result to the live config ---
	if resp, data := apiDo(t, "POST", "/recon/"+id+"/apply", access, nil); resp.StatusCode != 200 {
		t.Fatalf("POST /recon/%s/apply status %d body %s", id, resp.StatusCode, string(data))
	}

	// Live config now carries the discovered origins + login secrets. Both
	// seeded 3rd-level hosts (cdn/assets.muraena.anti) collapse to the wildcard
	// *.muraena.anti via SimplifyDomains during the merge — assert on that.
	const collapsedOrigin = "*.muraena.anti"
	_, cfgData := apiDo(t, "GET", "/config", access, nil)
	if !bytes.Contains(cfgData, []byte(collapsedOrigin)) {
		t.Errorf("GET /config missing applied origin %q; body=%s", collapsedOrigin, string(cfgData))
	}
	if !bytes.Contains(cfgData, []byte("/login")) {
		t.Errorf("GET /config missing applied secrets path /login")
	}

	// --- Persisted TOML is a VALID muraena config containing the origins ---
	runCfg := os.Getenv("MURAENA_RUN_CONFIG")
	if runCfg == "" {
		t.Log("MURAENA_RUN_CONFIG not set; skipping persisted-TOML validation")
	} else {
		raw, err := os.ReadFile(runCfg)
		if err != nil {
			t.Fatalf("read persisted config %s: %v", runCfg, err)
		}
		var cfg session.Configuration
		if err := toml.Unmarshal(raw, &cfg); err != nil {
			t.Fatalf("persisted config is not valid TOML/muraena config: %v", err)
		}
		if !containsStr(cfg.Origins.ExternalOrigins, collapsedOrigin) {
			t.Errorf("persisted config ExternalOrigins %v missing %q",
				cfg.Origins.ExternalOrigins, collapsedOrigin)
		}
	}
}

// TestAPIReconWebSocket verifies live progress streaming over /ws/recon and the
// auth + concurrency (409) + validation (400) contracts.
func TestAPIReconWebSocket(t *testing.T) {
	skipIfNotRecon(t)
	access, _ := apiLogin(t, adminUser, adminPass)

	// Bad target -> 400 before any spawn.
	if resp, _ := apiDo(t, "POST", "/recon", access, map[string]interface{}{
		"target": "not-a-url",
	}); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("POST /recon bad target status = %d, want 400", resp.StatusCode)
	}

	// Connect to the progress stream (plain ws://, API has no TLS).
	u := "ws://127.0.0.1:8444/api/v1/ws/recon?" + url.Values{"token": {access}}.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("ws dial /ws/recon: %v", err)
	}
	defer conn.Close()

	got := make(chan string, 8)
	go func() {
		for {
			var msg struct {
				Type string `json:"type"`
				Msg  string `json:"msg"`
			}
			if err := conn.ReadJSON(&msg); err != nil {
				close(got)
				return
			}
			if msg.Type == "progress" {
				got <- msg.Msg
			}
		}
	}()

	// Kick off a run; a second concurrent run must get 409.
	time.Sleep(200 * time.Millisecond)
	id := reconStart(t, access, scoglioReconTarget, 1, 5, false)

	if resp, _ := apiDo(t, "POST", "/recon", access, map[string]interface{}{
		"target": scoglioReconTarget, "depth": 1, "maxPages": 5,
	}); resp.StatusCode != http.StatusConflict {
		t.Errorf("concurrent POST /recon status = %d, want 409", resp.StatusCode)
	}

	// At least one progress line should arrive.
	select {
	case _, ok := <-got:
		if !ok {
			t.Fatal("/ws/recon closed before any progress line")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("timed out waiting for a progress line over /ws/recon")
	}

	// Drain the job so it doesn't leave the runner busy for later tests.
	reconWait(t, access, id, 90*time.Second)
}

// TestAPIReconLiveSites is an opt-in smoke test against real external sites.
// Skipped unless RECON_LIVE=1 — these sites drift and need network access, so
// assertions are loose: recon completes, discovers a non-empty set of origins,
// and the result never mutates the live config (apply=false).
func TestAPIReconLiveSites(t *testing.T) {
	skipIfNotRecon(t)
	if os.Getenv("RECON_LIVE") == "" {
		t.Skip("Skipping live-site recon test: set RECON_LIVE=1 (network + drift-prone)")
	}
	access, _ := apiLogin(t, adminUser, adminPass)

	for _, target := range []string{"https://caniuse.com", "https://hub.docker.com"} {
		target := target
		t.Run(target, func(t *testing.T) {
			id := reconStart(t, access, target, 1, 8, false)
			res := reconWait(t, access, id, 120*time.Second)
			if len(res.Origins) == 0 {
				t.Errorf("recon on %s discovered no external origins", target)
			}
			// Login-field presence is best-effort (only some sites expose a form
			// on the landing page); assert the result is well-formed instead.
			if res.Target == "" {
				t.Errorf("recon on %s returned empty target host", target)
			}
		})
	}
}
