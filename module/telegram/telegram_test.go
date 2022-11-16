package telegram

import (
	"regexp"
	"testing"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
)

var m *Telegram

// init test
func init() {
	log.Init(core.Options{Debug: &[]bool{true}[0]}, false, "")

	// LoadModules load modules
	m = &Telegram{
		Enabled:  true,
		BotToken: "1587304999:AAG4cH8VzJ1b8tbamq0VZM9C01KkDjY5IFo",
		ChatID:   []string{"@muraenatest_5305919037"},
	}
}

// TestModuleName ensures the module is the same, just in case :)
func TestModuleName(t *testing.T) {
	module := "telegram"
	want := regexp.MustCompile(Name)
	if !want.MatchString(Name) {
		t.Fatalf(`The module name does not match: %q != %q`, module, want)
	}
}

func TestSendMessage(t *testing.T) {
	m.Send("Muraena testing message")
}
