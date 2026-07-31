package store

import (
	"bytes"
	"testing"
)

func TestGetSigningKeyStable(t *testing.T) {
	newTestRedis(t)
	k1, err := GetSigningKey()
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != 32 {
		t.Fatalf("key len = %d, want 32", len(k1))
	}
	k2, err := GetSigningKey()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("signing key not stable across calls")
	}
}

func TestSetGetSetting(t *testing.T) {
	newTestRedis(t)
	if err := SetSetting("jwtAccessMinutes", "20"); err != nil {
		t.Fatal(err)
	}
	v, err := GetSetting("jwtAccessMinutes")
	if err != nil {
		t.Fatal(err)
	}
	if v != "20" {
		t.Fatalf("got %q, want 20", v)
	}
}
