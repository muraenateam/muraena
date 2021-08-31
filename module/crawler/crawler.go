package crawler

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ditashi/jsbeautifier-go/jsbeautifier"
	"github.com/evilsocket/islazy/tui"
	"github.com/gocolly/colly/v2"
	"github.com/icza/abcsort"
	"gopkg.in/resty.v1"
	"mvdan.cc/xurls/v2"

	"github.com/muraenateam/muraena/core/proxy"
	"github.com/muraenateam/muraena/session"
)

const (
	// Name of this module
	Name = "crawler"

	// Description of this module
	Description = "Crawls the target domain in order to retrieve most of the target external origins"

	// Author of this module
	Author = "Muraena Team"
)

// Crawler module
type Crawler struct {
	session.SessionModule

	Enabled bool
	Depth   int
	UpTo    int

	Domains []string
}

var (
	discoveredJsUrls []string
	waitGroup        sync.WaitGroup
	rgxURLS          *regexp.Regexp
)

// Name returns the module name
func (module *Crawler) Name() string {
	return Name
}

// Description returns the module description
func (module *Crawler) Description() string {
	return Description
}

// Author returns the module author
func (module *Crawler) Author() string {
	return Author
}

// Prompt prints module status based on the provided parameters
func (module *Crawler) Prompt() {
	module.Raw("No options are available for this module")
}

// Load configures the module by initializing its main structure and variables
func Load(s *session.Session) (m *Crawler, err error) {

	config := s.Config.Crawler
	m = &Crawler{
		SessionModule: session.NewSessionModule(Name, s),
		Enabled:       config.Enabled,
		UpTo:          config.UpTo,
		Depth:         config.Depth,
	}

	rgxURLS = xurls.Strict()

	// Armor domains
	config.ExternalOrigins = proxy.ArmorDomain(config.ExternalOrigins)
	if !m.Enabled {
		m.Debug("is disabled")
		return
	}

	m.explore()
	m.SimplifyDomains()
	config.ExternalOrigins = m.Domains

	m.Info("Domain crawling stats:")
	err = s.UpdateConfiguration(&m.Domains)

	return
}

func (module *Crawler) explore() {
	waitGroup.Wait()

	// Custom client
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	collyClient := &http.Client{Transport: tr}

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2228.0 Safari/537.36"),
		// MaxDepth is by default 1, so only the links on the scraped page are visited,
		// and no further links are followed
		colly.MaxDepth(module.Depth),
		colly.CheckHead(),
	)

	c.SetClient(collyClient)

	numVisited := 0
	c.OnRequest(func(r *colly.Request) {
		numVisited++
		if numVisited > module.UpTo {
			r.Abort()
			return
		}
	})

	c.OnHTML("script[src]", func(e *colly.HTMLElement) {
		res := e.Attr("src")
		if module.appendExternalDomain(res) {
			// if it is a script from an external domain, make sure to fetch it
			// beautify it and see it we need to replace things
			waitGroup.Add(1)
			go module.fetchJS(&waitGroup, res)
		}

	})

	// all other tags with src attribute (img/video/iframe/etc..)
	c.OnHTML("[src]", func(e *colly.HTMLElement) {
		res := e.Attr("src")
		module.appendExternalDomain(res)
	})

	c.OnHTML("link[href]", func(e *colly.HTMLElement) {
		res := e.Attr("href")
		module.appendExternalDomain(res)
	})

	c.OnHTML("meta[content]", func(e *colly.HTMLElement) {
		res := e.Attr("content")
		module.appendExternalDomain(res)
	})

	// Callback for links on scraped pages
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		res := e.Attr("href")
		module.appendExternalDomain(res)
	})

	if err := c.Limit(&colly.LimitRule{DomainGlob: "*", RandomDelay: 500 * time.Millisecond}); err != nil {
		module.Warning("[Colly Limit]%s", err)
	}

	c.OnResponse(func(r *colly.Response) {})

	c.OnRequest(func(r *colly.Request) {})

	var config *session.Configuration
	config = module.Session.Config

	module.Info("Starting exploration of %s (crawlDepth:%d crawlMaxReq: %d), just a few seconds...",
		config.Proxy.Target, module.Depth, module.UpTo)

	dest := fmt.Sprintf("%s%s", config.Protocol, config.Proxy.Target)
	err := c.Visit(dest)
	if err != nil {
		module.Info("Exploration error visiting %s: %s", dest, tui.Red(err.Error()))
	}
}

func (module *Crawler) fetchJS(waitGroup *sync.WaitGroup, res string) {

	defer waitGroup.Done()

	u, _ := url.Parse(res)
	if u.Scheme == "" {
		u.Scheme = "https://"
		res = "https:" + res
	}
	nu := fmt.Sprintf("%s%s", u.Host, u.Path)
	if !Contains(&discoveredJsUrls, nu) {
		discoveredJsUrls = append(discoveredJsUrls, nu)
		module.Debug("New JS: %s", nu)
		resp, err := resty.R().Get(res)
		if err != nil {
			module.Error("Error fetching JS at %s: %s", res, err)
			return
		}

		body := string(resp.Body())
		opts := jsbeautifier.DefaultOptions()
		beautyBody, err := jsbeautifier.Beautify(&body, opts)
		if err != nil {
			module.Error("Error beautifying JS at %s", res)
			return
		}

		jsUrls := rgxURLS.FindAllString(beautyBody, -1)
		if len(jsUrls) > 0 && len(jsUrls) < 100 { // prevent cases where we have a lots of domains
			for _, jsURL := range jsUrls {
				module.appendExternalDomain(jsURL)
			}
			module.Info("%d domain(s) found in JS at %s", len(jsUrls), res)
		}
	}
}

func (module *Crawler) appendExternalDomain(res string) bool {
	if strings.HasPrefix(res, "//") || strings.HasPrefix(res, "https://") || strings.HasPrefix(res, "http://") {
		u, err := url.Parse(res)
		if err != nil {
			module.Error("url.Parse error, skipping external domain %s: %s", res, err)
			return false
		}
		// update the Domains after doing some minimal checks that might happen from xurls when
		// parsing urls from JS files
		if len(u.Host) > 2 && (strings.Contains(u.Host, ".") || strings.Contains(u.Host, ":")) {
			module.Domains = append(module.Domains, u.Host)
		}

		return true
	}

	return false
}

func reverseString(ss []string) []string {
	last := len(ss) - 1
	for i := 0; i < len(ss)/2; i++ {
		ss[i], ss[last-i] = ss[last-i], ss[i]
	}

	return ss
}

// SimplifyDomains simplifies the Domains slice by grouping subdomains of 3rd and 4th level as *.<domain>
func (module *Crawler) SimplifyDomains() {

	var domains []string
	for _, d := range module.Domains {

		host := strings.TrimSpace(d)
		hostParts := reverseString(strings.Split(host, "."))

		switch len(hostParts) {
		case 3:
			host = fmt.Sprintf("*.%s.%s", hostParts[1], hostParts[0])
		case 4:
			host = fmt.Sprintf("*.%s.%s.%s", hostParts[2], hostParts[1], hostParts[0])

		default:
			// Don't do anything, more than 3rd level is too much
		}

		domains = append(domains, host)
	}

	sorter := abcsort.New("*")
	domains = proxy.ArmorDomain(domains)
	sorter.Strings(domains)

	module.Domains = domains
}
