package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// templates maps page names to their compiled templates.
// Each page template is a clone of the base template with its own
// "title" and "content" block definitions, avoiding block name collisions.
var templates map[string]*template.Template

func initTemplates() {
	base := template.Must(template.ParseFS(templateFS, "templates/base.html"))
	pages := []string{"home.html", "login.html", "signup.html", "dashboard.html", "profile.html"}

	templates = make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		clone := template.Must(base.Clone())
		templates[page] = template.Must(clone.ParseFS(templateFS, "templates/"+page))
	}
}

func renderTemplate(w http.ResponseWriter, name string, data *templateData) {
	t, ok := templates[name]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	if err := t.ExecuteTemplate(w, "base", data); err != nil {
		log.Printf("Template %s render error: %v", name, err)
	}
}

// templateData is passed to every template.
type templateData struct {
	User  *User
	Error string
	Now   time.Time
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderTemplate(w, "home.html", &templateData{Now: time.Now()})
}

func handleLoginPage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderTemplate(w, "login.html", &templateData{Now: time.Now()})
	}
}

func handleLoginSubmit(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			renderTemplate(w, "login.html", &templateData{Error: "Invalid form data", Now: time.Now()})
			return
		}

		email := r.FormValue("email")
		password := r.FormValue("password")

		user, err := getUserByEmail(db, email)
		if err != nil {
			renderTemplate(w, "login.html", &templateData{Error: "Invalid email or password", Now: time.Now()})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
			renderTemplate(w, "login.html", &templateData{Error: "Invalid email or password", Now: time.Now()})
			return
		}

		// Create 3 session tokens. MURAENA_SESS is a self-contained HMAC-signed
		// token — no server-side session state needed for validation.
		muraenaSess, necroBro, jsessionID, err := createSessionTokens(user)
		if err != nil {
			log.Printf("Failed to create session for %s: %v", email, err)
			renderTemplate(w, "login.html", &templateData{Error: "Internal server error", Now: time.Now()})
			return
		}

		// Set 3 cookies.
		http.SetCookie(w, &http.Cookie{
			Name:     "MURAENA_SESS",
			Value:    muraenaSess,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400, // 24 hours
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "NECRO_BRO",
			Value:    necroBro,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   3600, // 1 hour
		})
		http.SetCookie(w, &http.Cookie{
			Name:     "JSESSIONID",
			Value:    jsessionID,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   43200, // 12 hours
		})

		log.Printf("Login successful: %s (role: %s)", email, user.Role)
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	}
}

func handleSignupPage(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderTemplate(w, "signup.html", &templateData{Now: time.Now()})
	}
}

func handleSignupSubmit(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			renderTemplate(w, "signup.html", &templateData{Error: "Invalid form data", Now: time.Now()})
			return
		}

		email := r.FormValue("email")
		password := r.FormValue("password")

		if email == "" || password == "" {
			renderTemplate(w, "signup.html", &templateData{Error: "Email and password are required", Now: time.Now()})
			return
		}

		if len(password) < 6 {
			renderTemplate(w, "signup.html", &templateData{Error: "Password must be at least 6 characters", Now: time.Now()})
			return
		}

		if err := createUser(db, email, password, "user"); err != nil {
			renderTemplate(w, "signup.html", &templateData{Error: "Email already registered", Now: time.Now()})
			return
		}

		log.Printf("New user registered: %s", email)
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

func handleDashboard(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		renderTemplate(w, "dashboard.html", &templateData{
			User: user,
			Now:  time.Now(),
		})
	}
}

func handleProfile(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		renderTemplate(w, "profile.html", &templateData{
			User: user,
			Now:  time.Now(),
		})
	}
}

func handleLogout(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Clear all cookies.
		for _, name := range []string{"MURAENA_SESS", "NECRO_BRO", "JSESSIONID"} {
			http.SetCookie(w, &http.Cookie{
				Name:     name,
				Value:    "",
				Path:     "/",
				HttpOnly: true,
				Secure:   true,
				MaxAge:   -1,
			})
		}

		http.Redirect(w, r, "/", http.StatusFound)
	}
}
