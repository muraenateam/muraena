package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/muraenateam/muraena/api/auth"
	"github.com/muraenateam/muraena/api/traffic"
	"github.com/muraenateam/muraena/core/capture"
)

func (s *Server) handleListTraffic(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	flows, err := traffic.List(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	f := traffic.Filter{
		Host:     r.URL.Query().Get("host"),
		VictimID: r.URL.Query().Get("victimID"),
		Method:   r.URL.Query().Get("method"),
	}
	if st := r.URL.Query().Get("status"); st != "" {
		f.Status, _ = strconv.Atoi(st)
	}
	out := make([]traffic.Summary, 0, len(flows))
	for _, fl := range flows {
		sum := traffic.SummaryOf(fl)
		if f.Match(sum) {
			out = append(out, sum)
		}
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleGetTraffic(w http.ResponseWriter, r *http.Request) {
	f, ok, err := traffic.Get(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "flow expired or not found")
		return
	}
	writeJSON(w, 200, f)
}

func (s *Server) handleClearTraffic(w http.ResponseWriter, r *http.Request) {
	if err := traffic.Clear(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "cleared"})
}

func (s *Server) handleTrafficStats(w http.ResponseWriter, r *http.Request) {
	n, _ := traffic.Count()
	writeJSON(w, 200, map[string]interface{}{
		"count": n, "dropped": capture.Dropped(),
	})
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // localhost-only bind
}

func (s *Server) handleTrafficWS(w http.ResponseWriter, r *http.Request) {
	// Authenticate before upgrading: token via ?token= query param.
	if _, err := auth.Verify(r.URL.Query().Get("token"), "access"); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid token")
		return
	}
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := traffic.NewClient()
	client.Filter = traffic.Filter{
		Host:     r.URL.Query().Get("host"),
		VictimID: r.URL.Query().Get("victimID"),
		Method:   r.URL.Query().Get("method"),
	}
	s.hub.Register(client)
	defer func() {
		s.hub.Unregister(client)
		_ = conn.Close()
	}()

	// reader: drop client on disconnect. On error, unregister immediately —
	// Unregister closes client.send, which unblocks the writer loop below
	// even if no matching traffic ever arrives (idle disconnect). hub.Run is
	// single-goroutine and Unregister guards with `if h.clients[c]`, so the
	// deferred Unregister below becomes an idempotent no-op (no double-close).
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				s.hub.Unregister(client)
				return
			}
		}
	}()
	for sum := range client.Send() {
		if err := conn.WriteJSON(map[string]interface{}{"type": "flow", "data": sum}); err != nil {
			return
		}
	}
}
