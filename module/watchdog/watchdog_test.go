package watchdog

import (
	"net"
	"regexp"
	"strings"
	"testing"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
)

var w *Watchdog

// init test
func init() {
	log.Init(core.Options{Debug: &[]bool{true}[0]}, false, "")

	// LoadModules load modules
	w = &Watchdog{
		Enabled:       true,
		RulesFilePath: "",
		GeoDBFilePath: "../../config/GeoLite2-City.mmdb",
	}
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

	allowed := w.Allow(net.ParseIP("127.0.0.1"))
	if allowed == false {
		t.Fatalf(`%q should be allowed. result: %v`, "127.0.0.1", allowed)
	}

	allowed = w.Allow(net.ParseIP("10.0.0.0"))
	if allowed == true {
		t.Fatalf(`%q should be blocked. result: %v`, "10.0.0.0", allowed)
	}
}

func TestDefaultAllowIP(t *testing.T) {

	// Rule:
	// Default allow (!*)
	// block only 127.0.0.1 (127.0.0.1)
	w.Raw = `!*
			  127.0.0.1`
	w.Reload()

	allowed := w.Allow(net.ParseIP("127.0.0.1"))
	if allowed == true {
		t.Fatalf(`%q should be blocked. result: %v`, "127.0.0.1", allowed)
	}

	allowed = w.Allow(net.ParseIP("10.0.0.0"))
	if allowed == false {
		t.Fatalf(`%q should be allowed. result: %v`, "10.0.0.0", allowed)
	}
}

func TestBlockHostnameRegex(t *testing.T) {
	w.Raw = `~ ^.*\.1e100\.net`
	w.Reload()

	IP, _ := net.LookupIP("google.com")
	allowed := w.Allow(IP[0])
	if allowed {
		t.Fatalf(`Blacklist regex: %q should be blocked, allowed: %v`, "google.com", allowed)
	}
}

func TestAllowHostnameRegex(t *testing.T) {
	w.Raw = `!~ ^.*\.1e100\.net`
	w.Reload()

	IP, _ := net.LookupIP("google.com")
	allowed := w.Allow(IP[0])
	if !allowed {
		t.Fatalf(`Blacklist regex: %q should be allowed, allowed: %v`, "google.com", allowed)
	}
}

func TestGeofenceByCoordinates(t *testing.T) {
	// Rule:
	// Default deny (*)
	// allow only something in a lake
	w.Raw = `*
			  !@43.14,12.1 (600km)` // In a lake
	w.Reload()

	allowed := w.Allow(net.ParseIP("151.46.0.1"))
	if allowed == false {
		t.Fatalf(`%q should be allowed. result: %v`, "127.0.0.1", allowed)
	}

	allowed = w.Allow(net.ParseIP("172.217.21.78"))
	if allowed == true {
		t.Fatalf(`%q should be blocked. result: %v`, "10.0.0.0", allowed)
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

	allowed := w.Allow(net.ParseIP("151.46.0.1"))
	if allowed == false {
		t.Fatalf(`%q should be allowed. result: %v`, "151.46.0.1", allowed)
	}

	allowed = w.Allow(net.ParseIP("172.217.21.78"))
	if allowed == false {
		t.Fatalf(`%q should be allowed. result: %v`, "172.217.21.78", allowed)
	}

	allowed = w.Allow(net.ParseIP("178.0.0.1"))
	if allowed == true {
		t.Fatalf(`%q should be blocked. result: %v`, "178.0.0.1", allowed)
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
