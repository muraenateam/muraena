package proxy

import (
	"encoding/base64"
	"reflect"
	"sort"
	"testing"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
)

func init() {
	log.Init(core.Options{
		Debug:    &[]bool{true}[0],
		Verbose:  &[]bool{false}[0],
		NoColors: &[]bool{true}[0],
	}, false, "")
}

func TestArmorDomain(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "empty slice",
			input: []string{},
			want:  nil,
		},
		{
			name:  "nil slice",
			input: nil,
			want:  nil,
		},
		{
			name:  "single domain unchanged",
			input: []string{"example.com"},
			want:  []string{"example.com"},
		},
		{
			name:  "lowercase conversion",
			input: []string{"Example.COM"},
			want:  []string{"example.com"},
		},
		{
			name:  "strip http protocol",
			input: []string{"http://example.com"},
			want:  []string{"example.com"},
		},
		{
			name:  "strip https protocol",
			input: []string{"https://example.com"},
			want:  []string{"example.com"},
		},
		{
			name:  "strip path after domain",
			input: []string{"example.com/path/to/resource"},
			want:  []string{"example.com"},
		},
		{
			name:  "strip protocol and path",
			input: []string{"https://example.com/some/path?query=1"},
			want:  []string{"example.com"},
		},
		{
			name:  "deduplicate identical entries",
			input: []string{"example.com", "example.com"},
			want:  []string{"example.com"},
		},
		{
			name:  "deduplicate is case-sensitive on original entry",
			input: []string{"Example.com", "example.com"},
			want:  []string{"example.com", "example.com"},
		},
		{
			name:  "multiple different domains",
			input: []string{"https://a.com/path", "http://B.COM", "c.com"},
			want:  []string{"a.com", "b.com", "c.com"},
		},
		{
			name:  "protocol with uppercase",
			input: []string{"HTTP://Example.com"},
			want:  []string{"example.com"},
		},
		{
			name:  "domain with port",
			input: []string{"example.com:8080"},
			want:  []string{"example.com:8080"},
		},
		{
			name:  "domain with port and path",
			input: []string{"https://example.com:8080/path"},
			want:  []string{"example.com:8080"},
		},
		{
			name:  "mixed duplicates and protocols",
			input: []string{"https://foo.com", "http://foo.com", "foo.com"},
			want:  []string{"foo.com", "foo.com", "foo.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ArmorDomain(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ArmorDomain(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsSubdomain(t *testing.T) {
	tests := []struct {
		name      string
		root      string
		subdomain string
		want      bool
	}{
		{
			name:      "valid subdomain",
			root:      "example.com",
			subdomain: "sub.example.com",
			want:      true,
		},
		{
			name:      "exact match",
			root:      "example.com",
			subdomain: "example.com",
			want:      true,
		},
		{
			name:      "completely different domain",
			root:      "example.com",
			subdomain: "other.com",
			want:      false,
		},
		{
			name:      "suffix match but not real subdomain (known limitation)",
			root:      "example.com",
			subdomain: "notexample.com",
			want:      true,
		},
		{
			name:      "deep subdomain",
			root:      "example.com",
			subdomain: "a.b.c.example.com",
			want:      true,
		},
		{
			name:      "empty root",
			root:      "",
			subdomain: "example.com",
			want:      true,
		},
		{
			name:      "empty subdomain",
			root:      "example.com",
			subdomain: "",
			want:      false,
		},
		{
			name:      "both empty",
			root:      "",
			subdomain: "",
			want:      true,
		},
		{
			name:      "root longer than subdomain",
			root:      "sub.example.com",
			subdomain: "example.com",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSubdomain(tt.root, tt.subdomain)
			if got != tt.want {
				t.Errorf("IsSubdomain(%q, %q) = %v, want %v", tt.root, tt.subdomain, got, tt.want)
			}
		})
	}
}

func TestBase64RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		padding int32
	}{
		{
			name:    "simple string with standard padding",
			input:   "hello world",
			padding: '=',
		},
		{
			name:    "simple string with custom dot padding",
			input:   "hello world",
			padding: '.',
		},
		{
			name:    "empty string with standard padding",
			input:   "",
			padding: '=',
		},
		{
			name:    "string with special characters",
			input:   "https://example.com/path?key=value&foo=bar",
			padding: '=',
		},
		{
			name:    "unicode string",
			input:   "hello, world! unicode text",
			padding: '=',
		},
		{
			name:    "string that requires padding",
			input:   "ab",
			padding: '=',
		},
		{
			name:    "string requiring no padding",
			input:   "abc",
			padding: '=',
		},
		{
			name:    "no padding character (NoPadding)",
			input:   "hello world",
			padding: base64.NoPadding,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := base64Encode(tt.input, tt.padding)
			decoded, ok := base64Decode(encoded, tt.padding)
			if !ok {
				t.Fatalf("base64Decode(%q, %q) returned ok=false", encoded, tt.padding)
			}
			if decoded != tt.input {
				t.Errorf("round-trip failed: input=%q, encoded=%q, decoded=%q", tt.input, encoded, decoded)
			}
		})
	}
}

