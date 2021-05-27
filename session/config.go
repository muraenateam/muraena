package session

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"github.com/muraenateam/muraena/core"
)

const (
	DefaultIP        = "0.0.0.0"
	DefaultListener  = "tcp"
	DefaultHTTPPort  = 80
	DefaultHTTPSPort = 443
)

// Configuration
type Configuration struct {
	Protocol       string   `toml:"-"`
	SkipExtensions []string `toml:"-"`

	//
	// Proxy rules
	//
	Proxy struct {
		Phishing string `toml:"phishing"`
		Target   string `toml:"destination"`
		IP       string `toml:"IP"`
		Listener string `toml:"listener"`
		Port     int    `toml:"port"`
		PortMap  string `toml:"portmapping"`

		HTTPtoHTTPS struct {
			Enabled  bool `toml:"enabled"`
			HTTPport int  `toml:"HTTPport"`
		} `toml:"HTTPtoHTTPS"`
	} `toml:"proxy"`

	//
	// Transforming rules
	//
	Transform struct {
		Base64 struct {
			Enabled bool     `toml:"enabled"`
			Padding []string `toml:"padding"`
		} `toml:"base64"`

		SkipContentType []string `toml:"skipContentType"`

		Request struct {
			Headers []string `toml:"headers"`
		} `toml:"request"`

		Response struct {
			Headers []string   `toml:"headers"`
			Custom  [][]string `toml:"content"`
		} `toml:"response"`
	} `toml:"transform"`

	//
	// Wiping rules
	//
	Remove struct {
		Request struct {
			Headers []string `toml:"headers"`
		} `toml:"request"`

		Response struct {
			Headers []string `toml:"headers"`
		} `toml:"response"`
	} `toml:"remove"`

	//
	// Crafting rules
	// TODO: Merge this with Wiping rule in some standard approach
	//
	Craft struct {
		Add struct {
			Request struct {
				Headers []struct {
					Name  string `toml:"name"`
					Value string `toml:"value"`
				} `toml:"headers"`
			} `toml:"request"`

			Response struct {
				Headers []struct {
					Name  string `toml:"name"`
					Value string `toml:"value"`
				} `toml:"headers"`
			} `toml:"response"`
		} `toml:"add"`
	} `toml:"craft"`

	//
	// Redirection rules
	//
	Drop []struct {
		Path       string `toml:"path"`
		RedirectTo string `toml:"redirectTo"`
	} `toml:"drop"`

	//
	// Logging
	//
	Log struct {
		Enabled  bool   `toml:"enabled"`
		FilePath string `toml:"filePath"`
	} `toml:"log"`

	//
	// DB (Redis)
	//
	Redis struct {
		Host     string `toml:"host"`
		Port     int    `toml:"port"`
		Password string `toml:"password"`
	} `toml:"redis"`

	//
	// TLS
	//
	TLS struct {
		Enabled     bool   `toml:"enabled"`
		Expand      bool   `toml:"expand"`
		Certificate string `toml:"certificate"`
		Key         string `toml:"key"`
		Root        string `toml:"root"`

		CertificateContent string `toml:"-"`
		KeyContent         string `toml:"-"`
		RootContent        string `toml:"-"`

		// Minimum supported TLS version: SSL3, TLS1, TLS1.1, TLS1.2, TLS1.3
		MinVersion               string `toml:"minVersion"`
		PreferServerCipherSuites bool   `toml:"preferServerCipherSuites"`
		SessionTicketsDisabled   bool   `toml:"SessionTicketsDisabled"`
		InsecureSkipVerify       bool   `toml:"insecureSkipVerify"`
		RenegotiationSupport     string `toml:"renegotiationSupport"`
	} `toml:"tls"`

	//
	// Crawler & Origins
	//

	Crawler struct {
		Enabled bool `toml:"enabled"`
		Depth   int  `toml:"depth"`
		UpTo    int  `toml:"upto"`

		ExternalOriginPrefix string            `toml:"externalOriginPrefix"`
		ExternalOrigins      []string          `toml:"externalOrigins"`
		OriginsMapping       map[string]string `toml:"-"`
	} `toml:"crawler"`

	//
	// Necrobrowser
	//
	NecroBrowser struct {
		Enabled  bool   `toml:"enabled"`
		Endpoint string `toml:"endpoint"`
		Profile  string `toml:"profile"`

		Keepalive struct {
			Enabled bool `toml:"enabled"`
			Minutes int  `toml:"minutes"`
		} `toml:"keepalive"`

		Trigger struct {
			Type   string   `toml:"type"`
			Values []string `toml:"values"`
			Delay  int      `toml:"delay"`
		} `toml:"trigger"`
	} `toml:"necrobrowser"`

	//
	// Static Server
	//
	StaticServer struct {
		Enabled   bool   `toml:"enabled"`
		Port      int    `toml:"port"`
		LocalPath string `toml:"localPath"`
		URLPath   string `toml:"urlPath"`
	} `toml:"staticServer"`

	//
	// Watchdog
	//
	Watchdog struct {
		Enabled bool   `toml:"enabled"`
		Dynamic bool   `toml:"dynamic"`
		Rules   string `toml:"rules"`
		GeoDB   string `toml:"geoDB"`
	} `toml:"watchdog"`

	//
	// Tracking
	//
	Tracking struct {
		Enabled    bool   `toml:"enabled"`
		Type       string `toml:"type"`
		Identifier string `toml:"identifier"`
		Header     string `toml:"header"`
		Landing    string `toml:"landing"`
		Domain     string `toml:"domain"`
		IPSource   string `toml:"ipSource"`
		Regex      string `toml:"regex"`
		RedirectTo string `toml:"redirectTo"`

		Urls struct {
			Credentials []string `toml:"credentials"`
			AuthSession []string `toml:"authSession"`
		} `toml:"urls"`

		Patterns []struct {
			Label    string `toml:"label"`
			Matching string `toml:"matching"`
			Start    string `toml:"start"`
			End      string `toml:"end"`
		} `toml:"patterns"`
	} `toml:"tracking"`
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

	// Load TLS config
	s.Config.Protocol = "http://"

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

		s.Config.Protocol = "https://"

		s.Config.TLS.MinVersion = strings.ToUpper(s.Config.TLS.MinVersion)
		if !core.StringContains(s.Config.TLS.MinVersion, []string{"SSL3.0", "TLS1.0", "TLS1.1", "TLS1.2", "TLS1.3"}) {
			// Fallback to TLS1
			s.Config.TLS.MinVersion = "TLS1.0"
		}

		s.Config.TLS.RenegotiationSupport = strings.ToUpper(s.Config.TLS.RenegotiationSupport)
		if !core.StringContains(s.Config.TLS.RenegotiationSupport, []string{"NEVER", "ONCE", "FREELY"}) {
			// Fallback to NEVER
			s.Config.TLS.RenegotiationSupport = "NEVER"
		}

	}

	s.Config.Crawler.OriginsMapping = make(map[string]string)

	s.Config.SkipExtensions = []string{
		"ttf", "otf", "woff", "woff2", "eot", //fonts and images
		"ase", "art", "bmp", "blp", "cd5", "cit", "cpt", "cr2", "cut", "dds", "dib", "djvu", "egt", "exif", "gif",
		"gpl", "grf", "icns", "ico", "iff", "jng", "jpeg", "jpg", "jfif", "jp2", "jps", "lbm", "max", "miff", "mng",
		"msp", "nitf", "ota", "pbm", "pc1", "pc2", "pc3", "pcf", "pcx", "pdn", "pgm", "PI1", "PI2", "PI3", "pict",
		"pct", "pnm", "pns", "ppm", "psb", "psd", "pdd", "psp", "px", "pxm", "pxr", "qfx", "raw", "rle", "sct", "sgi",
		"rgb", "int", "bw", "tga", "tiff", "tif", "vtf", "xbm", "xcf", "xpm", "3dv", "amf", "ai", "awg", "cgm", "cdr",
		"cmx", "dxf", "e2d", "egt", "eps", "fs", "gbr", "odg", "svg", "stl", "vrml", "x3d", "sxd", "v2d", "vnd", "wmf",
		"emf", "art", "xar", "png", "webp", "jxr", "hdp", "wdp", "cur", "ecw", "iff", "lbm", "liff", "nrrd", "pam",
		"pcx", "pgf", "sgi", "rgb", "rgba", "bw", "int", "inta", "sid", "ras", "sun", "tga"}

	// Fix Craft config
	slice := s.Config.Craft.Add.Response.Headers
	for s, header := range s.Config.Craft.Add.Response.Headers {
		if header.Name == "" {
			slice = append(slice[:s], slice[s+1:]...)
		}
	}
	s.Config.Craft.Add.Response.Headers = slice

	slice = s.Config.Craft.Add.Request.Headers
	for s, header := range s.Config.Craft.Add.Request.Headers {
		if header.Name == "" {
			slice = append(slice[:s], slice[s+1:]...)
		}
	}
	s.Config.Craft.Add.Request.Headers = slice

	return
}

func (s *Session) UpdateConfiguration(domains *[]string) (err error) {
	config := s.Config

	//
	// Update config
	//
	// Disable crawler and update external domains
	config.Crawler.ExternalOrigins = *domains
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
