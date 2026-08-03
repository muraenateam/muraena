// Package logstream provides a Redis-backed ring buffer and WebSocket hub
// for Muraena's runtime log lines (see log.RegisterTap), mirroring the
// traffic capture ring in api/traffic.
package logstream

import (
	"encoding/json"

	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

const linesKey = "logs:lines"

// Append pushes a log line onto the ring and trims it to the newest max
// entries.
func Append(l log.LogLine, max int) error {
	rc := session.RedisPool.Get()
	defer rc.Close()

	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	if _, err := rc.Do("LPUSH", linesKey, b); err != nil {
		return err
	}
	if max > 0 {
		if _, err := rc.Do("LTRIM", linesKey, 0, max-1); err != nil {
			return err
		}
	}
	return nil
}

// List returns up to limit lines, newest first. When level is non-empty,
// only lines matching that level (case-sensitive, as stored) are returned.
func List(limit int, level string) ([]log.LogLine, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()
	if limit <= 0 {
		limit = 100
	}
	raw, err := redis.Strings(rc.Do("LRANGE", linesKey, 0, limit-1))
	if err != nil {
		return nil, err
	}
	out := make([]log.LogLine, 0, len(raw))
	for _, b := range raw {
		var l log.LogLine
		if err := json.Unmarshal([]byte(b), &l); err != nil {
			return nil, err
		}
		if level != "" && l.Level != level {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

// Clear removes all buffered log lines.
func Clear() error {
	rc := session.RedisPool.Get()
	defer rc.Close()
	_, err := rc.Do("DEL", linesKey)
	return err
}
