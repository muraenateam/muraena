package robotstxt

import (
	"testing"
)

func TestFromStatusAndStringBasic(t *testing.T) {
	if _, err := FromStatusAndString(200, ""); err != nil {
		t.Fatal("FromStatusAndString MUST accept 200/\"\"")
	}
	if _, err := FromStatusAndString(401, ""); err != nil {
		t.Fatal("FromStatusAndString MUST accept 401/\"\"")
	}
	if _, err := FromStatusAndString(403, ""); err != nil {
		t.Fatal("FromStatusAndString MUST accept 403/\"\"")
	}
	if _, err := FromStatusAndString(404, ""); err != nil {
		t.Fatal("FromStatusAndString MUST accept 404/\"\"")
	}
}

func ExpectAllow(t *testing.T, r *RobotsData, path, agent string) {
	if !r.TestAgent(path, agent) {
		t.Fatalf("Expected allow path '%s' agent '%s'", path, agent)
	}
}

func ExpectDisallow(t *testing.T, r *RobotsData, path, agent string) {
	if r.TestAgent(path, agent) {
		t.Fatalf("Expected disallow path '%s' agent '%s'", path, agent)
	}
}

func ExpectAllowAll(t *testing.T, r *RobotsData) {
	if !r.TestAgent("/", "Somebot") {
		t.Fatal("Expected allow all")
	}
}

func ExpectDisallowAll(t *testing.T, r *RobotsData) {
	if r.TestAgent("/", "Somebot") {
		t.Fatal("Expected disallow all")
	}
}

func TestStatus401(t *testing.T) {
	r, _ := FromStatusAndString(401, "")
	ExpectAllowAll(t, r)
}

func TestStatus403(t *testing.T) {
	r, _ := FromStatusAndString(403, "")
	ExpectAllowAll(t, r)
}

func TestStatus404(t *testing.T) {
	r, _ := FromStatusAndString(404, "")
	ExpectAllowAll(t, r)
}

func TestFromStringBasic(t *testing.T) {
	if _, err := FromString(""); err != nil {
		t.Fatal("FromString MUST accept \"\"")
	}
}

func TestFromStringEmpty(t *testing.T) {
	r, _ := FromString("")
	if allow := r.TestAgent("/", "Somebot"); !allow {
		t.Fatal("FromString(\"\") MUST allow everything.")
	}
}

func TestFromStringComment(t *testing.T) {
	if _, err := FromString("# comment"); err != nil {
		t.Fatal("FromString MUST accept \"# comment\"")
	}
}

func TestFromString001(t *testing.T) {
	r, err := FromString("User-Agent: *\r\nDisallow: /\r\n")
	if err != nil {
		t.Fatal(err.Error())
	}
	allow := r.TestAgent("/foobar", "SomeAgent")
	if allow {
		t.Fatal("Must deny.")
	}
}

func TestFromString002(t *testing.T) {
	r, err := FromString("User-Agent: *\r\nDisallow: /account\r\n")
	if err != nil {
		t.Fatal(err.Error())
	}
	allow := r.TestAgent("/foobar", "SomeAgent")
	if !allow {
		t.Fatal("Must allow.")
	}
}

const robotsText001 = "User-agent: * \nDisallow: /administrator/\nDisallow: /cache/\nDisallow: /components/\nDisallow: /editor/\nDisallow: /forum/\nDisallow: /help/\nDisallow: /images/\nDisallow: /includes/\nDisallow: /language/\nDisallow: /mambots/\nDisallow: /media/\nDisallow: /modules/\nDisallow: /templates/\nDisallow: /installation/\nDisallow: /getcid/\nDisallow: /tooltip/\nDisallow: /getuser/\nDisallow: /download/\nDisallow: /index.php?option=com_phorum*,quote=1\nDisallow: /index.php?option=com_phorum*phorum_query=search\nDisallow: /index.php?option=com_phorum*,newer\nDisallow: /index.php?option=com_phorum*,older\n\nUser-agent: Yandex\nAllow: /\nSitemap: http://www.pravorulya.com/sitemap.xml\nSitemap: http://www.pravorulya.com/sitemap1.xml"

func TestFromString003(t *testing.T) {
	r, err := FromString(robotsText001)
	if err != nil {
		t.Fatal(err.Error())
	}
	allow := r.TestAgent("/administrator/", "SomeBot")
	if allow {
		t.Fatal("Must deny.")
	}
}

func TestFromString004(t *testing.T) {
	r, err := FromString(robotsText001)
	if err != nil {
		t.Fatal(err.Error())
	}
	allow := r.TestAgent("/paruram", "SomeBot")
	if !allow {
		t.Fatal("Must allow.")
	}
}

