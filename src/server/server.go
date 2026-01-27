// SPDX-License-Identifier: MIT
package server

import (
	"context"
	"embed"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/graphql"
	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/server/service/admin"
	"github.com/apimgr/vidveil/src/swagger"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/ratelimit"
	"github.com/apimgr/vidveil/src/server/service/urlvars"
	"github.com/apimgr/vidveil/src/paths"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
)

//go:embed static/css/* static/js/* static/images/* static/icons/* static/manifest.json static/offline.html template/page/*.tmpl template/partial/public/*.tmpl template/partial/admin/*.tmpl template/layout/*.tmpl template/admin/*.tmpl template/component/*.tmpl template/nojs/*.tmpl
var embeddedFS embed.FS

// GetTemplatesFS returns the embedded templates filesystem
func GetTemplatesFS() embed.FS {
	return embeddedFS
}

// Server represents the HTTP server
type Server struct {
	appConfig     *config.AppConfig
	configDir     string
	dataDir       string
	engineMgr     *engine.EngineManager
	adminSvc      *admin.AdminService
	migrationMgr  MigrationManager
	scheduler     *scheduler.Scheduler
	logger        *logging.AppLogger
	router        *chi.Mux
	srv           *http.Server
	rateLimiter   *ratelimit.RateLimiter
	searchHandler *handler.SearchHandler
	adminHandler  *handler.AdminHandler
}

// MigrationManager interface for database migrations
type MigrationManager interface {
	GetMigrationStatus() ([]map[string]interface{}, error)
	RunMigrations() error
	RollbackMigration() error
}

// NewServer creates a new server instance
func NewServer(appConfig *config.AppConfig, configDir, dataDir string, engineMgr *engine.EngineManager, adminSvc *admin.AdminService, migrationMgr MigrationManager, sched *scheduler.Scheduler, logger *logging.AppLogger) *Server {
	// Set templates filesystem for handlers
	handler.SetTemplatesFS(embeddedFS)
	handler.SetAdminTemplatesFS(embeddedFS)

	// Create rate limiter per PART 12
	limiter := ratelimit.NewRateLimiter(
		appConfig.Server.RateLimit.Enabled,
		appConfig.Server.RateLimit.Requests,
		appConfig.Server.RateLimit.Window,
	)
	// Set logger for security event logging per AI.md PART 11
	limiter.SetLogger(logger)

	s := &Server{
		appConfig:    appConfig,
		configDir:    configDir,
		dataDir:      dataDir,
		engineMgr:    engineMgr,
		adminSvc:     adminSvc,
		migrationMgr: migrationMgr,
		scheduler:    sched,
		logger:       logger,
		router:       chi.NewRouter(),
		rateLimiter:  limiter,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// SetTorService sets the Tor service for handlers that need it
func (s *Server) SetTorService(t handler.TorService) {
	if s.adminHandler != nil {
		s.adminHandler.SetTorService(t)
	}
	if s.searchHandler != nil {
		s.searchHandler.SetTorService(t)
	}
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	// Request ID
	s.router.Use(middleware.RequestID)

	// Real IP
	s.router.Use(middleware.RealIP)

	// URL Variables resolution per AI.md PART 13 (reverse proxy headers)
	s.router.Use(urlvars.GlobalResolver().Middleware)

	// URL Normalization per AI.md PART 16 - redirect /path/ to /path (MUST be early)
	s.router.Use(URLNormalizeMiddleware)

	// Path Security (AI.md PART 5 - must be early in chain)
	s.router.Use(paths.PathSecurityMiddleware)

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

	// Security headers per AI.md PART 11 (NON-NEGOTIABLE)
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; media-src 'self' https:; connect-src 'self'")
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			w.Header().Set("X-Robots-Tag", "noindex, nofollow")
			// HSTS header per AI.md PART 11 - only when SSL enabled
			// max-age=31536000 (1 year), includeSubDomains, preload
			if s.appConfig.Server.SSL.Enabled {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}
			// Add Request ID to response headers per AI.md PART 14
			if reqID := middleware.GetReqID(r.Context()); reqID != "" {
				w.Header().Set("X-Request-ID", reqID)
			}
			// Cache-Control headers per AI.md PART 9
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

	// Rate limiting (AI.md PART 12)
	s.router.Use(s.rateLimiter.Middleware)

	// Extension stripping middleware per AI.md PART 14
	// Strips .txt and .json extensions from API paths for routing
	s.router.Use(extensionStripMiddleware)
}

// OriginalPathKey is the context key for storing the original request path
// Uses string type for cross-package compatibility
const OriginalPathKey = "vidveil.originalPath"

// extensionStripMiddleware strips .txt and .json extensions from paths
// Per AI.md PART 14: Content Negotiation - .txt and .json extensions should work on all API routes
func extensionStripMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Only process API routes
		if !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Store original path in context for detectResponseFormat
		ctx := context.WithValue(r.Context(), OriginalPathKey, path)
		r = r.WithContext(ctx)

		// Check for .txt or .json extension
		if strings.HasSuffix(path, ".txt") {
			// Strip .txt for routing
			r.URL.Path = strings.TrimSuffix(path, ".txt")
		} else if strings.HasSuffix(path, ".json") {
			// Strip .json for routing
			r.URL.Path = strings.TrimSuffix(path, ".json")
		}

		next.ServeHTTP(w, r)
	})
}

