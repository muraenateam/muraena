package db

import (
	"testing"

	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/session"
)

func TestDeleteVictim(t *testing.T) {
	newTestRedis(t)
	seedVictim(t, "v1", false, 1)
	if err := DeleteVictim("v1"); err != nil {
		t.Fatal(err)
	}
	all, err := GetAllVictims()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("victim still present: %d", len(all))
	}

	// Verify that all victim sub-keys were actually deleted
	rc := session.RedisPool.Get()
	defer rc.Close()
	remaining, err := redis.Strings(rc.Do("KEYS", "victim:v1*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("victim sub-keys still present: %v", remaining)
	}
}

func TestInstrumentedAndHijackedFilters(t *testing.T) {
	newTestRedis(t)
	seedVictim(t, "v1", true, 1)  // instrumented + hijacked
	seedVictim(t, "v2", false, 2) // hijacked only
	seedVictim(t, "v3", false, 0) // neither

	inst, err := GetInstrumentedVictims()
	if err != nil {
		t.Fatal(err)
	}
	if len(inst) != 1 || inst[0].ID != "v1" {
		t.Fatalf("instrumented = %+v", inst)
	}
	hj, err := GetHijackedVictims()
	if err != nil {
		t.Fatal(err)
	}
	if len(hj) != 2 {
		t.Fatalf("hijacked = %d, want 2", len(hj))
	}
}
