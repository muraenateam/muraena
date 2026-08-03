package integration

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestAPIVictims(t *testing.T) {
	skipIfNotScoglioAPI(t)
	flushVictimData(t)

	// Drive a real login through the proxy to create the SCOGTEST victim.
	scoglioLogin(t, "admin@scoglio.local", "Admin123!")

	access, _ := apiLogin(t, adminUser, adminPass)

	// The victim should appear via the REST API.
	found := pollUntil(t, 10*time.Second, func() bool {
		resp, data := apiDo(t, "GET", "/victims", access, nil)
		if resp.StatusCode != 200 {
			return false
		}
		var vs []map[string]interface{}
		if json.Unmarshal(data, &vs) != nil {
			return false
		}
		for _, v := range vs {
			if v["id"] == scoglioTrackingID {
				return true
			}
		}
		return false
	})
	if !found {
		t.Fatalf("victim %s never appeared in GET /victims", scoglioTrackingID)
	}

	// GET /victims/{id}: id + captured UA.
	resp, data := apiDo(t, "GET", "/victims/"+scoglioTrackingID, access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET victim status %d body %s", resp.StatusCode, string(data))
	}
	var v map[string]interface{}
	decodeJSON(t, data, &v)
	if v["id"] != scoglioTrackingID {
		t.Errorf("victim id = %v, want %s", v["id"], scoglioTrackingID)
	}
	if v["ua"] != testUserAgent {
		t.Errorf("victim ua = %v, want %s", v["ua"], testUserAgent)
	}

	// Credentials: Username + Password captured.
	resp, data = apiDo(t, "GET", "/victims/"+scoglioTrackingID+"/credentials", access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET credentials status %d body %s", resp.StatusCode, string(data))
	}
	if !bytes.Contains(data, []byte("admin@scoglio.local")) || !bytes.Contains(data, []byte("Admin123!")) {
		t.Errorf("credentials missing expected values: %s", string(data))
	}

	// Cookies: the 3 Scoglio session cookies present.
	resp, data = apiDo(t, "GET", "/victims/"+scoglioTrackingID+"/cookies", access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET cookies status %d body %s", resp.StatusCode, string(data))
	}
	for _, name := range []string{"MURAENA_SESS", "NECRO_BRO", "JSESSIONID"} {
		if !bytes.Contains(data, []byte(name)) {
			t.Errorf("cookie %s missing from /cookies response: %s", name, string(data))
		}
	}

	// Unknown victim -> 404.
	if resp, _ := apiDo(t, "GET", "/victims/NOPE1234", access, nil); resp.StatusCode != 404 {
		t.Errorf("GET unknown victim status = %d, want 404", resp.StatusCode)
	}
}

func TestAPISessions(t *testing.T) {
	skipIfNotScoglioAPI(t)
	access, _ := apiLogin(t, adminUser, adminPass)

	for _, path := range []string{"/sessions/hijacked", "/sessions/instrumented"} {
		resp, data := apiDo(t, "GET", path, access, nil)
		if resp.StatusCode != 200 {
			t.Fatalf("GET %s status %d body %s", path, resp.StatusCode, string(data))
		}
		var arr []map[string]interface{}
		decodeJSON(t, data, &arr) // must be a JSON array (possibly empty)
	}
}
