package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/core/capture"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/module/tracking"
	"github.com/muraenateam/muraena/session"
)

// drain empties the capture channel before a test.
func drain() {
	for {
		select {
		case <-capture.Flows():
		default:
			return
		}
	}
}

func newTapTestProxy(t *testing.T, target string) *MuraenaProxy {
	t.Helper()
	cfg := &session.Configuration{}
	cfg.Proxy.Phishing = "phish.example"
	cfg.Proxy.Target = "target.example"
	cfg.Proxy.Protocol = "http://"
	cfg.Origins.OriginsMapping = map[string]string{}
	cfg.Api.Traffic.Enable = true
	cfg.Api.Traffic.CaptureBodies = true
	cfg.Transform.Response.CustomContent = [][]string{{"TARGET", "PHISH"}}

	sess := &session.Session{Options: core.GetDefaultOptions()}
	log.Init(sess.Options, false, "")
	sess.SwapConfig(cfg)
	sess.Register(tracking.Load(sess))

	r := &Replacer{}
	if err := r.Init(sess); err != nil {
		t.Fatal(err)
	}

	init := &MuraenaProxyInit{Origin: "phish.example", Target: target, Session: sess, Replacer: r}
	return init.Spawn()
}

func TestTapEmitsOriginalAndModified(t *testing.T) {
	drain()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello TARGET world")
	}))
	defer upstream.Close()

	mp := newTapTestProxy(t, upstream.URL)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	mp.ReverseProxy.ServeHTTP(rec, req)

	select {
	case f := <-capture.Flows():
		if f.ResBodyOriginal == f.ResBodyModified {
			t.Fatalf("expected original != modified body, got %q == %q", f.ResBodyOriginal, f.ResBodyModified)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no flow emitted")
	}
}
