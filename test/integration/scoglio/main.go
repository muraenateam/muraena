package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", ":9443", "Listen address (host:port)")
	certFile := flag.String("cert", "", "TLS certificate file (required)")
	keyFile := flag.String("key", "", "TLS private key file (required)")
	dbPath := flag.String("db", "scoglio.db", "SQLite database path")
	flag.Parse()

	if *certFile == "" || *keyFile == "" {
		log.Fatal("--cert and --key flags are required")
	}

	// Initialize database.
	db, err := initDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Seed default users.
	if err := seedUsers(db); err != nil {
		log.Fatalf("Failed to seed users: %v", err)
	}

	// Initialize templates.
	initTemplates()

	// Set up routing.
	mux := http.NewServeMux()

	// Public routes.
	mux.HandleFunc("/", handleHome)

	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleLoginPage(db)(w, r)
		case http.MethodPost:
			handleLoginSubmit(db)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/signup", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleSignupPage(db)(w, r)
		case http.MethodPost:
			handleSignupSubmit(db)(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Embedded static files.
	staticSub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("Failed to create static sub-filesystem: %v", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	// Protected routes (require auth).
	protectedDashboard := requireAuth(db, http.HandlerFunc(handleDashboard(db)))
	mux.Handle("/dashboard", protectedDashboard)

	protectedProfile := requireAuth(db, http.HandlerFunc(handleProfile(db)))
	mux.Handle("/profile", protectedProfile)

	protectedLogout := requireAuth(db, http.HandlerFunc(handleLogout(db)))
	mux.Handle("/logout", protectedLogout)

	// Wrap with request logging.
	handler := requestLogger(mux)

	fmt.Printf("Scoglio starting on %s (TLS)\n", *addr)
	fmt.Printf("  cert: %s\n", *certFile)
	fmt.Printf("  key:  %s\n", *keyFile)
	fmt.Printf("  db:   %s\n", *dbPath)

	if err := http.ListenAndServeTLS(*addr, *certFile, *keyFile, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
