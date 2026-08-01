package capture

import (
	"sync/atomic"
	"time"
)

// Flow is a single captured request/response pair, including Muraena's edits.
type Flow struct {
	ID              string            `json:"id"`
	Timestamp       time.Time         `json:"timestamp"`
	VictimID        string            `json:"victimID"`
	Method          string            `json:"method"`
	Host            string            `json:"host"`
	Path            string            `json:"path"`
	Query           string            `json:"query"`
	Status          int               `json:"status"`
	ReqHeaders      map[string]string `json:"reqHeaders"`
	ReqBody         string            `json:"reqBody"`
	ResHeaders      map[string]string `json:"resHeaders"`
	ResBodyOriginal string            `json:"resBodyOriginal"`
	ResBodyModified string            `json:"resBodyModified"`
	ReqSize         int               `json:"reqSize"`
	ResSize         int               `json:"resSize"`
	DurationMs      int64             `json:"durationMs"`
	Truncated       bool              `json:"truncated"`
}

const bufferSize = 1024

var (
	ch      = make(chan Flow, bufferSize)
	dropped int64
)

// Emit queues a flow without ever blocking; drops (and counts) when the buffer is full.
func Emit(f Flow) {
	select {
	case ch <- f:
	default:
		atomic.AddInt64(&dropped, 1)
	}
}

func Flows() <-chan Flow { return ch }
func Dropped() int64     { return atomic.LoadInt64(&dropped) }
func BufferSize() int    { return bufferSize }
