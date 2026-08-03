package log

import (
	"time"
)

// LogLine is a single, ANSI-stripped log message delivered to taps.
type LogLine struct {
	Time  time.Time `json:"time"`
	Level string    `json:"level"`
	Text  string    `json:"text"`
}

// taps is guarded by the package-level `lock` (the same mutex `do()` and
// `Raw()` already hold while emitting), so registering/unregistering a tap
// can never race with a fire.
var taps = map[string]func(LogLine){}

// RegisterTap adds (or replaces) a tap identified by id. The callback is
// invoked synchronously while `lock` is held, so it MUST be non-blocking:
// consumers that need to do I/O (Redis, WebSocket fan-out, ...) should push
// onto a small buffered channel with a non-blocking send and drain it from a
// separate goroutine.
func RegisterTap(id string, fn func(LogLine)) {
	lock.Lock()
	defer lock.Unlock()
	taps[id] = fn
}

// UnregisterTap removes a previously registered tap. Safe to call even if
// the id was never registered (no-op).
func UnregisterTap(id string) {
	lock.Lock()
	defer lock.Unlock()
	delete(taps, id)
}

// stripEffects removes ANSI/format escape sequences from s using the same
// regexes `logger.emit` uses when NoEffects is set.
func stripEffects(s string) string {
	for _, re := range reEffects {
		s = re.ReplaceAllString(s, "")
	}
	return s
}

// fireTaps delivers a single LogLine to every registered tap. Callers MUST
// already hold `lock`.
func fireTaps(level string, msg string) {
	if len(taps) == 0 {
		return
	}
	line := LogLine{
		Time:  time.Now(),
		Level: level,
		Text:  stripEffects(msg),
	}
	for _, fn := range taps {
		fn(line)
	}
}
