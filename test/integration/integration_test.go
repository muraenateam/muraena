// Package integration provides end-to-end tests for Muraena's cookie capture
// and tracking pipeline. These tests require:
//   - Redis running on localhost:6379
//   - Muraena running with test/integration/config.toml
//   - Network access to authenticationtest.com (proxied through Muraena)
//
// Run with: go test -v -tags=integration ./test/integration/
package integration

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
)

const (
	// muraenaBase is the base URL of the Muraena proxy under test
	muraenaBase = "https://evil-authtest.local:8443"

	// trackingID is the test victim identifier
	trackingID = "TESTID01"

	// redisAddr is the Redis server address
	redisAddr = "127.0.0.1:6379"
)

// skipIfNotIntegration skips the test if the MURAENA_INTEGRATION env var is not set
// or if prerequisites (Redis, Muraena) are not available.
func skipIfNotIntegration(t *testing.T) {
	t.Helper()

	if os.Getenv("MURAENA_INTEGRATION") == "" {
		t.Skip("Skipping integration test: set MURAENA_INTEGRATION=1 to run")
	}

	// Check Redis connectivity
	rc, err := redis.Dial("tcp", redisAddr)
	if err != nil {
		t.Skipf("Skipping integration test: Redis not available at %s: %v", redisAddr, err)
	}
	rc.Close()

	// Check Muraena connectivity
	client := newInsecureClient()
	resp, err := client.Get(muraenaBase + "/")
	if err != nil {
		t.Skipf("Skipping integration test: Muraena not available at %s: %v", muraenaBase, err)
	}
	resp.Body.Close()
}

// newInsecureClient creates an HTTP client that skips TLS verification
// and follows redirects while preserving cookies.
func newInsecureClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 30 * time.Second,
	}
}

// newRedisConn returns a Redis connection, failing the test if unavailable.
func newRedisConn(t *testing.T) redis.Conn {
	t.Helper()
	rc, err := redis.Dial("tcp", redisAddr)
	if err != nil {
		t.Fatalf("Failed to connect to Redis: %v", err)
	}
	return rc
}

// flushRedis removes all keys from the current Redis database.
func flushRedis(t *testing.T) {
	t.Helper()
	rc := newRedisConn(t)
	defer rc.Close()
	_, err := rc.Do("FLUSHDB")
	if err != nil {
		t.Fatalf("Failed to FLUSHDB: %v", err)
	}
}

