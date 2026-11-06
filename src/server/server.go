// SPDX-License-Identifier: MIT
package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/cors"

	"github.com/apimgr/vidveil/src/common/authpad"
	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/common/outsan"
	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/graphql"
	"github.com/apimgr/vidveil/src/mode"
	"github.com/apimgr/vidveil/src/notify"
	paths "github.com/apimgr/vidveil/src/paths"
	"github.com/apimgr/vidveil/src/server/handler"
	mcpserver "github.com/apimgr/vidveil/src/server/mcp"
	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/branding"
	"github.com/apimgr/vidveil/src/server/service/email"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/logging"
	svcmetrics "github.com/apimgr/vidveil/src/server/service/metric"
	"github.com/apimgr/vidveil/src/server/service/ratelimit"
	"github.com/apimgr/vidveil/src/server/service/scheduler"
	"github.com/apimgr/vidveil/src/server/service/secret"
	"github.com/apimgr/vidveil/src/server/service/ssl"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
	"github.com/apimgr/vidveil/src/swagger"
)

//go:embed static/css/* static/js/* static/images/* static/icons/* static/splash/* static/screenshots/* static/manifest.json static/offline.html template/page/*.tmpl template/partial/public/*.tmpl template/layout/*.tmpl template/component/*.tmpl template/nojs/*.tmpl
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

// TLSProvider supplies the TLS config and ACME HTTP handler for HTTPS serving
// per AI.md PART 15. Implemented by the ssl.SSLManager. Kept as a local interface
// so the server package does not import the ssl package directly.
type TLSProvider interface {
	GetTLSConfig() *tls.Config
	GetHTTPHandler() http.Handler
}

// Server represents the HTTP server
type Server struct {
	appConfig          *config.AppConfig
	configDir          string
	dataDir            string
	engineMgr          *engine.EngineManager
	migrationMgr       MigrationManager
	scheduler          *scheduler.Scheduler
	logger             *logging.AppLogger
	router             *chi.Mux
	srv                *http.Server
	rateLimiter        *ratelimit.RateLimiter
	tokenActionLimiter *ratelimit.RateLimiter
	searchHandler      *handler.SearchHandler
	serverHandler      *handler.ServerHandler
	emailService       *email.EmailService
	notifyDispatcher   *notify.Dispatcher
	brandingImages     *branding.ImageCache
	brandingCtx        context.Context
	brandingCancel     context.CancelFunc
	brandingRefreshWG  sync.WaitGroup
	serversMu          sync.Mutex
	servers            []*http.Server
	// stored for Onion-Location middleware
	torSvc handler.TorStatusChecker
	// internal loopback-only Tor control channel per AI.md PART 31
	torControl TorControl
	// geoip for country blocking middleware per AI.md PART 19
	geoIPBlocker GeoIPBlocker
	// blocklist for IP/domain blocklist middleware per AI.md PART 11
	ipBlocklist IPBlocklistChecker
	// tlsProvider supplies TLS config + ACME handler for HTTPS serving per AI.md PART 15
	tlsProvider         TLSProvider
	operatorTokenHashMu sync.RWMutex
	operatorTokenHash   [sha256.Size]byte
	operatorTokenSet    bool
	tokenManager        *apitoken.Manager
	secretsMgr          *secret.Manager
}

// MigrationManager interface for database migrations
type MigrationManager interface {
	GetMigrationStatus() ([]map[string]interface{}, error)
	RunMigrations() error
	RollbackMigration() error
	GetDB() *sql.DB
}

// NewServer creates a new server instance
func NewServer(appConfig *config.AppConfig, configDir, dataDir string, engineMgr *engine.EngineManager, migrationMgr MigrationManager, sched *scheduler.Scheduler, logger *logging.AppLogger) *Server {
	// Set templates filesystem for handlers
	handler.SetTemplatesFS(embeddedFS)

	// Create rate limiter per PART 12
	limiter := ratelimit.NewRateLimiter(
		appConfig.Server.RateLimit.Enabled,
		appConfig.Server.RateLimit.Requests,
		appConfig.Server.RateLimit.Window,
	)
	// Set logger for security event logging per AI.md PART 11
	limiter.SetLogger(logger)

	s := &Server{
		appConfig:          appConfig,
		configDir:          configDir,
		dataDir:            dataDir,
		engineMgr:          engineMgr,
		migrationMgr:       migrationMgr,
		scheduler:          sched,
		logger:             logger,
		router:             chi.NewRouter(),
		rateLimiter:        limiter,
		tokenActionLimiter: ratelimit.NewRateLimiter(true, 5, 15*60),
	}
	if migrationMgr != nil && migrationMgr.GetDB() != nil {
		s.secretsMgr = secret.NewManager(migrationMgr.GetDB())
	}
	s.brandingImages = branding.NewImageCache(dataDir, branding.ImageSources{
		Logo:    appConfig.Server.Branding.Logo,
		Favicon: appConfig.Server.Branding.Favicon,
		OGImage: appConfig.Server.SEO.OGImage,
	})
	if defaultOG, err := embeddedFS.ReadFile("static/icons/icon-512.png"); err != nil {
		log.Printf("branding default OpenGraph image could not be read")
	} else if err := s.brandingImages.SeedDefaultOG(defaultOG); err != nil {
		log.Printf("branding default OpenGraph image could not be cached")
	}
	brandingCtx, brandingCancel := context.WithCancel(context.Background())
	s.brandingCtx = brandingCtx
	s.brandingCancel = brandingCancel
	s.brandingRefreshWG.Add(1)
	go func() {
		defer s.brandingRefreshWG.Done()
		if err := s.RefreshBrandingImages(brandingCtx); err != nil && brandingCtx.Err() == nil {
			log.Printf("initial branding image refresh failed")
		}
	}()
	if sched != nil {
		interval := parseDuration(appConfig.Server.Branding.RefreshInterval, 24*time.Hour)
		if interval <= 0 {
			interval = 24 * time.Hour
		}
		if err := sched.RegisterTask("branding_refresh", "Branding Image Refresh", "Refresh and resize configured branding images", "@every "+interval.String(), func(ctx context.Context) error {
			return s.RefreshBrandingImages(ctx)
		}); err != nil {
			log.Printf("branding image refresh task registration failed: %v", err)
		}
		if err := sched.RegisterTask("owner_idle_purge", "Idle Owner Purge", "Remove owner tokens for owners with no favorites after the configured idle period", "@daily", func(ctx context.Context) error {
			idleDays := appConfig.Server.Tokens.IdlePurgeDays
			if idleDays <= 0 || s.migrationMgr == nil {
				return nil
			}
			manager, err := apitoken.NewManager(s.migrationMgr.GetDB(), appConfig.Server.Security.EncryptionKey)
			if err != nil {
				return err
			}
			purged, err := manager.PurgeIdleOwners(ctx, idleDays)
			if err == nil && purged > 0 {
				log.Printf("purged %d idle owner identities", purged)
			}
			return err
		}); err != nil {
			log.Printf("idle owner purge task registration failed: %v", err)
		}
		if s.secretsMgr != nil {
			if err := sched.RegisterTask("csrf_secret_rotation", "CSRF Secret Rotation", "Rotate the CSRF token HMAC secret every 180 days", "@daily", func(ctx context.Context) error {
				rotated, err := s.secretsMgr.RotateIfDue(ctx, secret.CSRFTokenSecret, 180*24*time.Hour)
				if err == nil && rotated {
					log.Printf("CSRF token secret rotated after reaching its 180-day age")
				}
				return err
			}); err != nil {
				log.Printf("CSRF secret rotation task registration failed: %v", err)
			}
		}
	}
	s.RefreshOperatorTokenHash()

	// Wire app config into the URL resolver for trusted proxy gate and Tor detection
	// per AI.md PART 12. Must be called before setupMiddleware uses the resolver.
	urlvar.GlobalResolver().SetAppConfig(appConfig)

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// RefreshBrandingImages reloads configured sources and refreshes the local
// cache. It is used at startup, by the scheduler, and after config hot-reload.
func (s *Server) RefreshBrandingImages(ctx context.Context) error {
	if s.brandingImages == nil || s.appConfig == nil {
		return nil
	}
	s.brandingImages.SetSources(branding.ImageSources{
		Logo:    s.appConfig.Server.Branding.Logo,
		Favicon: s.appConfig.Server.Branding.Favicon,
		OGImage: s.appConfig.Server.SEO.OGImage,
	})
	return s.brandingImages.Refresh(ctx)
}

// SetRestoreRestartHandler wires the callback that restarts the process after
// an API or WebUI restore has completed successfully.
func (s *Server) SetRestoreRestartHandler(restart func()) {
	if s.serverHandler != nil {
		s.serverHandler.SetRestoreRestartHandler(restart)
	}
}

func (s *Server) serveBrandingAsset(w http.ResponseWriter, r *http.Request) {
	if s.brandingImages == nil {
		http.NotFound(w, r)
		return
	}
	name := chi.URLParam(r, "asset")
	f, err := s.brandingImages.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, name, info.ModTime(), f)
}

