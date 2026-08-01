package traffic

import (
	"strings"
	"testing"

	"github.com/muraenateam/muraena/core/capture"
)

func TestTruncate(t *testing.T) {
	f := capture.Flow{ResBodyModified: strings.Repeat("x", 3000)}
	Truncate(&f, 1) // 1 KB
	if len(f.ResBodyModified) > 1024 {
		t.Fatalf("body not truncated: %d", len(f.ResBodyModified))
	}
	if !f.Truncated {
		t.Fatal("Truncated flag not set")
	}
}

func TestSaveListGetRing(t *testing.T) {
	newTestRedis(t)
	for i := 0; i < 5; i++ {
		id := string(rune('a' + i))
		if err := Save(capture.Flow{ID: id, Host: "h", Path: "/" + id}, 3, 60); err != nil {
			t.Fatal(err)
		}
	}
	list, err := List(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 { // ring capped at 3
		t.Fatalf("list = %d, want 3 (ring cap)", len(list))
	}
	if list[0].ID != "e" { // newest first
		t.Fatalf("newest = %q, want e", list[0].ID)
	}
	f, ok, err := Get("e")
	if err != nil || !ok || f.Path != "/e" {
		t.Fatalf("get e failed ok=%v err=%v", ok, err)
	}
}
