package watchdog

import (
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
)

var w *Watchdog
var r *http.Request

// init test
func init() {
	log.Init(core.Options{Debug: &[]bool{true}[0]}, false, "")

	// LoadModules load modules
	w = &Watchdog{
		Enabled:       true,
		RulesFilePath: "",
		GeoDBFilePath: "../../config/GeoLite2-City.mmdb",
	}

	r = &http.Request{}
}

// TestModuleName ensures the module is the same, just in case :)
func TestModuleName(t *testing.T) {
	module := "watchdog"
	want := regexp.MustCompile(module)
	if !want.MatchString(Name) {
		t.Fatalf(`The module name does not match: %q != %q`, module, want)
	}
}

//
// Parsing Rules
//

func TestParseIP(t *testing.T) {
	w.Raw = "127.0.0.1"
	w.Reload()

	value := w.Rules.List[0].IP.String()

	if w.Raw != value {
		t.Fatalf(`Invalid IP parse: %q != %q`, w.Raw, value)
	}
}

func TestParseNetworkValid(t *testing.T) {
	w.Raw = "127.0.0.0/24"
	w.Reload()

	value := w.Rules.List[0].Network.String()
	if w.Raw != value {
		t.Fatalf(`Invalid Network parse: %q != %q`, w.Raw, value)
	}
}

func TestParseNetworkInvalid(t *testing.T) {
	w.Raw = "127.0.0.0/33"
	w.Reload()

	var value interface{} = nil
	if len(w.Rules.List) != 0 && w.Rules.List[0].Network != nil {
		value = w.Rules.List[0].Network.String()
	}

	if value != nil {
		t.Fatalf(`Invalid Network parse: %q`, value)
	}
}

func TestParseHostname(t *testing.T) {
	w.Raw = "phishing.click"
	w.Reload()

	value := w.Rules.List[0].Hostname
	if w.Raw != value {
		t.Fatalf(`Invalid Hostname parse: %q != %q`, w.Raw, value)
	}
}

func TestParseHostnameRegex(t *testing.T) {
	// 1e100.net is Google: https://support.google.com/faqs/answer/174717?hl=en
	w.Raw = `~ ^.*\.1e100\.net`
	w.Reload()

	value := w.Rules.List[0].Regexp
	if strings.TrimSpace(w.Raw[1:]) != value {
		t.Fatalf(`Invalid Hostname Regex parse: %q != %q`, w.Raw, value)
	}
}

//
// Module Functions
//

func TestAppendRawRule(t *testing.T) {
	w.Raw = `*
!127.0.0.1
!10.0.0.0/24`
	w.Reload()

	// Append an extra rule
	newRule := `!192.168.0.0/16`
	if !w.Rules.AppendRaw(newRule) || len(w.Rules.List) != 4 {
		t.Fatalf(`Error adding a raw rule`)
	}
}

func TestAppendRule(t *testing.T) {
	w.Raw = `*
!127.0.0.1
!10.0.0.0/24`
	w.Reload()

	// Append an extra rule
	newRule := `!192.168.0.0/16`
	newBlacklist := ParseRules(newRule)
	if len(newBlacklist.List) != 1 {
		t.Fatalf(`Error parsing rules: %q`, newRule)
	}

	w.Rules.Concatenate(newBlacklist.List)
	if len(w.Rules.List) != 4 {
		t.Fatalf(`Error concatenating the new rule`)
	}
}

func TestRemoveRule(t *testing.T) {
	w.Raw = `*
!127.0.0.1
!10.0.0.0/24`
	w.Reload()

	// Remove a extra rule
	delRule := w.Rules.List[1]
	if !w.Rules.Remove(delRule) {
		t.Fatalf(`Failed to remove rule`)
	}
}

//
// Blacklisting
//

