package proxy

import (
	"fmt"
	"strings"
	"testing"
)

// sliceContains checks whether the given slice contains the specified value.
func sliceContains(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// sliceContainsPair checks whether the given slice (of alternating old/new pairs)
// contains a specific old->new replacement pair.
func sliceContainsPair(slice []string, old, new string) bool {
	for i := 0; i+1 < len(slice); i += 2 {
		if slice[i] == old && slice[i+1] == new {
			return true
		}
	}
	return false
}

// newTestReplacer creates a Replacer with common defaults for testing.
func newTestReplacer(phishing, target, extPrefix string) *Replacer {
	return &Replacer{
		Phishing:             phishing,
		Target:               target,
		ExternalOriginPrefix: extPrefix,
		Origins:              make(map[string]string),
		WildcardMapping:      make(map[string]string),
	}
}

// ---------------------------------------------------------------------------
// TestGetSessionFileName
// ---------------------------------------------------------------------------
func TestGetSessionFileName(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{
			name:   "simple domain",
			target: "target.com",
			want:   "target.com.session.json",
		},
		{
			name:   "subdomain target",
			target: "login.target.com",
			want:   "login.target.com.session.json",
		},
		{
			name:   "empty target",
			target: "",
			want:   ".session.json",
		},
		{
			name:   "long domain",
			target: "very.long.subdomain.target.example.com",
			want:   "very.long.subdomain.target.example.com.session.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Replacer{Target: tt.target}
			got := r.GetSessionFileName()
			if got != tt.want {
				t.Errorf("GetSessionFileName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestWildcardPrefix
// ---------------------------------------------------------------------------
func TestWildcardPrefix(t *testing.T) {
	tests := []struct {
		name      string
		extPrefix string
		want      string
	}{
		{
			name:      "ext prefix",
			extPrefix: "ext",
			want:      "extwld",
		},
		{
			name:      "cdn prefix",
			extPrefix: "cdn-",
			want:      "cdn-wld",
		},
		{
			name:      "empty prefix",
			extPrefix: "",
			want:      "wld",
		},
		{
			name:      "single char prefix",
			extPrefix: "x",
			want:      "xwld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Replacer{ExternalOriginPrefix: tt.extPrefix}
			got := r.WildcardPrefix()
			if got != tt.want {
				t.Errorf("WildcardPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestGetCustomWildCardSeparator
// ---------------------------------------------------------------------------
func TestGetCustomWildCardSeparator(t *testing.T) {
	tests := []struct {
		name      string
		extPrefix string
		want      string
	}{
		{
			name:      "ext prefix",
			extPrefix: "ext",
			want:      "---extwld",
		},
		{
			name:      "cdn prefix",
			extPrefix: "cdn-",
			want:      "---cdn-wld",
		},
		{
			name:      "empty prefix",
			extPrefix: "",
			want:      "---wld",
		},
		{
			name:      "long prefix",
			extPrefix: "myprefix",
			want:      "---myprefixwld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Replacer{ExternalOriginPrefix: tt.extPrefix}
			got := r.getCustomWildCardSeparator()
			if got != tt.want {
				t.Errorf("getCustomWildCardSeparator() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestWildcardRegex
// ---------------------------------------------------------------------------
func TestWildcardRegex(t *testing.T) {
	tests := []struct {
		name      string
		extPrefix string
		custom    bool
		want      string
	}{
		{
			name:      "non-custom with ext prefix",
			extPrefix: "ext",
			custom:    false,
			want:      `[a-zA-Z0-9\.-]+extwld`,
		},
		{
			name:      "custom with ext prefix",
			extPrefix: "ext",
			custom:    true,
			want:      `[a-zA-Z0-9\.-]+---extwld`,
		},
		{
			name:      "non-custom with cdn prefix",
			extPrefix: "cdn-",
			custom:    false,
			want:      `[a-zA-Z0-9\.-]+cdn-wld`,
		},
		{
			name:      "custom with cdn prefix",
			extPrefix: "cdn-",
			custom:    true,
			want:      `[a-zA-Z0-9\.-]+---cdn-wld`,
		},
		{
			name:      "non-custom with empty prefix",
			extPrefix: "",
			custom:    false,
			want:      `[a-zA-Z0-9\.-]+wld`,
		},
		{
			name:      "custom with empty prefix",
			extPrefix: "",
			custom:    true,
			want:      `[a-zA-Z0-9\.-]+---wld`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Replacer{ExternalOriginPrefix: tt.extPrefix}
			got := r.WildcardRegex(tt.custom)
			if got != tt.want {
				t.Errorf("WildcardRegex(%v) = %q, want %q", tt.custom, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestMakeReplacements
// ---------------------------------------------------------------------------
func TestMakeReplacements(t *testing.T) {

	t.Run("basic phishing to target forward replacement", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.MakeReplacements()

		fwd := r.ForwardReplacements
		if !sliceContainsPair(fwd, "evil.com", "target.com") {
			t.Errorf("ForwardReplacements should contain evil.com -> target.com, got: %v", fwd)
		}
	})

	t.Run("basic target to phishing backward replacement with boundaries", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.MakeReplacements()

		bwd := r.BackwardReplacements
		// The backward replacements use boundary variations.
		// Check that at least some boundary-prefixed variations are present.
		boundaries := []string{" ", ",", ".", ":", "/", "(", ")", "!", "'", "\"", ";", "<", ">", "\n", "\t"}
		for _, b := range boundaries {
			expectedOld := b + "target.com"
			expectedNew := b + "evil.com"
			if !sliceContainsPair(bwd, expectedOld, expectedNew) {
				t.Errorf("BackwardReplacements should contain %q -> %q", expectedOld, expectedNew)
			}
		}
	})

	t.Run("external origins are mapped in forward and backward replacements", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.Origins = map[string]string{
			"cdn.example.com": "ext1",
			"api.example.com": "ext2",
		}
		r.MakeReplacements()

		// Forward: ext1.evil.com -> cdn.example.com
		if !sliceContainsPair(r.ForwardReplacements, "ext1.evil.com", "cdn.example.com") {
			t.Errorf("ForwardReplacements should map ext1.evil.com -> cdn.example.com, got: %v", r.ForwardReplacements)
		}
		if !sliceContainsPair(r.ForwardReplacements, "ext2.evil.com", "api.example.com") {
			t.Errorf("ForwardReplacements should map ext2.evil.com -> api.example.com, got: %v", r.ForwardReplacements)
		}

		// Backward: cdn.example.com -> ext1.evil.com
		if !sliceContainsPair(r.BackwardReplacements, "cdn.example.com", "ext1.evil.com") {
			t.Errorf("BackwardReplacements should map cdn.example.com -> ext1.evil.com, got: %v", r.BackwardReplacements)
		}
		if !sliceContainsPair(r.BackwardReplacements, "api.example.com", "ext2.evil.com") {
			t.Errorf("BackwardReplacements should map api.example.com -> ext2.evil.com, got: %v", r.BackwardReplacements)
		}
	})

	t.Run("wildcard origins are mapped in wildcard replacement slices", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.WildcardMapping = map[string]string{
			"cdn.wildcard.com": "extwld1",
		}
		r.MakeReplacements()

		// Forward wildcard: extwld1.evil.com -> cdn.wildcard.com
		if !sliceContainsPair(r.ForwardWildcardReplacements, "extwld1.evil.com", "cdn.wildcard.com") {
			t.Errorf("ForwardWildcardReplacements should map extwld1.evil.com -> cdn.wildcard.com, got: %v", r.ForwardWildcardReplacements)
		}

		// Backward wildcard: cdn.wildcard.com -> extwld1.evil.com
		if !sliceContainsPair(r.BackwardWildcardReplacements, "cdn.wildcard.com", "extwld1.evil.com") {
			t.Errorf("BackwardWildcardReplacements should map cdn.wildcard.com -> extwld1.evil.com, got: %v", r.BackwardWildcardReplacements)
		}
	})

	t.Run("wildcard origins are skipped in main forward replacements", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.Origins = map[string]string{
			"cdn.example.com": "wld1", // starts with WildcardLabel, should be skipped
		}
		r.MakeReplacements()

		// This origin should NOT appear in the regular forward replacements
		if sliceContainsPair(r.ForwardReplacements, "wld1.evil.com", "cdn.example.com") {
			t.Errorf("ForwardReplacements should NOT contain wildcard-prefixed origin wld1, got: %v", r.ForwardReplacements)
		}
	})

	t.Run("subdomain map is included in forward and backward replacements", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.SubdomainMap = [][]string{{"www"}, {"mail"}}
		r.MakeReplacements()

		// SubdomainMap entries are []string; the code does fmt.Sprintf("%s.%s", sub, r.Phishing)
		// where sub is []string like {"www"}, so %s gives "[www]".
		// Verify at least the forward replacements grew beyond the base pair.
		if len(r.ForwardReplacements) <= 2 {
			t.Errorf("ForwardReplacements should include subdomain map entries, got only %d elements", len(r.ForwardReplacements))
		}
	})

	t.Run("custom response transformations are included in last backward replacements", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.CustomResponseTransformations = [][]string{
			{"old-value", "new-value"},
		}
		r.MakeReplacements()

		if !sliceContainsPair(r.LastBackwardReplacements, "old-value", "new-value") {
			t.Errorf("LastBackwardReplacements should contain custom transformations, got: %v", r.LastBackwardReplacements)
		}
	})

	t.Run("empty replacer produces minimal replacements", func(t *testing.T) {
		r := newTestReplacer("phish.net", "real.net", "o")
		r.MakeReplacements()

		// Forward should at least contain phish.net -> real.net
		if !sliceContainsPair(r.ForwardReplacements, "phish.net", "real.net") {
			t.Errorf("ForwardReplacements should contain phish.net -> real.net, got: %v", r.ForwardReplacements)
		}

		// Backward should contain boundary variations
		if len(r.BackwardReplacements) == 0 {
			t.Errorf("BackwardReplacements should not be empty")
		}
	})
}

// ---------------------------------------------------------------------------
// TestDomainMapping
// ---------------------------------------------------------------------------
func TestDomainMapping(t *testing.T) {

	t.Run("regular domain gets numbered mapping", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.ExternalOrigin = []string{"cdn.example.com", "api.other.com"}

		err := r.DomainMapping()
		if err != nil {
			t.Fatalf("DomainMapping() returned error: %v", err)
		}

		origins := r.GetOrigins()
		if origins["cdn.example.com"] != "ext1" {
			t.Errorf("Expected cdn.example.com -> ext1, got %q", origins["cdn.example.com"])
		}
		if origins["api.other.com"] != "ext2" {
			t.Errorf("Expected api.other.com -> ext2, got %q", origins["api.other.com"])
		}
	})

	t.Run("wildcard domain gets wildcard mapping", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.ExternalOrigin = []string{"*.example.com"}

		err := r.DomainMapping()
		if err != nil {
			t.Fatalf("DomainMapping() returned error: %v", err)
		}

		wm := r.GetWildcardMapping()
		if wm["example.com"] != "extwld1" {
			t.Errorf("Expected example.com -> extwld1 in WildcardMapping, got %q", wm["example.com"])
		}

		// Should not appear in regular origins
		origins := r.GetOrigins()
		if _, ok := origins["*.example.com"]; ok {
			t.Errorf("Wildcard domain should not appear in regular Origins")
		}
		if _, ok := origins["example.com"]; ok {
			t.Errorf("Wildcard base domain should not appear in regular Origins from wildcard processing")
		}
	})

	t.Run("subdomain of target with fewer than 2 dots is skipped", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		// "www.target.com" is a subdomain of target.com. The trim gives "www."
		// which has exactly 1 dot, so < 2 means it gets skipped.
		r.ExternalOrigin = []string{"www.target.com"}

		err := r.DomainMapping()
		if err != nil {
			t.Fatalf("DomainMapping() returned error: %v", err)
		}

		origins := r.GetOrigins()
		if _, ok := origins["www.target.com"]; ok {
			t.Errorf("www.target.com should be skipped (subdomain with < 2 dots in trim), got: %v", origins)
		}
	})

	t.Run("deeply nested subdomain of target is NOT skipped", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		// "deep.sub.target.com" trims to "deep.sub." which has 2 dots, so >= 2 means it is mapped.
		r.ExternalOrigin = []string{"deep.sub.target.com"}

		err := r.DomainMapping()
		if err != nil {
			t.Fatalf("DomainMapping() returned error: %v", err)
		}

		origins := r.GetOrigins()
		if _, ok := origins["deep.sub.target.com"]; !ok {
			t.Errorf("deep.sub.target.com should be mapped, got: %v", origins)
		}
	})

	t.Run("mixed regular and wildcard domains", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.ExternalOrigin = []string{
			"cdn.example.com",
			"*.wildcard.net",
			"api.service.io",
		}

		err := r.DomainMapping()
		if err != nil {
			t.Fatalf("DomainMapping() returned error: %v", err)
		}

		origins := r.GetOrigins()
		if origins["cdn.example.com"] != "ext1" {
			t.Errorf("Expected cdn.example.com -> ext1, got %q", origins["cdn.example.com"])
		}
		if origins["api.service.io"] != "ext2" {
			t.Errorf("Expected api.service.io -> ext2, got %q", origins["api.service.io"])
		}

		wm := r.GetWildcardMapping()
		if wm["wildcard.net"] != "extwld1" {
			t.Errorf("Expected wildcard.net -> extwld1, got %q", wm["wildcard.net"])
		}
	})

	t.Run("multiple wildcard domains get incremented indices", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.ExternalOrigin = []string{
			"*.first.com",
			"*.second.com",
		}

		err := r.DomainMapping()
		if err != nil {
			t.Fatalf("DomainMapping() returned error: %v", err)
		}

		wm := r.GetWildcardMapping()
		// Both should be mapped; since order may vary in a slice, check both exist
		found := 0
		for _, v := range wm {
			if v == "extwld1" || v == "extwld2" {
				found++
			}
		}
		if found != 2 {
			t.Errorf("Expected 2 wildcard mappings (extwld1, extwld2), got: %v", wm)
		}
	})
}

// ---------------------------------------------------------------------------
// TestSetExternalOrigins and TestGetExternalOrigins
// ---------------------------------------------------------------------------
func TestSetAndGetExternalOrigins(t *testing.T) {

	t.Run("basic set and get", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.ExternalOrigin = []string{}

		r.SetExternalOrigins([]string{"cdn.example.com", "api.example.com"})
		got := r.GetExternalOrigins()

		if !sliceContains(got, "cdn.example.com") {
			t.Errorf("GetExternalOrigins() should contain cdn.example.com, got: %v", got)
		}
		if !sliceContains(got, "api.example.com") {
			t.Errorf("GetExternalOrigins() should contain api.example.com, got: %v", got)
		}
	})

	t.Run("deduplication", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.ExternalOrigin = []string{}

		r.SetExternalOrigins([]string{"cdn.example.com", "cdn.example.com", "cdn.example.com"})
		got := r.GetExternalOrigins()

		count := 0
		for _, v := range got {
			if v == "cdn.example.com" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("Expected cdn.example.com to appear exactly once, appeared %d times in %v", count, got)
		}
	})

	t.Run("lowercasing", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.ExternalOrigin = []string{}

		r.SetExternalOrigins([]string{"CDN.EXAMPLE.COM"})
		got := r.GetExternalOrigins()

		if !sliceContains(got, "cdn.example.com") {
			t.Errorf("GetExternalOrigins() should contain lowercased cdn.example.com, got: %v", got)
		}
	})

	t.Run("protocol stripping", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.ExternalOrigin = []string{}

		r.SetExternalOrigins([]string{"https://cdn.example.com", "http://api.example.com"})
		got := r.GetExternalOrigins()

		for _, v := range got {
			if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
				t.Errorf("Protocol should be stripped, got: %v", got)
			}
		}
		if !sliceContains(got, "cdn.example.com") {
			t.Errorf("Expected cdn.example.com after protocol stripping, got: %v", got)
		}
		if !sliceContains(got, "api.example.com") {
			t.Errorf("Expected api.example.com after protocol stripping, got: %v", got)
		}
	})

	t.Run("domains with custom wildcard separator are skipped", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.ExternalOrigin = []string{}

		// A domain containing the custom wildcard separator should be filtered out
		domainWithSep := fmt.Sprintf("foo%sexample.com", r.getCustomWildCardSeparator())
		r.SetExternalOrigins([]string{domainWithSep, "valid.example.com"})
		got := r.GetExternalOrigins()

		if sliceContains(got, strings.ToLower(domainWithSep)) {
			t.Errorf("Domains containing custom wildcard separator should be skipped, got: %v", got)
		}
		if !sliceContains(got, "valid.example.com") {
			t.Errorf("Valid domain should be present, got: %v", got)
		}
	})

	t.Run("returns a copy not the original", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.ExternalOrigin = []string{"original.com"}

		got := r.GetExternalOrigins()
		got[0] = "modified.com"

		// The internal slice should not be modified
		internal := r.GetExternalOrigins()
		if internal[0] != "original.com" {
			t.Errorf("GetExternalOrigins should return a copy, but modifying copy changed internal state")
		}
	})
}

