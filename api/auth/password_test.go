package auth

import "testing"

func TestHashAndCompare(t *testing.T) {
	h, err := HashPassword("s3cret")
	if err != nil {
		t.Fatal(err)
	}
	if h == "s3cret" {
		t.Fatal("hash equals plaintext")
	}
	if !ComparePassword(h, "s3cret") {
		t.Fatal("correct password rejected")
	}
	if ComparePassword(h, "wrong") {
		t.Fatal("wrong password accepted")
	}
}