func TestBase64Decode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		padding int32
		want    string
		wantOk  bool
	}{
		{
			name:    "valid base64 standard padding",
			input:   base64.StdEncoding.EncodeToString([]byte("test")),
			padding: '=',
			want:    "test",
			wantOk:  true,
		},
		{
			name:    "invalid base64",
			input:   "not-valid-base64!!!",
			padding: '=',
			want:    "",
			wantOk:  false,
		},
		{
			name:    "empty string",
			input:   "",
			padding: '=',
			want:    "",
			wantOk:  true,
		},
		{
			name:    "valid base64 with custom dot padding",
			input:   "aGVsbG8.",
			padding: '.',
			want:    "hello",
			wantOk:  true,
		},
		{
			name:    "standard base64 decoded with wrong padding fails",
			input:   "aGVsbG8=",
			padding: '.',
			want:    "",
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := base64Decode(tt.input, tt.padding)
			if ok != tt.wantOk {
				t.Errorf("base64Decode(%q, %q) ok = %v, want %v", tt.input, tt.padding, ok, tt.wantOk)
			}
			if got != tt.want {
				t.Errorf("base64Decode(%q, %q) = %q, want %q", tt.input, tt.padding, got, tt.want)
			}
		})
	}
}

func TestBase64Encode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		padding int32
		want    string
	}{
		{
			name:    "standard encoding with padding",
			input:   "hello",
			padding: '=',
			want:    "aGVsbG8=",
		},
		{
			name:    "custom dot padding",
			input:   "hello",
			padding: '.',
			want:    "aGVsbG8.",
		},
		{
			name:    "no padding needed",
			input:   "abc",
			padding: '=',
			want:    "YWJj",
		},
		{
			name:    "empty string",
			input:   "",
			padding: '=',
			want:    "",
		},
		{
			name:    "string with two padding chars",
			input:   "a",
			padding: '=',
			want:    "YQ==",
		},
		{
			name:    "string with two custom padding chars",
			input:   "a",
			padding: '.',
			want:    "YQ..",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := base64Encode(tt.input, tt.padding)
			if got != tt.want {
				t.Errorf("base64Encode(%q, %q) = %q, want %q", tt.input, tt.padding, got, tt.want)
			}
		})
	}
}

func TestIsWildcard(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid wildcard",
			input: "*.example.com",
			want:  true,
		},
		{
			name:  "no wildcard prefix",
			input: "example.com",
			want:  false,
		},
		{
			name:  "asterisk without dot",
			input: "*example.com",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "just wildcard prefix",
			input: "*.",
			want:  true,
		},
		{
			name:  "wildcard in middle",
			input: "sub.*.example.com",
			want:  false,
		},
		{
			name:  "double wildcard",
			input: "*.*.example.com",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWildcard(tt.input)
			if got != tt.want {
				t.Errorf("isWildcard(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGetPadding(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int32
	}{
		{
			name:  "standard equals padding",
			input: "=",
			want:  '=',
		},
		{
			name:  "custom dot padding",
			input: ".",
			want:  '.',
		},
		{
			name:  "multi-char falls back to Base64Padding",
			input: "ab",
			want:  Base64Padding,
		},
		{
			name:  "three chars falls back to Base64Padding",
			input: "abc",
			want:  Base64Padding,
		},
		{
			name:  "tilde single char",
			input: "~",
			want:  '~',
		},
		{
			name:  "space single char",
			input: " ",
			want:  ' ',
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPadding(tt.input)
			if got != tt.want {
				t.Errorf("getPadding(%q) = %v (%c), want %v (%c)", tt.input, got, got, tt.want, tt.want)
			}
		})
	}
}

// TestArmorDomainOrdering verifies that ArmorDomain preserves input order.
func TestArmorDomainOrdering(t *testing.T) {
	input := []string{"z.com", "a.com", "m.com"}
	got := ArmorDomain(input)
	want := []string{"z.com", "a.com", "m.com"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ArmorDomain(%v) = %v, want %v (order preserved)", input, got, want)
	}

	// Also verify it is NOT sorted (order should match input, not alphabetical)
	sorted := make([]string, len(got))
	copy(sorted, got)
	sort.Strings(sorted)
	if reflect.DeepEqual(got, sorted) && !reflect.DeepEqual(input, sorted) {
		t.Errorf("ArmorDomain(%v) appears to be sorted = %v, but should preserve input order", input, got)
	}
}

// TestArmorDomainProtocolCaseSensitivity verifies that protocol stripping handles mixed case.
func TestArmorDomainProtocolCaseSensitivity(t *testing.T) {
	// Note: ArmorDomain lowercases first, then strips protocol.
	// Since "HTTP://" becomes "http://" after ToLower, the prefix check should match.
	input := []string{"HTTP://Example.COM/path"}
	got := ArmorDomain(input)
	want := []string{"example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ArmorDomain(%v) = %v, want %v", input, got, want)
	}
}
