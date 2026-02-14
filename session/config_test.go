package session

import (
	"os"
	"testing"

	"github.com/muraenateam/muraena/core"
)

func TestSession_CheckRedirect(t *testing.T) {
	s := &Session{}
	s.Config = &Configuration{}

	// INVALID REDIRECTS
	r := []Redirect{
		{RedirectTo: "example.com", HTTPStatusCode: 200},
	}

	s.Config.Redirects = r
	s.CheckRedirect()
	if len(s.Config.Redirects) != 0 {
		t.Errorf("Expected %d, got %d", len(r), len(s.Config.Redirects))
	}

	// VALID REDIRECTS
	r = []Redirect{
		{Hostname: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Path: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Query: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Path: "TEST", Query: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Hostname: "TEST", Query: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Hostname: "TEST", Path: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Hostname: "TEST", Path: "TEST", Query: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Hostname: "TEST", Path: "TEST", Query: "TEST", RedirectTo: "example.com"},
	}

	s.Config.Redirects = r
	s.CheckRedirect()
	if len(s.Config.Redirects) != len(r) {
		t.Errorf("Expected %d, got %d", len(r), len(s.Config.Redirects))
	}

	// MIX REDIRECTS
	r = []Redirect{
		{Hostname: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},         // VALID
		{RedirectTo: "example.com", HTTPStatusCode: 200},                           // INVALID
		{Hostname: "TEST", Path: "TEST", Query: "TEST", RedirectTo: "example.com"}, // VALID
	}

	s.Config.Redirects = r
	s.CheckRedirect()

	// Expect length to be 2
	if len(s.Config.Redirects) != 2 {
		t.Errorf("Expected %d, got %d", 2, len(s.Config.Redirects))
	}

	// Expect last element to have HTTPStatusCode 302
	if s.Config.Redirects[1].HTTPStatusCode != 302 {
		t.Errorf("Expected %d, got %d", 302, s.Config.Redirects[1].HTTPStatusCode)
	}
}

// writeTestConfig writes a TOML config to a temp file and returns the path.
func writeTestConfig(t *testing.T, toml string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "muraena-*.toml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(toml); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestGetConfiguration_DefaultValues(t *testing.T) {
	// Minimal valid TOML with only required fields
	configTOML := `
[proxy]
    phishing = "evil.com"
    destination = "target.com"
`
	path := writeTestConfig(t, configTOML)

	s := &Session{
		Options: core.Options{
			ConfigFilePath: &path,
			Debug:          &[]bool{false}[0],
			Verbose:        &[]bool{false}[0],
			Proxy:          &[]bool{false}[0],
			Version:        &[]bool{false}[0],
			NoColors:       &[]bool{false}[0],
		},
	}

	if err := s.GetConfiguration(); err != nil {
		t.Fatalf("GetConfiguration returned error: %s", err)
	}

	// Verify defaults
	if s.Config.Proxy.IP != DefaultIP {
		t.Errorf("Expected IP=%q, got %q", DefaultIP, s.Config.Proxy.IP)
	}
	if s.Config.Proxy.Listener != DefaultListener {
		t.Errorf("Expected Listener=%q, got %q", DefaultListener, s.Config.Proxy.Listener)
	}
	if s.Config.Proxy.Port != DefaultHTTPPort {
		t.Errorf("Expected Port=%d, got %d", DefaultHTTPPort, s.Config.Proxy.Port)
	}
	if s.Config.Proxy.Protocol != "http://" {
		t.Errorf("Expected Protocol=%q, got %q", "http://", s.Config.Proxy.Protocol)
	}
	if len(s.Config.Transform.Base64.Padding) != len(DefaultBase64Padding) {
		t.Errorf("Expected Base64 padding length=%d, got %d", len(DefaultBase64Padding), len(s.Config.Transform.Base64.Padding))
	}
	if len(s.Config.Transform.Response.SkipContentType) != len(DefaultSkipContentType) {
		t.Errorf("Expected SkipContentType length=%d, got %d", len(DefaultSkipContentType), len(s.Config.Transform.Response.SkipContentType))
	}
	if s.Config.Origins.ExternalOriginPrefix != "ext" {
		t.Errorf("Expected ExternalOriginPrefix=%q, got %q", "ext", s.Config.Origins.ExternalOriginPrefix)
	}
}

