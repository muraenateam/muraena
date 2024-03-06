package statichttp

import (
	"testing"
	"time"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

// init test
func init() {

	opt := core.GetDefaultOptions()
	opt.Debug = &[]bool{true}[0]

	log.Init(opt, false, "")
}

func TestStaticHTTP_Start(t *testing.T) {

	s := &session.Session{}
	s.Config = &session.Configuration{}
	s.Config.StaticServer = session.StaticHTTPConfig{
		Enabled: true,
		// ListeningHost: "",
		// ListeningPort: 9090,
		LocalPath: "c:\\windows\\system32\\",
		URLPath:   "/test/",
	}

	_, err := Load(s)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("StaticHTTP started successfully")

	// Sleep for 2minutes
	time.Sleep(2 * time.Minute)

	t.Log("StaticHTTP stopped successfully")
}
