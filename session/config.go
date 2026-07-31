package session

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"github.com/muraenateam/muraena/core"
)

var (
	DefaultIP              = "0.0.0.0"
	DefaultListener        = "tcp"
	DefaultHTTPPort        = 80
	DefaultHTTPSPort       = 443
	DefaultBase64Padding   = []string{"=", "."}
	DefaultSkipContentType = []string{"font/*", "image/*"}
)

type Redirect struct {
	Hostname       string `toml:"hostname"`
	Path           string `toml:"path"`
	Query          string `toml:"query"`
	RedirectTo     string `toml:"redirectTo"`
	HTTPStatusCode int    `toml:"httpStatusCode"`
}

type StaticHTTPConfig struct {
	Enabled       bool   `toml:"enable"`
	LocalPath     string `toml:"localPath"`
	URLPath       string `toml:"urlPath"`
	ListeningHost string `toml:"listeningHost"`
	ListeningPort int    `toml:"listeningPort"`
}

type ApiJWTConfig struct {
	AccessMinutes int `toml:"accessMinutes"`
	RefreshDays   int `toml:"refreshDays"`
}

type ApiTrafficConfig struct {
	Enable        bool `toml:"enable"`
	MaxFlows      int  `toml:"maxFlows"`
	TTLMinutes    int  `toml:"ttlMinutes"`
	MaxBodyKB     int  `toml:"maxBodyKB"`
	CaptureBodies bool `toml:"captureBodies"`
}

type ApiConfig struct {
	Enable  bool             `toml:"enable"`
	Bind    string           `toml:"bind"`
	Port    int              `toml:"port"`
	JWT     ApiJWTConfig     `toml:"jwt"`
	Traffic ApiTrafficConfig `toml:"traffic"`
}

type ReconConfig struct {
	NodePath string `toml:"nodePath"`
	Script   string `toml:"script"`
}

// applyDefaults fills unset API config fields with safe defaults.
func (a *ApiConfig) applyDefaults() {
	if a.Bind == "" {
		a.Bind = "127.0.0.1"
	}
	if a.Port == 0 {
		a.Port = 8443
	}
	if a.JWT.AccessMinutes == 0 {
		a.JWT.AccessMinutes = 15
	}
	if a.JWT.RefreshDays == 0 {
		a.JWT.RefreshDays = 7
	}
	if a.Traffic.MaxFlows == 0 {
		a.Traffic.MaxFlows = 5000
	}
	if a.Traffic.TTLMinutes == 0 {
		a.Traffic.TTLMinutes = 1440
	}
	if a.Traffic.MaxBodyKB == 0 {
		a.Traffic.MaxBodyKB = 512
	}
}