// ---------------------------------------------------------------------------
// TestSetCustomResponseTransformations
// ---------------------------------------------------------------------------
func TestSetCustomResponseTransformations(t *testing.T) {

	t.Run("initial set", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")

		transformations := [][]string{
			{"old1", "new1"},
			{"old2", "new2"},
		}
		r.SetCustomResponseTransformations(transformations)

		if len(r.CustomResponseTransformations) < 2 {
			t.Fatalf("Expected at least 2 transformations, got %d", len(r.CustomResponseTransformations))
		}

		found := false
		for _, tr := range r.CustomResponseTransformations {
			if len(tr) == 2 && tr[0] == "old1" && tr[1] == "new1" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected transformation [old1, new1] to be present")
		}
	})

	t.Run("deduplication of transformations", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")

		transformations := [][]string{
			{"old1", "new1"},
			{"old2", "new2"},
		}
		r.SetCustomResponseTransformations(transformations)

		// Set the same transformations again; they should be deduplicated
		duplicates := [][]string{
			{"old1", "new1"},
			{"old3", "new3"},
		}
		r.SetCustomResponseTransformations(duplicates)

		// Count occurrences of old1->new1
		count := 0
		for _, tr := range r.CustomResponseTransformations {
			if len(tr) == 2 && tr[0] == "old1" && tr[1] == "new1" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("Expected old1->new1 to appear exactly once (dedup), appeared %d times", count)
		}

		// old3->new3 should be added
		found := false
		for _, tr := range r.CustomResponseTransformations {
			if len(tr) == 2 && tr[0] == "old3" && tr[1] == "new3" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected transformation [old3, new3] to be added")
		}
	})

	t.Run("wildcard mapping transformations are added", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.WildcardMapping = map[string]string{
			"wld.example.com": "extwld1",
		}

		r.SetCustomResponseTransformations(nil)

		// Should have added wildcard-based custom transformations
		found := false
		expected := fmt.Sprintf("\"%s.%s", "extwld1", "evil.com")
		expectedReplacement := fmt.Sprintf("\"%s%s.%s", CustomWildcardSeparator, "extwld1", "evil.com")
		for _, tr := range r.CustomResponseTransformations {
			if len(tr) == 2 && tr[0] == expected && tr[1] == expectedReplacement {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected wildcard transformation %q -> %q, got: %v", expected, expectedReplacement, r.CustomResponseTransformations)
		}
	})
}

