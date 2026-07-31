package store

import (
	"fmt"

	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/session"
)

type User struct {
	Name     string `redis:"-"`
	PassHash string `redis:"passhash"`
	Role     string `redis:"role"`
	Created  string `redis:"created"`
}

func userKey(name string) string { return fmt.Sprintf("api:user:%s", name) }

func CreateUser(u User) error {
	rc := session.RedisPool.Get()
	defer rc.Close()
	key := userKey(u.Name)
	if _, err := rc.Do("HMSET", key, "passhash", u.PassHash, "role", u.Role, "created", u.Created); err != nil {
		return err
	}
	_, err := rc.Do("SADD", "api:users", u.Name)
	return err
}

func GetUser(name string) (*User, bool, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()
	vals, err := redis.Values(rc.Do("HGETALL", userKey(name)))
	if err != nil {
		return nil, false, err
	}
	if len(vals) == 0 {
		return nil, false, nil
	}
	u := User{Name: name}
	if err := redis.ScanStruct(vals, &u); err != nil {
		return nil, false, err
	}
	return &u, true, nil
}

func ListUsers() ([]User, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()
	names, err := redis.Strings(rc.Do("SMEMBERS", "api:users"))
	if err != nil {
		return nil, err
	}
	var out []User
	for _, n := range names {
		u, ok, err := GetUser(n)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, *u)
		}
	}
	return out, nil
}

func DeleteUser(name string) error {
	rc := session.RedisPool.Get()
	defer rc.Close()
	if _, err := rc.Do("DEL", userKey(name)); err != nil {
		return err
	}
	_, err := rc.Do("SREM", "api:users", name)
	return err
}

func CountAdmins() (int, error) {
	users, err := ListUsers()
	if err != nil {
		return 0, err
	}
	n := 0
	for _, u := range users {
		if u.Role == "admin" {
			n++
		}
	}
	return n, nil
}
