package crawler

import (
	"regexp"
	"testing"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
)

var c *Crawler

// init test
func init() {
	log.Init(core.Options{Debug: &[]bool{true}[0]}, false, "")

	// LoadModules load modules
	c = &Crawler{
		Enabled: true,
	}
}

// TestModuleName ensures the module is the same, just in case :)
func TestCrawler_Name(t *testing.T) {
	module := "crawler"
	want := regexp.MustCompile(module)
	if !want.MatchString(Name) {
		t.Fatalf(`The module name does not match: %q != %q`, module, want)
	}
}

func TestCrawler_SimplifyDomains(t *testing.T) {
	c.Domains = []string{
		"a.com",
		"1.a.com",
		"2.a.com",
		"3.a.com",
		"4.a.com",
		"xyz.jkl.a.com",
		"b.com",
	}
	c.SimplifyDomains()
	if len(c.Domains) != 4 {
		t.Fatalf(`Number of simplified domains should be %d not %d`, 4, len(c.Domains))
	}
}
