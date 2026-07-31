package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/api/store"
	"github.com/muraenateam/muraena/session"
)

type Claims struct {
	jwt.RegisteredClaims
	Role string `json:"role,omitempty"`
	Typ  string `json:"typ"`
}

func newJTI() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())
}

// Issue returns a signed access and refresh token pair.
func Issue(sub, role string, accessMin, refreshDays int) (string, string, error) {
	key, err := store.GetSigningKey()
	if err != nil {
		return "", "", err
	}
	now := time.Now()

	mk := func(typ string, ttl time.Duration, withRole bool) (string, error) {
		c := Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   sub,
				ID:        newJTI(),
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			},
			Typ: typ,
		}
		if withRole {
			c.Role = role
		}
		return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(key)
	}

	access, err := mk("access", time.Duration(accessMin)*time.Minute, true)
	if err != nil {
		return "", "", err
	}
	refresh, err := mk("refresh", time.Duration(refreshDays)*24*time.Hour, false)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// Verify parses and validates a token, checks the expected typ, and (for
// refresh tokens) that the jti has not been revoked.
func Verify(token, typ string) (*Claims, error) {
	key, err := store.GetSigningKey()
	if err != nil {
		return nil, err
	}
	var c Claims
	_, err = jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return key, nil
	})
	if err != nil {
		return nil, err
	}
	if c.Typ != typ {
		return nil, fmt.Errorf("token type %q, want %q", c.Typ, typ)
	}
	if typ == "refresh" {
		revoked, err := isRevoked(c.ID)
		if err != nil {
			return nil, err
		}
		if revoked {
			return nil, errors.New("token revoked")
		}
	}
	return &c, nil
}

// Revoke marks a jti revoked for ttl.
func Revoke(jti string, ttl time.Duration) error {
	rc := session.RedisPool.Get()
	defer rc.Close()
	_, err := rc.Do("SET", "api:revoked:"+jti, "1", "EX", int(ttl.Seconds()))
	return err
}

func isRevoked(jti string) (bool, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()
	n, err := redis.Int(rc.Do("EXISTS", "api:revoked:"+jti))
	return n == 1, err
}
