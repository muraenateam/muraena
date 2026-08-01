// Command apitest runs ONLY the Muraena API control plane (no reverse proxy),
// for manually testing the WebUI without standing up the full phishing proxy.
//
// It reuses session.New() (which loads the -config file and, when
// [api].enable=true, initializes Redis) and then runs api.Run in the
// foreground. proxy.Run is intentionally NOT started.
//
// Usage:
//
//	redis-server &
//	go run ./cmd/apitest -config config/config.toml
//
// Then browse http://<api.bind>:<api.port>/ (embedded WebUI, after `make webui`).
// The bootstrapped admin password is printed once on first boot, or pin it via
// MURAENA_API_ADMIN_PASS.
package main

import (
	"fmt"
	"os"

	"github.com/muraenateam/muraena/api"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

func main() {
	sess, err := session.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	log.Init(sess.Options, sess.Config().Log.Enabled, sess.Config().Log.FilePath)

	if !sess.Config().Api.Enable {
		log.Error("API is disabled — set [api].enable = true in your config to use apitest")
		os.Exit(1)
	}

	log.Important("apitest: starting API-only control plane (no reverse proxy)")

	// Blocks: serves REST + WebSocket + embedded WebUI until the process exits.
	api.Run(sess)
}
