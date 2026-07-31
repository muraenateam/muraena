package store

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/session"
)

const settingsKey = "api:settings"

// GetSetting returns a single settings field ("" if unset).
func GetSetting(field string) (string, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()
	v, err := redis.String(rc.Do("HGET", settingsKey, field))
	if err == redis.ErrNil {
		return "", nil
	}
	return v, err
}

// SetSetting writes a single settings field.
func SetSetting(field, value string) error {
	rc := session.RedisPool.Get()
	defer rc.Close()
	_, err := rc.Do("HSET", settingsKey, field, value)
	return err
}

// GetSettings returns all settings fields.
func GetSettings() (map[string]string, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()
	return redis.StringMap(rc.Do("HGETALL", settingsKey))
}

// GetSigningKey returns the JWT signing key, creating one on first use.
func GetSigningKey() ([]byte, error) {
	enc, err := GetSetting("jwtSigningKey")
	if err != nil {
		return nil, err
	}
	if enc != "" {
		return base64.StdEncoding.DecodeString(enc)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := SetSetting("jwtSigningKey", base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, err
	}
	return key, nil
}
