// SPDX-License-Identifier: MIT
package server

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/graphql"
	"github.com/apimgr/vidveil/src/paths"
	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/server/service/admin"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/logging"
	"github.com/apimgr/vidveil/src/server/service/ratelimit"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
	"github.com/apimgr/vidveil/src/server/service/urlvars"
	"github.com/apimgr/vidveil/src/swagger"
)

//go:embed static/css/* static/js/* static/images/* static/icons/* static/manifest.json static/offline.html template/page/*.tmpl template/partial/public/*.tmpl template/partial/admin/*.tmpl template/layout/*.tmpl template/admin/*.tmpl template/component/*.tmpl template/nojs/*.tmpl
var embeddedFS embed.FS

// GetTemplatesFS returns the embedded templates filesystem
func GetTemplatesFS() embed.FS {
	return embeddedFS
}

// GeoIPBlocker is a minimal interface for country-based IP blocking per AI.md PART 19
type GeoIPBlocker interface {
	IsBlocked(ipStr string) bool
}

// IPBlocklistChecker is a minimal interface for IP/domain blocklist checks per AI.md PART 11
type IPBlocklistChecker interface {
	IsBlocked(ipOrDomain string) bool
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
	// stored for Onion-Location middleware
	torSvc handler.TorService
	// geoip for country blocking middleware per AI.md PART 19
	geoIPBlocker GeoIPBlocker
	// blocklist for IP/domain blocklist middleware per AI.md PART 11
	ipBlocklist IPBlocklistChecker
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
	s.torSvc = t
	if s.adminHandler != nil {
		s.adminHandler.SetTorService(t)
	}
	if s.searchHandler != nil {
		s.searchHandler.SetTorService(t)
	}
}

// SetGeoIPService sets the GeoIP service for content restriction checks and country blocking
func (s *Server) SetGeoIPService(g handler.GeoIPChecker) {
	if s.searchHandler != nil {
		s.searchHandler.SetGeoIPService(g)
	}
	// Also store as GeoIPBlocker for the country-blocking middleware per AI.md PART 19
	if blocker, ok := g.(GeoIPBlocker); ok {
		s.geoIPBlocker = blocker
	}
}

