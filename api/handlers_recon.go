package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os/exec"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/muraenateam/muraena/api/auth"
	"github.com/muraenateam/muraena/api/recon"
)

func (s *Server) handleStartRecon(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Target   string `json:"target"`
		Depth    int    `json:"depth"`
		MaxPages int    `json:"maxPages"`
		Apply    *bool  `json:"apply"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Target == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "target required")
		return
	}
	if in.Depth <= 0 {
		in.Depth = 1
	}
	if in.MaxPages <= 0 {
		in.MaxPages = 20
	}
	nodePath := s.sess.Config().Recon.NodePath
	if _, err := exec.LookPath(nodePath); err != nil {
		writeError(w, http.StatusServiceUnavailable, "node_missing",
			"node not found; install Node.js and puppeteer (see puppeteer/README)")
		return
	}
	script := s.sess.Config().Recon.Script
	apply := in.Apply == nil || *in.Apply // default: auto-apply

	id, err := s.recon.Start(nodePath, script, in.Target, in.Depth, in.MaxPages,
		func(msg string) { s.reconHub.Broadcast(msg) })
	if err != nil {
		if errors.Is(err, recon.ErrBusy) {
			writeError(w, http.StatusConflict, "busy", "recon already running")
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	// auto-apply on completion
	if apply {
		go s.applyReconWhenDone(id)
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"reconID": id})
}

func (s *Server) applyReconWhenDone(id string) {
	for {
		j, ok := s.recon.Get(id)
		if !ok {
			return
		}
		if j.Status == "done" && j.Result != nil {
			patch := j.Result.ToPatch(s.sess.Config())
			cand, restart, err := s.sess.BuildCandidate(patch)
			if err == nil && len(restart) == 0 {
				_ = s.applyCandidate(cand, true) // origins touched -> refresh replacer
			}
			return
		}
		if j.Status == "failed" {
			return
		}
		// poll briefly
		time.Sleep(200 * time.Millisecond)
	}
}

func (s *Server) handleGetRecon(w http.ResponseWriter, r *http.Request) {
	j, ok := s.recon.Get(chi.URLParam(r, "id"))
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "recon job not found")
		return
	}
	writeJSON(w, 200, j)
}

func (s *Server) handleApplyRecon(w http.ResponseWriter, r *http.Request) {
	j, ok := s.recon.Get(chi.URLParam(r, "id"))
	if !ok || j.Result == nil {
		writeError(w, http.StatusNotFound, "not_found", "no result to apply")
		return
	}
	patch := j.Result.ToPatch(s.sess.Config())
	cand, restart, err := s.sess.BuildCandidate(patch)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if len(restart) > 0 {
		writeJSON(w, http.StatusConflict, map[string]interface{}{"restartRequired": true, "fields": restart})
		return
	}
	if err := s.applyCandidate(cand, true); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "applied"})
}

func (s *Server) handleReconWS(w http.ResponseWriter, r *http.Request) {
	if _, err := auth.Verify(r.URL.Query().Get("token"), "access"); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid token")
		return
	}
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := s.reconHub.NewClient()
	s.reconHub.Register(client)
	defer func() { s.reconHub.Unregister(client); _ = conn.Close() }()
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				s.reconHub.Unregister(client)
				return
			}
		}
	}()
	for msg := range client.Send() {
		if err := conn.WriteJSON(map[string]string{"type": "progress", "msg": msg}); err != nil {
			return
		}
	}
}
