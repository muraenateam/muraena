package capture

import "testing"

func TestEmitDeliversAndDrops(t *testing.T) {
	// drain one
	Emit(Flow{ID: "a"})
	select {
	case f := <-Flows():
		if f.ID != "a" {
			t.Fatalf("got %q", f.ID)
		}
	default:
		t.Fatal("expected a flow")
	}

	// saturate the buffer, then one more must be dropped
	before := Dropped()
	for i := 0; i < BufferSize()+10; i++ {
		Emit(Flow{ID: "x"})
	}
	if Dropped() <= before {
		t.Fatal("expected dropped counter to increase when saturated")
	}
}
