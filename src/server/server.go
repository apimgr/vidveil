// SPDX-License-Identifier: MIT
package server

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/handlers"
	"github.com/apimgr/vidveil/src/services/engines"
	"github.com/apimgr/vidveil/src/services/ratelimit"
)

//go:embed static/css/* static/js/* static/img/* templates/*.tmpl templates/partials/*.tmpl templates/layouts/*.tmpl
var embeddedFS embed.FS

// GetTemplatesFS returns the embedded templates filesystem
func GetTemplatesFS() embed.FS {
	return embeddedFS
}

// Server represents the HTTP server
type Server struct {
	cfg         *config.Config
	engineMgr   *engines.Manager
	router      *chi.Mux
	srv         *http.Server
	rateLimiter *ratelimit.Limiter
}

// New creates a new server instance
func New(cfg *config.Config, engineMgr *engines.Manager) *Server {
	// Set templates filesystem for handlers
	handlers.SetTemplatesFS(embeddedFS)

	// Create rate limiter per PART 16
	limiter := ratelimit.New(
		cfg.Server.RateLimit.Enabled,
		cfg.Server.RateLimit.Requests,
		cfg.Server.RateLimit.Window,
	)

	s := &Server{
		cfg:         cfg,
		engineMgr:   engineMgr,
		router:      chi.NewRouter(),
		rateLimiter: limiter,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	// Request ID
	s.router.Use(middleware.RequestID)

	// Real IP
	s.router.Use(middleware.RealIP)

	// Logger
	s.router.Use(middleware.Logger)

	// Recoverer
	s.router.Use(middleware.Recoverer)

	// CORS
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Security headers (TEMPLATE.md PART 15 NON-NEGOTIABLE)
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; media-src 'self' https:; connect-src 'self'")
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			// Add Request ID to response headers per TEMPLATE.md PART 17
			if reqID := middleware.GetReqID(r.Context()); reqID != "" {
				w.Header().Set("X-Request-ID", reqID)
			}
			// Cache-Control headers per TEMPLATE.md PART 28
			path := r.URL.Path
			if strings.HasPrefix(path, "/static/") {
				// Static assets: cache for 1 year
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else if strings.HasPrefix(path, "/api/") {
				// API responses: no cache
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			} else {
				// HTML pages: no store
				w.Header().Set("Cache-Control", "no-store, must-revalidate")
			}
			next.ServeHTTP(w, r)
		})
	})

	// Rate limiting (TEMPLATE.md PART 16)
	s.router.Use(s.rateLimiter.Middleware)
}

