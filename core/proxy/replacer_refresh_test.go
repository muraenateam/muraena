package proxy

import (
	"testing"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

func TestRefreshReplacer(t *testing.T) {
	cfg := &session.Configuration{}
	cfg.Proxy.Phishing = "phish.example"
	cfg.Proxy.Target = "target.example"
	cfg.Origins.ExternalOriginPrefix = "ext"
	cfg.Origins.OriginsMapping = map[string]string{}
	sess := &session.Session{Options: core.GetDefaultOptions()}
	sess.SwapConfig(cfg)

	// log must have at least one output configured, mirroring what main.go
	// does via log.Init(sess.Options, ...) before Run() touches the Replacer.
	log.Init(sess.Options, false, "")

	if err := RefreshReplacer(sess); err != nil {
		t.Fatal(err)
	}
	if replacer == nil || replacer.Phishing != "phish.example" {
		t.Fatalf("replacer not refreshed: %+v", replacer)
	}
}