// ---------------------------------------------------------------------------
// TestSetOrigins and TestGetOrigins
// ---------------------------------------------------------------------------
func TestSetAndGetOrigins(t *testing.T) {

	t.Run("basic set and get", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")

		r.SetOrigins(map[string]string{
			"cdn.example.com": "ext1",
			"api.example.com": "ext2",
		})

		got := r.GetOrigins()
		if got["cdn.example.com"] != "ext1" {
			t.Errorf("Expected cdn.example.com -> ext1, got %q", got["cdn.example.com"])
		}
		if got["api.example.com"] != "ext2" {
			t.Errorf("Expected api.example.com -> ext2, got %q", got["api.example.com"])
		}
	})

	t.Run("auto-numbering with -1 value", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")

		r.SetOrigins(map[string]string{
			"first.com":  "-1",
			"second.com": "-1",
		})

		got := r.GetOrigins()
		// Both should be auto-numbered starting from count of existing (0) + 1
		values := make(map[string]bool)
		for _, v := range got {
			values[v] = true
		}

		// With 0 pre-existing origins, -1 should assign "1" and "2"
		if !values["1"] || !values["2"] {
			t.Errorf("Expected auto-numbered values 1 and 2, got: %v", got)
		}
	})

	t.Run("merge logic preserves existing origins", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")

		r.SetOrigins(map[string]string{
			"first.com": "ext1",
		})
		r.SetOrigins(map[string]string{
			"second.com": "ext2",
		})

		got := r.GetOrigins()
		if got["first.com"] != "ext1" {
			t.Errorf("Expected first.com -> ext1 to be preserved, got %q", got["first.com"])
		}
		if got["second.com"] != "ext2" {
			t.Errorf("Expected second.com -> ext2 to be added, got %q", got["second.com"])
		}
	})

	t.Run("lowercasing of keys", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")

		r.SetOrigins(map[string]string{
			"CDN.EXAMPLE.COM": "ext1",
		})

		got := r.GetOrigins()
		if _, ok := got["cdn.example.com"]; !ok {
			t.Errorf("Expected keys to be lowercased, got: %v", got)
		}
	})

	t.Run("empty origins does nothing", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.Origins = map[string]string{
			"existing.com": "ext1",
		}

		r.SetOrigins(map[string]string{})

		got := r.GetOrigins()
		if got["existing.com"] != "ext1" {
			t.Errorf("Empty SetOrigins should not modify existing origins, got: %v", got)
		}
	})

	t.Run("returns a copy not the original", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.Origins = map[string]string{
			"original.com": "ext1",
		}

		got := r.GetOrigins()
		got["original.com"] = "modified"

		// The internal map should not be modified
		internal := r.GetOrigins()
		if internal["original.com"] != "ext1" {
			t.Errorf("GetOrigins should return a copy, but modifying copy changed internal state")
		}
	})

	t.Run("auto-numbering with pre-existing origins", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.Origins = map[string]string{
			"existing1.com": "ext1",
			"existing2.com": "ext2",
		}

		r.SetOrigins(map[string]string{
			"new.com": "-1",
		})

		got := r.GetOrigins()
		// count starts at len(r.Origins) which was 2, then increments to 3
		if got["new.com"] != "3" {
			t.Errorf("Expected new.com -> 3 (auto-numbered after 2 existing), got %q", got["new.com"])
		}
	})
}

