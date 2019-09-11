package robotstxt

import (
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatus(t *testing.T) {
	t.Parallel()
	type tcase struct {
		code  int
		allow bool
	}
	cases := []tcase{
		{200, true},
		{400, true},
		{401, true},
		{403, true},
		{404, true},
		{500, false},
		{502, false},
		{503, false},
		{504, false},
	}
	for _, c := range cases {
		t.Run(strconv.Itoa(c.code), func(t *testing.T) {
			r, err := FromResponse(newHttpResponse(c.code, ""))
			require.NoError(t, err)
			expectAll(t, r, c.allow)

			r, err = FromStatusAndString(c.code, "")
			require.NoError(t, err)
			expectAll(t, r, c.allow)
		})
	}
}

func TestFromStringDisallowAll(t *testing.T) {
	r, err := FromString("User-Agent: *\r\nDisallow: /\r\n")
	require.NoError(t, err)
	expectAll(t, r, false)
}

func TestFromString002(t *testing.T) {
	t.Parallel()
	r, err := FromString("User-Agent: *\r\nDisallow: /account\r\n")
	require.NoError(t, err)
	expectAllAgents(t, r, true, "/foobar")
	expectAllAgents(t, r, false, "/account")
	expectAllAgents(t, r, false, "/account/sub")
}

const robotsText001 = "User-agent: * \nDisallow: /administrator/\nDisallow: /cache/\nDisallow: /components/\nDisallow: /editor/\nDisallow: /forum/\nDisallow: /help/\nDisallow: /images/\nDisallow: /includes/\nDisallow: /language/\nDisallow: /mambots/\nDisallow: /media/\nDisallow: /modules/\nDisallow: /templates/\nDisallow: /installation/\nDisallow: /getcid/\nDisallow: /tooltip/\nDisallow: /getuser/\nDisallow: /download/\nDisallow: /index.php?option=com_phorum*,quote=1\nDisallow: /index.php?option=com_phorum*phorum_query=search\nDisallow: /index.php?option=com_phorum*,newer\nDisallow: /index.php?option=com_phorum*,older\n\nUser-agent: Yandex\nAllow: /\nSitemap: http://www.pravorulya.com/sitemap.xml\nSitemap: http://www.pravorulya.com/sitemap1.xml"

func TestFromString003(t *testing.T) {
	t.Parallel()
	r, err := FromString(robotsText001)
	require.NoError(t, err)
	expectAllAgents(t, r, false, "/administrator/")
	expectAllAgents(t, r, true, "/paruram")
}

func TestInvalidEncoding(t *testing.T) {
	// Invalid UTF-8 encoding should not break parser.
	_, err := FromString("User-agent: H\xef\xbf\xbdm�h�kki\nDisallow: *")
	require.NoError(t, err)
}

// http://www.google.com/robots.txt on Wed, 12 Jan 2011 12:22:20 GMT
const robotsGoogle = ("User-agent: *\nDisallow: /search\nDisallow: /groups\nDisallow: /images\nDisallow: /catalogs\nDisallow: /catalogues\nDisallow: /news\nAllow: /news/directory\nDisallow: /nwshp\nDisallow: /setnewsprefs?\nDisallow: /index.html?\nDisallow: /?\nDisallow: /addurl/image?\nDisallow: /pagead/\nDisallow: /relpage/\nDisallow: /relcontent\nDisallow: /imgres\nDisallow: /imglanding\nDisallow: /keyword/\nDisallow: /u/\nDisallow: /univ/\nDisallow: /cobrand\nDisallow: /custom\nDisallow: /advanced_group_search\nDisallow: /googlesite\nDisallow: /preferences\nDisallow: /setprefs\nDisallow: /swr\nDisallow: /url\nDisallow: /default\nDisallow: /m?\nDisallow: /m/?\nDisallow: /m/blogs?\nDisallow: /m/directions?\nDisallow: /m/ig\nDisallow: /m/images?\nDisallow: /m/local?\nDisallow: /m/movies?\nDisallow: /m/news?\nDisallow: /m/news/i?\nDisallow: /m/place?\nDisallow: /m/products?\nDisallow: /m/products/\nDisallow: /m/setnewsprefs?\nDisallow: /m/search?\nDisallow: /m/swmloptin?\nDisallow: /m/trends\nDisallow: /m/video?\nDisallow: /wml?\nDisallow: /wml/?\nDisallow: /wml/search?\nDisallow: /xhtml?\nDisallow: /xhtml/?\nDisallow: /xhtml/search?\nDisallow: /xml?\nDisallow: /imode?\nDisallow: /imode/?\nDisallow: /imode/search?\nDisallow: /jsky?\nDisallow: /jsky/?\nDisallow: /jsky/search?\nDisallow: /pda?\nDisallow: /pda/?\nDisallow: /pda/search?\nDisallow: /sprint_xhtml\nDisallow: /sprint_wml\nDisallow: /pqa\nDisallow: /palm\nDisallow: /gwt/\nDisallow: /purchases\nDisallow: /hws\nDisallow: /bsd?\nDisallow: /linux?\nDisallow: /mac?\nDisallow: /microsoft?\nDisallow: /unclesam?\nDisallow: /answers/search?q=\nDisallow: /local?\nDisallow: /local_url\nDisallow: /froogle?\nDisallow: /products?\nDisallow: /products/\nDisallow: /froogle_\nDisallow: /product_\nDisallow: /products_\nDisallow: /products;\nDisallow: /print\nDisallow: /books\nDisallow: /bkshp?q=\nAllow: /booksrightsholders\nDisallow: /patents?\nDisallow: /patents/\nAllow: /patents/about\nDisallow: /scholar\nDisallow: /complete\nDisallow: /sponsoredlinks\nDisallow: /videosearch?\nDisallow: /videopreview?\nDisallow: /videoprograminfo?\nDisallow: /maps?\nDisallow: /mapstt?\nDisallow: /mapslt?\nDisallow: /maps/stk/\nDisallow: /maps/br?\nDisallow: /mapabcpoi?\nDisallow: /maphp?\nDisallow: /places/\nAllow: /places/$\nDisallow: /maps/place\nDisallow: /help/maps/streetview/partners/welcome/\nDisallow: /lochp?\nDisallow: /center\nDisallow: /ie?\nDisallow: /sms/demo?\nDisallow: /katrina?\nDisallow: /blogsearch?\nDisallow: /blogsearch/\nDisallow: /blogsearch_feeds\nDisallow: /advanced_blog_search\nDisallow: /reader/\nAllow: /reader/play\nDisallow: /uds/\nDisallow: /chart?\nDisallow: /transit?\nDisallow: /mbd?\nDisallow: /extern_js/\nDisallow: /calendar/feeds/\nDisallow: /calendar/ical/\nDisallow: /cl2/feeds/\n" +
	"Disallow: /cl2/ical/\nDisallow: /coop/directory\nDisallow: /coop/manage\nDisallow: /trends?\nDisallow: /trends/music?\nDisallow: /trends/hottrends?\nDisallow: /trends/viz?\nDisallow: /notebook/search?\nDisallow: /musica\nDisallow: /musicad\nDisallow: /musicas\nDisallow: /musicl\nDisallow: /musics\nDisallow: /musicsearch\nDisallow: /musicsp\nDisallow: /musiclp\nDisallow: /browsersync\nDisallow: /call\nDisallow: /archivesearch?\nDisallow: /archivesearch/url\nDisallow: /archivesearch/advanced_search\nDisallow: /base/reportbadoffer\nDisallow: /urchin_test/\nDisallow: /movies?\nDisallow: /codesearch?\nDisallow: /codesearch/feeds/search?\nDisallow: /wapsearch?\nDisallow: /safebrowsing\nAllow: /safebrowsing/diagnostic\nAllow: /safebrowsing/report_error/\nAllow: /safebrowsing/report_phish/\nDisallow: /reviews/search?\nDisallow: /orkut/albums\nAllow: /jsapi\nDisallow: /views?\nDisallow: /c/\nDisallow: /cbk\nDisallow: /recharge/dashboard/car\nDisallow: /recharge/dashboard/static/\nDisallow: /translate_a/\nDisallow: /translate_c\nDisallow: /translate_f\nDisallow: /translate_static/\nDisallow: /translate_suggestion\nDisallow: /profiles/me\nAllow: /profiles\nDisallow: /s2/profiles/me\nAllow: /s2/profiles\nAllow: /s2/photos\nAllow: /s2/static\nDisallow: /s2\nDisallow: /transconsole/portal/\nDisallow: /gcc/\nDisallow: /aclk\nDisallow: /cse?\nDisallow: /cse/home\nDisallow: /cse/panel\nDisallow: /cse/manage\nDisallow: /tbproxy/\nDisallow: /imesync/\nDisallow: /shenghuo/search?\nDisallow: /support/forum/search?\nDisallow: /reviews/polls/\nDisallow: /hosted/images/\nDisallow: /ppob/?\nDisallow: /ppob?\nDisallow: /ig/add?\nDisallow: /adwordsresellers\nDisallow: /accounts/o8\nAllow: /accounts/o8/id\nDisallow: /topicsearch?q=\nDisallow: /xfx7/\nDisallow: /squared/api\nDisallow: /squared/search\nDisallow: /squared/table\nDisallow: /toolkit/\nAllow: /toolkit/*.html\nDisallow: /globalmarketfinder/\nAllow: /globalmarketfinder/*.html\nDisallow: /qnasearch?\nDisallow: /errors/\nDisallow: /app/updates\nDisallow: /sidewiki/entry/\nDisallow: /quality_form?\nDisallow: /labs/popgadget/search\nDisallow: /buzz/post\nDisallow: /compressiontest/\nDisallow: /analytics/reporting/\nDisallow: /analytics/admin/\nDisallow: /analytics/web/\nDisallow: /analytics/feeds/\nDisallow: /analytics/settings/\nDisallow: /alerts/\nDisallow: /phone/compare/?\nAllow: /alerts/manage\nSitemap: http://www.gstatic.com/s2/sitemaps/profiles-sitemap.xml\nSitemap: http://www.google.com/hostednews/sitemap_index.xml\nSitemap: http://www.google.com/ventures/sitemap_ventures.xml\nSitemap: http://www.google.com/sitemaps_webmasters.xml\nSitemap: http://www.gstatic.com/trends/websites/sitemaps/sitemapindex.xml\nSitemap: http://www.gstatic.com/dictionary/static/sitemaps/sitemap_index.xml")

func TestFromGoogle(t *testing.T) {
	t.Parallel()
	r, err := FromString(robotsGoogle)
	require.NoError(t, err)
	expectAllAgents(t, r, true, "/ncr")
	expectAllAgents(t, r, false, "/search")
}

func TestAllowAll(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"# comment",
		"User-Agent: * \nAllow: /",
		"User-Agent: * \nDisallow: ",
	}
	for i, input := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			r, err := FromString(input)
			require.NoError(t, err)
			expectAll(t, r, true)
		})
	}
}

