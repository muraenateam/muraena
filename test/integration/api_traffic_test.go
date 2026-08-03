package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// generateProxyTraffic drives assorted requests through the Muraena proxy so
// the traffic-capture ring buffer fills. Includes a tracked login (victimID)
// and a POST so method/victimID/status filters have data to match.
func generateProxyTraffic(t *testing.T) {
	t.Helper()
	scoglioLogin(t, "admin@scoglio.local", "Admin123!") // GET login + POST creds (tracked)
	client := newInsecureClient()
	for _, p := range []string{"/", "/login", "/signup"} {
		if resp, err := client.Get(scoglioMuraenaBase + p); err == nil {
			resp.Body.Close()
		}
	}
}

func TestAPITrafficREST(t *testing.T) {
	skipIfNotScoglioAPI(t)
	flushVictimData(t)
	access, _ := apiLogin(t, adminUser, adminPass)

	// Clear existing flows, then generate fresh traffic.
	if resp, _ := apiDo(t, "DELETE", "/traffic", access, nil); resp.StatusCode != 200 {
		t.Fatalf("DELETE /traffic (setup) status = %d", resp.StatusCode)
	}
	generateProxyTraffic(t)

	// Flows should appear in the list.
	var summaries []map[string]interface{}
	ok := pollUntil(t, 10*time.Second, func() bool {
		resp, data := apiDo(t, "GET", "/traffic", access, nil)
		if resp.StatusCode != 200 {
			return false
		}
		summaries = nil
		if json.Unmarshal(data, &summaries) != nil {
			return false
		}
		return len(summaries) > 0
	})
	if !ok {
		t.Fatalf("no traffic flows captured via GET /traffic")
	}

	// stats: count > 0, dropped present.
	resp, data := apiDo(t, "GET", "/traffic/stats", access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /traffic/stats status %d body %s", resp.StatusCode, string(data))
	}
	var stats map[string]interface{}
	decodeJSON(t, data, &stats)
	if cnt, _ := stats["count"].(float64); cnt <= 0 {
		t.Errorf("traffic count = %v, want > 0", stats["count"])
	}
	if _, ok := stats["dropped"]; !ok {
		t.Errorf("stats missing 'dropped' field: %s", string(data))
	}

	// Method filter: POST-only results are all POST.
	resp, data = apiDo(t, "GET", "/traffic?method=POST", access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /traffic?method=POST status %d", resp.StatusCode)
	}
	var posts []map[string]interface{}
	decodeJSON(t, data, &posts)
	for _, s := range posts {
		if s["method"] != "POST" {
			t.Errorf("method filter leaked non-POST flow: %v", s["method"])
		}
	}

	// victimID filter narrows to the tracked victim's flows (login was tracked).
	resp, data = apiDo(t, "GET", "/traffic?victimID="+scoglioTrackingID, access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /traffic?victimID status %d", resp.StatusCode)
	}
	var byVictim []map[string]interface{}
	decodeJSON(t, data, &byVictim)
	for _, s := range byVictim {
		if s["victimID"] != scoglioTrackingID {
			t.Errorf("victimID filter leaked flow for %v", s["victimID"])
		}
	}

	// GET /traffic/{id}: full flow for a real id.
	id, _ := summaries[0]["id"].(string)
	if id == "" {
		t.Fatalf("first summary has no id: %v", summaries[0])
	}
	resp, data = apiDo(t, "GET", "/traffic/"+id, access, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("GET /traffic/%s status %d body %s", id, resp.StatusCode, string(data))
	}
	var flow map[string]interface{}
	decodeJSON(t, data, &flow)
	if flow["id"] != id {
		t.Errorf("flow id = %v, want %s", flow["id"], id)
	}

	// Bogus id -> 404.
	if resp, _ := apiDo(t, "GET", "/traffic/does-not-exist", access, nil); resp.StatusCode != 404 {
		t.Errorf("GET bogus flow id status = %d, want 404", resp.StatusCode)
	}

	// DELETE /traffic clears the buffer -> stats count back to 0.
	if resp, _ := apiDo(t, "DELETE", "/traffic", access, nil); resp.StatusCode != 200 {
		t.Errorf("DELETE /traffic status = %d, want 200", resp.StatusCode)
	}
	cleared := pollUntil(t, 5*time.Second, func() bool {
		_, d := apiDo(t, "GET", "/traffic/stats", access, nil)
		var st map[string]interface{}
		if json.Unmarshal(d, &st) != nil {
			return false
		}
		cnt, _ := st["count"].(float64)
		return cnt == 0
	})
	if !cleared {
		t.Errorf("traffic count not 0 after DELETE /traffic")
	}
}

// wsDialTraffic opens a /ws/traffic connection with the given query values
// (e.g. token, method, victimID). Returns the connection and HTTP response.
//
// NOTE: the API runs over plain HTTP (see api/server.go, http.ListenAndServe
// with no TLS), so this dials ws:// rather than wss:// and uses a plain
// websocket.Dialer with no TLS config.
func wsDialTraffic(t *testing.T, q url.Values) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	u := "ws://127.0.0.1:8444/api/v1/ws/traffic?" + q.Encode()
	d := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}
	return d.Dial(u, nil)
}

func TestAPITrafficWebSocket(t *testing.T) {
	skipIfNotScoglioAPI(t)
	flushVictimData(t)
	access, _ := apiLogin(t, adminUser, adminPass)

	// Missing token -> handshake rejected (401, no upgrade).
	noAuthConn, noAuthResp, noAuthErr := wsDialTraffic(t, url.Values{})
	if noAuthConn != nil {
		noAuthConn.Close()
	}
	if noAuthErr == nil {
		t.Errorf("ws without token: expected handshake failure, got success")
	}
	if noAuthResp != nil && noAuthResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("ws without token: status = %d, want 401", noAuthResp.StatusCode)
	}

	// Authenticated connection receives a live flow after we generate traffic.
	conn, _, err := wsDialTraffic(t, url.Values{"token": {access}})
	if err != nil {
		t.Fatalf("ws dial with token failed: %v", err)
	}
	defer conn.Close()

	// Read in a goroutine so we can generate traffic concurrently.
	got := make(chan map[string]interface{}, 4)
	go func() {
		for {
			var msg map[string]interface{}
			if err := conn.ReadJSON(&msg); err != nil {
				close(got)
				return
			}
			got <- msg
		}
	}()

	// Give the hub a moment to register the client, then generate traffic.
	time.Sleep(300 * time.Millisecond)
	generateProxyTraffic(t)

	deadline := time.After(10 * time.Second)
	for {
		select {
		case msg, ok := <-got:
			if !ok {
				t.Fatal("ws closed before a flow arrived")
			}
			if msg["type"] == "flow" {
				if _, hasData := msg["data"]; !hasData {
					t.Errorf("flow message missing 'data': %v", msg)
				}
				return // success
			}
		case <-deadline:
			t.Fatal("timed out waiting for a live flow over /ws/traffic")
		}
	}
}