// ---------------------------------------------------------------------------
// TestSetAndGetRegexResponseTransformations
// ---------------------------------------------------------------------------
func TestSetAndGetRegexResponseTransformations(t *testing.T) {

	t.Run("set and get", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")

		transformations := [][]string{
			{`pattern\d+`, "replacement"},
			{`another.*`, "value"},
		}
		r.SetRegexResponseTransformations(transformations)

		got := r.GetRegexResponseTransformations()
		if len(got) != 2 {
			t.Fatalf("Expected 2 transformations, got %d", len(got))
		}
		if got[0][0] != `pattern\d+` || got[0][1] != "replacement" {
			t.Errorf("Expected first transformation [pattern\\d+, replacement], got %v", got[0])
		}
	})

	t.Run("returns a copy not the original", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.SetRegexResponseTransformations([][]string{{"a", "b"}})

		got := r.GetRegexResponseTransformations()
		got[0][0] = "modified"

		internal := r.GetRegexResponseTransformations()
		if internal[0][0] != "a" {
			t.Errorf("GetRegexResponseTransformations should return a copy")
		}
	})
}

// ---------------------------------------------------------------------------
// TestMakeReplacementsIntegration
// ---------------------------------------------------------------------------
func TestMakeReplacementsIntegration(t *testing.T) {
	// Full integration test: set up a Replacer with origins, wildcards,
	// and custom transformations, then verify the complete replacement sets.

	t.Run("full integration", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.ExternalOrigin = []string{
			"cdn.example.com",
			"api.service.io",
			"*.wildcard.net",
		}

		err := r.DomainMapping()
		if err != nil {
			t.Fatalf("DomainMapping() returned error: %v", err)
		}

		r.CustomResponseTransformations = [][]string{
			{"custom-old", "custom-new"},
		}

		r.MakeReplacements()

		// Verify forward replacements
		fwd := r.ForwardReplacements
		if !sliceContainsPair(fwd, "evil.com", "target.com") {
			t.Errorf("Forward should map evil.com -> target.com")
		}

		// Check that ext origins are in forward
		origins := r.GetOrigins()
		for domain, mapping := range origins {
			if strings.HasPrefix(mapping, WildcardLabel) {
				continue
			}
			from := fmt.Sprintf("%s.%s", mapping, "evil.com")
			if !sliceContainsPair(fwd, from, domain) {
				t.Errorf("Forward should map %s -> %s", from, domain)
			}
		}

		// Verify backward replacements contain boundary variations
		bwd := r.BackwardReplacements
		if !sliceContainsPair(bwd, " target.com", " evil.com") {
			t.Errorf("Backward should contain space-boundary variation")
		}

		// Verify backward contains external origin mappings
		for domain, mapping := range origins {
			if strings.HasPrefix(mapping, WildcardLabel) {
				continue
			}
			to := fmt.Sprintf("%s.%s", mapping, "evil.com")
			if !sliceContainsPair(bwd, domain, to) {
				t.Errorf("Backward should map %s -> %s", domain, to)
			}
		}

		// Verify wildcard replacements
		wm := r.GetWildcardMapping()
		for domain, mapping := range wm {
			fwdFrom := fmt.Sprintf("%s.%s", mapping, "evil.com")
			if !sliceContainsPair(r.ForwardWildcardReplacements, fwdFrom, domain) {
				t.Errorf("ForwardWildcard should map %s -> %s", fwdFrom, domain)
			}
			if !sliceContainsPair(r.BackwardWildcardReplacements, domain, fwdFrom) {
				t.Errorf("BackwardWildcard should map %s -> %s", domain, fwdFrom)
			}
		}

		// Verify custom transformations are in last backward
		if !sliceContainsPair(r.LastBackwardReplacements, "custom-old", "custom-new") {
			t.Errorf("LastBackward should contain custom transformations")
		}
	})
}

