package auth

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/session"
)

func testRedis(t *testing.T) *miniredis.Miniredis {
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

func TestIssueVerify(t *testing.T) {
	testRedis(t)
	access, refresh, err := Issue("admin", "admin", 15, 7)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Verify(access, "access")
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != "admin" || c.Role != "admin" {
		t.Fatalf("claims = %+v", c)
	}
	if _, err := Verify(access, "refresh"); err == nil {
		t.Fatal("access token accepted as refresh")
	}
	rc, err := Verify(refresh, "refresh")
	if err != nil {
		t.Fatal(err)
	}
	if err := Revoke(rc.ID, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(refresh, "refresh"); err == nil {
		t.Fatal("revoked refresh still verifies")
	}
}
