package logstream

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/log"
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

func TestAppendCapsRingAtMax(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < 5; i++ {
		l := log.LogLine{Time: time.Now(), Level: "inf", Text: string(rune('a' + i))}
		if err := Append(l, 3); err != nil {
			t.Fatal(err)
		}
	}
	list, err := List(10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("list = %d, want 3 (ring cap)", len(list))
	}
}

func TestListReturnsNewestFirst(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < 3; i++ {
		l := log.LogLine{Time: time.Now(), Level: "inf", Text: string(rune('a' + i))}
		if err := Append(l, 10); err != nil {
			t.Fatal(err)
		}
	}
	list, err := List(10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].Text != "c" || list[2].Text != "a" {
		t.Fatalf("list = %+v, want newest first [c b a]", list)
	}
}

func TestListLevelFilterDropsOtherLevels(t *testing.T) {
	newTestRedis(t)
	_ = Append(log.LogLine{Level: "inf", Text: "info line"}, 10)
	_ = Append(log.LogLine{Level: "war", Text: "warn line"}, 10)
	_ = Append(log.LogLine{Level: "war", Text: "another warn"}, 10)

	list, err := List(10, "war")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("filtered list = %d, want 2", len(list))
	}
	for _, l := range list {
		if l.Level != "war" {
			t.Fatalf("unexpected level in filtered list: %+v", l)
		}
	}
}

func TestClearEmptiesList(t *testing.T) {
	newTestRedis(t)
	_ = Append(log.LogLine{Level: "inf", Text: "x"}, 10)
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	list, err := List(10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("list after Clear = %d, want 0", len(list))
	}
}
