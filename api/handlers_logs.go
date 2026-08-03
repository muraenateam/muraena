package api

import (
	"net/http"
	"strconv"

	"github.com/muraenateam/muraena/api/auth"
	"github.com/muraenateam/muraena/api/logstream"
)

// handleListLogs backfills the log ring, newest first, with an optional
// server-side level filter (?level=war, matching log.LevelNames values).
func (s *Server) handleListLogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	level := r.URL.Query().Get("level")
	lines, err := logstream.List(limit, level)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, 200, lines)
}

// handleLogsWS streams live log lines. Authenticates via ?token= (browsers
// cannot set an Authorization header on a WebSocket upgrade), same as
// handleTrafficWS/handleReconWS, so it must be mounted outside RequireAuth.
func (s *Server) handleLogsWS(w http.ResponseWriter, r *http.Request) {
	if _, err := auth.Verify(r.URL.Query().Get("token"), "access"); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid token")
		return
	}
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := logstream.NewClient()
	s.logHub.Register(client)
	defer func() {
		s.logHub.Unregister(client)
		_ = conn.Close()
	}()

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				s.logHub.Unregister(client)
				return
			}
		}
	}()

	for line := range client.Send() {
		if err := conn.WriteJSON(map[string]interface{}{"type": "log", "data": line}); err != nil {
			return
		}
	}
}
