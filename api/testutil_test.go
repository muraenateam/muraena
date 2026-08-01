package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/session"
)

func setupTestRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	session.RedisPool = &redis.Pool{Dial: func() (redis.Conn, error) {
		return redis.Dial("tcp", mr.Addr())
	}}
	return mr
}

func newTestServer() *Server {
	cfg := &session.Configuration{}
	// applyDefaults is unexported in the session package; set the fields the
	// API server reads directly.
	cfg.Api.Bind = "127.0.0.1"
	cfg.Api.Port = 8443
	cfg.Api.JWT.AccessMinutes = 15
	cfg.Api.JWT.RefreshDays = 7

	tmp := filepath.Join(os.TempDir(), "muraena-test-config.toml")
	opts := core.GetDefaultOptions()
	opts.ConfigFilePath = &tmp
	sess := &session.Session{Options: opts}
	sess.SwapConfig(cfg)
	srv := New(sess)
	go srv.hub.Run()
	return srv
}
