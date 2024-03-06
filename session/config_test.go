package session

import (
	"testing"
)

func TestSession_CheckRedirect(t *testing.T) {
	s := &Session{}
	s.Config = &Configuration{}

	// INVALID REDIRECTS
	r := []Redirect{
		{RedirectTo: "example.com", HTTPStatusCode: 200},
	}

	s.Config.Redirects = r
	s.CheckRedirect()
	if len(s.Config.Redirects) != 0 {
		t.Errorf("Expected %d, got %d", len(r), len(s.Config.Redirects))
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

	s.Config.Redirects = r
	s.CheckRedirect()
	if len(s.Config.Redirects) != len(r) {
		t.Errorf("Expected %d, got %d", len(r), len(s.Config.Redirects))
	}

	// MIX REDIRECTS
	r = []Redirect{
		{Hostname: "TEST", RedirectTo: "example.com", HTTPStatusCode: 200},         // VALID
		{RedirectTo: "example.com", HTTPStatusCode: 200},                           // INVALID
		{Hostname: "TEST", Path: "TEST", Query: "TEST", RedirectTo: "example.com"}, // VALID
	}

	s.Config.Redirects = r
	s.CheckRedirect()

	// Expect length to be 2
	if len(s.Config.Redirects) != 2 {
		t.Errorf("Expected %d, got %d", 2, len(s.Config.Redirects))
	}

	// Expect last element to have HTTPStatusCode 302
	if s.Config.Redirects[1].HTTPStatusCode != 302 {
		t.Errorf("Expected %d, got %d", 302, s.Config.Redirects[1].HTTPStatusCode)
	}
}
