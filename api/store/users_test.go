package store

import "testing"

func TestUserCRUD(t *testing.T) {
	newTestRedis(t)
	u := User{Name: "admin", PassHash: "h", Role: "admin", Created: "t"}
	if err := CreateUser(u); err != nil {
		t.Fatal(err)
	}
	got, ok, err := GetUser("admin")
	if err != nil || !ok {
		t.Fatalf("GetUser ok=%v err=%v", ok, err)
	}
	if got.Role != "admin" {
		t.Fatalf("role = %q", got.Role)
	}
	n, err := CountAdmins()
	if err != nil || n != 1 {
		t.Fatalf("CountAdmins = %d err=%v", n, err)
	}
	list, err := ListUsers()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListUsers len = %d err=%v", len(list), err)
	}
	if err := DeleteUser("admin"); err != nil {
		t.Fatal(err)
	}
	_, ok, _ = GetUser("admin")
	if ok {
		t.Fatal("user still present after delete")
	}
}
