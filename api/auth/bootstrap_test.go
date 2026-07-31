package auth

import (
	"os"
	"testing"

	"github.com/muraenateam/muraena/api/store"
)

func TestBootstrapEnvPassword(t *testing.T) {
	testRedis(t)
	os.Setenv("MURAENA_API_ADMIN_PASS", "envpass")
	defer os.Unsetenv("MURAENA_API_ADMIN_PASS")

	if err := Bootstrap(); err != nil {
		t.Fatal(err)
	}
	u, ok, err := store.GetUser("admin")
	if err != nil || !ok {
		t.Fatalf("admin missing ok=%v err=%v", ok, err)
	}
	if !ComparePassword(u.PassHash, "envpass") {
		t.Fatal("admin password does not match env value")
	}
	if u.Role != "admin" {
		t.Fatalf("role = %q", u.Role)
	}
}

func TestBootstrapIdempotent(t *testing.T) {
	testRedis(t)
	os.Setenv("MURAENA_API_ADMIN_PASS", "one")
	defer os.Unsetenv("MURAENA_API_ADMIN_PASS")
	if err := Bootstrap(); err != nil {
		t.Fatal(err)
	}
	// Second run with a different env value must NOT overwrite.
	os.Setenv("MURAENA_API_ADMIN_PASS", "two")
	if err := Bootstrap(); err != nil {
		t.Fatal(err)
	}
	u, _, _ := store.GetUser("admin")
	if !ComparePassword(u.PassHash, "one") {
		t.Fatal("bootstrap overwrote existing admin")
	}
}