func TestGetConfiguration_WithTLS(t *testing.T) {
	// TLS enabled requires cert/key/root content inline
	configTOML := `
[proxy]
    phishing = "evil.com"
    destination = "target.com"

[tls]
    enable = true
    certificate = """
-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJALRiMLAh/GLMMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RDQTAEFW0yMzAxMDEwMDAwMDBaFw0yNDAxMDEwMDAwMDBaMBExDzANBgNVBAMM
BnRlc3RDQTBcMA0GCSqGSIb3DQEBAQUAAwsAMEgCQQDFgXFLJJFP0VHfi/m86GUb
-----END CERTIFICATE-----
"""
    key = """
-----BEGIN RSA PRIVATE KEY-----
MIIBkTCB+wIJALRiMLAh/GLMMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
-----END RSA PRIVATE KEY-----
"""
    root = """
-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJALRiMLAh/GLMMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RDQTAEFW0yMzAxMDEwMDAwMDBaFw0yNDAxMDEwMDAwMDBaMBExDzANBgNVBAMM
BnRlc3RDQTBcMA0GCSqGSIb3DQEBAQUAAwsAMEgCQQDFgXFLJJFP0VHfi/m86GUb
-----END CERTIFICATE-----
"""
`
	path := writeTestConfig(t, configTOML)

	s := &Session{
		Options: core.Options{
			ConfigFilePath: &path,
			Debug:          &[]bool{false}[0],
			Verbose:        &[]bool{false}[0],
			Proxy:          &[]bool{false}[0],
			Version:        &[]bool{false}[0],
			NoColors:       &[]bool{false}[0],
		},
	}

	if err := s.GetConfiguration(); err != nil {
		t.Fatalf("GetConfiguration returned error: %s", err)
	}

	if s.Config.Proxy.Port != DefaultHTTPSPort {
		t.Errorf("Expected Port=%d with TLS, got %d", DefaultHTTPSPort, s.Config.Proxy.Port)
	}
	if s.Config.Proxy.Protocol != "https://" {
		t.Errorf("Expected Protocol=%q with TLS, got %q", "https://", s.Config.Proxy.Protocol)
	}
}

func TestGetConfiguration_TrackingConfig(t *testing.T) {
	configTOML := `
[proxy]
    phishing = "evil.com"
    destination = "target.com"

[tracking]
    enable = true
    trackRequestCookies = true

    [tracking.trace]
        identifier = "session"
        header = "X-Track"
        validator = "[a-zA-Z0-9]{8}"

        [tracking.trace.landing]
            type = "query"
            header = "X-Landing"

    [tracking.secrets]
        paths = ["/login/", "/auth/"]

        [[tracking.secrets.patterns]]
        label = "Username"
        matching = "email="
        start = "email="
        end = "&"
`
	path := writeTestConfig(t, configTOML)

	s := &Session{
		Options: core.Options{
			ConfigFilePath: &path,
			Debug:          &[]bool{false}[0],
			Verbose:        &[]bool{false}[0],
			Proxy:          &[]bool{false}[0],
			Version:        &[]bool{false}[0],
			NoColors:       &[]bool{false}[0],
		},
	}

	if err := s.GetConfiguration(); err != nil {
		t.Fatalf("GetConfiguration returned error: %s", err)
	}

	if !s.Config.Tracking.Enabled {
		t.Error("Expected tracking to be enabled")
	}
	if !s.Config.Tracking.TrackRequestCookies {
		t.Error("Expected TrackRequestCookies to be true")
	}
	if s.Config.Tracking.Trace.Identifier != "session" {
		t.Errorf("Expected Identifier=%q, got %q", "session", s.Config.Tracking.Trace.Identifier)
	}
	if s.Config.Tracking.Trace.Header != "X-Track" {
		t.Errorf("Expected Header=%q, got %q", "X-Track", s.Config.Tracking.Trace.Header)
	}
	if s.Config.Tracking.Trace.ValidatorRegex != "[a-zA-Z0-9]{8}" {
		t.Errorf("Expected Validator=%q, got %q", "[a-zA-Z0-9]{8}", s.Config.Tracking.Trace.ValidatorRegex)
	}
	if s.Config.Tracking.Trace.Landing.Type != "query" {
		t.Errorf("Expected Landing.Type=%q, got %q", "query", s.Config.Tracking.Trace.Landing.Type)
	}
	if len(s.Config.Tracking.Secrets.Paths) != 2 {
		t.Errorf("Expected 2 secret paths, got %d", len(s.Config.Tracking.Secrets.Paths))
	}
	if len(s.Config.Tracking.Secrets.Patterns) != 1 {
		t.Errorf("Expected 1 secret pattern, got %d", len(s.Config.Tracking.Secrets.Patterns))
	}
	if s.Config.Tracking.Secrets.Patterns[0].Label != "Username" {
		t.Errorf("Expected pattern label=%q, got %q", "Username", s.Config.Tracking.Secrets.Patterns[0].Label)
	}
}