func TestInvalidEncoding(t *testing.T) {
	// Invalid UTF-8 encoding should not break parser.
	_, err := FromString("User-agent: H\xef\xbf\xbdm�h�kki\nDisallow: *")
	if err != nil {
		t.Fatal(err.Error())
	}
}

// http://www.google.com/robots.txt on Wed, 12 Jan 2011 12:22:20 GMT
const robotsText002 = ("User-agent: *\nDisallow: /search\nDisallow: /groups\nDisallow: /images\nDisallow: /catalogs\nDisallow: /catalogues\nDisallow: /news\nAllow: /news/directory\nDisallow: /nwshp\nDisallow: /setnewsprefs?\nDisallow: /index.html?\nDisallow: /?\nDisallow: /addurl/image?\nDisallow: /pagead/\nDisallow: /relpage/\nDisallow: /relcontent\nDisallow: /imgres\nDisallow: /imglanding\nDisallow: /keyword/\nDisallow: /u/\nDisallow: /univ/\nDisallow: /cobrand\nDisallow: /custom\nDisallow: /advanced_group_search\nDisallow: /googlesite\nDisallow: /preferences\nDisallow: /setprefs\nDisallow: /swr\nDisallow: /url\nDisallow: /default\nDisallow: /m?\nDisallow: /m/?\nDisallow: /m/blogs?\nDisallow: /m/directions?\nDisallow: /m/ig\nDisallow: /m/images?\nDisallow: /m/local?\nDisallow: /m/movies?\nDisallow: /m/news?\nDisallow: /m/news/i?\nDisallow: /m/place?\nDisallow: /m/products?\nDisallow: /m/products/\nDisallow: /m/setnewsprefs?\nDisallow: /m/search?\nDisallow: /m/swmloptin?\nDisallow: /m/trends\nDisallow: /m/video?\nDisallow: /wml?\nDisallow: /wml/?\nDisallow: /wml/search?\nDisallow: /xhtml?\nDisallow: /xhtml/?\nDisallow: /xhtml/search?\nDisallow: /xml?\nDisallow: /imode?\nDisallow: /imode/?\nDisallow: /imode/search?\nDisallow: /jsky?\nDisallow: /jsky/?\nDisallow: /jsky/search?\nDisallow: /pda?\nDisallow: /pda/?\nDisallow: /pda/search?\nDisallow: /sprint_xhtml\nDisallow: /sprint_wml\nDisallow: /pqa\nDisallow: /palm\nDisallow: /gwt/\nDisallow: /purchases\nDisallow: /hws\nDisallow: /bsd?\nDisallow: /linux?\nDisallow: /mac?\nDisallow: /microsoft?\nDisallow: /unclesam?\nDisallow: /answers/search?q=\nDisallow: /local?\nDisallow: /local_url\nDisallow: /froogle?\nDisallow: /products?\nDisallow: /products/\nDisallow: /froogle_\nDisallow: /product_\nDisallow: /products_\nDisallow: /products;\nDisallow: /print\nDisallow: /books\nDisallow: /bkshp?q=\nAllow: /booksrightsholders\nDisallow: /patents?\nDisallow: /patents/\nAllow: /patents/about\nDisallow: /scholar\nDisallow: /complete\nDisallow: /sponsoredlinks\nDisallow: /videosearch?\nDisallow: /videopreview?\nDisallow: /videoprograminfo?\nDisallow: /maps?\nDisallow: /mapstt?\nDisallow: /mapslt?\nDisallow: /maps/stk/\nDisallow: /maps/br?\nDisallow: /mapabcpoi?\nDisallow: /maphp?\nDisallow: /places/\nAllow: /places/$\nDisallow: /maps/place\nDisallow: /help/maps/streetview/partners/welcome/\nDisallow: /lochp?\nDisallow: /center\nDisallow: /ie?\nDisallow: /sms/demo?\nDisallow: /katrina?\nDisallow: /blogsearch?\nDisallow: /blogsearch/\nDisallow: /blogsearch_feeds\nDisallow: /advanced_blog_search\nDisallow: /reader/\nAllow: /reader/play\nDisallow: /uds/\nDisallow: /chart?\nDisallow: /transit?\nDisallow: /mbd?\nDisallow: /extern_js/\nDisallow: /calendar/feeds/\nDisallow: /calendar/ical/\nDisallow: /cl2/feeds/\n" +
	"Disallow: /cl2/ical/\nDisallow: /coop/directory\nDisallow: /coop/manage\nDisallow: /trends?\nDisallow: /trends/music?\nDisallow: /trends/hottrends?\nDisallow: /trends/viz?\nDisallow: /notebook/search?\nDisallow: /musica\nDisallow: /musicad\nDisallow: /musicas\nDisallow: /musicl\nDisallow: /musics\nDisallow: /musicsearch\nDisallow: /musicsp\nDisallow: /musiclp\nDisallow: /browsersync\nDisallow: /call\nDisallow: /archivesearch?\nDisallow: /archivesearch/url\nDisallow: /archivesearch/advanced_search\nDisallow: /base/reportbadoffer\nDisallow: /urchin_test/\nDisallow: /movies?\nDisallow: /codesearch?\nDisallow: /codesearch/feeds/search?\nDisallow: /wapsearch?\nDisallow: /safebrowsing\nAllow: /safebrowsing/diagnostic\nAllow: /safebrowsing/report_error/\nAllow: /safebrowsing/report_phish/\nDisallow: /reviews/search?\nDisallow: /orkut/albums\nAllow: /jsapi\nDisallow: /views?\nDisallow: /c/\nDisallow: /cbk\nDisallow: /recharge/dashboard/car\nDisallow: /recharge/dashboard/static/\nDisallow: /translate_a/\nDisallow: /translate_c\nDisallow: /translate_f\nDisallow: /translate_static/\nDisallow: /translate_suggestion\nDisallow: /profiles/me\nAllow: /profiles\nDisallow: /s2/profiles/me\nAllow: /s2/profiles\nAllow: /s2/photos\nAllow: /s2/static\nDisallow: /s2\nDisallow: /transconsole/portal/\nDisallow: /gcc/\nDisallow: /aclk\nDisallow: /cse?\nDisallow: /cse/home\nDisallow: /cse/panel\nDisallow: /cse/manage\nDisallow: /tbproxy/\nDisallow: /imesync/\nDisallow: /shenghuo/search?\nDisallow: /support/forum/search?\nDisallow: /reviews/polls/\nDisallow: /hosted/images/\nDisallow: /ppob/?\nDisallow: /ppob?\nDisallow: /ig/add?\nDisallow: /adwordsresellers\nDisallow: /accounts/o8\nAllow: /accounts/o8/id\nDisallow: /topicsearch?q=\nDisallow: /xfx7/\nDisallow: /squared/api\nDisallow: /squared/search\nDisallow: /squared/table\nDisallow: /toolkit/\nAllow: /toolkit/*.html\nDisallow: /globalmarketfinder/\nAllow: /globalmarketfinder/*.html\nDisallow: /qnasearch?\nDisallow: /errors/\nDisallow: /app/updates\nDisallow: /sidewiki/entry/\nDisallow: /quality_form?\nDisallow: /labs/popgadget/search\nDisallow: /buzz/post\nDisallow: /compressiontest/\nDisallow: /analytics/reporting/\nDisallow: /analytics/admin/\nDisallow: /analytics/web/\nDisallow: /analytics/feeds/\nDisallow: /analytics/settings/\nDisallow: /alerts/\nDisallow: /phone/compare/?\nAllow: /alerts/manage\nSitemap: http://www.gstatic.com/s2/sitemaps/profiles-sitemap.xml\nSitemap: http://www.google.com/hostednews/sitemap_index.xml\nSitemap: http://www.google.com/ventures/sitemap_ventures.xml\nSitemap: http://www.google.com/sitemaps_webmasters.xml\nSitemap: http://www.gstatic.com/trends/websites/sitemaps/sitemapindex.xml\nSitemap: http://www.gstatic.com/dictionary/static/sitemaps/sitemap_index.xml")

