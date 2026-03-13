package admin

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	db        *sql.DB
	jwtSecret string
	botToken  string
	groups    []string // for group admin verification
	adminUI   fs.FS
	webApp    fs.FS
}

func NewServer(database *sql.DB, jwtSecret, botToken string, groups []string, adminUI, webApp fs.FS) *Server {
	return &Server{db: database, jwtSecret: jwtSecret, botToken: botToken, groups: groups, adminUI: adminUI, webApp: webApp}
}

// Start runs the admin HTTP server. Blocks until ctx is cancelled.
func Start(ctx context.Context, database *sql.DB, jwtSecret, botToken string, groups []string, host string, port int, adminUI, webApp fs.FS) {
	s := NewServer(database, jwtSecret, botToken, groups, adminUI, webApp)
	r := s.router()
	addr := fmt.Sprintf("%s:%d", host, port)
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	log.Printf("Admin dashboard listening on http://%s/admin/", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("admin server error: %v", err)
	}
}

func (s *Server) router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Static: Mini App
	r.Handle("/webapp/*", http.StripPrefix("/webapp/", http.FileServer(http.FS(s.webApp))))

	// Static: Admin UI — strip /admin/ so FileServer sees assets/style.css which matches the FS
	r.Handle("/admin/assets/*", http.StripPrefix("/admin/", http.FileServer(http.FS(s.adminUI))))

	// Admin HTML pages
	r.Get("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		data, _ := fs.ReadFile(s.adminUI, "login.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
	r.Get("/admin/", func(w http.ResponseWriter, r *http.Request) {
		data, _ := fs.ReadFile(s.adminUI, "index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
	r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})

	// Public routes (no auth required)
	r.Post("/admin/api/auth/login",          s.handleLogin)
	r.Post("/admin/api/auth/refresh",        s.handleRefresh)
	r.Post("/admin/api/auth/logout",         s.handleLogout)
	r.Post("/admin/api/auth/telegram",       s.handleTelegramAuth) // Mini App auto-login
	// Public: branding settings needed by the login page
	r.Get("/admin/api/settings", s.handleGetSettings)

	// Protected routes (require valid JWT)
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)

		r.Get("/admin/api/auth/me", s.handleMe)
		r.Get("/admin/api/stats", s.handleStats)
		r.Get("/admin/api/bot-info", s.handleBotInfo)
		r.Get("/admin/api/logs", s.handleGetLogs)
		r.Get("/admin/api/logs/export", s.handleExportLogs)

		// admin+ can modify appearance settings
		r.With(requireRole("superadmin", "admin")).Patch("/admin/api/settings", s.handlePatchSettings)

		// superadmin only: user management
		r.With(requireRole("superadmin")).Get("/admin/api/users", s.handleListUsers)
		r.With(requireRole("superadmin")).Post("/admin/api/users", s.handleCreateUser)
		r.With(requireRole("superadmin")).Patch("/admin/api/users/{id}", s.handlePatchUser)
		r.With(requireRole("superadmin")).Delete("/admin/api/users/{id}", s.handleDeleteUser)
		// superadmin: link/unlink Telegram ID to admin account
		r.With(requireRole("superadmin")).Patch("/admin/api/users/{id}/telegram", s.handleSetUserTelegram)
	})

	return r
}
