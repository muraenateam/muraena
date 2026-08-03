package integration

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)

const (
	apiBase   = "http://127.0.0.1:8444/api/v1"
	adminUser = "admin"
	// Must match MURAENA_API_ADMIN_PASS exported by run_scoglio_test.sh.
	adminPass = "integration-admin-pass"
)

// skipIfNotScoglioAPI skips unless the Scoglio integration env is set and the
// API control plane is reachable on :8444.
func skipIfNotScoglioAPI(t *testing.T) {
	t.Helper()
	if os.Getenv("MURAENA_INTEGRATION") == "" || os.Getenv("SCOGLIO_INTEGRATION") == "" {
		t.Skip("Skipping API integration test: set MURAENA_INTEGRATION=1 and SCOGLIO_INTEGRATION=1 to run")
	}
	rc, err := redis.Dial("tcp", redisAddr)
	if err != nil {
		t.Skipf("Skipping API integration test: Redis not available at %s: %v", redisAddr, err)
	}
	rc.Close()

	client := newInsecureClient()
	resp, err := client.Get(apiBase + "/healthz")
	if err != nil {
		t.Skipf("Skipping API integration test: API not reachable at %s: %v", apiBase, err)
	}
	resp.Body.Close()
}

// apiDo issues an HTTP request to the API. When token != "" it sets a Bearer
// header. A non-nil body is JSON-encoded. Returns the response and its body.
func apiDo(t *testing.T, method, path, token string, body interface{}) (*http.Response, []byte) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, apiBase+path, rdr)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := newInsecureClient().Do(req)
	if err != nil {
		t.Fatalf("do %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := ioutil.ReadAll(resp.Body)
	return resp, data
}

// decodeJSON unmarshals data into v, failing the test on error.
func decodeJSON(t *testing.T, data []byte, v interface{}) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode JSON: %v (body: %s)", err, string(data))
	}
}

// apiLogin logs in and returns the access + refresh tokens.
func apiLogin(t *testing.T, user, pass string) (string, string) {
	t.Helper()
	resp, data := apiDo(t, "POST", "/auth/login", "", map[string]string{
		"username": user, "password": pass,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("login %s: status %d body %s", user, resp.StatusCode, string(data))
	}
	var out struct {
		Access  string `json:"access"`
		Refresh string `json:"refresh"`
	}
	decodeJSON(t, data, &out)
	if out.Access == "" || out.Refresh == "" {
		t.Fatalf("login %s: empty tokens: %s", user, string(data))
	}
	return out.Access, out.Refresh
}

// flushVictimData deletes victim and traffic keys but PRESERVES api:* keys
// (users, settings, revoked tokens) so the bootstrapped admin survives.
func flushVictimData(t *testing.T) {
	t.Helper()
	rc := newRedisConn(t)
	defer rc.Close()
	if _, err := rc.Do("DEL", "victims"); err != nil {
		t.Fatalf("DEL victims: %v", err)
	}
	for _, pat := range []string{"victim:*", "traffic:*"} {
		keys, err := redis.Strings(rc.Do("KEYS", pat))
		if err != nil {
			t.Fatalf("KEYS %s: %v", pat, err)
		}
		for _, k := range keys {
			if _, err := rc.Do("DEL", k); err != nil {
				t.Fatalf("DEL %s: %v", k, err)
			}
		}
	}
}

// pollUntil calls fn every 200ms until it returns true or timeout elapses.
func pollUntil(t *testing.T, timeout time.Duration, fn func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fn()
}

func TestAPIHealthz(t *testing.T) {
	skipIfNotScoglioAPI(t)
	resp, data := apiDo(t, "GET", "/healthz", "", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("healthz status %d body %s", resp.StatusCode, string(data))
	}
	var out map[string]string
	decodeJSON(t, data, &out)
	if out["status"] != "ok" {
		t.Errorf("healthz status = %q, want ok", out["status"])
	}
}

func TestAPIAuth(t *testing.T) {
	skipIfNotScoglioAPI(t)

	// Login with valid admin creds.
	access, refresh := apiLogin(t, adminUser, adminPass)

	// Bad creds -> 401.
	if resp, _ := apiDo(t, "POST", "/auth/login", "", map[string]string{
		"username": adminUser, "password": "wrong",
	}); resp.StatusCode != 401 {
		t.Errorf("bad login status = %d, want 401", resp.StatusCode)
	}

	// /me reflects the admin identity.
	resp, data := apiDo(t, "GET", "/me", access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("/me status %d body %s", resp.StatusCode, string(data))
	}
	var me map[string]string
	decodeJSON(t, data, &me)
	if me["username"] != adminUser || me["role"] != "admin" {
		t.Errorf("/me = %v, want admin/admin", me)
	}

	// Protected route without a token -> 401.
	if resp, _ := apiDo(t, "GET", "/me", "", nil); resp.StatusCode != 401 {
		t.Errorf("unauthenticated /me status = %d, want 401", resp.StatusCode)
	}

	// Refresh rotates: new pair issued, old refresh token revoked.
	resp, data = apiDo(t, "POST", "/auth/refresh", "", map[string]string{"refresh": refresh})
	if resp.StatusCode != 200 {
		t.Fatalf("refresh status %d body %s", resp.StatusCode, string(data))
	}
	var refreshed struct {
		Refresh string `json:"refresh"`
	}
	decodeJSON(t, data, &refreshed)
	if resp2, _ := apiDo(t, "POST", "/auth/refresh", "", map[string]string{"refresh": refresh}); resp2.StatusCode != 401 {
		t.Errorf("reused old refresh status = %d, want 401 (rotation should revoke)", resp2.StatusCode)
	}

	// Logout revokes the (new) refresh token.
	if resp3, _ := apiDo(t, "POST", "/auth/logout", access, map[string]string{"refresh": refreshed.Refresh}); resp3.StatusCode != 200 {
		t.Errorf("logout status = %d, want 200", resp3.StatusCode)
	}
	if resp4, _ := apiDo(t, "POST", "/auth/refresh", "", map[string]string{"refresh": refreshed.Refresh}); resp4.StatusCode != 401 {
		t.Errorf("refresh after logout status = %d, want 401", resp4.StatusCode)
	}
}

func TestAPIAuthzNonAdminForbidden(t *testing.T) {
	skipIfNotScoglioAPI(t)
	access, _ := apiLogin(t, adminUser, adminPass)

	// Create a viewer (non-admin) user.
	const viewer, viewerPass = "viewer-authz", "viewer-pass-123"
	apiDo(t, "DELETE", "/users/"+viewer, access, nil) // ignore if absent
	if resp, data := apiDo(t, "POST", "/users", access, map[string]string{
		"username": viewer, "password": viewerPass, "role": "viewer",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create viewer status %d body %s", resp.StatusCode, string(data))
	}
	t.Cleanup(func() { apiDo(t, "DELETE", "/users/"+viewer, access, nil) })

	viewerAccess, _ := apiLogin(t, viewer, viewerPass)

	// Non-admin can hit a shared route...
	if resp, _ := apiDo(t, "GET", "/victims", viewerAccess, nil); resp.StatusCode != 200 {
		t.Errorf("viewer GET /victims status = %d, want 200", resp.StatusCode)
	}
	// ...but not an admin-only route.
	if resp, _ := apiDo(t, "GET", "/users", viewerAccess, nil); resp.StatusCode != 403 {
		t.Errorf("viewer GET /users status = %d, want 403", resp.StatusCode)
	}
}
