package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"
)

type contextKey string

const userContextKey contextKey = "user"

// requestLogger logs incoming HTTP requests.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s %s", r.RemoteAddr, r.Method, r.URL.Path, time.Since(start))
	})
}

// requireAuth validates the MURAENA_SESS cookie by verifying its HMAC signature
// and decoding the embedded user info. No server-side session state is needed —
// the cookie is self-contained. This ensures necrobrowser can replay captured
// cookies directly to Scoglio without depending on SQLite session records.
func requireAuth(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("MURAENA_SESS")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Verify HMAC signature and decode user info from the cookie value.
		email, role := verifyToken(cookie.Value)
		if email == "" {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Build User from the token payload — no DB lookup required for auth.
		user := &User{
			Email: email,
			Role:  role,
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// userFromContext retrieves the authenticated User from the request context.
func userFromContext(r *http.Request) *User {
	u, _ := r.Context().Value(userContextKey).(*User)
	return u
}