// TestCookieExpiryIsValid verifies that session cookies (like PHPSESSID) stored
// in Redis have a valid future expiry timestamp, not the Go zero time.
func TestCookieExpiryIsValid(t *testing.T) {
	skipIfNotIntegration(t)
	flushRedis(t)

	client := newInsecureClient()

	// Step 1: GET the login page with a tracking ID to establish the victim
	loginURL := fmt.Sprintf("%s/complexAuth/?session=%s", muraenaBase, trackingID)
	resp, err := client.Get(loginURL)
	if err != nil {
		t.Fatalf("Failed to GET login page: %v", err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200 for login page, got %d. Body: %s", resp.StatusCode, string(body[:min(len(body), 500)]))
	}

	// Step 2: POST login form with credentials
	formData := url.Values{
		"email":       {"simpleAuth@authenticationtest.com"},
		"password":    {"pa$$w0rd"},
		"selectLogin": {"yes"},
		"loveForm":    {"on"},
	}

	postURL := fmt.Sprintf("%s/login/?mode=complexAuth", muraenaBase)
	resp, err = client.Post(postURL, "application/x-www-form-urlencoded", strings.NewReader(formData.Encode()))
	if err != nil {
		t.Fatalf("Failed to POST login form: %v", err)
	}
	resp.Body.Close()

	// Allow some time for cookie processing
	time.Sleep(2 * time.Second)

	// Step 3: Verify PHPSESSID cookie in Redis has valid expiry
	rc := newRedisConn(t)
	defer rc.Close()

	cookieKey := fmt.Sprintf("victim:%s:cookiejar:PHPSESSID", trackingID)
	expires, err := redis.String(rc.Do("HGET", cookieKey, "expires"))
	if err != nil {
		// List all keys to help debug
		keys, _ := redis.Strings(rc.Do("KEYS", fmt.Sprintf("victim:%s:*", trackingID)))
		t.Fatalf("Failed to get PHPSESSID expires from Redis (key: %s). Available keys: %v. Error: %v",
			cookieKey, keys, err)
	}

	t.Logf("PHPSESSID expires value in Redis: %s", expires)

	// Verify it's not the Go zero time
	if strings.Contains(expires, "0001-01-01") {
		t.Errorf("PHPSESSID has Go zero time expiry: %s. The NormalizeCookieExpiry fix is not working.", expires)
	}

	// Parse and verify it's at least ~47 hours in the future
	const timeLayout = "2006-01-02 15:04:05 -0700 MST"
	parsed, err := time.Parse(timeLayout, expires)
	if err != nil {
		t.Fatalf("Failed to parse expires timestamp '%s': %v", expires, err)
	}

	untilExpiry := time.Until(parsed)
	if untilExpiry < 47*time.Hour {
		t.Errorf("PHPSESSID expiry is only %v in the future (expected at least ~47h). Parsed: %v", untilExpiry, parsed)
	}

	// Verify it converts to a positive Unix timestamp
	if parsed.Unix() < 1 {
		t.Errorf("PHPSESSID Unix timestamp is negative or zero: %d. This would break necrobrowser.", parsed.Unix())
	}

	t.Logf("PHPSESSID expiry is valid: %v (Unix: %d, %v from now)", parsed, parsed.Unix(), untilExpiry)
}

// TestNoPhantomVictims verifies that untracked requests do not create
// phantom victim entries in Redis.
func TestNoPhantomVictims(t *testing.T) {
	skipIfNotIntegration(t)
	flushRedis(t)

	client := newInsecureClient()

	// Make 10 untracked requests (no session parameter)
	paths := []string{
		"/",
		"/complexAuth/",
		"/simpleAuth/",
		"/favicon.ico",
		"/robots.txt",
		"/nonexistent-page",
		"/login/",
		"/about/",
		"/contact/",
		"/test/",
	}

	for _, path := range paths {
		resp, err := client.Get(muraenaBase + path)
		if err != nil {
			t.Logf("Warning: request to %s failed: %v", path, err)
			continue
		}
		resp.Body.Close()
	}

	// Allow processing time
	time.Sleep(1 * time.Second)

	// Verify zero victims were created
	rc := newRedisConn(t)
	defer rc.Close()

	victims, err := redis.Strings(rc.Do("LRANGE", "victims", "0", "-1"))
	if err != nil && err != redis.ErrNil {
		t.Fatalf("Failed to query victims list: %v", err)
	}

	if len(victims) > 0 {
		t.Errorf("Expected 0 phantom victims, but found %d: %v", len(victims), victims)
	} else {
		t.Log("No phantom victims created - tracking noise fix is working")
	}
}

// TestTrackedRequestCreatesVictim verifies that a request WITH a valid
// tracking ID properly creates a victim entry.
func TestTrackedRequestCreatesVictim(t *testing.T) {
	skipIfNotIntegration(t)
	flushRedis(t)

	client := newInsecureClient()

	// Make a tracked request
	trackedURL := fmt.Sprintf("%s/complexAuth/?session=%s", muraenaBase, trackingID)
	resp, err := client.Get(trackedURL)
	if err != nil {
		t.Fatalf("Failed to GET tracked URL: %v", err)
	}
	resp.Body.Close()

	// Allow processing time
	time.Sleep(1 * time.Second)

	// Verify exactly one victim was created with the correct ID
	rc := newRedisConn(t)
	defer rc.Close()

	victims, err := redis.Strings(rc.Do("LRANGE", "victims", "0", "-1"))
	if err != nil {
		t.Fatalf("Failed to query victims list: %v", err)
	}

	if len(victims) != 1 {
		t.Fatalf("Expected exactly 1 victim, got %d: %v", len(victims), victims)
	}

	if victims[0] != trackingID {
		t.Errorf("Expected victim ID %s, got %s", trackingID, victims[0])
	}

	// Verify victim data exists
	victimKey := fmt.Sprintf("victim:%s", trackingID)
	id, err := redis.String(rc.Do("HGET", victimKey, "id"))
	if err != nil {
		t.Fatalf("Failed to get victim ID from Redis: %v", err)
	}

	if id != trackingID {
		t.Errorf("Victim ID in Redis hash is %s, expected %s", id, trackingID)
	}

	t.Logf("Victim %s created successfully", trackingID)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
