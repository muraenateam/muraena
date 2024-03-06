package session

import (
	"crypto/tls"
)

var tlsVersionToConst = map[string]uint16{
	"SSL3.0": tls.VersionSSL30,
	"TLS1.0": tls.VersionTLS10,
	"TLS1.1": tls.VersionTLS11,
	"TLS1.2": tls.VersionTLS12,
	"TLS1.3": tls.VersionTLS13,
}

var tlsRenegotiationToConst = map[string]tls.RenegotiationSupport{
	"NEVER":  tls.RenegotiateNever,
	"ONCE":   tls.RenegotiateOnceAsClient,
	"FREELY": tls.RenegotiateFreelyAsClient,
}

func (s *Session) GetTLSClientConfig() *tls.Config {
	cTLS := s.Config.TLS

	return &tls.Config{
		MinVersion:               tlsVersionToConst[cTLS.MinVersion],
		MaxVersion:               tlsVersionToConst[cTLS.MaxVersion],
		PreferServerCipherSuites: cTLS.PreferServerCipherSuites,
		SessionTicketsDisabled:   cTLS.SessionTicketsDisabled,
		NextProtos:               []string{"http/1.1"},
		Certificates:             make([]tls.Certificate, 1),
		InsecureSkipVerify:       cTLS.InsecureSkipVerify,
		Renegotiation:            tlsRenegotiationToConst[cTLS.RenegotiationSupport],
	}
}
