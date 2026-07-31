package session

import (
	"testing"
)

func TestSession_CheckRedirect(t *testing.T) {
	c := &Configuration{}

	// INVALID REDIRECTS
	r := []Redirect{
		{RedirectTo: "example.com", HTTPStatusCode: 200},
	}

	c.Redirects = r
	c.CheckRedirect()
	if len(c.Redirects) != 0 {
		t.Errorf("Expected %d, got %d", len(r), len(c.Redirects))
	}

	// VALID REDIRECTS
	r = []Redirect{
		{Hostname: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Path: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Query: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Path: "TEST", Query: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Hostname: "TEST", Query: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Hostname: "TEST", Path: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Hostname: "TEST", Path: "TEST", Query: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},
		{Hostname: "TEST", Path: "TEST", Query: "TEST", RedirectTo: "example.com"},
	}

	c.Redirects = r
	c.CheckRedirect()
	if len(c.Redirects) != len(r) {
		t.Errorf("Expected %d, got %d", len(r), len(c.Redirects))
	}

	// MIX REDIRECTS
	r = []Redirect{
		{Hostname: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},         // VALID
		{RedirectTo: "example.com", HTTPStatusCode: 200},                           // INVALID
		{Hostname: "TEST", Path: "TEST", Query: "TEST", RedirectTo: "example.com"}, // VALID
	}

	c.Redirects = r
	c.CheckRedirect()

	// Expect length to be 2
	if len(c.Redirects) != 2 {
		t.Errorf("Expected %d, got %d", 2, len(c.Redirects))
	}

	// Expect last element to have HTTPStatusCode 302
	if c.Redirects[1].HTTPStatusCode != 302 {
		t.Errorf("Expected %d, got %d", 302, c.Redirects[1].HTTPStatusCode)
	}
}