func TestFromString005(t *testing.T) {
	r, err := FromString(robotsText002)
	if err != nil {
		t.Fatal(err.Error())
	}
	ExpectAllowAll(t, r)
}

func TestFromString006(t *testing.T) {
	r, err := FromString(robotsText002)
	if err != nil {
		t.Fatal(err.Error())
	}
	allow := r.TestAgent("/search", "SomeBot")
	if allow {
		t.Fatal("Must deny.")
	}
}

const robotsText003 = "User-Agent: * \nAllow: /"

func TestFromString007(t *testing.T) {
	r, err := FromString(robotsText003)
	if err != nil {
		t.Fatal(err.Error())
	}
	allow := r.TestAgent("/random", "SomeBot")
	if !allow {
		t.Fatal("Must allow.")
	}
}

const robotsText004 = "User-Agent: * \nDisallow: "

func TestFromString008(t *testing.T) {
	r, err := FromString(robotsText004)
	if err != nil {
		t.Log(robotsText004)
		t.Fatal(err.Error())
	}
	allow := r.TestAgent("/random", "SomeBot")
	if !allow {
		t.Fatal("Must allow.")
	}
}

const robotsText005 = `User-agent: Google
Disallow:
User-agent: *
Disallow: /`

func TestRobotstxtOrgCase1(t *testing.T) {
	if r, err := FromString(robotsText005); err != nil {
		t.Fatal(err.Error())
	} else if allow := r.TestAgent("/path/page1.html", "SomeBot"); allow {
		t.Fatal("Must disallow.")
	}
}

