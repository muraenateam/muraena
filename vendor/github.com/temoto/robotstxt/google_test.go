package robotstxt

import (
	"strings"
	"testing"
	"time"
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

	if r, e := FromString(robotsCaseOrder); e != nil {
		t.Fatal(e)
	} else {
		for i, a := range agents {
			ExpectDisallow(t, r, paths[i], a)
		}
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

	if r, e := FromString(robotsCaseSitemaps); e != nil {
		t.Fatal(e)
	} else {
		if len(r.Sitemaps) != 3 {
			for i, s := range r.Sitemaps {
				t.Logf("Sitemap %d: %s", i, s)
			}
			t.Fatalf("Expected 3 sitemaps, got %d:\n%v", len(r.Sitemaps), r.Sitemaps)
		}
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

	if r, e := FromString(robotsCaseDelays); e != nil {
		t.Fatal(e)
	} else {
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
}

func TestWildcards(t *testing.T) {
	const robotsCaseWildcards = `user-agent: *
Disallow: /path*l$`

	if r, e := FromString(robotsCaseWildcards); e != nil {
		t.Fatal(e)
	} else {
		if s := r.groups["*"].rules[0].pattern.String(); s != "/path.*l$" {
			t.Fatalf("Expected pattern to be /path.*l$, got %s", s)
		}
	}
}

func TestURLMatching(t *testing.T) {
	var ok bool

	cases := map[string][]string{
		"a": []string{
			"/",
			"/test",
			"",
			"/path/to/whatever",
		},
		"b": []string{
			"/",
			"/test",
			"",
			"/path/to/whatever",
		},
		"c": []string{
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
		"d": []string{
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
		"e": []string{
			"/fish/",
			"/fish/?id=anything",
			"/fish/salmon.htm",
			"^/fish",
			"^/fish.html",
			"^/Fish/Salmon.asp",
		},
		"f": []string{
			"/fish/",
			"/fish/?id=anything",
			"/fish/salmon.htm",
			"^/fish",
			"^/fish.html",
			"^/Fish/Salmon.asp",
		},
		"g": []string{
			"/filename.php",
			"/folder/filename.php",
			"/folder/filename.php?parameters",
			"/folder/any.php.file.html",
			"/filename.php/",
			"^/",
			"^/windows.PHP",
		},
		"h": []string{
			"/filename.php",
			"/folder/filename.php",
			"^/filename.php?parameters",
			"^/filename.php/",
			"^/filename.php5",
			"^/windows.PHP",
		},
		"i": []string{
			"/fish.php",
			"/fishheads/catfish.php?parameters",
			"^/Fish.PHP",
		},
	}
	if r, e := FromString(robotsCaseMatching); e != nil {
		t.Fatal(e)
	} else {
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
}

func TestURLPrecedence(t *testing.T) {
	var ok bool

	cases := map[string][]string{
		"a": []string{
			"/page",
			"^/test",
		},
		"b": []string{
			"/folder/page",
			"^/folder1",
			"^/folder.htm",
		},
		"c": []string{
			"^/page.htm",
			"/page1.asp",
		},
		"d": []string{
			"/",
			"^/index",
		},
		"e": []string{
			"^/page.htm",
			"/",
		},
	}
	if r, e := FromString(robotsCasePrecedence); e != nil {
		t.Fatal(e)
	} else {
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
}

func getIndexInSlice(ar []*Group, g *Group) int {
	for i, v := range ar {
		if v == g {
			return i
		}
	}
	return -1
}
