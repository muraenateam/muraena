package log

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupTestLog resets the package-level logger/tap registries and wires a
// single file-backed output so do()/Raw() do not panic ("No Output added")
// and do not spam the test runner's stdout.
func setupTestLog(t *testing.T) {
	t.Helper()
	lock.Lock()
	loggers = map[string]logger{}
	taps = map[string]func(LogLine){}
	lock.Unlock()

	f := filepath.Join(t.TempDir(), "test.log")
	if err := AddOutput(f, VERBOSE, FormatConfigBasic, true); err != nil {
		t.Fatalf("AddOutput: %v", err)
	}
}

func TestTapReceivesOneLineOnInfo(t *testing.T) {
	setupTestLog(t)
	ch := make(chan LogLine, 4)
	RegisterTap("t1", func(l LogLine) { ch <- l })
	defer UnregisterTap("t1")

	Info("hello %s", "world")

	select {
	case l := <-ch:
		if l.Text != "hello world" {
			t.Fatalf("text = %q, want %q", l.Text, "hello world")
		}
		if l.Level != "inf" {
			t.Fatalf("level = %q, want %q", l.Level, "inf")
		}
	case <-time.After(time.Second):
		t.Fatal("tap did not receive a line")
	}

	select {
	case extra := <-ch:
		t.Fatalf("received a second line, want exactly one: %+v", extra)
	default:
	}
}

func TestTapReceivesOneLineOnRawWithRawLevel(t *testing.T) {
	setupTestLog(t)
	ch := make(chan LogLine, 4)
	RegisterTap("t1", func(l LogLine) { ch <- l })
	defer UnregisterTap("t1")

	Raw("raw message %d", 42)

	select {
	case l := <-ch:
		if l.Text != "raw message 42" {
			t.Fatalf("text = %q, want %q", l.Text, "raw message 42")
		}
		if l.Level != "RAW" {
			t.Fatalf("level = %q, want %q", l.Level, "RAW")
		}
	case <-time.After(time.Second):
		t.Fatal("tap did not receive a line")
	}

	select {
	case extra := <-ch:
		t.Fatalf("received a second line, want exactly one: %+v", extra)
	default:
	}
}

func TestTapStripsANSIEffects(t *testing.T) {
	setupTestLog(t)
	ch := make(chan LogLine, 4)
	RegisterTap("t1", func(l LogLine) { ch <- l })
	defer UnregisterTap("t1")

	Info("colou\x1b[31mred\x1b[0m text")

	l := <-ch
	if strings.Contains(l.Text, "\x1b[") {
		t.Fatalf("ANSI effects not stripped: %q", l.Text)
	}
	if l.Text != "coloured text" {
		t.Fatalf("text = %q, want %q", l.Text, "coloured text")
	}
}

func TestUnregisterTapStopsDelivery(t *testing.T) {
	setupTestLog(t)
	ch := make(chan LogLine, 4)
	RegisterTap("t1", func(l LogLine) { ch <- l })
	UnregisterTap("t1")

	Info("should not arrive")

	select {
	case l := <-ch:
		t.Fatalf("received a line after UnregisterTap: %+v", l)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestTapFullChannelDoesNotBlockDo verifies the documented non-blocking-send
// pattern (select{ case ch <- l: default: }) really does let do() return
// promptly even when the tap's own channel is already full — a slow/full
// consumer must drop lines rather than stall logging while `lock` is held.
func TestTapFullChannelDoesNotBlockDo(t *testing.T) {
	setupTestLog(t)
	ch := make(chan LogLine, 1)
	ch <- LogLine{} // fill the buffer

	RegisterTap("t1", func(l LogLine) {
		select {
		case ch <- l:
		default:
			// dropped: consumer is full/slow, must not block do()
		}
	})
	defer UnregisterTap("t1")

	done := make(chan struct{})
	go func() {
		Info("dropped message")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("do() blocked despite full channel + non-blocking tap send")
	}
}
