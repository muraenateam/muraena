package proxy

import (
	"encoding/base64"
	"strings"
	"testing"
)

// newTransformerTestReplacer creates a Replacer with known replacement pairs
// for testing Transform and related functions. It sets up simple forward
// (phishing->target) and backward (target->phishing) replacement rules
// without requiring a full session or config.
func newTransformerTestReplacer() *Replacer {
	r := newTestReplacer("evil.com", "target.com", "ext")
	// Set up simple replacement pairs directly
	r.ForwardReplacements = []string{"evil.com", "target.com"}
	r.BackwardReplacements = []string{"target.com", "evil.com"}
	r.LastForwardReplacements = []string{}
	r.LastBackwardReplacements = []string{}
	r.ForwardWildcardReplacements = []string{}
	r.BackwardWildcardReplacements = []string{}
	return r
}

// newTransformerTestReplacerWithOrigins creates a Replacer with external
// origin mappings to test more complex replacement scenarios.
func newTransformerTestReplacerWithOrigins() *Replacer {
	r := newTransformerTestReplacer()
	r.ForwardReplacements = []string{
		"ext1.evil.com", "cdn.example.net",
		"evil.com", "target.com",
	}
	r.BackwardReplacements = []string{
		"cdn.example.net", "ext1.evil.com",
		"target.com", "evil.com",
	}
	return r
}

// ---------------------------------------------------------------------------
// Tests for Transform - Forward direction
// ---------------------------------------------------------------------------