// Configuration struct
type Configuration struct {
	//
	// Proxy rules
	//
	Proxy struct {
		Phishing    string `toml:"phishing"`
		Target      string `toml:"destination"`
		IP          string `toml:"IP"`
		Listener    string `toml:"listener"`
		Port        int    `toml:"port"`
		PortMap     string `toml:"portmapping"`
		HTTPtoHTTPS struct {
			Enabled  bool `toml:"enable"`
			HTTPport int  `toml:"port"`
		} `toml:"HTTPtoHTTPS"`

		Protocol string `toml:"-"`
	} `toml:"proxy"`

	//
	// Origins
	//
	Origins struct {
		ExternalOriginPrefix string            `toml:"externalOriginPrefix"`
		ExternalOrigins      []string          `toml:"externalOrigins"`
		OriginsMapping       map[string]string `toml:"-"`

		SubdomainMap [][]string `toml:"subdomainMap"`
	} `toml:"origins"`

	//
	// Transforming rules
	//
	Transform struct {
		Base64 struct {
			Enabled bool     `toml:"enable"`
			Padding []string `toml:"padding"`
		} `toml:"base64"`

		Request struct {
			SkipExtensions []string `toml:"-"`

			UserAgent string `toml:"userAgent"`
			// Headers list to consider for the transformation
			Headers []string `toml:"headers"`

			// CustomContent Transformations
			CustomContent [][]string `toml:"customContent"`

			Remove struct {
				Headers []string `toml:"headers"`
			} `toml:"remove"`

			Add struct {
				Headers []struct {
					Name  string `toml:"name"`
					Value string `toml:"value"`
				} `toml:"headers"`
			} `toml:"add"`
		} `toml:"request"`

		Response struct {
			SkipContentType []string `toml:"skipContentType"`

			Headers []string `toml:"headers"`

			// CustomContent Transformations
			CustomContent [][]string `toml:"customContent"`

			Cookie struct {
				SameSite string `toml:"sameSite"`
			} `toml:"cookie"`

			Remove struct {
				Headers []string `toml:"headers"`
			} `toml:"remove"`

			Add struct {
				Headers []struct {
					Name  string `toml:"name"`
					Value string `toml:"value"`
				} `toml:"headers"`
			} `toml:"add"`
		} `toml:"response"`
	} `toml:"transform"`

	Redirects []Redirect `toml:"redirect"`

	//
	// Logging
	//
	Log struct {
		Enabled  bool   `toml:"enable"`
		FilePath string `toml:"filePath"`
	} `toml:"log"`

	//
	// DB (Redis)
	//
	Redis struct {
		Host     string `toml:"host"`     // default: 127.0.0.1
		Port     int    `toml:"port"`     // default: 6379
		Password string `toml:"password"` // default: ""
	} `toml:"redis"`

	//
	// TLS
	//
	TLS struct {
		Enabled     bool   `toml:"enable"`
		Expand      bool   `toml:"expand"`
		Certificate string `toml:"certificate"`
		Key         string `toml:"key"`
		Root        string `toml:"root"`
		SSLKeyLog   string `toml:"sslKeyLog"`

		CertificateContent string `toml:"-"`
		KeyContent         string `toml:"-"`
		RootContent        string `toml:"-"`

		// Minimum supported TLS version: SSL3, TLS1, TLS1.1, TLS1.2, TLS1.3
		MinVersion               string `toml:"minVersion"`
		MaxVersion               string `toml:"maxVersion"`
		PreferServerCipherSuites bool   `toml:"preferServerCipherSuites"`
		SessionTicketsDisabled   bool   `toml:"SessionTicketsDisabled"`
		InsecureSkipVerify       bool   `toml:"insecureSkipVerify"`
		RenegotiationSupport     string `toml:"renegotiationSupport"`
	} `toml:"tls"`

	//
	// Tracking
	//
	Tracking struct {
		Enabled             bool `toml:"enable"`
		TrackRequestCookies bool `toml:"trackRequestCookies"`

		Trace struct {
			Identifier     string `toml:"identifier"`
			Header         string `toml:"header"`
			Domain         string `toml:"domain"`
			ValidatorRegex string `toml:"validator"`

			Landing struct {
				Type       string `toml:"type"` // path or query
				Header     string `toml:"header"`
				RedirectTo string `toml:"redirectTo"` // redirect url once the landing is detected (applicable only if type is path)
			} `toml:"landing"`
		} `toml:"trace"`

		Secrets struct {
			Paths []string `toml:"paths"`

			Patterns []struct {
				Label    string `toml:"label"`
				Matching string `toml:"matching"`
				Start    string `toml:"start"`
				End      string `toml:"end"`
			} `toml:"patterns"`
		} `toml:"secrets"`
	} `toml:"tracking"`

	// Crawler
	Crawler struct {
		Enabled bool `toml:"enable"`
		Depth   int  `toml:"depth"`
		UpTo    int  `toml:"upto"`
	} // `toml:"crawler"`  TODO: Temporarily disabled

	//
	// Necrobrowser
	//
	Necrobrowser struct {
		Enabled bool `toml:"enable"`

		SensitiveLocations struct {
			AuthSession         []string `toml:"authSession"`
			AuthSessionResponse []string `toml:"authSessionResponse"`
		} `toml:"urls"`

		Endpoint  string `toml:"endpoint"`
		Profile   string `toml:"profile"`
		Keepalive struct {
			Enable  bool   `toml:"enable"`
			Minutes int    `toml:"minutes"`
			Profile string `toml:"profile"`
		} `toml:"keepalive"`
		Trigger struct {
			Type   string   `toml:"type"`
			Values []string `toml:"values"`
			Delay  int      `toml:"delay"`
		} `toml:"trigger"`
	} `toml:"necrobrowser"`

	StaticServer StaticHTTPConfig `toml:"staticServer"`

	//
	// Watchdog
	//
	Watchdog struct {
		Enabled bool   `toml:"enable"`
		Dynamic bool   `toml:"dynamic"`
		Rules   string `toml:"rules"`
		GeoDB   string `toml:"geoDB"`
	} `toml:"watchdog"`

	//
	// Telegram
	//
	Telegram struct {
		Enabled  bool     `toml:"enable"`
		BotToken string   `toml:"botToken"`
		ChatIDs  []string `toml:"chatIDs"`
	} `toml:"telegram"`

	//
	// API control plane
	//
	Api ApiConfig `toml:"api"`

	//
	// Recon (puppeteer)
	//
	Recon ReconConfig `toml:"recon"`
}