// setupRoutes configures all routes
func (s *Server) setupRoutes() {
	h := handler.NewSearchHandler(s.appConfig, s.engineMgr)
	admin := handler.NewAdminHandler(s.appConfig, s.configDir, s.dataDir, s.engineMgr, s.adminSvc, s.migrationMgr)
	// Store handler references for later service injection
	s.searchHandler = h
	s.adminHandler = admin
	// Set scheduler for admin panel management per AI.md PART 19
	admin.SetScheduler(s.scheduler)
	// Set logger for audit and security event logging per AI.md PART 11
	admin.SetLogger(s.logger)
	// Set search cache for cache management per AI.md PART 9
	admin.SetSearchCache(h.GetSearchCache())
	metrics := handler.NewMetrics(s.appConfig, s.engineMgr)
	h.SetMetrics(metrics)

	// Metrics middleware per AI.md PART 13 - tracks requests and active connections
	s.router.Use(metrics.MetricsMiddleware)

	// Maintenance mode middleware (applied globally, but allows admin access)
	s.router.Use(h.MaintenanceModeMiddleware)

	// Static files (no age verification needed)
	staticFS, _ := fs.Sub(embeddedFS, "static")
	s.router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Age verification endpoints (before middleware)
	s.router.Get("/age-verify", h.AgeVerifyPage)
	s.router.Post("/age-verify", h.AgeVerifySubmit)

	// Health, robots, security.txt, and sitemap (no age verification)
	// Per AI.md PART 13: /healthz with extension support
	s.router.Get("/healthz", h.HealthCheck)
	s.router.Get("/healthz.json", h.HealthCheck)
	s.router.Get("/healthz.txt", h.HealthCheck)
	s.router.Get("/robots.txt", h.RobotsTxt)
	s.router.Get("/sitemap.xml", h.SitemapXML)
	s.router.Get("/.well-known/security.txt", h.SecurityTxt)
	s.router.Get("/.well-known/change-password", handler.ChangePasswordRedirect)
	s.router.Get("/humans.txt", h.HumansTxt)
	s.router.Get("/favicon.ico", h.Favicon)
	s.router.Get("/apple-touch-icon.png", h.AppleTouchIcon)

	// Debug endpoints (PART 6: only when --debug flag or DEBUG=true)
	s.registerDebugRoutes(s.router)

	// OpenAPI/Swagger documentation (AI.md PART 14: JSON only, no YAML)
	// Per AI.md PART 14: Swagger handlers in src/swagger/
	s.router.Get("/openapi", swagger.Handler(s.appConfig))
	s.router.Get("/openapi.json", swagger.SpecHandler(s.appConfig))
	s.router.Get("/swagger", swagger.Handler(s.appConfig))
	s.router.Get("/api-docs", swagger.Handler(s.appConfig))

	// GraphQL endpoint (AI.md PART 14: GraphQL handler in src/graphql/)
	gql := graphql.NewHandler(s.appConfig, s.engineMgr)
	s.router.HandleFunc("/graphql", gql.Handle)
	s.router.Get("/graphiql", gql.GraphiQL)
	s.router.Get("/graphql/schema", gql.Schema)

	// Prometheus metrics
	if s.appConfig.Server.Metrics.Enabled {
		s.router.Get(s.appConfig.Server.Metrics.Endpoint, metrics.Handler())
	}

	// Routes that require age verification (project-specific per PART 14)
	s.router.Group(func(r chi.Router) {
		r.Use(h.AgeVerifyMiddleware)

		r.Get("/", h.HomePage)
		r.Get("/search", h.SearchPage)
		r.Get("/preferences", h.PreferencesPage)
		// About/privacy are at /server/* per PART 14 Route Scopes
	})

	// Server routes per AI.md PART 14 (Route Scopes)
	server := handler.NewServerHandler(s.appConfig)
	s.router.Route("/server", func(r chi.Router) {
		r.Get("/about", server.AboutPage)
		r.Get("/privacy", server.PrivacyPage)
		r.Get("/contact", server.ContactPage)
		r.Post("/contact", server.ContactPage)
		r.Get("/help", server.HelpPage)
	})

	// Auth routes per AI.md PART 14 (Route Scopes)
	auth := handler.NewAuthHandler(s.appConfig)
	// Link admin handler for authentication
	auth.SetAdminHandler(admin)
	// Admin auth routes per AI.md PART 17
	// VidVeil is stateless - no PART 34 (Multi-User), only Server Admin auth
	s.router.Route("/auth", func(r chi.Router) {
		r.Get("/login", auth.LoginPage)
		r.Post("/login", auth.LoginPage)
		r.Get("/logout", auth.LogoutPage)
		// Per AI.md PART 17: 2FA verification step (after password, before session)
		r.Get("/2fa", auth.TwoFactorPage)
		r.Post("/2fa", auth.TwoFactorPage)
		r.Get("/password/forgot", auth.PasswordForgotPage)
		r.Post("/password/forgot", auth.PasswordForgotPage)
		r.Get("/password/reset/{token}", auth.PasswordResetPage)
		r.Post("/password/reset", auth.PasswordResetPage)
	})

	// Admin panel routes - PART 14 (routes), PART 17 (admin panel)
	// Path is configurable via server.admin.path (default: "admin")
	adminPath := "/" + s.appConfig.Server.Admin.Path
	s.router.Route(adminPath, func(r chi.Router) {
		// Login page per AI.md PART 17
		r.Get("/login", admin.LoginPage)
		r.Post("/login", admin.LoginPage)

		// Logout handler
		r.Get("/logout", admin.LogoutHandler)
		r.Post("/logout", admin.LogoutHandler)

		// Root: Setup token entry (first run) or dashboard (authenticated)
		// Per AI.md PART 17: User navigates to /admin, enters setup token
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			if admin.IsFirstRun() {
				admin.SetupTokenPage(w, req)
				return
			}
			// After setup, apply auth middleware and show dashboard
			admin.AuthMiddleware(http.HandlerFunc(admin.DashboardPage)).ServeHTTP(w, req)
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			if admin.IsFirstRun() {
				admin.SetupTokenPage(w, req)
				return
			}
			admin.AuthMiddleware(http.HandlerFunc(admin.DashboardPage)).ServeHTTP(w, req)
		})

		// Protected admin routes per AI.md PART 17
		// Valid root paths: /, /profile, /preferences, /notifications, /server/*
		r.Group(func(r chi.Router) {
			r.Use(admin.AuthMiddleware)
			r.Use(admin.CSRFMiddleware)

			// Dashboard (root)
			r.Get("/dashboard", admin.DashboardPage)

			// Admin's OWN profile per AI.md PART 17 (valid at root)
			r.Get("/profile", admin.ProfilePage)

			// Admin's OWN preferences per AI.md PART 17 (valid at root)
			r.Get("/preferences", admin.PreferencesPage)

			// Admin's OWN notifications per AI.md PART 17 (valid at root)
			r.Get("/notifications", admin.AdminNotificationsPage)

			// Logout (valid at root)
			r.Get("/logout", admin.LogoutHandler)

			// Server section - ALL server management per AI.md PART 17
			r.Route("/server", func(r chi.Router) {
				// Settings
				r.Get("/", admin.ServerSettingsPage)
				r.Get("/settings", admin.ServerSettingsPage)
				r.Get("/branding", admin.BrandingPage)
				r.Get("/ssl", admin.SSLPage)
				r.Get("/scheduler", admin.SchedulerPage)
				r.Get("/email", admin.EmailPage)
				r.Get("/logs", admin.LogsPage)
				r.Get("/logs/audit", admin.AuditLogsPage)
				r.Get("/database", admin.DatabasePage)
				r.Get("/web", admin.WebSettingsPage)
				r.Get("/pages", admin.PagesPage)
				r.Get("/notifications", admin.NotificationsPage)
				r.Get("/nodes", admin.NodesPage)
				r.Get("/nodes/add", admin.AddNodePage)
				r.Post("/nodes/add", admin.AddNodePage)
				r.Get("/nodes/remove", admin.RemoveNodePage)
				r.Get("/nodes/settings", admin.NodeSettingsPage)
				r.Get("/nodes/{node}", admin.NodeDetailPage)

				// Security section per AI.md PART 17 - under /server/
				r.Route("/security", func(r chi.Router) {
					r.Get("/", admin.SecurityAuthPage)
					r.Get("/auth", admin.SecurityAuthPage)
					r.Get("/tokens", admin.SecurityTokensPage)
					r.Get("/ratelimit", admin.SecurityRateLimitPage)
					r.Get("/firewall", admin.SecurityFirewallPage)
				})

				// Network section per AI.md PART 17 - under /server/
				r.Route("/network", func(r chi.Router) {
					r.Get("/", admin.TorPage)
					r.Get("/tor", admin.TorPage)
					r.Get("/geoip", admin.GeoIPPage)
					r.Get("/blocklists", admin.BlocklistsPage)
				})

				// System routes per AI.md PART 17 - directly under /server/
				r.Get("/backup", admin.BackupPage)
				r.Get("/maintenance", admin.MaintenancePage)
				r.Get("/updates", admin.UpdatesPage)
				r.Get("/info", admin.SystemInfoPage)

				// Users section per AI.md PART 17 - under /server/
				r.Route("/users", func(r chi.Router) {
					r.Get("/admins", admin.UsersAdminsPage)
				})

				// Project-specific - under /server/
				r.Get("/engines", admin.EnginesPage)

				// Help - under /server/
				r.Get("/help", admin.HelpPage)
			})
		})

		// Setup wizard at /admin/server/setup (no auth, but requires valid token cookie)
		r.Get("/server/setup", admin.SetupWizardPage)
		r.Post("/server/setup", admin.SetupWizardPage)

		// Admin invite page (public, token validated in handler)
		r.Get("/invite/{token}", admin.AdminInvitePage)
		r.Post("/invite/{token}", admin.AdminInvitePage)
	})

	// API autodiscover endpoint (non-versioned per AI.md PART 37)
	// Clients need this BEFORE they know the API version
	s.router.Get("/api/autodiscover", h.Autodiscover)

	// API v1 routes
	s.router.Route("/api/v1", func(r chi.Router) {
		// Search endpoint (public) - content negotiation for JSON, SSE, text
		// Accept: application/json (default) - JSON response with caching
		// Accept: text/event-stream - SSE streaming results as engines respond
		// Accept: text/plain or .txt extension - plain text format
		r.Get("/search", h.APISearch)

		// Bang endpoints (public) - per AI.md PART 37
		r.Get("/bangs", h.APIBangs)
		r.Get("/bangs/autocomplete", h.APIAutocomplete)

		// Engine endpoints (public)
		r.Get("/engines", h.APIEngines)
		r.Get("/engines/{name}", h.APIEngineDetails)

		// Stats (public)
		r.Get("/stats", h.APIStats)

		// Debug endpoints (development only per IDEA.md)
		r.Route("/debug", func(r chi.Router) {
			r.Get("/engines", h.DebugEnginesList)
			r.Get("/engine/{name}", h.DebugEngine)
		})

		// Health and version (public)
		r.Get("/healthz", h.APIHealthCheck)
		r.Get("/version", h.APIVersion)

		// Server API per AI.md PART 14
		r.Route("/server", func(r chi.Router) {
			r.Get("/about", server.APIAbout)
			r.Get("/privacy", server.APIPrivacy)
			r.Post("/contact", server.APIContact)
			r.Get("/help", server.APIHelp)
		})

		// Proxy endpoints (plural per PART 14)
		r.Get("/proxy/thumbnails", h.ProxyThumbnail)

		// Admin Profile API (session or token) - PART 17
		// Uses configurable admin path
		r.Route("/"+s.appConfig.Server.Admin.Path+"/profile", func(r chi.Router) {
			r.Use(admin.SessionOrTokenMiddleware)
			r.Post("/password", admin.APIProfilePassword)
			r.Post("/token", admin.APIProfileToken)
			r.Delete("/sessions", admin.APIRevokeSessions)
			r.Get("/recovery-keys", admin.APIRecoveryKeysStatus)
			r.Post("/recovery-keys/generate", admin.APIRecoveryKeysGenerate)
		})

		// Admin API (token required) - PART 12, PART 17
		r.Route("/"+s.appConfig.Server.Admin.Path, func(r chi.Router) {
			r.Use(admin.APITokenMiddleware)

			// Users management per AI.md PART 17
			r.Post("/users/admins/invite", admin.APIUsersAdminsInvite)
			r.Get("/users/admins/invites", admin.APIUsersAdminsInvites)
			r.Delete("/users/admins/invites/{id}", admin.APIUsersAdminsInviteRevoke)

			// Server settings per AI.md PART 12
			r.Route("/server", func(r chi.Router) {
				// Settings
				r.Get("/settings", admin.APIConfig)
				r.Patch("/settings", admin.APIConfig)
				r.Get("/status", admin.APIStatus)
				r.Get("/health", admin.APIHealth)
				r.Post("/restart", admin.APIMaintenanceMode)

				// Branding per PART 17
				r.Route("/branding", func(r chi.Router) {
					r.Patch("/", admin.APIBranding)
					r.Post("/upload", admin.APIBrandingUpload)
				})

				// SSL per PART 15
				r.Route("/ssl", func(r chi.Router) {
					r.Get("/", admin.APIConfig)
					r.Patch("/", admin.APIConfig)
					r.Post("/renew", admin.APIConfig)
					r.Post("/upload", admin.APISSLUpload)
				})

				// Tor per PART 32
				r.Route("/tor", func(r chi.Router) {
					r.Get("/", admin.APITorStatus)
					r.Patch("/", admin.APITorUpdate)
					r.Post("/regenerate", admin.APITorRegenerate)
					r.Post("/test", admin.APITorTest)
					r.Get("/vanity", admin.APITorVanityStatus)
					r.Post("/vanity", admin.APITorVanityStart)
					r.Delete("/vanity", admin.APITorVanityCancel)
					r.Post("/vanity/apply", admin.APITorVanityApply)
					r.Post("/import", admin.APITorImport)
				})

				// Email per PART 18
				r.Route("/email", func(r chi.Router) {
					r.Get("/", admin.APIConfig)
					r.Patch("/", admin.APIConfig)
					r.Post("/test", admin.APITestEmail)
				})

				// Scheduler per PART 19
				r.Route("/scheduler", func(r chi.Router) {
					r.Get("/", admin.APISchedulerTasks)
					r.Get("/{id}", admin.APISchedulerTasks)
					r.Patch("/{id}", admin.APISchedulerTasks)
					r.Post("/{id}/run", admin.APISchedulerRunTask)
					r.Post("/{id}/enable", admin.APISchedulerTasks)
					r.Post("/{id}/disable", admin.APISchedulerTasks)
				})

				// Backup per PART 22
				r.Route("/backup", func(r chi.Router) {
					r.Get("/", admin.APIBackup)
					r.Post("/", admin.APIBackup)
					r.Get("/{id}", admin.APIBackup)
					r.Delete("/{id}", admin.APIBackup)
					r.Get("/{id}/download", admin.APIBackup)
					r.Post("/restore", admin.APIRestore)
				})

				// Logs per PART 11
				r.Route("/logs", func(r chi.Router) {
					r.Get("/", admin.APILogsAccess)
					r.Get("/{type}", admin.APILogsAccess)
					r.Get("/{type}/download", admin.APILogsAccess)
				})

				// Pages per PART 17
				r.Route("/pages", func(r chi.Router) {
					r.Get("/", admin.APIPagesGet)
					r.Put("/{slug}", admin.APIPageUpdate)
					r.Post("/{slug}/reset", admin.APIPageReset)
				})

				// Notifications per PART 18
				r.Route("/notifications", func(r chi.Router) {
					r.Get("/", admin.APINotificationsGet)
					r.Put("/", admin.APINotificationsUpdate)
					r.Post("/test", admin.APINotificationsTest)
				})

				// Database per PART 10
				r.Route("/database", func(r chi.Router) {
					r.Get("/migrations", admin.APIDatabaseMigrations)
					r.Post("/migrate", admin.APIDatabaseMigrate)
					r.Post("/vacuum", admin.APIDatabaseVacuum)
					r.Post("/analyze", admin.APIDatabaseAnalyze)
					r.Post("/test", admin.APIDatabaseTest)
					r.Put("/backend", admin.APIDatabaseBackend)
				})

				// Cache management per PART 9
				r.Route("/cache", func(r chi.Router) {
					r.Post("/clear", admin.APICacheClear)
				})

				// Nodes per PART 10
				r.Route("/nodes", func(r chi.Router) {
					r.Get("/", admin.APINodesGet)
					r.Post("/", admin.APINodeAdd)
					r.Post("/test", admin.APINodeTest)
					r.Post("/token", admin.APINodeTokenRegenerate)
					r.Post("/leave", admin.APINodeLeave)
					r.Put("/settings", admin.APINodeSettings)
					r.Post("/stepdown", admin.APINodeStepDown)
					r.Post("/regenerate-id", admin.APINodeRegenerateID)
					r.Post("/{id}/ping", admin.APINodePing)
					r.Delete("/{id}", admin.APINodeRemove)
				})

				// Updates per PART 23
				r.Route("/updates", func(r chi.Router) {
					r.Get("/", admin.APIUpdatesStatus)
					r.Post("/check", admin.APIUpdatesCheck)
				})
			})
		})
	})

	// Custom 404 handler per AI.md PART 14
	s.router.NotFound(h.NotFoundHandler)
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe(addr string) error {
	// Parse timeouts from config per AI.md PART 13
	readTimeout := parseDuration(s.appConfig.Server.Limits.ReadTimeout, 30*time.Second)
	writeTimeout := parseDuration(s.appConfig.Server.Limits.WriteTimeout, 30*time.Second)
	idleTimeout := parseDuration(s.appConfig.Server.Limits.IdleTimeout, 120*time.Second)

	s.srv = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	return s.srv.ListenAndServe()
}