func TestRobotstxtOrg(t *testing.T) {
	t.Parallel()
	const robotsText005 = `User-agent: Google
Disallow:
User-agent: *
Disallow: /`
	r, err := FromString(robotsText005)
	require.NoError(t, err)
	expectAccess(t, r, false, "/path/page1.html", "SomeBot")
	expectAccess(t, r, true, "/path/page1.html", "Googlebot")
}

func TestHost(t *testing.T) {
	type tcase struct {
		input  string
		expect string
	}
	cases := []tcase{
		{"#Host: site.ru", ""},
		{"Host: site.ru", "site.ru"},
		{"Host: яндекс.рф", "яндекс.рф"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			r, err := FromString(c.input)
			require.NoError(t, err)
			assert.Equal(t, c.expect, r.Host)
		})
	}
}

func TestParseErrors(t *testing.T) {
	const robotsTextErrs = `Disallow: /
User-agent: Google
Crawl-delay: bad-time-value`
	_, err := FromString(robotsTextErrs)
	assert.Error(t, err)
	pe, ok := err.(*ParseError)
	require.True(t, ok, "Expected ParseError")
	require.Equal(t, 2, len(pe.Errs))
	assert.Contains(t, pe.Errs[0].Error(), "Disallow before User-agent")
	assert.Contains(t, pe.Errs[1].Error(), "bad-time-value")
	assert.Contains(t, pe.Errs[1].Error(), "invalid syntax")
}