// GetConfiguration returns the configuration object
func (s *Session) GetConfiguration() (err error) {

	cb, err := ioutil.ReadFile(*s.Options.ConfigFilePath)
	if err != nil {
		return errors.New(fmt.Sprintf("Error reading configuration file %s: %s", *s.Options.ConfigFilePath, err))
	}
	c := &Configuration{}
	if err := toml.Unmarshal(cb, c); err != nil {
		return errors.New(fmt.Sprintf("Error unmarshalling TOML configuration file %s: %s", *s.Options.ConfigFilePath,
			err))
	}

	if c.Proxy.Phishing == "" || c.Proxy.Target == "" {
		return errors.New(fmt.Sprintf("Missing phishing/destination from configuration!"))
	}

	// Listening
	if c.Proxy.IP == "" {
		c.Proxy.IP = DefaultIP
	}

	// Network Listener
	if c.Proxy.Listener == "" {
		c.Proxy.Listener = DefaultListener
	} else if !core.StringContains(strings.ToLower(c.Proxy.Listener), []string{"tcp", "tcp4", "tcp6"}) {
		c.Proxy.Listener = DefaultListener
	}

	if c.Proxy.Port == 0 {
		c.Proxy.Port = DefaultHTTPPort
		if c.TLS.Enabled {
			c.Proxy.Port = DefaultHTTPSPort
		}
	}

	// HTTPtoHTTPS
	if c.Proxy.HTTPtoHTTPS.Enabled {
		if c.Proxy.HTTPtoHTTPS.HTTPport == 0 {
			c.Proxy.HTTPtoHTTPS.HTTPport = DefaultHTTPPort
		}
	}

	//
	// Origins
	//

	// ExternalOriginPrefix must match the a-zA-Z0-9\- regex pattern
	if c.Origins.ExternalOriginPrefix != "" {
		m, err := regexp.MatchString("^[a-zA-Z0-9-]+$", c.Origins.ExternalOriginPrefix)
		if err != nil {
			return errors.New(fmt.Sprintf("Error matching ExternalOriginPrefix %s: %s", c.Origins.ExternalOriginPrefix, err))
		}

		if !m {
			return errors.New(fmt.Sprintf("Invalid ExternalOriginPrefix %s. It must match the a-zA-Z0-9\\- regex pattern.", c.Origins.ExternalOriginPrefix))
		}
	} else {
		c.Origins.ExternalOriginPrefix = "ext"
	}

	c.Origins.OriginsMapping = make(map[string]string)

	// Load TLS config
	c.Proxy.Protocol = "http://"

	if c.TLS.Enabled {

		// Load TLS Certificate
		c.TLS.CertificateContent = c.TLS.Certificate

		if !strings.HasPrefix(c.TLS.Certificate, "-----BEGIN CERTIFICATE-----\n") {
			er := errors.New(fmt.Sprintf("Error reading TLS cert %s: %s", c.TLS.Certificate, err))
			if _, err := os.Stat(c.TLS.CertificateContent); err == nil {
				crt, err := ioutil.ReadFile(c.TLS.CertificateContent)
				if err != nil {
					return er
				}
				c.TLS.CertificateContent = string(crt)
			} else {
				return er
			}
		}

		// Load TLS Root CA Certificate
		c.TLS.RootContent = c.TLS.Root
		if !strings.HasPrefix(c.TLS.Root, "-----BEGIN CERTIFICATE-----\n") {
			er := errors.New(fmt.Sprintf("Error reading TLS cert pool %s: %s", c.TLS.Root, err))
			if _, err := os.Stat(c.TLS.RootContent); err == nil {
				crtp, err := ioutil.ReadFile(c.TLS.RootContent)
				if err != nil {
					return er
				}
				c.TLS.RootContent = string(crtp)
			} else {
				return er
			}
		}

		// Load TLS Certificate Key
		c.TLS.KeyContent = c.TLS.Key
		if !strings.HasPrefix(c.TLS.Key, "-----BEGIN") {
			er := errors.New(fmt.Sprintf("Error reading TLS cert key %s: %s", c.TLS.Key, err))
			if _, err := os.Stat(c.TLS.KeyContent); err == nil {
				k, err := ioutil.ReadFile(c.TLS.KeyContent)
				if err != nil {
					return er
				}
				c.TLS.KeyContent = string(k)
			} else {
				return er
			}
		}

		c.Proxy.Protocol = "https://"

		c.TLS.MinVersion = strings.ToUpper(c.TLS.MinVersion)
		if !core.StringContains(c.TLS.MinVersion, []string{"SSL3.0", "TLS1.0", "TLS1.1", "TLS1.2", "TLS1.3"}) {
			// Fallback to TLS1
			c.TLS.MinVersion = "TLS1.0"
		}

		c.TLS.MaxVersion = strings.ToUpper(c.TLS.MaxVersion)
		if !core.StringContains(c.TLS.MaxVersion, []string{"SSL3.0", "TLS1.0", "TLS1.1", "TLS1.2", "TLS1.3"}) {
			// Fallback to TLS1.3
			c.TLS.MaxVersion = "TLS1.3"
		}

		c.TLS.RenegotiationSupport = strings.ToUpper(c.TLS.RenegotiationSupport)
		if !core.StringContains(c.TLS.RenegotiationSupport, []string{"NEVER", "ONCE", "FREELY"}) {
			// Fallback to NEVER
			c.TLS.RenegotiationSupport = "NEVER"
		}

	}

	//
	// Transforming rules
	//
	if c.Transform.Base64.Padding == nil {
		c.Transform.Base64.Padding = DefaultBase64Padding
	}

	if c.Transform.Response.SkipContentType == nil {
		c.Transform.Response.SkipContentType = DefaultSkipContentType
	}

	c.Transform.Request.SkipExtensions = []string{
		"ttf", "otf", "woff", "woff2", "eot", // fonts and images
		"ase", "art", "bmp", "blp", "cd5", "cit", "cpt", "cr2", "cut", "dds", "dib", "djvu", "egt", "exif", "gif",
		"gpl", "grf", "icns", "ico", "iff", "jng", "jpeg", "jpg", "jfif", "jp2", "jps", "lbm", "max", "miff", "mng",
		"msp", "nitf", "ota", "pbm", "pc1", "pc2", "pc3", "pcf", "pcx", "pdn", "pgm", "PI1", "PI2", "PI3", "pict",
		"pct", "pnm", "pns", "ppm", "psb", "psd", "pdd", "psp", "px", "pxm", "pxr", "qfx", "raw", "rle", "sct", "sgi",
		"rgb", "int", "bw", "tga", "tiff", "tif", "vtf", "xbm", "xcf", "xpm", "3dv", "amf", "ai", "awg", "cgm", "cdr",
		"cmx", "dxf", "e2d", "egt", "eps", "fs", "gbr", "odg", "svg", "stl", "vrml", "x3d", "sxd", "v2d", "vnd", "wmf",
		"emf", "art", "xar", "png", "webp", "jxr", "hdp", "wdp", "cur", "ecw", "iff", "lbm", "liff", "nrrd", "pam",
		"pcx", "pgf", "sgi", "rgb", "rgba", "bw", "int", "inta", "sid", "ras", "sun", "tga"}

	// Fix Craft config
	slice := c.Transform.Response.Add.Headers
	for s, header := range c.Transform.Response.Add.Headers {
		if header.Name == "" {
			slice = append(slice[:s], slice[s+1:]...)
		}
	}
	c.Transform.Response.Add.Headers = slice

	slice = c.Transform.Request.Add.Headers
	for s, header := range c.Transform.Request.Add.Headers {
		if header.Name == "" {
			slice = append(slice[:s], slice[s+1:]...)
		}
	}
	c.Transform.Request.Add.Headers = slice

	//
	// API control plane
	//
	c.Api.applyDefaults()
	if c.Recon.NodePath == "" {
		c.Recon.NodePath = "node"
	}
	if c.Recon.Script == "" {
		c.Recon.Script = "puppeteer/recon.js"
	}

	// Final Checks
	if err := c.DoChecks(); err != nil {
		return err
	}
	s.SwapConfig(c)
	return nil
}