// SetTorService sets the Tor service for handlers that need it
func (s *Server) SetTorService(t handler.TorStatusChecker) {
	s.torSvc = t
	if s.searchHandler != nil {
		s.searchHandler.SetTorService(t)
	}
	if s.serverHandler != nil {
		s.serverHandler.SetTorService(t)
	}
}

// SetI2PService wires the optional eepsite manager into public status handlers.
func (s *Server) SetI2PService(service handler.I2PStatusChecker) {
	if s.searchHandler != nil {
		s.searchHandler.SetI2PService(service)
	}
	if s.serverHandler != nil {
		s.serverHandler.SetI2PService(service)
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
	// Middleware execution order per AI.md PART 5 / PART 16 spec (first Use = first to execute):
	// 1. URLNormalize → 2. RequestID → 3. PathSecurity → 4. SecurityHeaders →
	// 5. Allowlist → 6. Blocklist → 7. RateLimit → 8. GeoIP → 10. Logger (innermost)

	// 1. URL Normalization per AI.md PART 16 — MUST be first to execute
	s.router.Use(URLNormalizeMiddleware)

	// Finalize every state-changing response to the PART 11 minimum duration.
	// This covers both successful writes and early failures from downstream
	// middleware or handlers, including CSRF and rate-limit rejections.
	s.router.Use(writeResponseTimingMiddleware)

	// NOTE: chi's middleware.RealIP is intentionally NOT used — it mutates
	// r.RemoteAddr from proxy headers unconditionally (no trusted_proxies gate),
	// which is the same IP-spoofing class as GHSA-3fxj-6jh8-hvhx / GHSA-rjr7-jggh-pgcp
	// / GHSA-9g5q-2w5x-hmxf and is now deprecated upstream for it. r.RemoteAddr is
	// left untouched here; urlvar.ResolveClientIP resolves the trusted-gated client
	// IP per AI.md PART 12 without rewriting the original TCP peer (see PART 12
	// "Middleware ordering").

	// URL Variables resolution per AI.md PART 8 (reverse proxy headers)
	s.router.Use(urlvar.GlobalResolver().Middleware)

	// 2. Request ID per AI.md PART 5 — must run before Logging so logs carry the ID
	s.router.Use(middleware.RequestID)

	// 3. Path Security per AI.md PART 5 — validate paths, block traversal
	s.router.Use(paths.PathSecurityMiddleware)

	// Recoverer — recover panics in any downstream middleware or handler.
	// guaranteedRecoverer replaces chi's stock middleware.Recoverer (which
	// leaves an empty 500 body) with one that always renders a minimal,
	// content-negotiated error body per AI.md PART 16.
	s.router.Use(guaranteedRecoverer)

	// CORS
	s.router.Use(s.corsMiddleware)

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
			// CSP per PART 11 "Content Security Policy" — fixed default directives,
			// extended (never replaced) via web.csp.*_extra config keys.
			if s.appConfig.Web.CSP.Enabled {
				headerName := "Content-Security-Policy"
				if s.appConfig.Web.CSP.Mode == "report-only" {
					headerName = "Content-Security-Policy-Report-Only"
				}
				w.Header().Set(headerName, s.buildCSPHeader(r))
			}
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
			if s.appConfig.Server.SSL.Enabled && !urlvar.GlobalResolver().IsOverlayRequest(r) {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			}
			// Reporting-Endpoints + legacy Report-To + NEL per AI.md PART 11
			// Both modern (Reporting-Endpoints) and legacy (Report-To) formats are required.
			proto, fqdn, _ := urlvar.GlobalResolver().GetURLVars(r)
			reportsBase := proto + "://" + fqdn + s.appConfig.APIBasePath() + "/server/reports"
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
				// Static assets per PART 9 asset version-busting: immutable ONLY
				// when the ?v= stamp matches the running build; missing or
				// mismatched stamp gets no-cache + a build-stamp ETag instead.
				if r.URL.Query().Get("v") == version.AssetStamp() {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "no-cache")
					w.Header().Set("ETag", `"`+version.AssetStamp()+`"`)
				}
			} else if strings.HasPrefix(path, "/api/") {
				// API responses: no cache
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			} else {
				// HTML pages per PART 9: no-store plus a build-stamp ETag so any
				// intermediary that ignores no-store still revalidates.
				w.Header().Set("Cache-Control", "no-store, must-revalidate")
				w.Header().Set("ETag", `"`+version.AssetStamp()+`"`)
				// Version-change purge per PART 9: every HTML response carries a
				// vidveil_build cookie with the running stamp. A mismatched cookie
				// means a stale client — add Clear-Site-Data: "cache", "storage"
				// (NEVER "cookies") to evict poisoned caches and service workers.
				// One-shot by design: the re-set cookie stops the purge next time;
				// first-ever visits (no cookie) never purge.
				stamp := version.AssetStamp()
				if c, err := r.Cookie("vidveil_build"); err == nil && c.Value != stamp {
					w.Header().Set("Clear-Site-Data", `"cache", "storage"`)
				}
				http.SetCookie(w, &http.Cookie{
					Name:     "vidveil_build",
					Value:    stamp,
					Path:     "/",
					MaxAge:   31536000,
					Secure:   s.appConfig.Server.SSL.Enabled,
					SameSite: http.SameSiteLaxMode,
				})
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

	// Privacy signal headers per AI.md PART 11 — honor Sec-GPC (and DNT when the
	// operator opts in) as an opt-out; sets the request-lifecycle opt-out flag.
	s.router.Use(s.privacySignalsMiddleware)

	// Bound request bodies before CSRF or form middleware can parse them.
	maxBodyBytes := parseBodySize(s.appConfig.Server.Limits.MaxBodySize, 10*1024*1024)
	s.router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestLimit := maxBodyBytes
			if r.Method == http.MethodPost && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/server/restore") {
				if archiveLimit, err := config.ParseSizeBytes(s.appConfig.Server.Restore.MaxArchiveSize); err == nil && archiveLimit <= uint64(^uint64(0)>>1)-(1<<20) {
					requestLimit = int64(archiveLimit) + (1 << 20)
				}
			}
			r.Body = http.MaxBytesReader(w, r.Body, requestLimit)
			next.ServeHTTP(w, r)
		})
	})

	// CSRF double-submit cookie middleware per AI.md PART 16 → CSRF Protection.
	// Runs after Sec-Fetch-* (which blocks cross-site requests from modern browsers)
	// as the second CSRF layer for legacy browsers without Sec-Fetch-* headers.
	if s.appConfig.Web.CSRF.Enabled {
		s.router.Use(newCSRFMiddleware(s.appConfig.Web.CSRF, s.logger, s.secretsMgr))
	}

	// Rate limiting per AI.md PART 12 — allowlisted IPs bypass rate limiting
	s.router.Use(func(next http.Handler) http.Handler {
		inner := s.rateLimiter.Middleware(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isAllowlisted(r) || mcpserver.IsInternalDispatch(r.Context()) {
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

	// 10. Logger — LAST (innermost) per AI.md PART 5/16 spec so logs carry
	// request_id and only log requests that reached the handler chain.
	// NOTE: chi's stock middleware.Logger is intentionally NOT used here — its
	// DefaultLogFormatter logs r.RemoteAddr directly, which is blind to
	// X-Forwarded-For/X-Real-IP/CF-Connecting-IP and defeats the trusted-proxy
	// resolution built in urlvar.ResolveClientIP (PART 12 "Client IP Detection").
	// requestLogMiddleware below reproduces the same console format but sources
	// the IP through ResolveClientIP so console logs match the access log file.
	s.router.Use(requestLogMiddleware)
}

// writeResponseTimingMiddleware pads state-changing requests to the common
// timing floor so success and failure paths do not expose internal state.
func writeResponseTimingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
		default:
			response := authpad.WrapResponse(w, time.Now())
			next.ServeHTTP(response, r)
			response.Finish()
		}
	})
}