// Serve serves on the given listener (for Tor hidden service)
// Per AI.md PART 32: HTTP server serves on both TCP (clearnet) and Tor listener
func (s *Server) Serve(listener net.Listener) error {
	// Parse timeouts from config per AI.md PART 13
	readTimeout := parseDuration(s.appConfig.Server.Limits.ReadTimeout, 30*time.Second)
	writeTimeout := parseDuration(s.appConfig.Server.Limits.WriteTimeout, 30*time.Second)
	idleTimeout := parseDuration(s.appConfig.Server.Limits.IdleTimeout, 120*time.Second)

	torSrv := &http.Server{
		Handler:      s.router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	return torSrv.Serve(listener)
}

// parseDuration parses a duration string, returning the default if parsing fails
func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	return d
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}

// URLNormalizeMiddleware normalizes URLs for consistent routing per AI.md PART 16
// - Removes trailing slashes (except for root "/")
// - Redirects to canonical URL with 301 if normalization changed path
func URLNormalizeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Root path "/" stays as-is
		if path == "/" {
			next.ServeHTTP(w, r)
			return
		}

		// Remove trailing slash (canonical form: no trailing slash)
		if strings.HasSuffix(path, "/") {
			// Exception: explicit file requests (e.g., /dir/index.html)
			lastSlashIdx := strings.LastIndex(path, "/")
			if lastSlashIdx >= 0 && !strings.Contains(path[lastSlashIdx:], ".") {
				canonical := strings.TrimSuffix(path, "/")
				// Preserve query string
				if r.URL.RawQuery != "" {
					canonical += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, canonical, http.StatusMovedPermanently)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
