package robotstxt

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	robotsCaseMatching = `user-agent: a
Disallow: /
user-agent: b
Disallow: /*
user-agent: c
Disallow: /fish
user-agent: d
Disallow: /fish*
user-agent: e
Disallow: /fish/
user-agent: f
Disallow: fish/
user-agent: g
Disallow: /*.php
user-agent: h
Disallow: /*.php$
user-agent: i
Disallow: /fish*.php`

	robotsCasePrecedence = `user-agent: a
Disallow: /
Allow: /p
user-agent: b
Disallow: /folder
Allow: /folder/
user-agent: c
Disallow: /*.htm
Allow: /page
user-agent: d
Disallow: /
Allow: /$
user-agent: e
Disallow: /
Allow: /$`
)

func TestGroupOrder(t *testing.T) {
	const robotsCaseOrder = `user-agent: googlebot-news
Disallow: /
user-agent: *
Disallow: /
user-agent: googlebot
Disallow: /`
	agents := []string{"Googlebot-News (Googlebot)", "Googlebot", "Googlebot-Image (Googlebot)", "Otherbot (web)", "Otherbot (News)"}
	paths := []string{"/1", "/3", "/3", "/2", "/2"}

	r, err := FromString(robotsCaseOrder)
	require.NoError(t, err)
	for i, a := range agents {
		expectAccess(t, r, false, paths[i], a)
	}
}

func TestSitemaps(t *testing.T) {
	const robotsCaseSitemaps = `sitemap: http://test.com/a
user-agent: a
disallow: /c
sitemap: http://test.com/b
user-agent: b
disallow: /d
user-agent: e
sitemap: http://test.com/c
user-agent: f
disallow: /g`

	r, err := FromString(robotsCaseSitemaps)
	require.NoError(t, err)
	if len(r.Sitemaps) != 3 {
		for i, s := range r.Sitemaps {
			t.Logf("Sitemap %d: %s", i, s)
		}
		t.Fatalf("Expected 3 sitemaps, got %d:\n%v", len(r.Sitemaps), r.Sitemaps)
	}
}

func TestCrawlDelays(t *testing.T) {
	const robotsCaseDelays = `useragent: a
# some comment : with colon
disallow: /c
user-agent: b
crawldelay: 3.5
disallow: /d
user-agent: e
sitemap: http://test.com/c
user-agent: f
disallow: /g
crawl-delay: 5`

	r, err := FromString(robotsCaseDelays)
	require.NoError(t, err)
	if len(r.Sitemaps) != 1 {
		t.Fatalf("Expected 1 sitemaps, got %d", len(r.Sitemaps))
	}
	if g := r.groups["b"]; g.CrawlDelay != time.Duration(3.5*float64(time.Second)) {
		t.Fatalf("Expected crawl delay of 3.5 for group 2, got %v", g.CrawlDelay)
	}
	if g := r.groups["f"]; g.CrawlDelay != (5 * time.Second) {
		t.Fatalf("Expected crawl delay of 5 for group 3, got %v", g.CrawlDelay)
	}
}

func TestWildcards(t *testing.T) {
	const robotsCaseWildcards = `user-agent: *
Disallow: /path*l$`

	r, err := FromString(robotsCaseWildcards)
	require.NoError(t, err)
	assert.Equal(t, "/path.*l$", r.groups["*"].rules[0].pattern.String())
}

func TestURLMatching(t *testing.T) {
	var ok bool

	cases := map[string][]string{
		"a": {
			"/",
			"/test",
			"",
			"/path/to/whatever",
		},
		"b": {
			"/",
			"/test",
			"",
			"/path/to/whatever",
		},
		"c": {
			"/fish",
			"/fish.html",
			"/fish/salmon.html",
			"/fishheads",
			"/fishheads/yummy.html",
			"/fish.php?id=anything",
			"^/Fish.asp",
			"^/catfish",
			"^/?id=fish",
		},
		"d": {
			"/fish",
			"/fish.html",
			"/fish/salmon.html",
			"/fishheads",
			"/fishheads/yummy.html",
			"/fish.php?id=anything",
			"^/Fish.asp",
			"^/catfish",
			"^/?id=fish",
		},
		"e": {
			"/fish/",
			"/fish/?id=anything",
			"/fish/salmon.htm",
			"^/fish",
			"^/fish.html",
			"^/Fish/Salmon.asp",
		},
		"f": {
			"/fish/",
			"/fish/?id=anything",
			"/fish/salmon.htm",
			"^/fish",
			"^/fish.html",
			"^/Fish/Salmon.asp",
		},
		"g": {
			"/filename.php",
			"/folder/filename.php",
			"/folder/filename.php?parameters",
			"/folder/any.php.file.html",
			"/filename.php/",
			"^/",
			"^/windows.PHP",
		},
		"h": {
			"/filename.php",
			"/folder/filename.php",
			"^/filename.php?parameters",
			"^/filename.php/",
			"^/filename.php5",
			"^/windows.PHP",
		},
		"i": {
			"/fish.php",
			"/fishheads/catfish.php?parameters",
			"^/Fish.PHP",
		},
	}
	r, err := FromString(robotsCaseMatching)
	require.NoError(t, err)
	for k, ar := range cases {
		for _, p := range ar {
			ok = strings.HasPrefix(p, "^")
			if ok {
				p = p[1:]
			}
			if allow := r.TestAgent(p, k); allow != ok {
				t.Errorf("Agent %s, path %s, expected %v, got %v", k, p, ok, allow)
			}
		}
	}
}

func TestURLPrecedence(t *testing.T) {
	var ok bool

	cases := map[string][]string{
		"a": {
			"/page",
			"^/test",
		},
		"b": {
			"/folder/page",
			"^/folder1",
			"^/folder.htm",
		},
		"c": {
			"^/page.htm",
			"/page1.asp",
		},
		"d": {
			"/",
			"^/index",
		},
		"e": {
			"^/page.htm",
			"/",
		},
	}
	r, err := FromString(robotsCasePrecedence)
	require.NoError(t, err)
	for k, ar := range cases {
		for _, p := range ar {
			ok = !strings.HasPrefix(p, "^")
			if !ok {
				p = p[1:]
			}
			if allow := r.TestAgent(p, k); allow != ok {
				t.Errorf("Agent %s, path %s, expected %v, got %v", k, p, ok, allow)
			}
		}
	}
}