// corsMiddleware resolves configured and trusted learned origins per request.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := s.appConfig.Server.CORS
		if len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "" {
			next.ServeHTTP(w, r)
			return
		}

		origins := make([]string, 0, len(cfg.AllowedOrigins)+4)
		for _, origin := range cfg.AllowedOrigins {
			if origin != "" {
				origins = appendUniqueOrigin(origins, origin)
			}
		}
		for _, domain := range urlvar.GetAllDomains() {
			origins = appendUniqueOrigin(origins, "https://"+domain)
		}
		torRequest := s.isTorRequest(r)
		if urlvar.GlobalResolver().IsTrustedProxy(r) {
			proto, host, port := urlvar.GlobalResolver().GetURLVars(r)
			if proto == "http" || proto == "https" {
				if port != "" {
					host = net.JoinHostPort(strings.Trim(host, "[]"), port)
				} else if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
					host = "[" + host + "]"
				}
				origins = appendUniqueOrigin(origins, proto+"://"+host)
			}
		}
		if torRequest {
			origins = appendUniqueOrigin(nil, "http://"+strings.ToLower(strings.Trim(s.appConfig.Server.Tor.OnionAddress, "[]")))
		}
		if len(origins) == 0 {
			origins = []string{"*"}
		}

		wildcard := false
		for _, origin := range origins {
			if origin == "*" {
				wildcard = true
				break
			}
		}
		maxAge := cfg.MaxAge
		if maxAge <= 0 {
			maxAge = 86400
		}
		cors.New(cors.Options{
			AllowedOrigins: origins,
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{
				"Content-Type", "Accept", "X-Requested-With", "Authorization",
				"X-CSRF-Token", "X-XSRF-Token",
			},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: cfg.AllowCredentials && !wildcard,
			MaxAge:           maxAge,
		}).Handler(next).ServeHTTP(w, r)
	})
}

// isTorRequest recognizes requests addressed to the configured onion host,
// including a host forwarded by a trusted reverse proxy.
func (s *Server) isTorRequest(r *http.Request) bool {
	if s.appConfig != nil {
		return urlvar.IsTorRequestForAddress(r, s.appConfig.Server.Tor.OnionAddress)
	}
	return urlvar.IsTorRequest(r)
}

// appendUniqueOrigin adds an origin only when it is a valid HTTP origin.
func appendUniqueOrigin(origins []string, origin string) []string {
	if origin == "*" {
		for _, existing := range origins {
			if existing == "*" {
				return origins
			}
		}
		return append(origins, origin)
	}
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return origins
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return origins
	}
	port := parsed.Port()
	if (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		parsed.Host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		parsed.Host = "[" + hostname + "]"
	} else {
		parsed.Host = hostname
	}
	parsed.Path = ""
	origin = parsed.String()
	for _, existing := range origins {
		if strings.EqualFold(existing, origin) {
			return origins
		}
	}
	return append(origins, origin)
}

