package db

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/session"
)

func newTestRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	session.RedisPool = &redis.Pool{
		MaxIdle: 3, IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) { return redis.Dial("tcp", mr.Addr()) },
	}
	return mr
}

func seedVictim(t *testing.T, id string, instrumented bool, creds int) {
	t.Helper()
	v := &Victim{ID: id, IP: "1.2.3.4", UA: "ua", SessionInstrumented: instrumented, CredsCount: creds}
	if err := v.Store(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < creds; i++ {
		vc := &VictimCredential{Key: "Username", Value: "u", Time: "t"}
		if err := vc.Store(id); err != nil {
			t.Fatal(err)
		}
	}
}
