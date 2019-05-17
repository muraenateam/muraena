package crawler

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ditashi/jsbeautifier-go/jsbeautifier"
	"github.com/evilsocket/islazy/tui"
	"github.com/gocolly/colly"
	"gopkg.in/resty.v1"
	"mvdan.cc/xurls"

	"github.com/muraenateam/muraena/proxy"
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
}

var (
	crawledDomains            []string
	subdomains, uniqueDomains []string
	externalOrigins           []string
	baseDom                   string
	waitGroup                 sync.WaitGroup

	discoveredJsUrls []string
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
func (module *Crawler) Prompt(what string) {
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

	// Armor domains
	config.ExternalOrigins = proxy.ArmorDomain(config.ExternalOrigins)

	if !m.Enabled {
		m.Debug("is disabled")
		return
	}

	m.explore()
	waitGroup.Wait()
	config.ExternalOrigins = proxy.ArmorDomain(crawledDomains)

	// save new config file with externalDomains to prevent crawling at next start
	//save new config file with externalDomains to prevent crawling at next start
	m.Info("Domain crawling stats:")
	err = s.UpdateConfiguration(&externalOrigins, &subdomains, &uniqueDomains)

	return
}

func (module *Crawler) explore() {

	var config *session.Configuration
	config = module.Session.Config

	module.Info("Starting exploration of %s (crawlDepth:%d crawlMaxReq: %d), just a few seconds...",
		config.Proxy.Target, module.Depth, module.UpTo)

	c := colly.NewCollector(
		colly.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X x.y; rv:10.0) Gecko/20100101 Firefox/10.0"),

		// MaxDepth is by default 1, so only the links on the scraped page
		// is visited, and no further links are followed
		colly.MaxDepth(module.Depth), // first page and also links in second pages
		//colly.AllowedDomains(Config.Target),
	)

	numVisited := 0
	c.OnRequest(func(r *colly.Request) {
		numVisited++
		if numVisited > module.UpTo {
			//module.Info("Ending exploration...")
			r.Abort()
			return
		}
	})

	c.OnHTML("script[src]", func(e *colly.HTMLElement) {
		res := e.Attr("src")
		if module.appendExternalDomain(res, &crawledDomains) {
			// if it is a script from an external domain, make sure to fetch it
			// beautify it and see it we need to replace things
			waitGroup.Add(1)
			go module.fetchJS(&waitGroup, res, config.Proxy.Target, &crawledDomains)
		}

	})

	// all other tags with src attribute (img/video/iframe/etc..)
	c.OnHTML("[src]", func(e *colly.HTMLElement) {
		res := e.Attr("src")
		module.appendExternalDomain(res, &crawledDomains)
	})

	c.OnHTML("link[href]", func(e *colly.HTMLElement) {
		res := e.Attr("href")
		module.appendExternalDomain(res, &crawledDomains)
	})

	c.OnHTML("meta[content]", func(e *colly.HTMLElement) {
		res := e.Attr("content")
		module.appendExternalDomain(res, &crawledDomains)
	})

	// Callback for links on scraped pages
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		res := e.Attr("href")
		module.appendExternalDomain(res, &crawledDomains)

		// crawl
		if err := c.Visit(e.Request.AbsoluteURL(res)); err != nil {
			// module.Debug("[Colly Visit]%s", err)
		}
	})

	if err := c.Limit(&colly.LimitRule{DomainGlob: "*", RandomDelay: 500 * time.Millisecond}); err != nil {
		module.Warning("[Colly Limit]%s", err)
	}

	c.OnResponse(func(r *colly.Response) {})

	c.OnRequest(func(r *colly.Request) {})

	dest := fmt.Sprintf("%s%s", config.Protocol, config.Proxy.Target)
	err := c.Visit(dest)
	if err != nil {
		module.Info("Exploration error visiting %s: %s", dest, tui.Red(err.Error()))
	}
}

func (module *Crawler) fetchJS(waitGroup *sync.WaitGroup, res string, dest string, crawledDomains *[]string) {

	defer waitGroup.Done()

	u, _ := url.Parse(res)
	if u.Scheme == "" {
		u.Scheme = "https://"
		res = "https:" + res
	}
	nu := fmt.Sprintf("%s%s", u.Host, u.Path)
	if !Contains(&discoveredJsUrls, nu) {
		discoveredJsUrls = append(discoveredJsUrls, nu)
		module.Info("Fetching new JS URL: %s", nu)

		resp, err := resty.R().Get(res)
		if err != nil {
			module.Error("Error fetching JS at %s: %s", res, err)
		}
		body := string(resp.Body())

		opts := jsbeautifier.DefaultOptions()
		beautyBody, err := jsbeautifier.Beautify(&body, opts)
		if err != nil {
			module.Error("Error beautifying JS at %s", res)
		}

		jsUrls := xurls.Strict().FindAllString(beautyBody, -1)
		if len(jsUrls) > 0 && len(jsUrls) < 100 { // prevent cases where we have a lots of domains
			for _, jsURL := range jsUrls {
				module.appendExternalDomain(jsURL, crawledDomains)
			}
			module.Info("Domains found in JS at %s: %d", res, len(jsUrls))
		}
	}
}

func (module *Crawler) appendExternalDomain(res string, crawledDomains *[]string) bool {
	if strings.HasPrefix(res, "//") || strings.HasPrefix(res, "https://") || strings.HasPrefix(res, "http://") {
		u, err := url.Parse(res)
		if err != nil {
			module.Error("url.Parse error, skipping external domain %s: %s", res, err)
			return false
		}
		// update the crawledDomains after doing some minimal checks that might happen from xurls when
		// parsing urls from JS files
		if len(u.Host) > 2 && (strings.Contains(u.Host, ".") || strings.Contains(u.Host, ":")) {
			*crawledDomains = append(*crawledDomains, u.Host)
		}

		return true
	}
	return false
}