// SetBlocklistService sets the IP/domain blocklist service for the blocklist middleware
// per AI.md PART 11. Must be called after NewServer().
func (s *Server) SetBlocklistService(b IPBlocklistChecker) {
	s.ipBlocklist = b
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	// Request ID
	s.router.Use(middleware.RequestID)

	// Real IP
	s.router.Use(middleware.RealIP)

	// URL Variables resolution per AI.md PART 8 (reverse proxy headers)
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
			// Required security headers per PART 11
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
			w.Header().Set("Origin-Agent-Cluster", "?1")
			// Cross-Origin headers per PART 11 — defaults per "everyone" tier
			w.Header().Set("Cross-Origin-Opener-Policy", "unsafe-none")
			w.Header().Set("Cross-Origin-Embedder-Policy", "unsafe-none")
			w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
			// CSP per PART 11 default policy (all required directives)
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline'; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data: blob: https:; "+
					"font-src 'self' https:; "+
					"connect-src 'self'; "+
					"media-src 'self' blob:; "+
					"worker-src 'self' blob:; "+
					"manifest-src 'self'; "+
					"frame-src 'self'; "+
					"frame-ancestors 'self'; "+
					"base-uri 'self'; "+
					"form-action 'self'; "+
					"object-src 'none'; "+
					"upgrade-insecure-requests",
			)
			// Permissions-Policy per PART 11 spec defaults
			w.Header().Set("Permissions-Policy",
				"accelerometer=(), ambient-light-sensor=(), battery=(), camera=(), "+
					"display-capture=(), geolocation=(), gyroscope=(), hid=(), "+
					"idle-detection=(), magnetometer=(), microphone=(), midi=(), "+
					"screen-wake-lock=(), serial=(), usb=(), xr-spatial-tracking=(), "+
					"attribution-reporting=(), browsing-topics=(), interest-cohort=(), "+
					"autoplay=(self), encrypted-media=(self), fullscreen=(self), "+
					"payment=(self), picture-in-picture=(self), "+
					"publickey-credentials-get=(self), storage-access=(self), web-share=(self)",
			)
			// HSTS per PART 11 — max-age=63072000 (2 years), includeSubDomains, preload
			if s.appConfig.Server.SSL.Enabled {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}
			// Reporting-Endpoints + legacy Report-To + NEL per AI.md PART 11
			// Both modern (Reporting-Endpoints) and legacy (Report-To) formats are required.
			// api_version is "v1" per IDEA.md project variable.
			proto, fqdn, _ := urlvars.GlobalResolver().GetURLVars(r)
			reportsBase := proto + "://" + fqdn + "/api/v1/server/reports"
			w.Header().Set("Reporting-Endpoints", `default="`+reportsBase+`/default"`)
			w.Header().Set("Report-To", `{"group":"default","max_age":10886400,"endpoints":[{"url":"`+reportsBase+`/default"}]}`)
			w.Header().Set("NEL", `{"report_to":"default","max_age":2592000,"include_subdomains":true}`)
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

	// Allowlist middleware per AI.md PART 11 — sets trusted-IP context flag so
	// downstream blocklist / rate-limit / geoip middleware can skip enforcement.
	// Auth middleware IGNORES this flag; authentication is always required.
	s.router.Use(s.allowlistMiddleware)

	// Blocklist middleware per AI.md PART 11 — checks IP against external
	// IP/domain blocklists (e.g., abuse databases). Allowlisted IPs are exempt.
	s.router.Use(s.blocklistMiddleware)

	// Sec-Fetch-* validation per AI.md PART 11 — defense-in-depth against CSRF
	// and clickjacking. Present-and-bad reject only; absence is a legacy-browser pass.
	s.router.Use(secFetchValidationMiddleware)

	// Request body size limiting per AI.md PART 12 (max_body_size default 10MB)
	// Applied before handler so untrusted input is size-capped per memory safety rules
	maxBodyBytes := parseBodySize(s.appConfig.Server.Limits.MaxBodySize, 10*1024*1024)
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			next.ServeHTTP(w, r)
		})
	})

	// Rate limiting per AI.md PART 12 — allowlisted IPs bypass rate limiting
	s.router.Use(func(next http.Handler) http.Handler {
		inner := s.rateLimiter.Middleware(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAllowlisted(r) {
				next.ServeHTTP(w, r)
				return
			}
			inner.ServeHTTP(w, r)
		})
	})

	// GeoIP country blocking per AI.md PART 19 — enforces deny_countries /
	// allow_countries config. Allowlisted IPs are exempt.
	s.router.Use(s.geoIPMiddleware)

	// Onion-Location header per Tor spec: when a hidden service is running,
	// clearnet responses include the .onion address so Tor Browser auto-redirects.
	s.router.Use(s.onionLocationMiddleware)

	// Extension stripping middleware per AI.md PART 14
	// Strips .txt and .json extensions from API paths for routing
	s.router.Use(extensionStripMiddleware)
}

// OriginalPathKey is the context key for storing the original request path
// Uses string type for cross-package compatibility
const OriginalPathKey = "vidveil.originalPath"

// onionLocationMiddleware adds the Onion-Location header on clearnet HTML responses
// when a Tor hidden service is running. This allows Tor Browser to auto-redirect
// to the .onion address. Per Tor Project spec:
// https://community.torproject.org/onion-services/advanced/onion-location/
//
// ONLY set on HTML page responses — never on SSE streams, JSON API, static assets,
// RSS/Atom feeds, or plain-text responses. Setting it on non-HTML responses causes
// Tor Browser to abort live streams (EventSource) mid-flight.
func (s *Server) onionLocationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip entirely if Tor is not running or this is already an .onion request
		if s.torSvc == nil || strings.HasSuffix(r.Host, ".onion") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip non-HTML paths upfront to avoid wrapping overhead
		// API routes, static files, SSE streams, feeds — never get Onion-Location
		path := r.URL.Path
		if strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		// Also skip SSE requests (Accept: text/event-stream)
		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip non-HTML Accept headers (JSON, plain-text, RSS, Atom, CSV clients)
		accept := r.Header.Get("Accept")
		if accept != "" &&
			!strings.Contains(accept, "text/html") &&
			!strings.Contains(accept, "*/*") {
			next.ServeHTTP(w, r)
			return
		}

		info := s.torSvc.GetInfo()
		addr, ok := info["onion_address"].(string)
		if !ok || addr == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Wrap ResponseWriter to intercept Content-Type and only set the header
		// when the actual response is HTML (e.g. not a redirect to /static/).
		onionURL := "http://" + addr + r.URL.RequestURI()
		rw := &onionLocationWriter{ResponseWriter: w, onionURL: onionURL}
		next.ServeHTTP(rw, r)
	})
}

