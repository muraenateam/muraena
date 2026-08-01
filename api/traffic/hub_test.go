package traffic

import (
	"testing"
	"time"

	"github.com/muraenateam/muraena/core/capture"
)

func TestHubBroadcastRespectsFilter(t *testing.T) {
	h := NewHub()
	go h.Run()

	match := NewClient()
	match.Filter = Filter{Host: "good.example"}
	nomatch := NewClient()
	nomatch.Filter = Filter{Host: "other.example"}
	h.Register(match)
	h.Register(nomatch)

	h.Broadcast(SummaryOf(capture.Flow{ID: "1", Host: "good.example"}))

	select {
	case s := <-match.Send():
		if s.ID != "1" {
			t.Fatalf("got %q", s.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("matching client did not receive")
	}
	select {
	case <-nomatch.Send():
		t.Fatal("non-matching client received")
	case <-time.After(100 * time.Millisecond):
	}
}