func TestRobotstxtOrgCase2(t *testing.T) {
	if r, err := FromString(robotsText005); err != nil {
		t.Fatal(err.Error())
	} else if allow := r.TestAgent("/path/page1.html", "Googlebot"); !allow {
		t.Fatal("Must allow.")
	}
}

const robotsText006 = `
Host: site.ru`

func TestRobotstxtHostCase1(t *testing.T) {
	if r, err := FromString(robotsText006); err != nil {
		t.Fatal(err.Error())
	} else if r.Host != "site.ru" {
		t.Fatal("Incorrect host detection")
	}
}

const robotsText007 = `
#Host: site.ru`

func TestRobotstxtHostCase2(t *testing.T) {
	if r, err := FromString(robotsText007); err != nil {
		t.Fatal(err.Error())
	} else if r.Host != "" {
		t.Fatal("Incorrect host detection")
	}
}

const robotsText008 = `
Host: яндекс.рф`

func TestRobotstxtHostCase3(t *testing.T) {
	if r, err := FromString(robotsText008); err != nil {
		t.Fatal(err.Error())
	} else if r.Host != "яндекс.рф" {
		t.Fatal("Incorrect host detection")
	}
}

const robotsTextErrs = `Disallow: /
User-agent: Google
Crawl-delay: fail`

func TestParseErrors(t *testing.T) {
	if _, err := FromString(robotsTextErrs); err == nil {
		t.Fatal("Expected error.")
	} else {
		if pe, ok := err.(*ParseError); !ok {
			t.Fatal("Expected ParseError.")
		} else if len(pe.Errs) != 2 {
			t.Fatalf("Expected 2 errors, got %d:\n%v", len(pe.Errs), pe.Errs)
		}
	}
}

const robotsTextJustHTML = `<!DOCTYPE html>
<html>
<title></title>
<p>Hello world! This is valid HTML but invalid robots.txt.`

func TestHtmlInstead(t *testing.T) {
	r, err := FromString(robotsTextJustHTML)
	if err != nil {
		// According to Google spec, invalid robots.txt file
		// must be parsed silently.
		t.Fatal(err.Error())
	}
	group := r.FindGroup("SuperBot")
	if group == nil {
		t.Fatal("Group must not be nil.")
	}
	if !group.Test("/") {
		t.Fatal("Must allow by default.")
	}
}

// http://perche.vanityfair.it/robots.txt on Sat, 13 Sep 2014 23:00:29 GMT
const robotsTextVanityfair = "\xef\xbb\xbfUser-agent: *\nDisallow: */oroscopo-di-oggi/*"

func TestWildcardPrefix(t *testing.T) {
	r, err := FromString(robotsTextVanityfair)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !r.TestAgent("/foo/bar", "WildcardBot") {
		t.Fatal("Must allow /foo/bar")
	}
	if r.TestAgent("/oroscopo-di-oggi/bar", "WildcardBot") {
		t.Fatal("Must not allow /oroscopo-di-oggi/bar")
	}
	if r.TestAgent("/foo/oroscopo-di-oggi/bar", "WildcardBot") {
		t.Fatal("Must not allow /foo/oroscopo-di-oggi/bar")
	}
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

	if r, e := FromString(robotsCaseGrouping); e != nil {
		t.Fatal(e)
	} else {
		ExpectDisallow(t, r, "/a", "a")
		ExpectDisallow(t, r, "/b", "a")
		ExpectAllow(t, r, "/c", "a")

		ExpectDisallow(t, r, "/a", "b")
		ExpectDisallow(t, r, "/b", "b")
		ExpectDisallow(t, r, "/c", "b")

		ExpectAllow(t, r, "/a", "c")
		ExpectDisallow(t, r, "/b", "c")
		ExpectDisallow(t, r, "/c", "c")
	}
}

func BenchmarkParseFromString001(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FromString(robotsText001)
		b.SetBytes(int64(len(robotsText001)))
	}
}

func BenchmarkParseFromString002(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FromString(robotsText002)
		b.SetBytes(int64(len(robotsText002)))
	}
}

func BenchmarkParseFromStatus401(b *testing.B) {
	for i := 0; i < b.N; i++ {
		FromStatusAndString(401, "")
	}
}
