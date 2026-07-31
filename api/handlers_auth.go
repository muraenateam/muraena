package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/muraenateam/muraena/api/auth"
	"github.com/muraenateam/muraena/api/store"
)

func (s *Server) jwtCfg() (int, int) {
	j := s.sess.Config().Api.JWT
	return j.AccessMinutes, j.RefreshDays
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	u, ok, err := store.GetUser(in.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !ok || !auth.ComparePassword(u.PassHash, in.Password) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}
	am, rd := s.jwtCfg()
	access, refresh, err := auth.Issue(u.Name, u.Role, am, rd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"access": access, "refresh": refresh, "expiresIn": am * 60,
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Refresh string `json:"refresh"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid body")
		return
	}
	claims, err := auth.Verify(in.Refresh, "refresh")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid refresh token")
		return
	}
	// rotate: revoke the old jti for its remaining life
	ttl := time.Until(claims.ExpiresAt.Time)
	_ = auth.Revoke(claims.ID, ttl)

	u, ok, _ := store.GetUser(claims.Subject)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unknown user")
		return
	}
	am, rd := s.jwtCfg()
	access, refresh, err := auth.Issue(u.Name, u.Role, am, rd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{"access": access, "refresh": refresh, "expiresIn": am * 60})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Refresh string `json:"refresh"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.Refresh != "" {
		if claims, err := auth.Verify(in.Refresh, "refresh"); err == nil {
			_ = auth.Revoke(claims.ID, time.Until(claims.ExpiresAt.Time))
		}
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	c := claimsFrom(r)
	writeJSON(w, 200, map[string]string{"username": c.Subject, "role": c.Role})
}
