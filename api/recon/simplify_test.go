package recon

import (
	"testing"
)

func TestSimplifyDomains(t *testing.T) {
	in := []string{
		"www.example.com",
		"cdn.assets.example.com",
		"a.b.example.com",
		"example.com",
	}
	got := SimplifyDomains(in)
	// 3rd-level -> *.example.com ; 4th-level -> *.<3rd>.example.com ; apex kept.
	// The real algorithm collapses on the sub-label immediately below the 4th
	// level, so a.b.example.com -> *.b.example.com (NOT *.example.com).
	want := map[string]bool{
		"*.example.com":        true, // from www.example.com
		"*.assets.example.com": true, // from cdn.assets.example.com
		"*.b.example.com":      true, // from a.b.example.com
		"example.com":          true, // apex kept as-is
	}
	for _, d := range got {
		if !want[d] {
			t.Fatalf("unexpected domain %q in %v", d, got)
		}
	}
	if len(got) == 0 {
		t.Fatal("no domains returned")
	}
}