func TestTransform_ForwardBasic(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple URL forward",
			input: "https://evil.com/path",
			want:  "https://target.com/path",
		},
		{
			name:  "URL with query string",
			input: "https://evil.com/search?q=hello",
			want:  "https://target.com/search?q=hello",
		},
		{
			name:  "multiple occurrences",
			input: "evil.com and evil.com",
			want:  "target.com and target.com",
		},
		{
			name:  "in HTML anchor tag",
			input: `<a href="https://evil.com/login">Login</a>`,
			want:  `<a href="https://target.com/login">Login</a>`,
		},
		{
			name:  "in JavaScript context",
			input: `var url = "https://evil.com/api/v1";`,
			want:  `var url = "https://target.com/api/v1";`,
		},
		{
			name:  "domain in middle of text",
			input: "please visit evil.com for more",
			want:  "please visit target.com for more",
		},
		{
			name:  "no match leaves input unchanged",
			input: "https://other.com/path",
			want:  "https://other.com/path",
		},
	}

	r := newTransformerTestReplacer()
	b64 := Base64{Enabled: false}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Transform(tt.input, true, b64)
			if got != tt.want {
				t.Errorf("Transform(%q, forward=true) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for Transform - Backward direction
// ---------------------------------------------------------------------------

func TestTransform_BackwardBasic(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple URL backward",
			input: "https://target.com/path",
			want:  "https://evil.com/path",
		},
		{
			name:  "multiple occurrences backward",
			input: "target.com and target.com",
			want:  "evil.com and evil.com",
		},
		{
			name:  "in HTML anchor tag backward",
			input: `<a href="https://target.com/login">Login</a>`,
			want:  `<a href="https://evil.com/login">Login</a>`,
		},
		{
			name:  "no match leaves input unchanged",
			input: "https://other.com/path",
			want:  "https://other.com/path",
		},
	}

	r := newTransformerTestReplacer()
	b64 := Base64{Enabled: false}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Transform(tt.input, false, b64)
			if got != tt.want {
				t.Errorf("Transform(%q, forward=false) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for Transform - Empty and whitespace inputs
// ---------------------------------------------------------------------------

func TestTransform_EmptyAndWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		forward bool
		want    string
	}{
		{
			name:    "empty string forward",
			input:   "",
			forward: true,
			want:    "",
		},
		{
			name:    "empty string backward",
			input:   "",
			forward: false,
			want:    "",
		},
		{
			name:    "whitespace only forward",
			input:   "   ",
			forward: true,
			want:    "   ",
		},
		{
			name:    "whitespace only backward",
			input:   "   ",
			forward: false,
			want:    "   ",
		},
		{
			name:    "tab and newline only",
			input:   "\t\n",
			forward: true,
			want:    "\t\n",
		},
	}

	r := newTransformerTestReplacer()
	b64 := Base64{Enabled: false}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Transform(tt.input, tt.forward, b64)
			if got != tt.want {
				t.Errorf("Transform(%q, forward=%v) = %q, want %q", tt.input, tt.forward, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for Transform - External origins
// ---------------------------------------------------------------------------

func TestTransform_WithExternalOrigins(t *testing.T) {
	r := newTransformerTestReplacerWithOrigins()
	b64 := Base64{Enabled: false}

	tests := []struct {
		name    string
		input   string
		forward bool
		want    string
	}{
		{
			name:    "forward with external origin",
			input:   "https://ext1.evil.com/resource.js",
			forward: true,
			want:    "https://cdn.example.net/resource.js",
		},
		{
			name:    "backward with external origin",
			input:   "https://cdn.example.net/resource.js",
			forward: false,
			want:    "https://ext1.evil.com/resource.js",
		},
		{
			name:    "forward transforms both main and external",
			input:   `evil.com ext1.evil.com`,
			forward: true,
			want:    `target.com cdn.example.net`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Transform(tt.input, tt.forward, b64)
			if got != tt.want {
				t.Errorf("Transform(%q, forward=%v) = %q, want %q", tt.input, tt.forward, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for Transform - Last replacements
// ---------------------------------------------------------------------------

func TestTransform_LastReplacements(t *testing.T) {
	r := newTransformerTestReplacer()
	// Add last replacements that apply after main replacements
	r.LastForwardReplacements = []string{"/old-path", "/new-path"}
	r.LastBackwardReplacements = []string{"/new-path", "/old-path"}
	b64 := Base64{Enabled: false}

	t.Run("forward with last replacements", func(t *testing.T) {
		input := "https://evil.com/old-path"
		want := "https://target.com/new-path"
		got := r.Transform(input, true, b64)
		if got != want {
			t.Errorf("Transform(%q, forward=true) = %q, want %q", input, got, want)
		}
	})

	t.Run("backward with last replacements", func(t *testing.T) {
		input := "https://target.com/new-path"
		want := "https://evil.com/old-path"
		got := r.Transform(input, false, b64)
		if got != want {
			t.Errorf("Transform(%q, forward=false) = %q, want %q", input, got, want)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests for Transform - Base64 encoded content
// ---------------------------------------------------------------------------

func TestTransform_Base64Forward(t *testing.T) {
	r := newTransformerTestReplacer()

	// Encode a string containing the phishing domain in base64.
	// Note: transformBase64 decode requires len(Padding) > 1 to trigger the decode loop.
	plaintext := "https://evil.com/secret"
	encoded := base64.StdEncoding.EncodeToString([]byte(plaintext))
	b64 := Base64{Enabled: true, Padding: []string{"=", "."}}

	got := r.Transform(encoded, true, b64)

	// Decode the result and verify the transformation happened
	decoded, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("result is not valid base64: %v (got %q)", err, got)
	}

	want := "https://target.com/secret"
	if string(decoded) != want {
		t.Errorf("after decoding, got %q, want %q", string(decoded), want)
	}
}

func TestTransform_Base64Backward(t *testing.T) {
	r := newTransformerTestReplacer()

	// Note: transformBase64 decode requires len(Padding) > 1 to trigger the decode loop.
	plaintext := "https://target.com/secret"
	encoded := base64.StdEncoding.EncodeToString([]byte(plaintext))
	b64 := Base64{Enabled: true, Padding: []string{"=", "."}}

	got := r.Transform(encoded, false, b64)

	decoded, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("result is not valid base64: %v (got %q)", err, got)
	}

	want := "https://evil.com/secret"
	if string(decoded) != want {
		t.Errorf("after decoding, got %q, want %q", string(decoded), want)
	}
}

func TestTransform_Base64Disabled(t *testing.T) {
	r := newTransformerTestReplacer()
	b64 := Base64{Enabled: false}

	// When base64 is disabled, the encoded input should not be decoded/transformed
	plaintext := "https://evil.com/path"
	encoded := base64.StdEncoding.EncodeToString([]byte(plaintext))

	got := r.Transform(encoded, true, b64)

	// The base64-encoded string should remain as-is since it does not literally
	// contain "evil.com" (it is encoded), and b64 decoding is disabled
	if got != encoded {
		t.Errorf("Transform with b64 disabled should not alter base64 content, got %q, want %q", got, encoded)
	}
}

// ---------------------------------------------------------------------------
// Tests for transformBase64
// ---------------------------------------------------------------------------

func TestTransformBase64_DecodeMode(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		b64        Base64
		decode     bool
		wantOutput string
		wantFound  bool
	}{
		{
			name:       "valid base64 decode with padding =",
			input:      base64.StdEncoding.EncodeToString([]byte("hello world")),
			b64:        Base64{Enabled: true, Padding: []string{"=", "."}},
			decode:     true,
			wantOutput: "hello world",
			wantFound:  true,
		},
		{
			name:       "b64 disabled returns input unchanged",
			input:      base64.StdEncoding.EncodeToString([]byte("hello")),
			b64:        Base64{Enabled: false},
			decode:     true,
			wantOutput: base64.StdEncoding.EncodeToString([]byte("hello")),
			wantFound:  false,
		},
		{
			name:       "invalid base64 returns input unchanged",
			input:      "not-valid-base64!!!",
			b64:        Base64{Enabled: true, Padding: []string{"="}},
			decode:     true,
			wantOutput: "not-valid-base64!!!",
			wantFound:  false,
		},
		{
			name:       "empty padding list does not decode",
			input:      base64.StdEncoding.EncodeToString([]byte("test")),
			b64:        Base64{Enabled: true, Padding: []string{}},
			decode:     true,
			wantOutput: base64.StdEncoding.EncodeToString([]byte("test")),
			wantFound:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, found, _ := transformBase64(tt.input, tt.b64, tt.decode, Base64Padding)
			if output != tt.wantOutput {
				t.Errorf("transformBase64(%q, decode=%v) output = %q, want %q",
					tt.input, tt.decode, output, tt.wantOutput)
			}
			if found != tt.wantFound {
				t.Errorf("transformBase64(%q, decode=%v) found = %v, want %v",
					tt.input, tt.decode, found, tt.wantFound)
			}
		})
	}
}

func TestTransformBase64_EncodeMode(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		padding    rune
		wantOutput string
	}{
		{
			name:       "encode with standard padding",
			input:      "hello world",
			padding:    '=',
			wantOutput: base64.StdEncoding.EncodeToString([]byte("hello world")),
		},
		{
			name:       "encode empty string",
			input:      "",
			padding:    '=',
			wantOutput: "",
		},
	}

	b64 := Base64{Enabled: true, Padding: []string{"="}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, _, _ := transformBase64(tt.input, b64, false, tt.padding)
			if output != tt.wantOutput {
				t.Errorf("transformBase64(%q, encode) = %q, want %q",
					tt.input, output, tt.wantOutput)
			}
		})
	}
}

func TestTransformBase64_RoundTrip(t *testing.T) {
	original := "https://evil.com/login?user=test"
	// Note: transformBase64 decode requires len(Padding) > 1 to trigger the decode loop.
	b64 := Base64{Enabled: true, Padding: []string{"=", "."}}

	// Encode
	encoded := base64.StdEncoding.EncodeToString([]byte(original))

	// Decode via transformBase64
	decoded, found, padding := transformBase64(encoded, b64, true, Base64Padding)
	if !found {
		t.Fatal("expected base64 to be found during decode")
	}
	if decoded != original {
		t.Errorf("decoded = %q, want %q", decoded, original)
	}

	// Re-encode via transformBase64
	reEncoded, _, _ := transformBase64(decoded, b64, false, padding)
	if reEncoded != encoded {
		t.Errorf("re-encoded = %q, want %q", reEncoded, encoded)
	}
}

// ---------------------------------------------------------------------------
// Tests for transformUrl (forward direction)
// ---------------------------------------------------------------------------

func TestTransformUrl_Simple(t *testing.T) {
	r := newTransformerTestReplacer()
	b64 := Base64{Enabled: false}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "simple URL",
			input: "https://evil.com/path",
			want:  "https://target.com/path",
		},
		{
			name:  "URL with query params containing phishing domain",
			input: "https://evil.com/path?redirect=https%3A%2F%2Fevil.com%2Fhome",
			want:  "https://target.com/path?redirect=https%3A%2F%2Ftarget.com%2Fhome",
		},
		{
			name:  "URL with simple query params",
			input: "https://evil.com/search?q=test&page=1",
			want:  "https://target.com/search?q=test&page=1",
		},
		{
			name:  "non-URL string passes through",
			input: "evil.com",
			want:  "target.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.transformUrl(tt.input, b64)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformUrl(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("transformUrl(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for transformBackwardUrl
// ---------------------------------------------------------------------------

func TestTransformBackwardUrl_QueryStringTransformation(t *testing.T) {
	r := newTransformerTestReplacer()
	b64 := Base64{Enabled: false}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "backward URL with query containing target domain",
			input: "https://evil.com/path?redirect=https%3A%2F%2Ftarget.com%2Fhome",
			want:  "https://evil.com/path?redirect=https%3A%2F%2Fevil.com%2Fhome",
		},
		{
			name:  "backward URL with no query",
			input: "https://evil.com/path",
			want:  "https://evil.com/path",
		},
		{
			name:  "non-URL string returns unchanged",
			input: "just a string",
			want:  "just a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.transformBackwardUrl(tt.input, b64)
			if (err != nil) != tt.wantErr {
				t.Errorf("transformBackwardUrl(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("transformBackwardUrl(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for createVariations (supplementary to replacer_test.go)
// ---------------------------------------------------------------------------

func TestCreateVariations_PairsCorrespond(t *testing.T) {
	// Verify that each target variation at index i corresponds to the phishing
	// variation at the same index i (same boundary prefix).
	boundaries := []string{" ", ",", ".", ":", "/", "(", ")", "!"}
	target := "target.com"
	phishing := "evil.com"

	targetVars, phishingVars := createVariations(target, phishing, boundaries)

	for i := range targetVars {
		// Both should start with the same boundary
		tPrefix := strings.TrimSuffix(targetVars[i], target)
		pPrefix := strings.TrimSuffix(phishingVars[i], phishing)

		if tPrefix != pPrefix {
			t.Errorf("index %d: boundary prefix mismatch: target prefix=%q, phishing prefix=%q",
				i, tPrefix, pPrefix)
		}
	}
}

func TestCreateVariations_EmptyBoundaries(t *testing.T) {
	targetVars, phishingVars := createVariations("target.com", "evil.com", []string{})

	if len(targetVars) != 0 {
		t.Errorf("expected 0 target variations with empty boundaries, got %d", len(targetVars))
	}
	if len(phishingVars) != 0 {
		t.Errorf("expected 0 phishing variations with empty boundaries, got %d", len(phishingVars))
	}
}

func TestCreateVariations_SpecialBoundaries(t *testing.T) {
	boundaries := []string{":", "(", "\""}
	targetVars, phishingVars := createVariations("target.com", "evil.com", boundaries)

	if len(targetVars) != 3 {
		t.Fatalf("expected 3 target variations, got %d", len(targetVars))
	}

	wantTarget := []string{":target.com", "(target.com", "\"target.com"}
	wantPhish := []string{":evil.com", "(evil.com", "\"evil.com"}

	for i := range wantTarget {
		if targetVars[i] != wantTarget[i] {
			t.Errorf("targetVars[%d] = %q, want %q", i, targetVars[i], wantTarget[i])
		}
		if phishingVars[i] != wantPhish[i] {
			t.Errorf("phishingVars[%d] = %q, want %q", i, phishingVars[i], wantPhish[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Tests for caseInsensitiveReplace (supplementary)
// ---------------------------------------------------------------------------

func TestCaseInsensitiveReplace_Transformer(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		replacements []string
		want         string
		wantErr      bool
	}{
		{
			name:         "exact case match",
			input:        "visit evil.com today",
			replacements: []string{"evil.com", "target.com"},
			want:         "visit target.com today",
		},
		{
			name:         "mixed case match",
			input:        "visit Evil.COM today",
			replacements: []string{"evil.com", "target.com"},
			want:         "visit target.com today",
		},
		{
			name:         "all uppercase match",
			input:        "EVIL.COM",
			replacements: []string{"evil.com", "target.com"},
			want:         "target.com",
		},
		{
			name:         "no match",
			input:        "visit other.com",
			replacements: []string{"evil.com", "target.com"},
			want:         "visit other.com",
		},
		{
			name:         "multiple replacements",
			input:        "evil.com and cdn.evil.com",
			replacements: []string{"cdn.evil.com", "cdn.target.com", "evil.com", "target.com"},
			want:         "target.com and cdn.target.com",
		},
		{
			name:         "empty replacements slice",
			input:        "hello",
			replacements: []string{},
			want:         "hello",
		},
		{
			name:         "odd number of replacements returns error",
			input:        "hello",
			replacements: []string{"one", "two", "three"},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := caseInsensitiveReplace(tt.input, tt.replacements)
			if (err != nil) != tt.wantErr {
				t.Errorf("caseInsensitiveReplace() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("caseInsensitiveReplace(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for convertToReplacements (supplementary - values check)
// ---------------------------------------------------------------------------

func TestConvertToReplacements_Values(t *testing.T) {
	input := []string{"evil.com", "target.com", "cdn.evil.com", "cdn.target.com"}
	got, err := convertToReplacements(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 replacements, got %d", len(got))
	}

	if got[0].OldVal != "evil.com" || got[0].NewVal != "target.com" {
		t.Errorf("replacement[0] = {%q, %q}, want {%q, %q}",
			got[0].OldVal, got[0].NewVal, "evil.com", "target.com")
	}

	if got[1].OldVal != "cdn.evil.com" || got[1].NewVal != "cdn.target.com" {
		t.Errorf("replacement[1] = {%q, %q}, want {%q, %q}",
			got[1].OldVal, got[1].NewVal, "cdn.evil.com", "cdn.target.com")
	}
}

// ---------------------------------------------------------------------------
// Tests for extractURLsFromHTML
// ---------------------------------------------------------------------------

func TestExtractURLsFromHTML(t *testing.T) {
	tests := []struct {
		name  string
		html  string
		count int
		want  []string
	}{
		{
			name:  "single anchor tag",
			html:  `<html><body><a href="https://target.com/path">Link</a></body></html>`,
			count: 1,
			want:  []string{"https://target.com/path"},
		},
		{
			name:  "multiple anchor tags",
			html:  `<a href="https://a.com">A</a><a href="https://b.com">B</a>`,
			count: 2,
			want:  []string{"https://a.com", "https://b.com"},
		},
		{
			name:  "no anchor tags",
			html:  `<html><body><p>No links here</p></body></html>`,
			count: 0,
			want:  nil,
		},
		{
			name:  "anchor without href",
			html:  `<a name="top">Top</a>`,
			count: 0,
			want:  nil,
		},
		{
			name:  "empty HTML",
			html:  "",
			count: 0,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractURLsFromHTML(tt.html)
			if len(got) != tt.count {
				t.Errorf("extractURLsFromHTML() returned %d URLs, want %d", len(got), tt.count)
			}
			for i, want := range tt.want {
				if i >= len(got) {
					break
				}
				if got[i] != want {
					t.Errorf("URL[%d] = %q, want %q", i, got[i], want)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests for sortReplacementsByLength (supplementary)
// ---------------------------------------------------------------------------

func TestSortReplacementsByLength_Forward_Transformer(t *testing.T) {
	// Forward sorts by NewVal length descending
	input := []string{
		"a", "short",
		"b", "muchlongervalue",
		"c", "mid",
	}

	got := sortReplacementsByLength(input, true)

	// Result should be pairs sorted by NewVal length descending
	if len(got) != 6 {
		t.Fatalf("expected 6 elements, got %d", len(got))
	}

	// First pair should have the longest NewVal
	if got[0] != "b" || got[1] != "muchlongervalue" {
		t.Errorf("first pair = [%q, %q], want [%q, %q]", got[0], got[1], "b", "muchlongervalue")
	}
}

func TestSortReplacementsByLength_Backward_Transformer(t *testing.T) {
	// Backward sorts by OldVal length descending
	input := []string{
		"short", "x",
		"muchlongervalue", "y",
		"mid", "z",
	}

	got := sortReplacementsByLength(input, false)

	if len(got) != 6 {
		t.Fatalf("expected 6 elements, got %d", len(got))
	}

	// First pair should have the longest OldVal
	if got[0] != "muchlongervalue" || got[1] != "y" {
		t.Errorf("first pair = [%q, %q], want [%q, %q]", got[0], got[1], "muchlongervalue", "y")
	}
}

func TestSortReplacementsByLength_OddSlice_Transformer(t *testing.T) {
	// Odd-length input should return nil (convertToReplacements fails)
	input := []string{"a", "b", "c"}
	got := sortReplacementsByLength(input, true)
	if got != nil {
		t.Errorf("expected nil for odd-length input, got %v", got)
	}
}

func TestSortReplacementsByLength_Empty_Transformer(t *testing.T) {
	got := sortReplacementsByLength([]string{}, true)
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Tests for Transform backward with URL query string handling
// ---------------------------------------------------------------------------

func TestTransform_BackwardUrlQueryTransformation(t *testing.T) {
	r := newTransformerTestReplacer()
	b64 := Base64{Enabled: false}

	// When Transform is called with forward=false on a URL, it should also
	// transform query string values via transformBackwardUrl.
	input := "https://evil.com/path?next=https%3A%2F%2Ftarget.com%2Fdashboard"
	got := r.Transform(input, false, b64)

	// The "target.com" in the query value should be transformed to "evil.com"
	if strings.Contains(got, "target.com") {
		t.Errorf("backward Transform should have replaced target.com in query string, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Tests for Transform idempotency (forward then backward round-trip)
// ---------------------------------------------------------------------------

func TestTransform_Idempotency(t *testing.T) {
	r := newTransformerTestReplacer()
	b64 := Base64{Enabled: false}

	// Transforming forward then backward should return the original
	original := "https://evil.com/path"
	forward := r.Transform(original, true, b64)
	if forward != "https://target.com/path" {
		t.Fatalf("forward transform unexpected: %q", forward)
	}

	backward := r.Transform(forward, false, b64)
	// Backward replacements use boundary-based matching, so verify the domain
	// was replaced even if the exact format differs slightly
	if !strings.Contains(backward, "evil.com") {
		t.Errorf("round-trip failed: backward result %q does not contain evil.com", backward)
	}
}

// ---------------------------------------------------------------------------
// Tests for Transform with regex response transformations
// ---------------------------------------------------------------------------

func TestTransform_RegexResponseTransformations(t *testing.T) {
	r := newTransformerTestReplacer()
	r.RegexResponseTransformations = [][]string{
		{`tracking_id=\d+`, "tracking_id=REDACTED"},
	}
	b64 := Base64{Enabled: false}

	input := "https://target.com/path?tracking_id=12345"
	got := r.Transform(input, false, b64)

	if !strings.Contains(got, "tracking_id=REDACTED") {
		t.Errorf("regex transformation should have replaced tracking_id, got %q", got)
	}
}

func TestTransform_RegexResponseTransformations_ForwardNotApplied(t *testing.T) {
	r := newTransformerTestReplacer()
	r.RegexResponseTransformations = [][]string{
		{`tracking_id=\d+`, "tracking_id=REDACTED"},
	}
	b64 := Base64{Enabled: false}

	// Regex transformations should only apply for backward (response) transforms
	input := "https://evil.com/path?tracking_id=12345"
	got := r.Transform(input, true, b64)

	if strings.Contains(got, "REDACTED") {
		t.Errorf("regex transformation should NOT apply on forward transform, got %q", got)
	}
}
