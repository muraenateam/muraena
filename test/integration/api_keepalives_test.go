package integration

import (
	"bytes"
	"net/http"
	"testing"
	"time"
)

func TestAPIKeepalives(t *testing.T) {
	skipIfNotScoglioAPI(t)
	flushVictimData(t)

	// Establish a victim so instrument/keepalive-now have a target.
	scoglioLogin(t, "admin@scoglio.local", "Admin123!")
	access, _ := apiLogin(t, adminUser, adminPass)
	if !pollUntil(t, 10*time.Second, func() bool {
		resp, _ := apiDo(t, "GET", "/victims/"+scoglioTrackingID, access, nil)
		return resp.StatusCode == 200
	}) {
		t.Fatalf("victim %s not available before keepalive tests", scoglioTrackingID)
	}

	// List keepalives (200 + array).
	resp, data := apiDo(t, "GET", "/keepalives", access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET keepalives status %d body %s", resp.StatusCode, string(data))
	}
	var kas []map[string]interface{}
	decodeJSON(t, data, &kas)

	// Create a keepalive (admin) -> 201.
	resp, data = apiDo(t, "POST", "/keepalives", access, map[string]interface{}{
		"victimID": scoglioTrackingID, "intervalMin": 10,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST keepalive status %d body %s", resp.StatusCode, string(data))
	}

	// It should now show up in the list.
	present := pollUntil(t, 5*time.Second, func() bool {
		_, d := apiDo(t, "GET", "/keepalives", access, nil)
		return bytes.Contains(d, []byte(scoglioTrackingID))
	})
	if !present {
		t.Errorf("created keepalive for %s not present in list", scoglioTrackingID)
	}

	// Delete it (id == victimID) -> 200.
	if resp, _ := apiDo(t, "DELETE", "/keepalives/"+scoglioTrackingID, access, nil); resp.StatusCode != 200 {
		t.Errorf("DELETE keepalive status = %d, want 200", resp.StatusCode)
	}

	// Force instrument: necro module is loaded (config enable=true) so the API
	// accepts the job (202 Accepted) even though the endpoint is unreachable.
	if resp, data := apiDo(t, "POST", "/victims/"+scoglioTrackingID+"/instrument", access, nil); resp.StatusCode != http.StatusAccepted {
		t.Errorf("POST instrument status = %d, want 202. body=%s", resp.StatusCode, string(data))
	}

	// Keepalive-now: same contract -> 202 Accepted.
	if resp, data := apiDo(t, "POST", "/victims/"+scoglioTrackingID+"/keepalive", access, nil); resp.StatusCode != http.StatusAccepted {
		t.Errorf("POST keepalive-now status = %d, want 202. body=%s", resp.StatusCode, string(data))
	}

	// Instrument on an unknown victim -> 404.
	if resp, _ := apiDo(t, "POST", "/victims/NOPE1234/instrument", access, nil); resp.StatusCode != 404 {
		t.Errorf("instrument unknown victim status = %d, want 404", resp.StatusCode)
	}
}