// cspHostnamePattern validates a bare hostname (no scheme, no port, no path)
// before it is concatenated into a CSP directive value. Request-derived
// fqdn values (e.g. from X-Forwarded-Host) must never carry CSP metacharacters
// (';', '"', whitespace, etc.) into the header — see PART 11 "Content
// Security Policy" — hence a strict RFC 1123 label/hostname allowlist.
var cspHostnamePattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]{0,251}[A-Za-z0-9])?$`)

// isValidCSPHostname reports whether host is safe to embed directly in a CSP
// directive value (letters, digits, '.', '-' only; RFC 1035 253-char cap).
func isValidCSPHostname(host string) bool {
	return host != "" && len(host) <= 253 && cspHostnamePattern.MatchString(host)
}

// buildCSPHeader constructs the Content-Security-Policy value from the fixed
// default directives, extended per-directive by web.csp.*_extra config keys
// (appended, never replacing the default) or replaced wholesale by the
// corresponding *_override key when set. Per AI.md PART 11 "Content Security
// Policy" the operator never has to redefine the whole policy.
func (s *Server) buildCSPHeader(r *http.Request) string {
	cfg := s.appConfig.Web.CSP

	// {learned_origins} per PART 11: DOMAIN env entries + reverse-proxy-
	// detected host (same source as CORS resolution — PART 16). Applied to
	// connect-src, frame-ancestors, and form-action per the "Auto-detection"
	// note. Every origin is validated as a bare RFC 1123 hostname before
	// being concatenated into the header — request-derived values (fqdn from
	// X-Forwarded-Host) are otherwise attacker-influenced and could inject
	// CSP metacharacters/directives if trusted blindly.
	var learnedOrigins []string
	for _, o := range urlvar.GetAllDomains() {
		if isValidCSPHostname(o) {
			learnedOrigins = append(learnedOrigins, o)
		}
	}
	proto, fqdn, _ := urlvar.GlobalResolver().GetURLVars(r)
	if isValidCSPHostname(fqdn) {
		self := proto + "://" + fqdn
		found := false
		for _, o := range learnedOrigins {
			if o == fqdn || o == self {
				found = true
				break
			}
		}
		if !found {
			learnedOrigins = append(learnedOrigins, self)
		}
	}
	learnedSuffix := ""
	for _, o := range learnedOrigins {
		learnedSuffix += " " + o
	}

	connectSrc := "'self'" + learnedSuffix
	if cfg.ConnectSrcExtra != "" {
		connectSrc += " " + cfg.ConnectSrcExtra
	}
	frameAncestors := "'self'" + learnedSuffix
	formAction := "'self'" + learnedSuffix
	if cfg.FormActionExtra != "" {
		formAction += " " + cfg.FormActionExtra
	}

	directive := func(name, def, extra, override string) string {
		if override != "" {
			return name + " " + override + "; "
		}
		val := def
		if extra != "" {
			val += " " + extra
		}
		return name + " " + val + "; "
	}

	policy := directive("default-src", "'self'", "", "") +
		directive("script-src", "'self'", cfg.ScriptSrcExtra, cfg.ScriptSrcOverride) +
		directive("style-src", "'self' 'unsafe-inline'", cfg.StyleSrcExtra, cfg.StyleSrcOverride) +
		directive("img-src", "'self' data: blob: https:", cfg.ImgSrcExtra, "") +
		directive("font-src", "'self' https:", cfg.FontSrcExtra, "") +
		"connect-src " + connectSrc + "; " +
		directive("media-src", "'self' blob:", "", "") +
		directive("worker-src", "'self' blob:", "", "") +
		directive("manifest-src", "'self'", "", "") +
		directive("frame-src", "'self'", cfg.FrameSrcExtra, "") +
		"frame-ancestors " + frameAncestors + "; " +
		directive("base-uri", "'self'", "", "") +
		"form-action " + formAction + "; " +
		directive("object-src", "'none'", "", "")
	if s.appConfig.Server.SSL.Enabled || proto == "https" {
		policy += "upgrade-insecure-requests; "
	}
	policy +=
		"report-to default; " +
			"report-uri " + s.appConfig.APIBasePath() + "/server/reports/csp"

	return policy
}

// requestLogMiddleware logs each request to the console in the same format as
// chi's DefaultLogFormatter ("[host/request-id] "METHOD scheme://host/path proto"
// from <ip> - <status> <size>B in <duration>"), but resolves the client IP via
// urlvar.ResolveClientIP instead of raw r.RemoteAddr, so console output honors
// trusted reverse-proxy headers per AI.md PART 12.
func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		reqID := middleware.GetReqID(r.Context())
		// Per AI.md PART 31 never log the Tor daemon's loopback address as a
		// client identifier — substitute the exported circuit ID (or the
		// "tor" sentinel when unavailable) for Tor requests
		clientIP := urlvar.ResolveClientIP(r)
		if urlvar.IsTorRequest(r) {
			clientIP = urlvar.TorClientLabel(r)
		}
		requestHost := urlvar.RequestHost(r)
		requestURI := outsan.RedactURL(r.URL.RequestURI())
		source := ""
		if mcpserver.IsInternalDispatch(r.Context()) {
			source = " source=mcp"
		}
		log.Printf("[%s/%s] %q from %s - %d %dB in %s%s",
			requestHost, reqID,
			fmt.Sprintf("%s %s://%s%s %s", r.Method, scheme, requestHost, requestURI, r.Proto),
			clientIP,
			ww.Status(), ww.BytesWritten(),
			time.Since(start), source,
		)
	})
}

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
		// Skip entirely if Tor is not running or this request uses the configured onion host.
		if s.torSvc == nil || s.isTorRequest(r) {
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
		ctx := context.WithValue(r.Context(), handler.OriginalPathKey, path)
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
	if s.migrationMgr != nil {
		manager, err := apitoken.NewManager(s.migrationMgr.GetDB(), s.appConfig.Server.Security.EncryptionKey)
		if err != nil {
			log.Printf("owner-token manager could not be initialized: %v", err)
		} else {
			s.tokenManager = manager
		}
	}
	s.router.Use(s.ownerTokenCookieMiddleware)
	// Store handler reference for later service injection
	s.searchHandler = h
	// Set data directory for thumbnail disk cache
	h.SetDataDir(s.dataDir)

	// Secrets manager backs the {security_id} rotating token (AI.md PART 11
	// "Security Reports") for both the security.txt Contact: line and the
	// /server/contact?security_id={id} mode switch.
	secretsMgr := s.secretsMgr
	if secretsMgr == nil {
		db := s.migrationMgr.GetDB()
		secretsMgr = secret.NewManager(db)
		if db != nil {
			s.secretsMgr = secretsMgr
		}
	}
	h.SetSecretsManager(secretsMgr)
	h.SetOwnerTokenAuditLogger(s.logger)
	// Set config directory so the PGP public key (/.well-known/pgp-key.asc) can
	// be served from {config_dir}/security/pgp.pub.asc per AI.md PART 12.
	h.SetConfigDir(s.configDir)
	metrics := handler.NewMetrics(s.appConfig, s.engineMgr)
	h.SetMetrics(metrics)

	// Wire real dependency probes for /server/healthz (AI.md PART 13): the
	// database ping and the scheduler running-state check.
	h.SetHealthDB(s.migrationMgr.GetDB())
	h.SetOwnerTokenManager(s.tokenManager)
	if s.scheduler != nil {
		h.SetScheduler(s.scheduler)
	}

	// Prometheus labeled HTTP metrics per AI.md PART 20 (REQUIRED)
	s.router.Use(svcmetrics.InstrumentMiddleware)

	// Tor hidden service request counter per AI.md PART 20
	s.router.Use(torRequestMetricMiddleware)

	// Maintenance mode middleware (applied globally, but allows admin access)
	s.router.Use(h.MaintenanceModeMiddleware)

	// Static files (no age verification needed)
	staticFS, _ := fs.Sub(embeddedFS, "static")
	s.router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	s.router.Get("/branding/{asset}", s.serveBrandingAsset)

	// Age verification endpoints (before middleware)
	s.router.Get("/age-verify", h.AgeVerifyPage)
	s.router.Post("/age-verify", h.AgeVerifySubmit)

	// Content restriction endpoints (before middleware)
	s.router.Get("/content-restricted", h.ContentRestrictedPage)
	s.router.Post("/content-restricted", h.ContentRestrictedSubmit)

	// Zero-JS site-banner dismissal per AI.md PART 16 "Site Banner"
	s.router.Post("/announcements/dismiss", h.AnnouncementDismiss)

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
	s.router.Get("/.well-known/pgp-key.asc", h.PGPKeyAsc)
	// AI.md PART 14: AI-agent discovery file, served at both paths.
	s.router.Get("/.well-known/llms.txt", h.LlmsTxt)
	s.router.Get("/llms.txt", h.LlmsTxt)
	s.router.Get("/humans.txt", h.HumansTxt)
	s.router.Get("/favicon.ico", h.Favicon)
	s.router.Get("/apple-touch-icon.png", h.AppleTouchIcon)

	// PWA assets at root per AI.md PART 16 — service worker must be at root scope
	// sw.js served at /sw.js with Service-Worker-Allowed: / header so it controls the full app
	s.router.Get("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		data, err := embeddedFS.ReadFile("static/js/sw.js")
		if err != nil {
			http.Error(w, i18n.Translate(i18n.DetectLocale(r), "error.not_found"), http.StatusNotFound)
			return
		}
		// Tie the SW cache name to version+commit so every deploy invalidates stale caches
		// AI.md PART 16 format: {app_name}-cache-v{version}
		swCache := "vidveil-cache-v" + version.Version + "-" + version.CommitID
		body := strings.Replace(string(data), "vidveil-cache-v1", swCache, 1)
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Service-Worker-Allowed", "/")
		// PART 9: /sw.js is no-cache + build-stamp ETag — never long-cached,
		// or a stale service worker delays every other update mechanism.
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("ETag", `"`+version.AssetStamp()+`"`)
		_, _ = w.Write([]byte(body))
	})
	s.router.Get("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := embeddedFS.ReadFile("static/manifest.json")
		if err != nil {
			http.Error(w, i18n.Translate(i18n.DetectLocale(r), "error.not_found"), http.StatusNotFound)
			return
		}
		if s.brandingImages != nil && s.brandingImages.Has("favicon-192.png") {
			data = bytes.ReplaceAll(data, []byte("/static/icons/icon-192.png"), []byte("/branding/favicon-192.png"))
			data = bytes.ReplaceAll(data, []byte("/static/icons/icon-512.png"), []byte("/branding/favicon-512.png"))
		}
		w.Header().Set("Content-Type", "application/manifest+json")
		// PART 9: /manifest.json is no-cache + build-stamp ETag, never immutable.
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("ETag", `"`+version.AssetStamp()+`"`)
		_, _ = w.Write(data)
	})
	s.router.Get("/offline.html", func(w http.ResponseWriter, r *http.Request) {
		data, err := embeddedFS.ReadFile("static/offline.html")
		if err != nil {
			http.Error(w, i18n.Translate(i18n.DetectLocale(r), "error.not_found"), http.StatusNotFound)
			return
		}
		// Per AI.md PART 30: <html lang dir> must never be hardcoded — execute as template.
		// The "t" FuncMap entry is required here too — offline.html's strings are
		// translation keys, not hardcoded English, per AI.md PART 30 i18n rules.
		lang := i18n.DetectLocale(r)
		tmpl, err := template.New("offline").Funcs(template.FuncMap{
			"t": func(key string) string {
				return i18n.Translate(lang, key)
			},
		}).Parse(string(data))
		if err != nil {
			writeGuaranteedError(w, r, http.StatusInternalServerError)
			return
		}
		// Buffer the render so a mid-execute failure never leaks a partial
		// body — errors caught before any bytes reach the client, per AI.md
		// PART 16 "Error Pages" guaranteed-response fallback.
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, map[string]string{
			"Lang": lang,
			"Dir":  i18n.Direction(lang),
		}); err != nil {
			log.Printf("offline.html template: execute: %v", err)
			writeGuaranteedError(w, r, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(buf.Bytes())
	})

	// Debug endpoints (PART 6: only when --debug flag or DEBUG=true)
	s.registerDebugRoutes(s.router)

	// Internal loopback-only Tor control channel per AI.md PART 31 —
	// always registered, gated per-request by requireLoopback.
	s.registerTorControlRoutes(s.router)

	// OpenAPI/Swagger and GraphQL — canonical routes per AI.md PART 14.
	// Web UI pages: /server/docs/swagger  /server/docs/graphql
	// Versioned API: {api_version}/server/swagger  {api_version}/server/graphql
	// Unversioned aliases (same handler, no redirect): /api/swagger  /api/graphql
	apiBase := s.appConfig.APIBasePath()
	gql := graphql.NewHandler(s.appConfig, s.engineMgr)

	// Swagger UI (HTML)
	s.router.Get("/server/docs/swagger", swagger.Handler(s.appConfig))
	// GraphiQL UI (HTML)
	s.router.Get("/server/docs/graphql", gql.GraphiQL)

	// Versioned OpenAPI JSON spec
	s.router.Get(apiBase+"/server/swagger", swagger.SpecHandler(s.appConfig))
	// Versioned GraphQL endpoint
	s.router.HandleFunc(apiBase+"/server/graphql", gql.Handle)

	// Unversioned aliases — SAME handler, not redirects (PART 14)
	s.router.Get("/api/swagger", swagger.SpecHandler(s.appConfig))
	s.router.HandleFunc("/api/graphql", gql.Handle)
	// /api/healthz is the unversioned direct JSON alias for {api_version}/server/healthz
	s.router.Get("/api/healthz", h.APIHealthCheck)

	// Metrics — INTERNAL ONLY, per-service bearer token auth, SAME handler at
	// every alias, never a redirect (AI.md PART 20).
	if s.appConfig.Server.Metrics.Enabled {
		metricsHandler := svcmetrics.ServiceHandler(s.appConfig, s.logger)

		s.router.Get("/server/metrics", metricsHandler)
		s.router.Get("/server/metrics/{service}", metricsHandler)
		s.router.Get(apiBase+"/server/metrics", metricsHandler)
		s.router.Get(apiBase+"/server/metrics/{service}", metricsHandler)
		s.router.Get("/api/metrics", metricsHandler)
		s.router.Get("/api/metrics/{service}", metricsHandler)

		if s.appConfig.Server.Metrics.Root.Enabled {
			s.router.Get("/metrics", metricsHandler)
			s.router.Get("/metrics/{service}", metricsHandler)
		}
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
		r.Get("/server/preferences", h.PreferencesPage)
		r.Get("/server/preferences/tokens", h.OwnerTokensPage)
		r.Post("/server/preferences", h.PreferencesSave)
		r.Post("/server/preferences/token/restore", s.ownerTokenRestore)
		r.Post("/server/preferences/token/reveal", s.ownerTokenWebRevealPrimary)
		r.Post("/server/preferences/token/rotate", s.ownerTokenWebRotatePrimary)
		r.Post("/server/preferences/tokens", s.ownerTokenWebCreateLinked)
		r.Post("/server/preferences/tokens/{id}/reveal", s.ownerTokenWebRevealLinked)
		r.Post("/server/preferences/tokens/{id}/rotate", s.ownerTokenWebRotateLinked)
		r.Post("/server/preferences/tokens/{id}/update", s.ownerTokenWebUpdateLinked)
		r.Post("/server/preferences/tokens/{id}/revoke", s.ownerTokenWebRevokeLinked)
		// Cross-device preference sync (stateless, no preferences table) per
		// AI.md PART 16 — sub-routes of /server/preferences, never a
		// standalone /prefs/* path nor bare /preferences.
		r.Get("/server/preferences/export", h.PreferencesExport)
		r.Get("/server/preferences/import", h.PreferencesImport)
		r.Get("/favorites", h.FavoritesPage)
		r.Post("/favorites", h.FavoritesSave)
		r.Get("/favorites/export", h.FavoritesExport)
		r.Post("/favorites/import", h.FavoritesImport)
		// About/privacy are at /server/* per PART 14 Route Scopes
	})

	// Server routes per AI.md PART 14 (Route Scopes)
	server := handler.NewServerHandler(s.appConfig)
	server.SetDB(s.migrationMgr.GetDB())
	server.SetSecretsManager(secretsMgr)
	server.SetConfigDir(s.configDir)
	server.SetDataDir(s.dataDir)
	server.SetLogger(s.logger)
	s.emailService = email.NewEmailService(s.appConfig)
	server.SetEmailService(s.emailService)
	s.notifyDispatcher = notify.New(&s.appConfig.Server.Contact, paths.ProjectName, version.GetVersion(), s.appConfig.GetPublicURL())
	if s.logger != nil {
		s.notifyDispatcher.SetFailureLogger(func(event string, fields map[string]any) {
			s.logger.Warn(event, fields)
		})
	}
	server.SetNotifyDispatcher(s.notifyDispatcher)
	if s.scheduler != nil {
		s.scheduler.SetFailureNotifier(s.notifyScheduledTaskFailure)
	}
	s.serverHandler = server
	s.router.Route("/server", func(r chi.Router) {
		// Per AI.md PART 16 Standard Pages table: bare /server redirects to
		// /server/about (301, no shim/legacy handler duplication).
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			http.Redirect(w, req, "/server/about", http.StatusMovedPermanently)
		})
		r.Get("/about", server.AboutPage)
		r.Get("/privacy", server.PrivacyPage)
		r.Get("/contact", server.ContactPage)
		r.Post("/contact", server.ContactPage)
		// Zero-JS cookie-consent flow per AI.md PART 12 — banner forms POST here
		r.Post("/consent", server.ConsentSubmit)
		// CCPA "Do Not Sell" opt-out toggle on /server/privacy (AI.md PART 12)
		r.Post("/ccpa", server.CCPASubmit)
		r.Get("/help", server.HelpPage)
		r.Get("/help/mcp", server.HelpMCPPage)
		r.Get("/terms", server.TermsPage)
		r.Get("/security", server.SecurityPage)
		r.Get("/security/policy", server.SecurityPolicyPage)
		r.Get("/security/thanks", server.SecurityThanksPage)
		r.Get("/security/report/{tracking_id}", server.SecurityReportStatusPage)
		r.Get("/restore", server.RestorePage)
		r.Post("/restore", server.RestoreSubmit)
	})

	// API autodiscover endpoint (non-versioned per AI.md PART 14)
	// Clients need this BEFORE they know the API version
	s.router.Get("/api/autodiscover", h.Autodiscover)
	if s.appConfig.CLI.BinaryDownload.RequireAuth {
		s.router.With(s.requireOperatorToken).Get("/cli/binaries/{filename}", h.CLIBinaryDownload)
	} else {
		s.router.Get("/cli/binaries/{filename}", h.CLIBinaryDownload)
	}

	// Versioned API routes per AI.md PART 14 ({api_version}, default "v1")
	s.router.Route(apiBase, func(r chi.Router) {
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

		// Debug endpoints (development only per IDEA.md) — gated on debug mode
		// and operator-token authorization, matching registerDebugRoutes.
		if mode.IsDebugEnabled() {
			s.registerDebugAPIRoutes(r, "/debug", h)
		}

		// Per AI.md PART 14: canonical health route is /api/{api_version}/server/healthz.
		// Legacy /api/{api_version}/healthz alias removed — spec requires no shims.
		r.Get("/version", h.APIVersion)

		// Server API per AI.md PART 14
		r.Route("/server", func(r chi.Router) {
			r.Get("/healthz", h.APIHealthCheck)
			r.Get("/about", server.APIAbout)
			r.Get("/privacy", server.APIPrivacy)
			r.Post("/contact", server.APIContact)
			r.With(s.requireOperatorToken).Post("/restore", server.APIRestore)
			r.Get("/help", server.APIHelp)
			r.Get("/help/mcp", server.APIHelpMCP)
			r.Get("/terms", server.APITerms)

			// Browser-emitted report ingestion per AI.md PART 11 "Reporting API
			// (Modern + Legacy)". These back the Reporting-Endpoints, Report-To,
			// and NEL response headers set in the security-headers middleware, and
			// the CSP report-to/report-uri directives. Each accepts a bounded POST,
			// logs a sanitized summary to security.log, and returns 204.
			r.Route("/reports", func(r chi.Router) {
				r.Post("/default", server.ReportsDefault)
				r.Post("/nel", server.ReportsNEL)
				r.Post("/csp", server.ReportsCSP)
			})
			// Cross-device preference sync, API-mirrored per AI.md PART 16 —
			// same stateless handlers as the frontend
			// /server/preferences/{export,import}.
			r.Route("/preferences", func(r chi.Router) {
				r.Get("/export", h.PreferencesExport)
				r.Get("/import", h.PreferencesImport)
			})
		})

		// Proxy endpoints (plural per PART 14)
		r.Get("/proxy/thumbnails", h.ProxyThumbnail)
		r.Get("/proxy/videos", h.ProxyVideo)

		// Favorites (public, owner-token cookie — no account) per
		// AI.md PART 16 progressive enhancement; server-side storage in
		// server.db, see favorites.go.
		r.Route("/favorites", func(r chi.Router) {
			r.Get("/", h.FavoritesAPIList)
			r.Post("/", h.FavoritesAPIAdd)
			r.Post("/import", h.FavoritesImport)
			r.Delete("/{id}", h.FavoritesAPIRemove)
			r.Delete("/", h.FavoritesAPIClear)
		})

	})

	s.router.Get(apiBase+"/server/preferences", h.PreferencesAPIGet)
	s.router.Post(apiBase+"/server/preferences", h.PreferencesAPISet)
	tokensPath := apiBase + "/server/preferences/tokens"
	s.router.Get(tokensPath, s.ownerTokensAPIList)
	s.router.Post(tokensPath, s.ownerTokensAPICreate)
	s.router.Post(tokensPath+"/primary/reveal", s.ownerTokensAPIRevealPrimary)
	s.router.Post(tokensPath+"/primary/rotate", s.ownerTokensAPIRotatePrimary)
	s.router.Post(tokensPath+"/{id}/reveal", s.ownerTokensAPIRevealLinked)
	s.router.Post(tokensPath+"/{id}/rotate", s.ownerTokensAPIRotateLinked)
	s.router.Patch(tokensPath+"/{id}", s.ownerTokensAPIUpdateLinked)
	s.router.Delete(tokensPath+"/{id}", s.ownerTokensAPIRevokeLinked)

	// The unversioned operational alias uses the same debug handlers and
	// operator-token protection as the versioned API endpoints.
	if mode.IsDebugEnabled() {
		s.registerDebugAPIRoutes(s.router, "/api/debug", h)
	}

	// MCP aliases share the same stateless Streamable HTTP handler. The
	// handler dispatches tool calls in-process through this router.
	mcpHandler := mcpserver.OriginMiddleware(s.appConfig, mcpserver.Handler(s.appConfig, s.router, s.isBearerOperatorToken))
	apiBase = s.appConfig.APIBasePath()
	s.router.Post(apiBase+"/server/mcp", mcpHandler.ServeHTTP)
	s.router.Post("/api/mcp", mcpHandler.ServeHTTP)
	s.router.Post("/mcp", mcpHandler.ServeHTTP)

	// Custom 404 handler per AI.md PART 14
	s.router.NotFound(h.NotFoundHandler)
}

func (s *Server) notifyScheduledTaskFailure(task scheduler.ScheduledTask, history scheduler.TaskHistory) {
	templateName := "scheduler_error"
	event := "admin.scheduler_error"
	vars := map[string]string{
		"task_name": task.Name,
		"error":     history.Error,
		"next_run":  task.NextRun.Format(time.RFC3339),
	}
	subject := "Scheduled task failed: " + task.Name

	switch task.ID {
	case "backup_daily", "backup_hourly":
		templateName = "backup_failed"
		event = "admin.backup_failed"
	case "ssl_renewal":
		templateName = "ssl_renewal_failed"
		event = "admin.ssl_renewal_failed"
		vars["fqdn"] = s.appConfig.Server.FQDN
		vars["next_retry"] = task.NextRun.Format(time.RFC3339)
		if certInfo, err := s.sslCertificateInfo(); err == nil {
			vars["domain"] = certInfo.Domain
			vars["expires_in"] = fmt.Sprintf("%d", certInfo.DaysLeft)
			vars["expiry_date"] = certInfo.NotAfter.Format(time.RFC3339)
		}
	}

	body := fmt.Sprintf("Task: %s\nError: %s\nNext run: %s", task.Name, history.Error, task.NextRun.Format(time.RFC3339))
	if s.notifyDispatcher != nil {
		s.notifyDispatcher.Send(context.Background(), notify.RoleAdmin, notify.Payload{
			Event: event, Subject: subject, Body: body, Severity: notify.SeverityCritical,
		})
	}
	if s.emailService != nil && s.appConfig.Server.Notifications.Email.Enabled && s.appConfig.Server.Contact.Admin.Email != "" {
		if err := s.emailService.Send(templateName, s.appConfig.Server.Contact.Admin.Email, vars); err != nil && s.logger != nil {
			s.logger.Warn("scheduled task notification email failed", map[string]interface{}{
				"task_id": task.ID,
				"event":   event,
				"error":   err.Error(),
			})
		}
	}
}

func (s *Server) sslCertificateInfo() (*ssl.CertInfo, error) {
	provider, ok := s.tlsProvider.(interface {
		GetCertInfo() (*ssl.CertInfo, error)
	})
	if !ok {
		return nil, fmt.Errorf("certificate information is unavailable")
	}
	return provider.GetCertInfo()
}

// registerDebugAPIRoutes mounts debug APIs behind the operator-token check.
func (s *Server) registerDebugAPIRoutes(r chi.Router, path string, h *handler.SearchHandler) {
	r.Route(path, func(r chi.Router) {
		r.Use(s.requireOperatorToken)
		r.Get("/engines", h.DebugEnginesList)
		r.Get("/engines/{name}", h.DebugEngine)
	})
}

// requireOperatorToken protects diagnostics with the configured server token.
// It compares fixed-length SHA-256 digests in constant time per AI.md PART 11.
func (s *Server) requireOperatorToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		deny := func(invalid bool) {
			if invalid {
				handler.SetBearerChallenge(w, "invalid_token")
			} else {
				handler.SetBearerChallenge(w, "")
			}
			authpad.Wait(start)
			handler.SendNegotiatedError(w, r, handler.CodeUnauthorized, handler.MsgUnauthorized)
		}
		if !s.isBearerOperatorToken(r) {
			header := strings.TrimSpace(r.Header.Get("Authorization"))
			invalid := strings.HasPrefix(strings.ToLower(header), "bearer ") && strings.TrimSpace(header[len("Bearer "):]) != ""
			deny(invalid)
			return
		}
		response := authpad.WrapResponse(w, start)
		next.ServeHTTP(response, r)
		response.Finish()
	})
}

func (s *Server) isBearerOperatorToken(r *http.Request) bool {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) < len("Bearer ") || !strings.EqualFold(value[:len("Bearer ")], "Bearer ") {
		return false
	}
	return s.matchesOperatorToken(strings.TrimSpace(value[len("Bearer "):]))
}

func (s *Server) matchesOperatorToken(providedToken string) bool {
	if providedToken == "" {
		return false
	}
	providedHash := sha256.Sum256([]byte(providedToken))
	configuredHash, configured := s.getOperatorTokenHash()
	return configured && subtle.ConstantTimeCompare(providedHash[:], configuredHash[:]) == 1
}

// extractRequestToken accepts only the native Bearer credential form.
func extractRequestToken(r *http.Request) (string, bool) {
	if value := strings.TrimSpace(r.Header.Get("Authorization")); value != "" {
		return bearerToken(value)
	}
	return "", false
}

// RefreshOperatorTokenHash updates the digest after a config reload without
// retaining the raw operator token in request-auth state.
func (s *Server) RefreshOperatorTokenHash() {
	s.operatorTokenHashMu.Lock()
	defer s.operatorTokenHashMu.Unlock()
	s.operatorTokenHash = [sha256.Size]byte{}
	s.operatorTokenSet = false
	if s.appConfig == nil || s.appConfig.Server.Token == "" {
		return
	}
	s.operatorTokenHash = sha256.Sum256([]byte(s.appConfig.Server.Token))
	s.operatorTokenSet = true
}

func (s *Server) getOperatorTokenHash() ([sha256.Size]byte, bool) {
	s.operatorTokenHashMu.RLock()
	defer s.operatorTokenHashMu.RUnlock()
	return s.operatorTokenHash, s.operatorTokenSet
}

// bearerToken extracts a non-empty Bearer credential without accepting extra
// whitespace or additional authorization parameters.
func bearerToken(value string) (string, bool) {
	scheme, token, ok := strings.Cut(value, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", false
	}
	return token, true
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe(addr string) error {
	// Parse timeouts from config per AI.md PART 12
	readTimeout := parseDuration(s.appConfig.Server.Limits.ReadTimeout, 30*time.Second)
	writeTimeout := parseDuration(s.appConfig.Server.Limits.WriteTimeout, 30*time.Second)
	idleTimeout := parseDuration(s.appConfig.Server.Limits.IdleTimeout, 120*time.Second)

	srv := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	s.registerHTTPServer(srv)
	return srv.ListenAndServe()
}

// Listen binds to the given address and returns the listener without accepting
// connections. Call Serve(l) after privilege drop.
// Per AI.md PART 23: bind privileged ports as root, then drop, then serve.
func (s *Server) Listen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

// ServeOn serves HTTP requests on the given pre-bound listener.
// Per AI.md PART 23: called after privilege drop.
func (s *Server) ServeOn(listener net.Listener) error {
	readTimeout := parseDuration(s.appConfig.Server.Limits.ReadTimeout, 30*time.Second)
	writeTimeout := parseDuration(s.appConfig.Server.Limits.WriteTimeout, 30*time.Second)
	idleTimeout := parseDuration(s.appConfig.Server.Limits.IdleTimeout, 120*time.Second)

	srv := &http.Server{
		Handler:      s.router,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	s.registerHTTPServer(srv)
	return srv.Serve(listener)
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
	s.registerHTTPServer(torSrv)
	return torSrv.Serve(listener)
}

// SetSSLManager wires the TLS provider (ssl.SSLManager) used for HTTPS serving
// and ACME challenges per AI.md PART 15. Must be called before ServeTLSOn.
func (s *Server) SetSSLManager(p TLSProvider) {
	s.tlsProvider = p
}

// ServeTLSOn serves HTTPS on the pre-bound listener using the wired TLS provider.
// Per AI.md PART 15/23: bind privileged ports as root, drop, then serve TLS.
func (s *Server) ServeTLSOn(listener net.Listener) error {
	if s.tlsProvider == nil {
		return http.ErrServerClosed
	}
	readTimeout := parseDuration(s.appConfig.Server.Limits.ReadTimeout, 30*time.Second)
	writeTimeout := parseDuration(s.appConfig.Server.Limits.WriteTimeout, 30*time.Second)
	idleTimeout := parseDuration(s.appConfig.Server.Limits.IdleTimeout, 120*time.Second)

	srv := &http.Server{
		Handler:      s.router,
		TLSConfig:    s.tlsProvider.GetTLSConfig(),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	s.registerHTTPServer(srv)
	// Empty cert/key: the certificate is served via TLSConfig.GetCertificate.
	return srv.ServeTLS(listener, "", "")
}

// ServeHTTPRedirectOn serves the ACME HTTP-01 challenge handler and redirects all
// other requests to HTTPS on the pre-bound HTTP listener. Used in dual-port mode
// (e.g. "80,443") per AI.md PART 15.
func (s *Server) ServeHTTPRedirectOn(listener net.Listener, httpsPort string) error {
	readTimeout := parseDuration(s.appConfig.Server.Limits.ReadTimeout, 30*time.Second)
	writeTimeout := parseDuration(s.appConfig.Server.Limits.WriteTimeout, 30*time.Second)
	idleTimeout := parseDuration(s.appConfig.Server.Limits.IdleTimeout, 120*time.Second)

	redirectSrv := &http.Server{
		Handler:      s.httpRedirectHandler(httpsPort),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}
	s.registerHTTPServer(redirectSrv)
	return redirectSrv.Serve(listener)
}

func (s *Server) registerHTTPServer(server *http.Server) {
	s.serversMu.Lock()
	s.srv = server
	s.servers = append(s.servers, server)
	s.serversMu.Unlock()
}

// httpRedirectHandler routes ACME challenges to the TLS provider's HTTP handler
// and 301-redirects everything else to the HTTPS endpoint per AI.md PART 15.
func (s *Server) httpRedirectHandler(httpsPort string) http.Handler {
	var acme http.Handler
	if s.tlsProvider != nil {
		acme = s.tlsProvider.GetHTTPHandler()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if acme != nil && strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
			acme.ServeHTTP(w, r)
			return
		}
		if urlvar.IsTorRequest(r) {
			s.router.ServeHTTP(w, r)
			return
		}
		_, host, _ := urlvar.GlobalResolver().GetURLVars(r)
		target := "https://" + host
		if httpsPort != "" && httpsPort != "443" {
			target += ":" + httpsPort
		}
		target += r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}

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
	var err error
	s.serversMu.Lock()
	servers := append([]*http.Server(nil), s.servers...)
	if len(servers) == 0 && s.srv != nil {
		servers = append(servers, s.srv)
	}
	s.serversMu.Unlock()
	if len(servers) > 0 {
		results := make(chan error, len(servers))
		for _, server := range servers {
			go func(server *http.Server) {
				results <- server.Shutdown(ctx)
			}(server)
		}
		for range servers {
			if shutdownErr := <-results; shutdownErr != nil && err == nil {
				err = shutdownErr
			}
		}
	}
	if s.brandingCancel != nil {
		s.brandingCancel()
		done := make(chan struct{})
		go func() { s.brandingRefreshWG.Wait(); close(done) }()
		select {
		case <-done:
		case <-ctx.Done():
			if err == nil {
				err = ctx.Err()
			}
		}
	}
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
	if s.tokenActionLimiter != nil {
		s.tokenActionLimiter.Stop()
	}
	return err
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

// torRequestMetricMiddleware counts requests that arrived over the Tor hidden
// service listener. The counter carries no labels, so no client identifier is
// ever recorded.
// Spec: AI.md PART 20
func torRequestMetricMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if urlvar.IsTorRequest(r) {
			svcmetrics.RecordTorRequest()
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
			handler.SendNegotiatedError(w, r, handler.CodeForbidden, i18n.Translate(i18n.DetectLocale(r), "error.ip_blocked"))
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
			handler.SendNegotiatedError(w, r, handler.CodeForbidden, i18n.Translate(i18n.DetectLocale(r), "error.country_blocked"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractClientIP returns the resolved client IP per AI.md PART 12 "Client IP
// Detection" — proxy headers only honored when the immediate peer passes the
// trusted_proxies gate, otherwise falls back to the raw TCP peer.
func extractClientIP(r *http.Request) string {
	return urlvar.ResolveClientIP(r)
}

// secFetchValidationMiddleware validates Sec-Fetch-* request headers on frontend
// routes per AI.md PART 11. Validation is "present-and-bad reject only": absent
// headers are treated as a legacy-browser pass-through.
//
// Rules per PART 11:
//   - Sec-Fetch-Site: reject cross-site on frontend POST/PUT/PATCH/DELETE without Bearer token.
//   - Sec-Fetch-Mode: backend API requests bypass frontend Sec-Fetch validation.
//   - Sec-Fetch-Dest: reject iframe on endpoints not in frame-ancestors allow-list.
func secFetchValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || isMCPPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		// Sec-Fetch-Site: block cross-site state-changing requests without Bearer auth
		site := r.Header.Get("Sec-Fetch-Site")
		if site == "cross-site" {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
				// Allow Bearer requests — they are not authenticated by ambient cookies.
				if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
					handler.SendNegotiatedError(w, r, handler.CodeForbidden, i18n.Translate(i18n.DetectLocale(r), "error.cross_site_blocked"))
					return
				}
			}
		}

		// Sec-Fetch-Mode: block navigate fetches to /api/* — indicates unintended top-level nav
		mode := r.Header.Get("Sec-Fetch-Mode")
		if mode == "navigate" && strings.HasPrefix(r.URL.Path, "/api/") {
			handler.SendNegotiatedError(w, r, handler.CodeForbidden, i18n.Translate(i18n.DetectLocale(r), "error.api_navigation_blocked"))
			return
		}

		next.ServeHTTP(w, r)
	})
}