func TestGetConfiguration_InvalidListener(t *testing.T) {
	configTOML := `
[proxy]
    phishing = "evil.com"
    destination = "target.com"
    listener = "invalid"
`
	path := writeTestConfig(t, configTOML)

	s := &Session{
		Options: core.Options{
			ConfigFilePath: &path,
			Debug:          &[]bool{false}[0],
			Verbose:        &[]bool{false}[0],
			Proxy:          &[]bool{false}[0],
			Version:        &[]bool{false}[0],
			NoColors:       &[]bool{false}[0],
		},
	}

	if err := s.GetConfiguration(); err != nil {
		t.Fatalf("GetConfiguration returned error: %s", err)
	}

	// Invalid listener should fall back to default
	if s.Config.Proxy.Listener != DefaultListener {
		t.Errorf("Expected Listener=%q for invalid input, got %q", DefaultListener, s.Config.Proxy.Listener)
	}
}

func TestGetConfiguration_MissingPhishing(t *testing.T) {
	configTOML := `
[proxy]
    destination = "target.com"
`
	path := writeTestConfig(t, configTOML)

	s := &Session{
		Options: core.Options{
			ConfigFilePath: &path,
			Debug:          &[]bool{false}[0],
			Verbose:        &[]bool{false}[0],
			Proxy:          &[]bool{false}[0],
			Version:        &[]bool{false}[0],
			NoColors:       &[]bool{false}[0],
		},
	}

	err := s.GetConfiguration()
	if err == nil {
		t.Error("Expected error for missing phishing domain, got nil")
	}
}

func TestGetConfiguration_TLSWithoutRoot(t *testing.T) {
	// TLS enabled with cert and key but NO root CA (mkcert use case:
	// the root CA is in the system trust store, so root="" is valid).
	configTOML := `
[proxy]
    phishing = "evil.com"
    destination = "target.com"

[tls]
    enable = true
    certificate = """
-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJALRiMLAh/GLMMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RDQTAEFW0yMzAxMDEwMDAwMDBaFw0yNDAxMDEwMDAwMDBaMBExDzANBgNVBAMM
BnRlc3RDQTBcMA0GCSqGSIb3DQEBAQUAAwsAMEgCQQDFgXFLJJFP0VHfi/m86GUb
-----END CERTIFICATE-----
"""
    key = """
-----BEGIN RSA PRIVATE KEY-----
MIIBkTCB+wIJALRiMLAh/GLMMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
-----END RSA PRIVATE KEY-----
"""
`
	path := writeTestConfig(t, configTOML)

	s := &Session{
		Options: core.Options{
			ConfigFilePath: &path,
			Debug:          &[]bool{false}[0],
			Verbose:        &[]bool{false}[0],
			Proxy:          &[]bool{false}[0],
			Version:        &[]bool{false}[0],
			NoColors:       &[]bool{false}[0],
		},
	}

	if err := s.GetConfiguration(); err != nil {
		t.Fatalf("GetConfiguration with TLS and no root CA returned error: %s", err)
	}

	if s.Config.Proxy.Protocol != "https://" {
		t.Errorf("Expected Protocol=%q with TLS, got %q", "https://", s.Config.Proxy.Protocol)
	}
	if s.Config.TLS.RootContent != "" {
		t.Errorf("Expected empty RootContent when root is omitted, got %q", s.Config.TLS.RootContent)
	}
	if s.Config.TLS.CertificateContent == "" {
		t.Error("Expected CertificateContent to be populated")
	}
	if s.Config.TLS.KeyContent == "" {
		t.Error("Expected KeyContent to be populated")
	}
}

func TestGetConfiguration_CustomPort(t *testing.T) {
	configTOML := `
[proxy]
    phishing = "evil.com"
    destination = "target.com"
    port = 8080
`
	path := writeTestConfig(t, configTOML)

	s := &Session{
		Options: core.Options{
			ConfigFilePath: &path,
			Debug:          &[]bool{false}[0],
			Verbose:        &[]bool{false}[0],
			Proxy:          &[]bool{false}[0],
			Version:        &[]bool{false}[0],
			NoColors:       &[]bool{false}[0],
		},
	}

	if err := s.GetConfiguration(); err != nil {
		t.Fatalf("GetConfiguration returned error: %s", err)
	}

	if s.Config.Proxy.Port != 8080 {
		t.Errorf("Expected Port=%d, got %d", 8080, s.Config.Proxy.Port)
	}
}