func (s *Session) UpdateConfiguration(domains *[]string) (err error) {
	config := s.Config()

	//
	// Update config
	//
	// Disable crawler and update external domains
	config.Origins.ExternalOrigins = *domains
	config.Crawler.Enabled = false

	// Update TLS accordingly
	if !config.TLS.Expand {
		config.TLS.Root = config.TLS.RootContent
		config.TLS.Key = config.TLS.KeyContent
		config.TLS.Certificate = config.TLS.CertificateContent
	}

	newConf, err := toml.Marshal(config)
	if err != nil {
		return
	}

	return ioutil.WriteFile(*s.Options.ConfigFilePath, newConf, 0644)
}

// DoChecks runs all configuration validation/normalization checks.
func (c *Configuration) DoChecks() (err error) {

	// Check Redirect
	c.CheckRedirect()

	// Check Log
	err = c.CheckLog()
	if err != nil {
		return
	}

	// Check Tracking
	err = c.CheckTracking()
	if err != nil {
		return
	}

	// Check Static Server
	err = c.CheckStaticServer()
	if err != nil {
		return
	}

	return
}

// CheckRedirect checks the redirect rules and removes invalid ones.
func (c *Configuration) CheckRedirect() {
	var redirects []Redirect
	for _, drop := range c.Redirects {
		if drop.RedirectTo == "" {
			continue
		}

		if drop.Hostname == "" && drop.Path == "" && drop.Query == "" {
			continue
		}

		// Unset HTTPStatusCode will default to 302
		if drop.HTTPStatusCode == 0 {
			drop.HTTPStatusCode = 302
		}

		redirects = append(redirects, drop)
	}

	c.Redirects = redirects
}

