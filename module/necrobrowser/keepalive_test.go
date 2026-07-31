package necrobrowser

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/core/db"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

func init() {
	log.Init(core.GetDefaultOptions(), false, "")
}

func TestKeepaliveOncePosts(t *testing.T) {
	var hits int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(200)
	}))
	defer ts.Close()

	sess := &session.Session{Options: core.GetDefaultOptions()}
	sess.SwapConfig(&session.Configuration{})
	m := &Necrobrowser{
		SessionModule:   session.NewSessionModule(Name, sess),
		Endpoint:        ts.URL,
		RequestTemplate: `{"tracker":"%%%TRACKER%%%","cookies":%%%COOKIES%%%,"creds":%%%CREDENTIALS%%%}`,
	}
	m.KeepaliveTemplate = m.RequestTemplate

	m.KeepaliveOnce("v1", []db.VictimCookie{{Name: "c", Value: "1", Expires: "2040-01-01 00:00:00 +0000 UTC"}}, `{"username":"u"}`)
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("necrobrowser hits = %d, want 1", hits)
	}
}
