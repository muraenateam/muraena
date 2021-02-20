package module

import (
	"github.com/muraenateam/muraena/module/crawler"
	"github.com/muraenateam/muraena/module/necrobrowser"
	"github.com/muraenateam/muraena/module/statichttp"
	"github.com/muraenateam/muraena/module/tracking"
	"github.com/muraenateam/muraena/module/watchdog"
	"github.com/muraenateam/muraena/session"
)

// LoadModules load modules
func LoadModules(s *session.Session) {
	s.Register(crawler.Load(s))
	s.Register(statichttp.Load(s))
	s.Register(tracking.Load(s))
	s.Register(necrobrowser.Load(s))
	s.Register(watchdog.Load(s))
}
