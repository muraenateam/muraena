package tracking

import (
	"regexp"
	"testing"
	"time"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/core/db"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/module/telegram"
	"github.com/muraenateam/muraena/session"
)

var m *Tracker

// init test
func init() {
	log.Init(core.Options{Debug: &[]bool{true}[0]}, false, "")

	s := &session.Session{}
	s.Config = &session.Configuration{}

	m = &Tracker{
		SessionModule: session.NewSessionModule(Name, s),
		Enabled:       true,
	}

	s.Register(&telegram.Telegram{
		SessionModule: session.NewSessionModule(telegram.Name, s),
		Enabled:       true,
		BotToken:      "1587304999:AAG4cH8VzJ1b8tbamq0VZM9C01KkDjY5IFo",
		ChatID:        []string{"-1001856562703"},
	}, nil)

	s.InitRedis()
}

// TestModuleName ensures the module is the same, just in case :)
func TestModuleName(t *testing.T) {
	module := "tracking"
	want := regexp.MustCompile(Name)
	if !want.MatchString(Name) {
		t.Fatalf(`The module name does not match: %q != %q`, module, want)
	}
}

// TestModuleName ensures the module is the same, just in case :)
func TestPushVictim(t *testing.T) {

	v := &db.Victim{
		ID:           "AAAAA",
		IP:           "192.157.1.1",
		UA:           "Parakalo file mou",
		RequestCount: 0,
		FirstSeen:    time.Now().UTC().Format("2006-01-02 15:04:05"),
		LastSeen:     time.Now().UTC().Format("2006-01-02 15:04:05"),
	}

	m.PushVictim(v)
}
