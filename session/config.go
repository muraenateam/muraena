package session

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/evilsocket/islazy/tui"
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
)

// Configuration
type Configuration struct {
	Protocol       string `toml:"-"`
	SkipExtensions []string `toml:"-"`

	//
	// Proxy rules
	//
	Proxy struct {
		Phishing string `toml:"phishing"`
		Target   string `toml:"destination"`
		Listener struct {
			IP          string `toml:"IP"`
			Port        int    `toml:"port"`
			PortMap     string `toml:"portmapping"`
			HTTPtoHTTPS struct {
				Enabled  bool `toml:"enabled"`
				HTTPport int  `toml:"HTTPport"`
			} `toml:"HTTPtoHTTPS"`
		} `toml:"listener"`

		SkipContentType []string `toml:"skipContentType"`

		Transform struct {
			Base64 struct {
				Enabled bool     `toml:"enabled"`
				Padding []string `toml:"padding"`
			} `toml:"base64"`

			Request struct {
				Header []string `toml:"header"`
			} `toml:"request"`

			Response struct {
				Header []string   `toml:"header"`
				Custom [][]string `toml:"custom"`
			} `toml:"response"`
		} `toml:"transform"`

		Remove struct {
			Request struct {
				Header []string `toml:"header"`
			} `toml:"request"`

			Response struct {
				Header []string `toml:"header"`
			} `toml:"response"`
		} `toml:"remove"`

		Drop []struct {
			Url        string `toml:"url"`
			RedirectTo string `toml:"redirectTo"`
		} `toml:"drop"`

		Log struct {
			Enabled  bool   `toml:"enabled"`
			FilePath string `toml:"filePath"`
		} `toml:"log"`
	} `toml:"proxy"`

	//
	// TLS
	//
	TLS struct {
		Enabled            bool   `toml:"enabled"`
		Expand             bool   `toml:"expand"`
		Certificate        string `toml:"certificate"`
		CertificateContent string `toml:"-"`
		Key                string `toml:"key"`
		KeyContent         string `toml:"-"`
		Root               string `toml:"root"`
		RootContent        string `toml:"-"`
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
	// Tracking
	//
	Tracking struct {
		Enabled    bool   `toml:"enabled"`
		Type       string `toml:"type"`
		Identifier string `toml:"identifier"`
		Domain     string `toml:"domain"`
		IPSource   string `toml:"ipSource"`
		Regex      string `toml:"regex"`

		Urls struct {
			Credentials []string `toml:"credentials"`
			AuthSession []string `toml:"authSession"`
		} `toml:"urls"`
		Params   []string `toml:"params"`
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
	if s.Config.Proxy.Listener.IP == "" {
		s.Config.Proxy.Listener.IP = "0.0.0.0"
	}

	if s.Config.Proxy.Listener.Port == 0 {
		s.Config.Proxy.Listener.Port = 80
		if s.Config.TLS.Enabled {
			s.Config.Proxy.Listener.Port = 443
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

	return
}

func (s *Session) UpdateConfiguration(externalOrigins, subdomains, uniqueDomains *[]string) (err error) {
	config := s.Config


	// ASCII tables on the terminal
	columns := []string{"Domains", "#"}
	rows := [][]string{
		{"External domains", fmt.Sprintf("%v", len(*externalOrigins))},
		{"Subdomains", fmt.Sprintf("%v", len(*subdomains))},
		{"----------------", fmt.Sprintf("---")},
		{"Unique domains", fmt.Sprintf("%v", len(*uniqueDomains))},
	}

	tui.Table(os.Stdout, columns, rows)

	//
	// Update config
	//
	// Disable crawler and update external domains
	sort.Sort(sort.StringSlice(*externalOrigins))
	config.Crawler.ExternalOrigins = *externalOrigins
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
