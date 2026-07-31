package api

import (
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
	sess := &session.Session{
		Options: core.GetDefaultOptions(),
		Config:  &session.Configuration{},
	}
	// applyDefaults is unexported in the session package; set the fields the
	// API server reads directly.
	sess.Config.Api.Bind = "127.0.0.1"
	sess.Config.Api.Port = 8443
	sess.Config.Api.JWT.AccessMinutes = 15
	sess.Config.Api.JWT.RefreshDays = 7
	return New(sess)
}
