package db

import (
	"testing"
	"time"
)

func TestKeepaliveCRUD(t *testing.T) {
	newTestRedis(t)
	if err := SetKeepalive("v1", 10); err != nil {
		t.Fatal(err)
	}
	k, ok, err := GetKeepalive("v1")
	if err != nil || !ok {
		t.Fatalf("get ok=%v err=%v", ok, err)
	}
	if k.IntervalMin != "10" || k.Enabled != "1" {
		t.Fatalf("keepalive = %+v", k)
	}
	all, err := GetAllKeepalives()
	if err != nil || len(all) != 1 {
		t.Fatalf("all = %d err=%v", len(all), err)
	}
	now := time.Now()
	if err := TouchKeepalive("v1", now, now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	k, _, _ = GetKeepalive("v1")
	if k.NextRun == "" {
		t.Fatal("NextRun not set")
	}
	if err := DeleteKeepalive("v1"); err != nil {
		t.Fatal(err)
	}
	_, ok, _ = GetKeepalive("v1")
	if ok {
		t.Fatal("keepalive still present after delete")
	}
}