// setupRoutes configures all routes
func (s *Server) setupRoutes() {
	h := handlers.New(s.cfg, s.engineMgr)
	admin := handlers.NewAdminHandler(s.cfg, s.engineMgr)
	metrics := handlers.NewMetrics(s.cfg, s.engineMgr)

	// Maintenance mode middleware (applied globally, but allows admin access)
	s.router.Use(h.MaintenanceModeMiddleware)

	// Static files (no age verification needed)
	staticFS, _ := fs.Sub(embeddedFS, "static")
	s.router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Age verification endpoints (before middleware)
	s.router.Get("/age-verify", h.AgeVerifyPage)
	s.router.Post("/age-verify", h.AgeVerifySubmit)

	// Health, robots, security.txt, and sitemap (no age verification)
	s.router.Get("/healthz", h.HealthCheck)
	s.router.Get("/robots.txt", h.RobotsTxt)
	s.router.Get("/sitemap.xml", h.SitemapXML)
	s.router.Get("/.well-known/security.txt", h.SecurityTxt)
	s.router.Get("/.well-known/change-password", handlers.ChangePasswordRedirect)

	// OpenAPI/Swagger documentation (TEMPLATE.md PART 11)
	s.router.Get("/openapi", handlers.SwaggerUI(s.cfg))
	s.router.Get("/openapi.json", handlers.OpenAPISpec(s.cfg))
	s.router.Get("/openapi.yaml", handlers.OpenAPISpecYAML(s.cfg))
	s.router.Get("/swagger", handlers.SwaggerUI(s.cfg))
	s.router.Get("/api-docs", handlers.SwaggerUI(s.cfg))

	// GraphQL endpoint
	gql := handlers.NewGraphQLHandler(s.cfg, s.engineMgr)
	s.router.HandleFunc("/graphql", gql.Handle)
	s.router.Get("/graphiql", gql.GraphiQL)
	s.router.Get("/graphql/schema", gql.GraphQLSchema)

	// Prometheus metrics
	if s.cfg.Server.Metrics.Enabled {
		s.router.Get(s.cfg.Server.Metrics.Endpoint, metrics.Handler())
	}

	// Debug endpoints (development mode only per BASE.md spec)
	if s.cfg.IsDevelopmentMode() {
		s.router.Route("/debug", func(r chi.Router) {
			r.Get("/vars", handlers.DebugVars)
			r.Get("/pprof/", handlers.DebugPprof)
			r.Get("/pprof/cmdline", handlers.DebugPprofCmdline)
			r.Get("/pprof/profile", handlers.DebugPprofProfile)
			r.Get("/pprof/symbol", handlers.DebugPprofSymbol)
			r.Get("/pprof/trace", handlers.DebugPprofTrace)
			r.Get("/pprof/{name}", handlers.DebugPprofHandler)
		})
	}

	// Routes that require age verification
	s.router.Group(func(r chi.Router) {
		r.Use(h.AgeVerifyMiddleware)

		r.Get("/", h.HomePage)
		r.Get("/search", h.SearchPage)
		r.Get("/preferences", h.PreferencesPage)
		r.Get("/about", h.AboutPage)
		r.Get("/privacy", h.PrivacyPage)
	})

	// Server routes per TEMPLATE.md PART 31
	server := handlers.NewServerHandler(s.cfg)
	s.router.Route("/server", func(r chi.Router) {
		r.Get("/about", server.AboutPage)
		r.Get("/privacy", server.PrivacyPage)
		r.Get("/contact", server.ContactPage)
		r.Post("/contact", server.ContactPage)
		r.Get("/help", server.HelpPage)
	})

	// Auth routes per TEMPLATE.md PART 31
	auth := handlers.NewAuthHandler(s.cfg)
	s.router.Route("/auth", func(r chi.Router) {
		r.Get("/login", auth.LoginPage)
		r.Post("/login", auth.LoginPage)
		r.Get("/logout", auth.LogoutPage)
		r.Get("/register", auth.RegisterPage)
		r.Post("/register", auth.RegisterPage)
		r.Get("/password/forgot", auth.PasswordForgotPage)
		r.Post("/password/forgot", auth.PasswordForgotPage)
		r.Get("/password/reset/{token}", auth.PasswordResetPage)
		r.Post("/password/reset", auth.PasswordResetPage)
		r.Get("/verify/{token}", auth.VerifyPage)
	})

	// User routes per TEMPLATE.md PART 31
	user := handlers.NewUserHandler(s.cfg)
	s.router.Route("/user", func(r chi.Router) {
		r.Get("/profile", user.ProfilePage)
		r.Get("/settings", user.SettingsPage)
		r.Get("/tokens", user.TokensPage)
		r.Get("/security", user.SecurityPage)
		r.Get("/security/sessions", user.SecurityPage)
		r.Get("/security/2fa", user.SecurityPage)
	})

	// Admin panel routes - PART 12 compliant with all 11 sections
	s.router.Route("/admin", func(r chi.Router) {
		// Login page (no auth required, CSRF not needed for login itself)
		r.Get("/login", admin.LoginPage)
		r.Post("/login", admin.LoginPage)

		// Protected admin routes - all 11 sections per TEMPLATE.md PART 12
		// Uses both AuthMiddleware and CSRFMiddleware for PART 12 compliance
		r.Group(func(r chi.Router) {
			r.Use(admin.AuthMiddleware)
			r.Use(admin.CSRFMiddleware) // CSRF protection per PART 12
			r.Get("/", admin.DashboardPage)          // Section 1: Dashboard
			r.Get("/server", admin.ServerSettingsPage) // Section 2: Server Settings
			r.Get("/web", admin.WebSettingsPage)     // Section 3: Web Settings
			r.Get("/security", admin.SecuritySettingsPage) // Section 4: Security
			r.Get("/database", admin.DatabasePage)   // Section 5: Database & Cache
			r.Get("/email", admin.EmailPage)         // Section 6: Email & Notifications
			r.Get("/ssl", admin.SSLPage)             // Section 7: SSL/TLS
			r.Get("/scheduler", admin.SchedulerPage) // Section 8: Scheduler
			r.Get("/engines", admin.EnginesPage)     // Section 9: Engines (project-specific)
			r.Get("/logs", admin.LogsPage)           // Section 9: Logs
			r.Get("/backup", admin.BackupPage)       // Section 10: Backup & Maintenance
			r.Get("/system", admin.SystemInfoPage)   // Section 11: System Info
			r.Get("/settings", admin.SettingsPage)   // Legacy settings page
			r.Get("/logout", admin.LogoutHandler)
		})
	})

	// API v1 routes
	s.router.Route("/api/v1", func(r chi.Router) {
		// Search endpoints (public)
		r.Get("/search", h.APISearch)
		r.Get("/search.txt", h.APISearchText)

		// Engine endpoints (public)
		r.Get("/engines", h.APIEngines)
		r.Get("/engines/{name}", h.APIEngineDetails)

		// Stats (public)
		r.Get("/stats", h.APIStats)

		// Health (public)
		r.Get("/healthz", h.APIHealthCheck)

		// Server API per TEMPLATE.md PART 31
		r.Route("/server", func(r chi.Router) {
			r.Get("/about", server.APIAbout)
			r.Get("/privacy", server.APIPrivacy)
			r.Post("/contact", server.APIContact)
			r.Get("/help", server.APIHelp)
		})

		// Auth API per TEMPLATE.md PART 31
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", auth.APIRegister)
			r.Post("/login", auth.APILogin)
			r.Post("/logout", auth.APILogout)
			r.Post("/password/forgot", auth.APIPasswordForgot)
			r.Post("/password/reset", auth.APIPasswordReset)
			r.Post("/verify", auth.APIVerify)
			r.Post("/refresh", auth.APIRefresh)
		})

		// User API per TEMPLATE.md PART 31
		r.Route("/user", func(r chi.Router) {
			r.Get("/profile", user.APIProfile)
			r.Patch("/profile", user.APIProfile)
			r.Post("/password", user.APIPassword)
			r.Get("/tokens", user.APITokens)
			r.Post("/tokens", user.APITokens)
			r.Get("/sessions", user.APISessions)
			r.Get("/2fa", user.API2FA)
		})

		// Admin API (token required) - PART 12 compliant
		r.Route("/admin", func(r chi.Router) {
			r.Use(admin.APITokenMiddleware)
			// Existing endpoints
			r.Get("/stats", admin.APIStats)
			r.Get("/engines", admin.APIEngines)
			r.Post("/backup", admin.APIBackup)
			r.Post("/maintenance", admin.APIMaintenanceMode)
			// PART 12 required endpoints
			r.Get("/config", admin.APIConfig)
			r.Put("/config", admin.APIConfig)
			r.Patch("/config", admin.APIConfig)
			r.Get("/status", admin.APIStatus)
			r.Get("/health", admin.APIHealth)
			r.Get("/logs/access", admin.APILogsAccess)
			r.Get("/logs/error", admin.APILogsError)
			r.Post("/restore", admin.APIRestore)
			r.Post("/test/email", admin.APITestEmail)
			r.Post("/password", admin.APIPassword)
			r.Post("/token/regenerate", admin.APITokenRegenerate)
			// Scheduler API
			r.Get("/scheduler/tasks", admin.APISchedulerTasks)
			r.Post("/scheduler/run", admin.APISchedulerRunTask)
			r.Get("/scheduler/history", admin.APISchedulerHistory)
		})
	})

	// Shortcut API routes (without version prefix)
	s.router.Get("/api/search", h.APISearch)
	s.router.Get("/api/engines", h.APIEngines)
	s.router.Get("/api/health", h.APIHealthCheck)
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe(addr string) error {
	s.srv = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	return s.srv.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}