// CheckLog checks the log configuration and disables it if the file is not accessible.
func (c *Configuration) CheckLog() (err error) {
	if !c.Log.Enabled {
		return
	}

	if c.Log.FilePath == "" {
		c.Log.FilePath = "muraena.log"
	}

	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(c.Log.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		c.Log.Enabled = false
		return errors.New(fmt.Sprintf("Error opening log file %s: %s", c.Log.FilePath, err))
	}
	defer f.Close()

	return
}

// CheckTracking checks the tracking configuration and disables it if the file is not accessible.
func (c *Configuration) CheckTracking() (err error) {
	if !c.Tracking.Enabled {
		return
	}

	return
}

// CheckStaticServer checks the static server configuration and disables it if the file is not accessible.
func (c *Configuration) CheckStaticServer() (err error) {
	if !c.StaticServer.Enabled {
		return
	}

	if c.StaticServer.LocalPath == "" {
		c.StaticServer.Enabled = false
		return errors.New(fmt.Sprintf("Error opening static server local path %s: %s", c.StaticServer.LocalPath, err))
	}

	if c.StaticServer.URLPath == "" {
		c.StaticServer.Enabled = false
		return errors.New(fmt.Sprintf("Error opening static server URL path %s: %s", c.StaticServer.URLPath, err))
	}

	return
}

// CloneConfig returns a deep copy of c.
func CloneConfig(c *Configuration) (*Configuration, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	var out Configuration
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