func TestDefaultDenyIP(t *testing.T) {

	// Rule:
	// Default deny (*)
	// allow only 127.0.0.1 (!127.0.0.1)
	w.Raw = `*
             !127.0.0.1`
	w.Reload()

	var tests = []struct {
		rAddr string
		want  bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", false},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s", tt.rAddr)
		t.Run(testname, func(t *testing.T) {
			r.RemoteAddr = tt.rAddr
			ans := w.Allow(r)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}

func TestDefaultAllowIP(t *testing.T) {

	// Rule:
	// Default allow (!*)
	// block only 127.0.0.1 (127.0.0.1)
	w.Raw = `!*
			  127.0.0.1`
	w.Reload()

	var tests = []struct {
		rAddr string
		want  bool
	}{
		{"127.0.0.1", false},
		{"10.0.0.1", true},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s", tt.rAddr)
		t.Run(testname, func(t *testing.T) {
			r.RemoteAddr = tt.rAddr
			ans := w.Allow(r)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}

func TestBlockHostnameRegex(t *testing.T) {
	w.Raw = `~ ^.*\.1e100\.net`
	w.Reload()

	var tests = []struct {
		rAddr string
		want  bool
	}{
		{"google.com", false},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s", tt.rAddr)
		t.Run(testname, func(t *testing.T) {

			IP, _ := net.LookupIP(tt.rAddr)
			r.RemoteAddr = IP[0].String()
			ans := w.Allow(r)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}

func TestAllowHostnameRegex(t *testing.T) {
	w.Raw = `!~ ^.*\.1e100\.net`
	w.Reload()

	var tests = []struct {
		rAddr string
		want  bool
	}{
		{"google.com", true},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s", tt.rAddr)
		t.Run(testname, func(t *testing.T) {

			IP, _ := net.LookupIP(tt.rAddr)
			r.RemoteAddr = IP[0].String()
			ans := w.Allow(r)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}

func TestAllowUserAgent(t *testing.T) {
	w.Raw = `!> Mozilla/5.0`
	w.Reload()

	var tests = []struct {
		UA   string
		want bool
	}{
		{"Mozilla/5.0", true},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s", tt.UA)
		t.Run(testname, func(t *testing.T) {

			r.Header = http.Header{
				"User-Agent": []string{tt.UA},
			}
			ans := w.Allow(r)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}

func TestDenyUserAgent(t *testing.T) {
	w.Raw = `> Mozilla/5.0`
	w.Reload()

	var tests = []struct {
		UA   string
		want bool
	}{
		{"Mozilla/5.0", false},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s", tt.UA)
		t.Run(testname, func(t *testing.T) {

			r.Header = http.Header{
				"User-Agent": []string{tt.UA},
			}
			ans := w.Allow(r)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}

func TestDenyUserAgentRegex(t *testing.T) {
	w.Raw = `>~ Mozilla.*`
	w.Reload()

	var tests = []struct {
		UA   string
		want bool
	}{
		{"Mozilla/5.0", false},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s", tt.UA)
		t.Run(testname, func(t *testing.T) {

			r.Header = http.Header{
				"User-Agent": []string{tt.UA},
			}
			ans := w.Allow(r)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}

func TestGeofenceByCoordinates(t *testing.T) {
	// Rule:
	// Default deny (*)
	// allow only something in a lake
	w.Raw = `*
			  !@43.14,12.1 (600km)` // In a lake
	w.Reload()

	var tests = []struct {
		rAddr string
		want  bool
	}{
		{"151.46.0.1", true},
		{"172.217.21.78", false},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s", tt.rAddr)
		t.Run(testname, func(t *testing.T) {
			r.RemoteAddr = tt.rAddr
			ans := w.Allow(r)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}

func TestGeofenceByParameters(t *testing.T) {
	// Rule:
	// Default deny (*)
	// allow only Italy and IPs from Mountain View
	w.Raw = `*
			  !@Country:IT
			  !@City:Mountain View`
	w.Reload()

	var tests = []struct {
		rAddr string
		want  bool
	}{
		{"151.46.0.1", true},
		{"172.217.21.78", true},
		{"178.0.0.1", false},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s", tt.rAddr)
		t.Run(testname, func(t *testing.T) {
			r.RemoteAddr = tt.rAddr
			ans := w.Allow(r)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}

func TestIntersection(t *testing.T) {
	tests := []struct {
		Mi     Geofence
		Tu     Geofence
		Result SetIntersection
	}{
		{Geofence{Location, "", "", 36.1699, -115.1398, 1000.0}, Geofence{Location, "", "", 36.1699, -115.1398, 10.0}, IsSuperset},
		{Geofence{Location, "", "", 36.1699, -115.1398, 10.0}, Geofence{Location, "", "", 36.1699, -115.1398, 1000.0}, IsSubset},
		{Geofence{Location, "", "", 36.1699, -115.1398, 10.0}, Geofence{Location, "", "", 37.7749, -122.4194, 1000.0}, IsDisjoint},
		{Geofence{Location, "", "", 36.1699, -115.1398, 10.0}, Geofence{Location, "", "", 36.1699, -115.1398, 10.0}, IsSubset | IsSuperset},
		{Geofence{Location, "", "", 36.1699, -115.13983, 100.0}, Geofence{Location, "", "", 36.1699, -115.1398, 100.0}, 0},
	}

	for _, test := range tests {
		got := test.Mi.Intersection(&test.Tu)
		want := test.Result
		if want != got {
			t.Fatalf("With %v and %v: expected intersection code %b, got %b",
				Rule{Geofence: &test.Mi}, Rule{Geofence: &test.Tu}, want, got)
		}
	}
}