// onionLocationWriter wraps ResponseWriter to defer Onion-Location until WriteHeader,
// at which point we know the Content-Type and can safely add the header.
type onionLocationWriter struct {
	http.ResponseWriter
	onionURL    string
	wroteHeader bool
}

func (o *onionLocationWriter) WriteHeader(code int) {
	if !o.wroteHeader {
		o.wroteHeader = true
		ct := o.ResponseWriter.Header().Get("Content-Type")
		if strings.HasPrefix(ct, "text/html") {
			o.ResponseWriter.Header().Set("Onion-Location", o.onionURL)
		}
	}
	o.ResponseWriter.WriteHeader(code)
}

func (o *onionLocationWriter) Write(b []byte) (int, error) {
	if !o.wroteHeader {
		// Implicit 200 — trigger our header check
		o.WriteHeader(http.StatusOK)
	}
	return o.ResponseWriter.Write(b)
}

// Unwrap allows http.Flusher and other interfaces to be accessed through the wrapper
func (o *onionLocationWriter) Unwrap() http.ResponseWriter {
	return o.ResponseWriter
}

// Implement http.Flusher so SSE (which bypasses us via the path check above) still works
func (o *onionLocationWriter) Flush() {
	if f, ok := o.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// extensionStripMiddleware strips .txt, .json, .rss, and .atom extensions from paths
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

		// Strip known format extensions for routing
		switch {
		case strings.HasSuffix(path, ".txt"):
			r.URL.Path = strings.TrimSuffix(path, ".txt")
		case strings.HasSuffix(path, ".json"):
			r.URL.Path = strings.TrimSuffix(path, ".json")
		case strings.HasSuffix(path, ".rss"):
			r.URL.Path = strings.TrimSuffix(path, ".rss")
		case strings.HasSuffix(path, ".atom"):
			r.URL.Path = strings.TrimSuffix(path, ".atom")
		case strings.HasSuffix(path, ".csv"):
			r.URL.Path = strings.TrimSuffix(path, ".csv")
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
	// Set scheduler for admin panel management per AI.md PART 18
	admin.SetScheduler(s.scheduler)
	// Set logger for audit and security event logging per AI.md PART 11
	admin.SetLogger(s.logger)
	// Set search cache for cache management per AI.md PART 9
	admin.SetSearchCache(h.GetSearchCache())
	// Set data directory for thumbnail disk cache
	h.SetDataDir(s.dataDir)
	metrics := handler.NewMetrics(s.appConfig, s.engineMgr)
	h.SetMetrics(metrics)
	// Share metrics with admin handler for analytics dashboard
	admin.SetMetrics(metrics)

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

	// Content restriction endpoints (before middleware)
	s.router.Get("/content-restricted", h.ContentRestrictedPage)
	s.router.Post("/content-restricted", h.ContentRestrictedSubmit)

	// Per AI.md PART 13/14: /server/healthz is the canonical frontend health route.
	// /healthz is an optional root alias gated on server.healthz.root.enabled (default false).
	// When enabled it MUST be a direct handler mapping to the same handler (NEVER redirect).
	s.router.Get("/server/healthz", h.HealthCheck)
	s.router.Get("/server/healthz.json", h.HealthCheck)
	s.router.Get("/server/healthz.txt", h.HealthCheck)
	if s.appConfig.Server.Healthz.Root.Enabled {
		s.router.Get("/healthz", h.HealthCheck)
		s.router.Get("/healthz.json", h.HealthCheck)
		s.router.Get("/healthz.txt", h.HealthCheck)
	}
	s.router.Get("/robots.txt", h.RobotsTxt)
	s.router.Get("/sitemap.xml", h.SitemapXML)
	s.router.Get("/.well-known/security.txt", h.SecurityTxt)
	s.router.Get("/.well-known/change-password", handler.ChangePasswordRedirect(s.appConfig))
	s.router.Get("/.well-known/vidveil.json", h.WellKnownVidVeil)
	s.router.Get("/humans.txt", h.HumansTxt)
	s.router.Get("/favicon.ico", h.Favicon)
	s.router.Get("/apple-touch-icon.png", h.AppleTouchIcon)

	// PWA assets at root per AI.md PART 16 — service worker must be at root scope
	// sw.js served at /sw.js with Service-Worker-Allowed: / header so it controls the full app
	s.router.Get("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		data, err := embeddedFS.ReadFile("static/js/sw.js")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Service-Worker-Allowed", "/")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(data)
	})
	s.router.Get("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := embeddedFS.ReadFile("static/manifest.json")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/manifest+json")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(data)
	})
	s.router.Get("/offline.html", func(w http.ResponseWriter, r *http.Request) {
		data, err := embeddedFS.ReadFile("static/offline.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// Per AI.md PART 30: <html lang dir> must never be hardcoded — execute as template
		tmpl, err := template.New("offline").Parse(string(data))
		if err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
		lang := i18n.DetectLocale(r)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		tmpl.Execute(w, map[string]string{
			"Lang": lang,
			"Dir":  i18n.Direction(lang),
		})
	})

	// Debug endpoints (PART 6: only when --debug flag or DEBUG=true)
	s.registerDebugRoutes(s.router)

	// OpenAPI/Swagger and GraphQL — canonical routes per AI.md PART 14.
	// Web UI pages: /server/docs/swagger  /server/docs/graphql
	// Versioned API: /api/v1/server/swagger  /api/v1/server/graphql
	// Unversioned aliases (same handler, no redirect): /api/swagger  /api/graphql
	gql := graphql.NewHandler(s.appConfig, s.engineMgr)

	// Swagger UI (HTML)
	s.router.Get("/server/docs/swagger", swagger.Handler(s.appConfig))
	// GraphiQL UI (HTML)
	s.router.Get("/server/docs/graphql", gql.GraphiQL)

	// Versioned OpenAPI JSON spec
	s.router.Get("/api/v1/server/swagger", swagger.SpecHandler(s.appConfig))
	// Versioned GraphQL endpoint
	s.router.HandleFunc("/api/v1/server/graphql", gql.Handle)

	// Unversioned aliases — SAME handler, not redirects (PART 14)
	s.router.Get("/api/swagger", swagger.SpecHandler(s.appConfig))
	s.router.HandleFunc("/api/graphql", gql.Handle)
	// /api/healthz is the unversioned direct JSON alias for /api/v1/server/healthz
	s.router.Get("/api/healthz", h.APIHealthCheck)

	// Prometheus metrics
	if s.appConfig.Server.Metrics.Enabled {
		s.router.Get(s.appConfig.Server.Metrics.Endpoint, metrics.Handler())
	}

	// Routes that require age verification (project-specific per PART 14)
	s.router.Group(func(r chi.Router) {
		// Content restriction check comes first (geographic restrictions)
		r.Use(h.ContentRestrictionMiddleware)
		// Age verification check comes second
		r.Use(h.AgeVerifyMiddleware)

		r.Get("/", h.HomePage)
		r.Get("/search", h.SearchPage)
		r.Get("/search.rss", h.SearchRSSFeed)
		r.Get("/search.atom", h.SearchAtomFeed)
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
	// Admin auth routes per AI.md PART 11
	// VidVeil is stateless - no PART 34 (Multi-User), only Server Admin auth
	s.router.Route("/auth", func(r chi.Router) {
		r.Get("/login", auth.LoginPage)
		r.Post("/login", auth.LoginPage)
		r.Get("/logout", auth.LogoutPage)
		// Per AI.md PART 11: 2FA verification step (after password, before session)
		r.Get("/2fa", auth.TwoFactorPage)
		r.Post("/2fa", auth.TwoFactorPage)
		r.Get("/password/forgot", auth.PasswordForgotPage)
		r.Post("/password/forgot", auth.PasswordForgotPage)
		r.Get("/password/reset/{token}", auth.PasswordResetPage)
		r.Post("/password/reset", auth.PasswordResetPage)
	})

	// Admin panel routes - PART 14 (routes), PART 16 (admin panel UI)
	// Spec-canonical mount: /server/{admin_path} (AI.md PART 12 admin path config)
	// Path is configurable via server.admin.path (default: "admin")
	adminBasePath := s.appConfig.AdminURLPrefix()
	s.router.Route(adminBasePath, func(r chi.Router) {
		// Login page per AI.md PART 11
		r.Get("/login", admin.LoginPage)
		r.Post("/login", admin.LoginPage)

		// Logout handler
		r.Get("/logout", admin.LogoutHandler)
		r.Post("/logout", admin.LogoutHandler)

		// Root: Setup token entry (first run) or dashboard (authenticated)
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			if admin.IsFirstRun() {
				admin.SetupTokenPage(w, req)
				return
			}
			admin.AuthMiddleware(http.HandlerFunc(admin.DashboardPage)).ServeHTTP(w, req)
		})
		r.Post("/", func(w http.ResponseWriter, req *http.Request) {
			if admin.IsFirstRun() {
				admin.SetupTokenPage(w, req)
				return
			}
			admin.AuthMiddleware(http.HandlerFunc(admin.DashboardPage)).ServeHTTP(w, req)
		})

		// Protected admin routes per AI.md PART 16
		r.Group(func(r chi.Router) {
			r.Use(admin.AuthMiddleware)
			r.Use(admin.CSRFMiddleware)

			r.Get("/dashboard", admin.DashboardPage)
			r.Get("/profile", admin.ProfilePage)
			r.Get("/preferences", admin.PreferencesPage)
			r.Get("/notifications", admin.AdminNotificationsPage)
			r.Get("/logout", admin.LogoutHandler)

			// Spec-canonical: ALL server management goes under /config (AI.md PART 14 route scopes)
			r.Route("/config", func(r chi.Router) {
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

				r.Route("/security", func(r chi.Router) {
					r.Get("/", admin.SecurityAuthPage)
					r.Get("/auth", admin.SecurityAuthPage)
					r.Get("/tokens", admin.SecurityTokensPage)
					r.Get("/ratelimit", admin.SecurityRateLimitPage)
					r.Get("/firewall", admin.SecurityFirewallPage)
				})

				r.Route("/network", func(r chi.Router) {
					r.Get("/", admin.TorPage)
					r.Get("/tor", admin.TorPage)
					r.Get("/geoip", admin.GeoIPPage)
					r.Get("/blocklists", admin.BlocklistsPage)
				})

				r.Get("/backup", admin.BackupPage)
				r.Get("/maintenance", admin.MaintenancePage)
				r.Get("/updates", admin.UpdatesPage)
				r.Get("/info", admin.SystemInfoPage)

				r.Route("/users", func(r chi.Router) {
					r.Get("/admins", admin.UsersAdminsPage)
				})

				r.Get("/engines", admin.EnginesPage)
				r.Get("/help", admin.HelpPage)
			})
		})

		// Setup wizard at /server/{admin_path}/config/setup (token-cookie gated)
		r.Get("/config/setup", admin.SetupWizardPage)
		r.Post("/config/setup", admin.SetupWizardPage)

		// Admin invite page (public, token validated in handler)
		r.Get("/invite/{token}", admin.AdminInvitePage)
		r.Post("/invite/{token}", admin.AdminInvitePage)
	})

	// Legacy redirects: keep old /{admin_path}/... bookmarks alive per AUDIT plan.
	legacyAdminPath := "/" + s.appConfig.Server.Admin.Path
	legacyAdminPrefix := legacyAdminPath + "/"
	canonicalAdminPrefix := adminBasePath + "/"
	legacyRedirect := func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		var target string
		switch {
		case path == legacyAdminPath:
			target = adminBasePath
		case strings.HasPrefix(path, legacyAdminPrefix):
			tail := strings.TrimPrefix(path, legacyAdminPrefix)
			tail = strings.TrimPrefix(tail, "server/")
			if tail != "" && !strings.HasPrefix(tail, "config/") &&
				tail != "dashboard" && tail != "profile" && tail != "preferences" &&
				tail != "notifications" && tail != "login" && tail != "logout" &&
				!strings.HasPrefix(tail, "invite/") {
				tail = "config/" + tail
			}
			target = canonicalAdminPrefix + tail
		default:
			target = adminBasePath
		}
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusPermanentRedirect)
	}
	s.router.Get(legacyAdminPath, legacyRedirect)
	s.router.Get(legacyAdminPath+"/*", legacyRedirect)

	// API autodiscover endpoint (non-versioned per AI.md PART 14)
	// Clients need this BEFORE they know the API version
	s.router.Get("/api/autodiscover", h.Autodiscover)

	// API v1 routes
	s.router.Route("/api/v1", func(r chi.Router) {
		// Search endpoint (public) - content negotiation for JSON, SSE, text
		// Accept: application/json (default) - JSON response with caching
		// Accept: text/event-stream - SSE streaming results as engines respond
		// Accept: text/plain or .txt extension - plain text format
		r.Get("/search", h.APISearch)
		r.Post("/search/batch", h.BatchSearch)

		// Bang endpoints (public) - per AI.md PART 14
		r.Get("/bangs", h.APIBangs)
		r.Get("/bangs/autocomplete", h.APIAutocomplete)

		// Engine endpoints (public)
		r.Get("/engines", h.APIEngines)
		r.Get("/engines/health", h.APIEngineHealth)
		r.Get("/engines/{name}", h.APIEngineDetails)

		// Stats (public)
		r.Get("/stats", h.APIStats)

		// Debug endpoints (development only per IDEA.md)
		r.Route("/debug", func(r chi.Router) {
			r.Get("/engines", h.DebugEnginesList)
			r.Get("/engines/{name}", h.DebugEngine)
		})

		// Per AI.md PART 14: canonical health route is /api/{api_version}/server/healthz.
		// Legacy /api/{api_version}/healthz alias removed — spec requires no shims.
		r.Get("/version", h.APIVersion)

		// Server API per AI.md PART 14
		r.Route("/server", func(r chi.Router) {
			r.Get("/healthz", h.APIHealthCheck)
			r.Get("/about", server.APIAbout)
			r.Get("/privacy", server.APIPrivacy)
			r.Post("/contact", server.APIContact)
			r.Get("/help", server.APIHelp)
		})

		// Proxy endpoints (plural per PART 14)
		r.Get("/proxy/thumbnails", h.ProxyThumbnail)
		r.Get("/proxy/videos", h.ProxyVideo)

		// Admin Profile API (session or token) - PART 11
		// Spec-canonical: /api/{ver}/server/{admin_path}/profile (AI.md PART 14 line 4584)
		r.Route(s.appConfig.AdminAPIPrefix()+"/profile", func(r chi.Router) {
			r.Use(admin.SessionOrTokenMiddleware)
			r.Post("/password", admin.APIProfilePassword)
			r.Post("/token", admin.APIProfileToken)
			r.Delete("/sessions", admin.APIRevokeSessions)
			r.Get("/recovery-keys", admin.APIRecoveryKeysStatus)
			r.Post("/recovery-keys/generate", admin.APIRecoveryKeysGenerate)
			r.Post("/2fa/setup", admin.APIProfile2FASetup)
			r.Post("/2fa/verify", admin.APIProfile2FAVerify)
			r.Delete("/2fa", admin.APIProfile2FADisable)
		})

		// Engine management API (session or token — accessible from browser admin panel)
		r.Route(s.appConfig.AdminAPIPrefix()+"/engines", func(r chi.Router) {
			r.Use(admin.SessionOrTokenMiddleware)
			r.Patch("/{name}", admin.APIEnginePatch)
			r.Post("/{name}/reset", admin.APIEngineReset)
		})

		// Admin API (token required) - PART 12, PART 14
		r.Route(s.appConfig.AdminAPIPrefix(), func(r chi.Router) {
			r.Use(admin.APITokenMiddleware)

			// Spec-canonical: ALL admin API endpoints under /config (AI.md PART 14 line 4584)
			r.Route("/config", func(r chi.Router) {
				// Users management per AI.md PART 11
				r.Post("/users/admins/invite", admin.APIUsersAdminsInvite)
				r.Get("/users/admins/invites", admin.APIUsersAdminsInvites)
				r.Delete("/users/admins/invites/{id}", admin.APIUsersAdminsInviteRevoke)
				// Settings
				r.Get("/settings", admin.APIConfig)
				r.Patch("/settings", admin.APIConfig)
				r.Get("/status", admin.APIStatus)
				r.Get("/health", admin.APIHealth)
				r.Post("/restart", admin.APIMaintenanceMode)

				// Branding per PART 16
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

				// Tor per PART 31
				r.Route("/tor", func(r chi.Router) {
					r.Get("/", admin.APITorStatus)
					r.Patch("/", admin.APITorUpdate)
					r.Post("/regenerate", admin.APITorRegenerate)
					r.Post("/test", admin.APITorTest)
					r.Post("/validate", admin.APITorValidate)
					r.Post("/restart", admin.APITorRestart)
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

				// Scheduler per PART 18
				r.Route("/scheduler", func(r chi.Router) {
					r.Get("/", admin.APISchedulerTasks)
					r.Get("/{id}", admin.APISchedulerTasks)
					r.Patch("/{id}", admin.APISchedulerTasks)
					r.Post("/{id}/run", admin.APISchedulerRunTask)
					r.Post("/{id}/enable", admin.APISchedulerTasks)
					r.Post("/{id}/disable", admin.APISchedulerTasks)
				})

				// Backup per PART 21
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

				// Pages per PART 16
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

				// Analytics (privacy-safe aggregate counters)
				r.Get("/analytics", admin.APIAnalytics)
			})
		})
	})

	// Custom 404 handler per AI.md PART 14
	s.router.NotFound(h.NotFoundHandler)
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe(addr string) error {
	// Parse timeouts from config per AI.md PART 12
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

// Listen binds to the given address and returns the listener without accepting
// connections. Call Serve(l) after privilege drop.
// Per AI.md PART 23: bind privileged ports as root, then drop, then serve.
func (s *Server) Listen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

// ServeOn serves HTTP requests on the given pre-bound listener.
// Per AI.md PART 24: called after privilege drop.
func (s *Server) ServeOn(listener net.Listener) error {
	readTimeout := parseDuration(s.appConfig.Server.Limits.ReadTimeout, 30*time.Second)
	writeTimeout := parseDuration(s.appConfig.Server.Limits.WriteTimeout, 30*time.Second)
	idleTimeout := parseDuration(s.appConfig.Server.Limits.IdleTimeout, 120*time.Second)

	s.srv = &http.Server{
		Handler:      s.router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	return s.srv.Serve(listener)
}

// Serve serves on the given listener (for Tor hidden service)
// Per AI.md PART 31: HTTP server serves on both TCP (clearnet) and Tor listener
func (s *Server) Serve(listener net.Listener) error {
	// Parse timeouts from config per AI.md PART 12
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
// parseBodySize parses size string like "10MB", "100KB" to bytes per AI.md PART 12
func parseBodySize(s string, defaultVal int64) int64 {
	if s == "" {
		return defaultVal
	}
	s = strings.TrimSpace(strings.ToUpper(s))
	var multiplier int64 = 1
	switch {
	case strings.HasSuffix(s, "GB"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "MB"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "KB"):
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "B"):
		s = strings.TrimSuffix(s, "B")
	}
	val := int64(0)
	for _, c := range s {
		if c < '0' || c > '9' {
			return defaultVal
		}
		val = val*10 + int64(c-'0')
	}
	if val == 0 {
		return defaultVal
	}
	return val * multiplier
}

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

// ctxKeyAllowlisted is the context key used to flag a request as allowlisted.
type ctxKeyAllowlistType struct{}

var ctxKeyAllowlisted = ctxKeyAllowlistType{}

// isAllowlisted reports whether the request context carries the allowlisted flag.
func isAllowlisted(r *http.Request) bool {
	v, _ := r.Context().Value(ctxKeyAllowlisted).(bool)
	return v
}

// allowlistMiddleware sets the allowlisted context flag when the client IP matches
// a trusted CIDR in server.security.allowlist. Downstream middleware (blocklist,
// rate limit, geoip) must check isAllowlisted() and skip enforcement for flagged
// requests. Auth middleware IGNORES this flag — authentication is always required.
// Spec: AI.md PART 11
func (s *Server) allowlistMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entries := s.appConfig.Server.Security.Allowlist
		if len(entries) > 0 {
			ip := extractClientIP(r)
			parsed := net.ParseIP(ip)
			if parsed != nil {
				for _, entry := range entries {
					cidr := entry.CIDR
					// Auto-expand bare IPs: IPv4 → /32, IPv6 → /128
					if !strings.Contains(cidr, "/") {
						if strings.Contains(cidr, ":") {
							cidr += "/128"
						} else {
							cidr += "/32"
						}
					}
					_, network, err := net.ParseCIDR(cidr)
					if err == nil && network.Contains(parsed) {
						ctx := r.Context()
						ctx = context.WithValue(ctx, ctxKeyAllowlisted, true)
						r = r.WithContext(ctx)
						break
					}
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// blocklistMiddleware checks the client IP against the configured IP/domain
// blocklist. Allowlisted IPs are exempt. Blocked IPs receive 403 Forbidden.
// Spec: AI.md PART 11
func (s *Server) blocklistMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allowlisted IPs bypass blocklist per spec
		if isAllowlisted(r) {
			next.ServeHTTP(w, r)
			return
		}
		checker := s.ipBlocklist
		if checker == nil || !s.appConfig.Server.Security.Blocklists.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		ip := extractClientIP(r)
		if ip != "" && checker.IsBlocked(ip) {
			http.Error(w, "Your IP address has been blocked.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// geoIPMiddleware enforces country blocking per server.geoip.deny_countries /
// server.geoip.allow_countries config. Allowlisted IPs are exempt. Private/
// internal IPs are never blocked (handled by GeoIPService.IsBlocked).
// Spec: AI.md PART 19
func (s *Server) geoIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allowlisted IPs bypass country blocking per spec
		if isAllowlisted(r) {
			next.ServeHTTP(w, r)
			return
		}
		blocker := s.geoIPBlocker
		if blocker == nil || !s.appConfig.Server.GeoIP.Enabled {
			next.ServeHTTP(w, r)
			return
		}
		// Only enforce when deny_countries or allow_countries is configured
		if len(s.appConfig.Server.GeoIP.DenyCountries) == 0 &&
			len(s.appConfig.Server.GeoIP.AllowCountries) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		ip := extractClientIP(r)
		if ip != "" && blocker.IsBlocked(ip) {
			http.Error(w, "Access from your country is not permitted.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractClientIP returns the best-effort client IP from a request.
// chi's RealIP middleware has already normalized r.RemoteAddr to the real IP.
func extractClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// No port separator — RemoteAddr is already a bare IP
		ip = r.RemoteAddr
	}
	return ip
}

// secFetchValidationMiddleware validates Sec-Fetch-* request headers per AI.md PART 11.
// This is a defense-in-depth layer against CSRF and clickjacking — it runs BEFORE
// the CSRF token check. Validation is "present-and-bad reject only": absent headers
// are treated as a legacy-browser pass-through and fall through to the CSRF token check.
//
// Rules per PART 11:
//   - Sec-Fetch-Site: reject cross-site on POST/PUT/PATCH/DELETE without Bearer token
//     and path not in CSRF exempt paths.
//   - Sec-Fetch-Mode: reject navigate on /api/* endpoints (unintended top-level nav).
//   - Sec-Fetch-Dest: reject iframe on endpoints not in frame-ancestors allow-list.
func secFetchValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sec-Fetch-Site: block cross-site state-changing requests without Bearer auth
		site := r.Header.Get("Sec-Fetch-Site")
		if site == "cross-site" {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				// Allow if Bearer token present — Bearer-authenticated APIs are CORS-protected
				if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
					http.Error(w, "Cross-site requests are not permitted.", http.StatusForbidden)
					return
				}
			}
		}

		// Sec-Fetch-Mode: block navigate fetches to /api/* — indicates unintended top-level nav
		mode := r.Header.Get("Sec-Fetch-Mode")
		if mode == "navigate" && strings.HasPrefix(r.URL.Path, "/api/") {
			http.Error(w, "Direct navigation to API endpoints is not permitted.", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
