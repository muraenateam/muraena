package auth

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"time"

	"github.com/muraenateam/muraena/api/store"
	"github.com/muraenateam/muraena/log"
)

// Bootstrap creates the default admin user if none exists.
func Bootstrap() error {
	_, ok, err := store.GetUser("admin")
	if err != nil {
		return err
	}
	if ok {
		return nil // already bootstrapped
	}

	pass := os.Getenv("MURAENA_API_ADMIN_PASS")
	generated := false
	if pass == "" {
		b := make([]byte, 18)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		pass = base64.RawURLEncoding.EncodeToString(b)
		generated = true
	}

	hash, err := HashPassword(pass)
	if err != nil {
		return err
	}
	if err := store.CreateUser(store.User{
		Name:     "admin",
		PassHash: hash,
		Role:     "admin",
		Created:  time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return err
	}

	if generated {
		log.Important("API admin bootstrapped. Username: admin  Password: %s  (shown once)", pass)
	} else {
		log.Info("API admin bootstrapped from MURAENA_API_ADMIN_PASS")
	}
	return nil
}
