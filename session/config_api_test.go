package session

import "testing"

func TestApiConfigDefaults(t *testing.T) {
	c := Configuration{}
	c.Api.applyDefaults()
	if c.Api.Bind != "127.0.0.1" {
		t.Fatalf("Bind default = %q, want 127.0.0.1", c.Api.Bind)
	}
	if c.Api.Port != 8443 {
		t.Fatalf("Port default = %d, want 8443", c.Api.Port)
	}
	if c.Api.JWT.AccessMinutes != 15 {
		t.Fatalf("AccessMinutes default = %d, want 15", c.Api.JWT.AccessMinutes)
	}
	if c.Api.JWT.RefreshDays != 7 {
		t.Fatalf("RefreshDays default = %d, want 7", c.Api.JWT.RefreshDays)
	}
}