const robotsTextJustHTML = `<!DOCTYPE html>
<html>
<title></title>
<p>Hello world! This is valid HTML but invalid robots.txt.`

func TestHtmlInstead(t *testing.T) {
	r, err := FromString(robotsTextJustHTML)
	// According to Google spec, invalid robots.txt file
	// must be parsed silently.
	require.NoError(t, err)
	group := r.FindGroup("SuperBot")
	require.NotNil(t, group)
	assert.True(t, group.Test("/"))
}

// http://perche.vanityfair.it/robots.txt on Sat, 13 Sep 2014 23:00:29 GMT
const robotsTextVanityfair = "\xef\xbb\xbfUser-agent: *\nDisallow: */oroscopo-di-oggi/*"

func TestWildcardPrefix(t *testing.T) {
	t.Parallel()
	r, err := FromString(robotsTextVanityfair)
	require.NoError(t, err)
	expectAllAgents(t, r, true, "/foo/bar")
	expectAllAgents(t, r, false, "/oroscopo-di-oggi/bar")
	expectAllAgents(t, r, false, "/foo/oroscopo-di-oggi/bar")
}

func TestGrouping(t *testing.T) {
	const robotsCaseGrouping = `user-agent: a
user-agent: b
disallow: /a
disallow: /b

user-agent: ignore
Disallow: /separator

user-agent: b
user-agent: c
disallow: /b
disallow: /c`

	r, err := FromString(robotsCaseGrouping)
	require.NoError(t, err)
	expectAccess(t, r, false, "/a", "a")
	expectAccess(t, r, false, "/b", "a")
	expectAccess(t, r, true, "/c", "a")

	expectAccess(t, r, false, "/a", "b")
	expectAccess(t, r, false, "/b", "b")
	expectAccess(t, r, false, "/c", "b")

	expectAccess(t, r, true, "/a", "c")
	expectAccess(t, r, false, "/b", "c")
	expectAccess(t, r, false, "/c", "c")
}

func BenchmarkParseFromString001(b *testing.B) {
	input := robotsText001
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := FromString(input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFromString002(b *testing.B) {
	input := robotsGoogle
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := FromString(input); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFromStatus401(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := FromStatusAndString(401, ""); err != nil {
			b.Fatal(err)
		}
	}
}

func expectAll(t *testing.T, r *RobotsData, allow bool) {
	// TODO fuzz path
	expectAllAgents(t, r, allow, "/")
	expectAllAgents(t, r, allow, "/admin/")
	expectAllAgents(t, r, allow, "/search")
	expectAllAgents(t, r, allow, "/.htaccess")
}

func expectAllAgents(t *testing.T, r *RobotsData, allow bool, path string) {
	f := func(agent string) bool { return expectAccess(t, r, allow, path, agent) }
	if err := quick.Check(f, nil); err != nil {
		t.Fatalf("Expected allow path '%s' %v", path, err)
	}
}

func expectAccess(t *testing.T, r *RobotsData, allow bool, path, agent string) bool {
	return assert.Equal(t, allow, r.TestAgent(path, agent), "path='%s' agent='%s'", path, agent)
}

func newHttpResponse(code int, body string) *http.Response {
	return &http.Response{
		StatusCode:    code,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          ioutil.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}
