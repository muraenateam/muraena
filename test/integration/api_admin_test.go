package integration

import (
	"bytes"
	"net/http"
	"testing"
)

func TestAPIAdminUsersSettingsConfig(t *testing.T) {
	skipIfNotScoglioAPI(t)
	access, _ := apiLogin(t, adminUser, adminPass)

	// --- Users ---
	const u, p = "extra-admin", "extra-pass-123"
	apiDo(t, "DELETE", "/users/"+u, access, nil) // clean slate
	if resp, data := apiDo(t, "POST", "/users", access, map[string]string{
		"username": u, "password": p, "role": "admin",
	}); resp.StatusCode != http.StatusCreated {
		t.Fatalf("create user status %d body %s", resp.StatusCode, string(data))
	}
	t.Cleanup(func() { apiDo(t, "DELETE", "/users/"+u, access, nil) })

	// The new user can log in.
	apiLogin(t, u, p)

	// It appears in the list.
	if _, data := apiDo(t, "GET", "/users", access, nil); !bytes.Contains(data, []byte(u)) {
		t.Errorf("created user %s not in GET /users: %s", u, string(data))
	}

	// Delete it -> 200.
	if resp, _ := apiDo(t, "DELETE", "/users/"+u, access, nil); resp.StatusCode != 200 {
		t.Errorf("delete user status = %d, want 200", resp.StatusCode)
	}

	// --- Settings round-trip ---
	if resp, _ := apiDo(t, "PUT", "/settings", access, map[string]string{
		"integrationProbe": "hello",
	}); resp.StatusCode != 200 {
		t.Errorf("PUT settings status = %d, want 200", resp.StatusCode)
	}
	resp, data := apiDo(t, "GET", "/settings", access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET settings status %d", resp.StatusCode)
	}
	var settings map[string]string
	decodeJSON(t, data, &settings)
	if settings["integrationProbe"] != "hello" {
		t.Errorf("settings round-trip: got %q, want hello", settings["integrationProbe"])
	}
	if _, leaked := settings["jwtSigningKey"]; leaked {
		t.Errorf("GET /settings leaked jwtSigningKey")
	}

	// --- Config ---
	// GET returns the config as a serialized JSON object.
	resp, data = apiDo(t, "GET", "/config", access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET config status %d", resp.StatusCode)
	}
	var cfg map[string]interface{}
	decodeJSON(t, data, &cfg)
	if len(cfg) == 0 {
		t.Errorf("GET /config returned an empty object")
	}
	if _, ok := cfg["Proxy"]; !ok {
		t.Errorf("GET /config missing expected \"Proxy\" key: %s", string(data))
	}

	// Capture the current live UserAgent value so we can restore it after
	// the PATCH below mutates shared live state.
	var origUserAgent string
	if transform, ok := cfg["Transform"].(map[string]interface{}); ok {
		if request, ok := transform["Request"].(map[string]interface{}); ok {
			if ua, ok := request["UserAgent"].(string); ok {
				origUserAgent = ua
			}
		}
	}
	t.Cleanup(func() {
		apiDo(t, "PATCH", "/config", access, map[string]interface{}{
			"Transform": map[string]interface{}{
				"Request": map[string]interface{}{"UserAgent": origUserAgent},
			},
		})
	})

	// PATCH a live (hot-swappable) field -> 200 applied.
	resp, data = apiDo(t, "PATCH", "/config", access, map[string]interface{}{
		"Transform": map[string]interface{}{
			"Request": map[string]interface{}{"UserAgent": "integration-ua"},
		},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("PATCH live config status %d body %s", resp.StatusCode, string(data))
	}
	// Re-GET reflects the applied value.
	_, data = apiDo(t, "GET", "/config", access, nil)
	if !bytes.Contains(data, []byte("integration-ua")) {
		t.Errorf("patched UserAgent not reflected in GET /config")
	}

	// PATCH a restart-required field -> 409 with the offending fields.
	resp, data = apiDo(t, "PATCH", "/config", access, map[string]interface{}{
		"Proxy": map[string]interface{}{"Port": 9999},
	})
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("PATCH restart field status = %d, want 409. body=%s", resp.StatusCode, string(data))
	}
	if !bytes.Contains(data, []byte("restartRequired")) {
		t.Errorf("409 body missing restartRequired: %s", string(data))
	}

	// restart-fields lists known prefixes (non-empty).
	resp, data = apiDo(t, "GET", "/config/restart-fields", access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET restart-fields status %d", resp.StatusCode)
	}
	var rf struct {
		Fields []string `json:"fields"`
	}
	decodeJSON(t, data, &rf)
	if len(rf.Fields) == 0 {
		t.Errorf("restart-fields returned no fields")
	}

	// reload re-reads config from disk -> 200.
	if resp, _ := apiDo(t, "POST", "/config/reload", access, nil); resp.StatusCode != 200 {
		t.Errorf("POST config/reload status = %d, want 200", resp.StatusCode)
	}
}