// ---------------------------------------------------------------------------
// TestGetAndSetWildcardMapping
// ---------------------------------------------------------------------------
func TestGetAndSetWildcardMapping(t *testing.T) {

	t.Run("set and get wildcard mapping", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")

		r.SetWildcardMapping("example.com", "extwld1")
		r.SetWildcardMapping("other.com", "extwld2")

		got := r.GetWildcardMapping()
		if got["example.com"] != "extwld1" {
			t.Errorf("Expected example.com -> extwld1, got %q", got["example.com"])
		}
		if got["other.com"] != "extwld2" {
			t.Errorf("Expected other.com -> extwld2, got %q", got["other.com"])
		}
	})

	t.Run("returns a copy not the original", func(t *testing.T) {
		r := newTestReplacer("evil.com", "target.com", "ext")
		r.SetWildcardMapping("original.com", "extwld1")

		got := r.GetWildcardMapping()
		got["original.com"] = "modified"

		internal := r.GetWildcardMapping()
		if internal["original.com"] != "extwld1" {
			t.Errorf("GetWildcardMapping should return a copy")
		}
	})
}

// ---------------------------------------------------------------------------
// TestSetWildcardDomain
// ---------------------------------------------------------------------------
func TestSetWildcardDomain(t *testing.T) {
	r := newTestReplacer("evil.com", "target.com", "ext")

	r.SetWildcardDomain("extwld1")
	if r.WildcardDomain != "extwld1" {
		t.Errorf("Expected WildcardDomain = extwld1, got %q", r.WildcardDomain)
	}

	r.SetWildcardDomain("extwld5")
	if r.WildcardDomain != "extwld5" {
		t.Errorf("Expected WildcardDomain = extwld5, got %q", r.WildcardDomain)
	}
}

// ---------------------------------------------------------------------------
// TestGetForwardReplacements (sorted getter)
// ---------------------------------------------------------------------------
func TestGetForwardReplacements(t *testing.T) {
	r := newTestReplacer("evil.com", "target.com", "ext")
	r.ForwardReplacements = []string{
		"a.evil.com", "short.com",
		"b.evil.com", "muchlongervalue.com",
	}
	r.ForwardWildcardReplacements = []string{
		"c.evil.com", "wild.com",
	}

	got := r.GetForwardReplacements()

	// Should include both regular and wildcard, sorted by NewVal length descending
	if len(got) < 6 {
		t.Fatalf("Expected at least 6 elements, got %d: %v", len(got), got)
	}

	// The first pair should be the one with the longest NewVal
	if got[0] != "b.evil.com" || got[1] != "muchlongervalue.com" {
		t.Errorf("First pair should be longest NewVal, got [%q, %q]", got[0], got[1])
	}
}

// ---------------------------------------------------------------------------
// TestGetBackwardReplacements (sorted getter)
// ---------------------------------------------------------------------------
func TestGetBackwardReplacements(t *testing.T) {
	r := newTestReplacer("evil.com", "target.com", "ext")
	r.BackwardReplacements = []string{
		"short.com", "a.evil.com",
		"muchlongervalue.com", "b.evil.com",
	}

	got := r.GetBackwardReplacements()

	// Should be sorted by OldVal length descending
	if len(got) < 4 {
		t.Fatalf("Expected at least 4 elements, got %d: %v", len(got), got)
	}

	// The first pair should be the one with the longest OldVal
	if got[0] != "muchlongervalue.com" || got[1] != "b.evil.com" {
		t.Errorf("First pair should be longest OldVal, got [%q, %q]", got[0], got[1])
	}
}

// ---------------------------------------------------------------------------
// TestGetLastBackwardReplacements (includes wildcard)
// ---------------------------------------------------------------------------
func TestGetLastBackwardReplacements(t *testing.T) {
	r := newTestReplacer("evil.com", "target.com", "ext")
	r.LastBackwardReplacements = []string{
		"last.com", "last.evil.com",
	}
	r.BackwardWildcardReplacements = []string{
		"wildcard.com", "wld.evil.com",
	}

	got := r.GetLastBackwardReplacements()

	// Should include both last backward and wildcard backward
	foundLast := false
	foundWild := false
	for i := 0; i+1 < len(got); i += 2 {
		if got[i] == "last.com" && got[i+1] == "last.evil.com" {
			foundLast = true
		}
		if got[i] == "wildcard.com" && got[i+1] == "wld.evil.com" {
			foundWild = true
		}
	}
	if !foundLast {
		t.Errorf("GetLastBackwardReplacements should include last backward replacements")
	}
	if !foundWild {
		t.Errorf("GetLastBackwardReplacements should include backward wildcard replacements")
	}
}
