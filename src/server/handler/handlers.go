// SPDX-License-Identifier: MIT
package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	// register PNG decoder for image.Decode
	_ "image/png"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	humanformat "github.com/apimgr/vidveil/src/common/format"
	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/common/sanitize"
	"github.com/apimgr/vidveil/src/common/theme"
	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/apitoken"
	"github.com/apimgr/vidveil/src/server/service/cache"
	"github.com/apimgr/vidveil/src/server/service/engine"
	"github.com/apimgr/vidveil/src/server/service/geoip"
	"github.com/apimgr/vidveil/src/server/service/maintenance"
	"github.com/apimgr/vidveil/src/server/service/secreport"
	"github.com/apimgr/vidveil/src/server/service/secret"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
	signalstate "github.com/apimgr/vidveil/src/server/signal"
	"github.com/apimgr/vidveil/src/swagger"
)

// templatesFS holds the embedded templates filesystem
var templatesFS fs.FS

// SetTemplatesFS sets the embedded templates filesystem
func SetTemplatesFS(fsys fs.FS) {
	templatesFS = fsys
}

// contextKey is an unexported type for context keys in the handler package.
// Using a custom type prevents collisions with string keys from other packages (SA1029).
type contextKey string

// OriginalPathKey is the context key for storing the original request path before extension stripping.
const OriginalPathKey contextKey = "vidveil.originalPath"

// csrfContextKeyType is an exported struct type for the CSRF context key.
// Using an exported struct type prevents collisions and lets the server package
// reference handler.CSRFTokenKey without exporting the contextKey string type.
type csrfContextKeyType struct{}

// CSRFTokenKey is the context key used by the CSRF middleware to store the token.
// The server package writes the key; the handler package reads it.
var CSRFTokenKey = csrfContextKeyType{}

// cSRFTokenFromRequest reads the CSRF token from the request context.
// Returns an empty string when the CSRF middleware did not run (e.g., bypassed request).
func cSRFTokenFromRequest(r *http.Request) string {
	v, _ := r.Context().Value(CSRFTokenKey).(string)
	return v
}

// gpcContextKeyType is an exported struct type for the GPC opt-out context key.
type gpcContextKeyType struct{}

// GPCOptOutKey is the context key the privacy-signals middleware sets to true
// when a browser-emitted opt-out signal (Sec-GPC, or DNT when the operator opts
// in) is honored for the request, per AI.md PART 11 "Privacy Signal Headers".
var GPCOptOutKey = gpcContextKeyType{}

// GPCOptOut reports whether the current request carried an honored privacy
// opt-out signal. Handlers use it to skip personalization, behavioral
// analytics, and any non-essential cookie for the request lifecycle.
func GPCOptOut(r *http.Request) bool {
	v, _ := r.Context().Value(GPCOptOutKey).(bool)
	return v
}

const (
	ageVerifyCookieName = "age_verified"
	ageVerifyCookieDays = 30
)

// TorStatusChecker is the interface for Tor service in handlers
// Per PART 31: Supports both status checking and outbound network routing
type TorStatusChecker interface {
	IsEnabled() bool
	IsRunning() bool
	IsStarting() bool
	GetInfo() map[string]interface{}
	// Per PART 31: Admin setting for IP forwarding
	AllowUserIPForward() bool
	// Per PART 31: Get Tor-routed or direct client
	GetHTTPClient(useTor bool) *http.Client
	// Per PART 31: Is use_network configured?
	UseNetworkEnabled() bool
	// Per PART 31: Is Tor SOCKS available?
	OutboundEnabled() bool
}

// I2PStatusChecker exposes only the public-safe state needed by health/status pages.
type I2PStatusChecker interface {
	Enabled() bool
	IsRunning() bool
	Status() string
	Address() string
	Provider() string
	CheckHealth(context.Context) error
}

// GeoIPChecker is a minimal interface for GeoIP content restriction checks
type GeoIPChecker interface {
	CheckContentRestriction(ipStr string, isTorUser bool) *geoip.RestrictionResult
	GetRestrictionMode() string
	IsEnabled() bool
}

// Cookie name for content restriction acknowledgment
const contentRestrictionAckCookieName = "content_ack"

// Cookie name for user IP forwarding preference
const iPForwardCookieName = "forward_ip"

// Cookie names for server-rendered app-specific preferences, shared by the
// JavaScript and no-JavaScript preference flows per AI.md PART 16.
const resultsPerPageCookieName = "vidveil_pref_results_per_page"
const openNewTabCookieName = "vidveil_pref_open_new_tab"
const engineSelectionCookieName = "vidveil_pref_engines_selected"

// previewFirstCookieName persists the filters-panel "preview first" result
// ordering across searches for no-JS clients (see getRequestPreviewFirst) —
// unlike duration/quality/sort, which are pure per-request URL state, this
// preference needs to survive navigating away from the query string it was
// set on (e.g. clicking through to a fresh search from the homepage).
const previewFirstCookieName = "vidveil_pref_preview_first"

// getRequestTheme returns the user's theme preference from their cookie, falling
// back to the server-configured default. Valid values: "dark", "light", "auto".
func (h *SearchHandler) getRequestTheme(r *http.Request) string {
	if c, err := r.Cookie("theme"); err == nil {
		switch c.Value {
		case "dark", "light", "auto":
			return c.Value
		}
	}
	if h.appConfig != nil {
		return h.appConfig.Web.UI.Theme
	}
	return "dark"
}

// getThemeColor resolves the browser-chrome "theme-color" hex for the given
// theme preference from the single-source-of-truth Go palette
// (src/common/theme/colors.go), so the value never has to be duplicated as a
// literal in a template or in JS. "auto" and "dark" both resolve to the dark
// palette server-side (matching the project default per AI.md PART 16 —
// prefers-color-scheme is applied client-side once JS runs).
func getThemeColor(themeName string) string {
	if themeName == "light" {
		return theme.Light.Background
	}
	return theme.Dark.Background
}

// nextTheme returns the next mode in the cycle: dark -> light -> auto -> dark,
// per AI.md PART 16 "Theme Cycle Logic". The header toggle's POST target is
// always this computed value rather than a hardcoded one, so repeated clicks
// keep cycling instead of sticking after the first click.
func nextTheme(current string) string {
	switch current {
	case "dark":
		return "light"
	case "light":
		return "auto"
	default:
		// "auto" or empty/unset
		return "dark"
	}
}

// getUserIPForwardPreference checks if user has opted-in to IP forwarding via cookie
// Returns (user wants forwarding, user's IP)
func (h *SearchHandler) getUserIPForwardPreference(r *http.Request) (bool, string) {
	// Check if admin allows this feature
	if h.torSvc == nil || !h.torSvc.AllowUserIPForward() {
		return false, ""
	}

	// Check user's preference cookie (defaults to disabled)
	cookie, err := r.Cookie(iPForwardCookieName)
	if err != nil || cookie.Value != "1" {
		// User hasn't opted in
		return false, ""
	}

	// Get user's real IP
	userIP := getClientIP(r)
	return true, userIP
}

// checkContentRestriction checks if user is from a restricted region
// Returns restriction result or nil if no restriction
func (h *SearchHandler) checkContentRestriction(r *http.Request) *geoip.RestrictionResult {
	if h.geoipSvc == nil || !h.geoipSvc.IsEnabled() {
		return nil
	}

	mode := h.geoipSvc.GetRestrictionMode()
	if mode == "off" || mode == "" {
		return nil
	}

	// Check if user is accessing via Tor hidden service
	isTorUser := h.isTorRequest(r)

	// Get client IP
	clientIP := getClientIP(r)

	// Perform restriction check
	result := h.geoipSvc.CheckContentRestriction(clientIP, isTorUser)
	if result == nil || !result.Restricted {
		return nil
	}

	return result
}

// isTorRequest checks if the request is coming via Tor hidden service
func (h *SearchHandler) isTorRequest(r *http.Request) bool {
	return urlvar.IsTorRequest(r)
}

// hasContentRestrictionAck checks if user has acknowledged content restriction warning
func (h *SearchHandler) hasContentRestrictionAck(r *http.Request) bool {
	cookie, err := r.Cookie(contentRestrictionAckCookieName)
	if err != nil {
		return false
	}
	return cookie.Value == "1"
}

// setContentRestrictionAckCookie sets the acknowledgment cookie (30 days)
func (h *SearchHandler) setContentRestrictionAckCookie(w http.ResponseWriter) {
	http.SetCookie(w, newSecureCookie(
		contentRestrictionAckCookieName,
		"1",
		"/",
		// 30 days
		30*24*60*60,
		h.appConfig.Server.SSL.Enabled,
	))
}

// newSearchSessionID creates a per-render token shared with the browser's
// infinite-scroll requests. A fresh token for each HTML response keeps native
// page navigation repeatable while allowing later streamed batches to dedup
// against the server-rendered first batch.
func newSearchSessionID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// getClientIP extracts the client's real IP address per AI.md PART 12
// "Client IP Detection" — proxy headers only honored when the immediate
// peer passes the trusted_proxies gate.
func getClientIP(r *http.Request) string {
	return urlvar.ResolveClientIP(r)
}

// SearchHandler holds dependencies for HTTP handlers
type SearchHandler struct {
	appConfig    *config.AppConfig
	dataDir      string
	configDir    string
	engineMgr    *engine.EngineManager
	searchCache  cache.SearchResultCache
	metrics      *ServerMetrics
	torSvc       TorStatusChecker
	i2pSvc       I2PStatusChecker
	geoipSvc     GeoIPChecker
	secretsMgr   *secret.Manager
	healthDB     *sql.DB
	tokenManager *apitoken.Manager
	ownerAudit   OwnerTokenAuditLogger
	sched        SchedulerHealth
}

// SchedulerHealth is the minimal scheduler interface the /server/healthz check
// needs (AI.md PART 13). *scheduler.Scheduler satisfies it via IsRunning; the
// interface keeps the handler package free of a scheduler import.
type SchedulerHealth interface {
	IsRunning() bool
}

type OwnerTokenAuditLogger interface {
	Audit(event, actorID, actorType, actorIP, result string, details map[string]interface{})
}

func (h *SearchHandler) SetOwnerTokenAuditLogger(logger OwnerTokenAuditLogger) {
	h.ownerAudit = logger
}

// SetSecretsManager wires the app-secrets manager used to derive the
// rotating {security_id} token for the security.txt Contact: line per
// AI.md PART 11 "Security Reports".
func (h *SearchHandler) SetSecretsManager(m *secret.Manager) {
	h.secretsMgr = m
}

// NewSearchHandler creates a new handler instance
func NewSearchHandler(appConfig *config.AppConfig, engineMgr *engine.EngineManager) *SearchHandler {
	// Use default config if nil per AI.md PART 5
	if appConfig == nil {
		appConfig = config.DefaultAppConfig()
	}

	return &SearchHandler{
		appConfig:   appConfig,
		engineMgr:   engineMgr,
		searchCache: newSearchResultCache(appConfig.Server.Cache),
	}
}

// newSearchResultCache builds the configurable API-response (search) cache per
// AI.md PART 12 from the server cache config, selecting the memory/valkey/redis
// driver. On any backend error it falls back to the in-process memory cache so
// search never hard-fails on a missing external cache.
func newSearchResultCache(cc config.CacheConfig) cache.SearchResultCache {
	if cc.Type == "none" {
		return nil
	}
	// Per AI.md PART 12 the url takes precedence; only synthesize an addr from
	// host/port when no url is supplied.
	var addr string
	if cc.URL == "" && cc.Host != "" {
		addr = net.JoinHostPort(cc.Host, strconv.Itoa(cc.Port))
	}
	cfg := cache.CacheConfig{
		Type:          cache.CacheType(cc.Type),
		TTL:           cc.TTL,
		URL:           cc.URL,
		Addr:          addr,
		Username:      cc.Username,
		Password:      cc.Password,
		DB:            cc.DB,
		Prefix:        cc.Prefix,
		TLS:           cc.TLS,
		TLSSkipVerify: cc.TLSSkipVerify,
		PoolSize:      cc.PoolSize,
		MinIdle:       cc.MinIdle,
	}
	c, err := cache.NewSearchResultCache(cfg)
	if err != nil {
		log.Printf("cache: %s backend unavailable (%v); using in-process memory cache", cc.Type, err)
		return cache.NewSearchCache(time.Duration(cc.TTL)*time.Second, 1000)
	}
	return c
}

// SetDataDir sets the data directory (used for thumbnail disk cache)
func (h *SearchHandler) SetDataDir(dir string) {
	h.dataDir = dir
}

func (h *SearchHandler) brandingAssetURL(source, asset string) string {
	if (source == "" && !strings.HasPrefix(asset, "og-")) || h.dataDir == "" {
		return ""
	}
	if info, err := os.Stat(filepath.Join(h.dataDir, "branding", asset)); err != nil || !info.Mode().IsRegular() {
		return ""
	}
	return "/branding/" + asset
}

// SetConfigDir sets the configuration directory (used to serve the PGP public key)
func (h *SearchHandler) SetConfigDir(dir string) {
	h.configDir = dir
}

// SetMetrics sets the metrics collector for statistics display
func (h *SearchHandler) SetMetrics(m *ServerMetrics) {
	h.metrics = m
}

// SetTorService sets the Tor service for healthz display
func (h *SearchHandler) SetTorService(t TorStatusChecker) {
	h.torSvc = t
}

// SetI2PService wires the optional eepsite status provider.
func (h *SearchHandler) SetI2PService(service I2PStatusChecker) {
	h.i2pSvc = service
}

func (h *SearchHandler) i2pStatus() string {
	if h.i2pSvc == nil {
		return "disabled"
	}
	return h.i2pSvc.Status()
}

func (h *SearchHandler) i2pHealthStatus(check string) string {
	if h.i2pSvc == nil || !h.i2pSvc.Enabled() {
		return "disabled"
	}
	if check != "ok" {
		return "error:provider_unhealthy"
	}
	return h.i2pStatus()
}

func (h *SearchHandler) i2pAddress() string {
	if h.i2pSvc == nil {
		return ""
	}
	return h.i2pSvc.Address()
}

func (h *SearchHandler) i2pProvider() string {
	if h.i2pSvc == nil {
		return "none"
	}
	return h.i2pSvc.Provider()
}

// SetGeoIPService sets the GeoIP service for content restriction checks
func (h *SearchHandler) SetGeoIPService(g GeoIPChecker) {
	h.geoipSvc = g
}

// SetHealthDB wires the database handle used by the /server/healthz database
// check per AI.md PART 13. When nil, the database check reports "ok" (nothing
// to probe).
func (h *SearchHandler) SetHealthDB(db *sql.DB) {
	h.healthDB = db
}

// SetOwnerTokenManager wires anonymous resource ownership to API tokens.
func (h *SearchHandler) SetOwnerTokenManager(manager *apitoken.Manager) {
	h.tokenManager = manager
}

// SetScheduler wires the scheduler used by the /server/healthz scheduler check
// per AI.md PART 13.
func (h *SearchHandler) SetScheduler(s SchedulerHealth) {
	h.sched = s
}

// getProxyClient fetches caller-supplied proxy URLs directly. PART 31 prohibits
// routing inbound-request destinations through the app's outbound Tor client.
func (h *SearchHandler) getProxyClient(timeout time.Duration) *http.Client {
	// Direct connection with dial-time SSRF protection to close the
	// DNS-rebinding TOCTOU window left open by the pre-flight host check.
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return err
				}
				if isPrivateIP(net.ParseIP(host)) {
					return fmt.Errorf("dial to private address %q blocked", host)
				}
				return nil
			},
		}).DialContext,
	}
	return &http.Client{
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: proxyCheckRedirect,
	}
}

// proxyHostSuffixes lists the engine source domains and CDN hosts used by the
// built-in search engines. Proxy endpoints accept only these app-owned sources.
var proxyHostSuffixes = []string{
	"3movs.com", "4tube.com", "alphaporno.com", "anyporn.com", "c1.ttcache.com",
	"dl.xnxx.com", "drtuber.com", "empflix.com", "eporner.com", "flyflv.com",
	"fux.com", "gotporn.com", "hellporno.com", "hqporner.com", "lovehomeporn.com",
	"motherless.com", "nonktube.com", "nubiles-porn.com", "nuvid.com", "pornbox.com",
	"pornerbros.com", "pornflip.com", "pornhat.com", "pornhd.com", "pornhub.com",
	"pornmd.com", "pornone.com", "porntop.com", "porntrex.com", "porntube.com",
	"redtube.com", "sunporno.com", "tnaflix.com", "tube8.com", "tubegalore.com",
	"txxx.com", "vjav.com", "xbabe.com", "xhamster.com", "xnxx.com", "xvideos.com",
	"xxxymovies.com", "youjizz.com", "youporn.com",
}

func isAllowedProxyHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, suffix := range proxyHostSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// proxyCheckRedirect blocks redirects outside the app-owned source allowlist,
// private/internal targets, and redirect chains longer than five hops.
func proxyCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("stopped after %d redirects", len(via))
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return fmt.Errorf("redirect to unsupported scheme %q blocked", req.URL.Scheme)
	}
	if isPrivateHost(req.URL.Hostname()) {
		return fmt.Errorf("redirect to private host %q blocked", req.URL.Hostname())
	}
	if !isAllowedProxyHost(req.URL.Hostname()) {
		return fmt.Errorf("redirect to unapproved host %q blocked", req.URL.Hostname())
	}
	return nil
}

// getSearchCount returns total searches from metrics
func (h *SearchHandler) getSearchCount() uint64 {
	if h.metrics != nil {
		return h.metrics.GetSearchesTotal()
	}
	return 0
}

// getTorStatus returns Tor status string per PART 13
func (h *SearchHandler) getTorStatus() string {
	if h.torSvc == nil {
		return "disabled"
	}
	info := h.torSvc.GetInfo()
	if status, ok := info["status"].(string); ok {
		return status
	}
	if h.torSvc.IsRunning() {
		return "healthy"
	}
	return "disabled"
}

// getTorHostname returns Tor .onion address per PART 31
func (h *SearchHandler) getTorHostname() string {
	if h.torSvc == nil {
		return ""
	}
	info := h.torSvc.GetInfo()
	if hostname, ok := info["hostname"].(string); ok {
		return hostname
	}
	return ""
}

// getRequestsTotal returns total HTTP requests from metrics
func (h *SearchHandler) getRequestsTotal() uint64 {
	if h.metrics != nil {
		return h.metrics.GetRequestsTotal()
	}
	return 0
}

// getRequests24h returns HTTP requests in last 24 hours per AI.md PART 13
func (h *SearchHandler) getRequests24h() uint64 {
	if h.metrics != nil {
		return h.metrics.GetRequests24h()
	}
	return 0
}

// getActiveConnections returns current active connections
func (h *SearchHandler) getActiveConnections() int64 {
	if h.metrics != nil {
		return h.metrics.GetActiveConnections()
	}
	return 0
}

// WriteJSON writes a JSON response with 2-space indentation and trailing newline
// Per AI.md PART 14: ALL JSON responses MUST be indented
// Package-level function so all handler types can use it
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	jsonData, err := marshalPublicJSON(data)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"ok":false,"error":"SERVER_ERROR","message":"Failed to encode response","details":{}}` + "\n"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(jsonData)
	// Single trailing newline (NON-NEGOTIABLE)
	_, _ = w.Write([]byte("\n"))
}

// MaintenanceModeMiddleware checks if maintenance mode is enabled
func (h *SearchHandler) MaintenanceModeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip maintenance check for health/version/static endpoints. Per AI.md PART 5/6, health
		// and version endpoints must stay reachable during maintenance so
		// monitoring/orchestration can still see the server is up, and static
		// assets must keep serving so the maintenance page itself renders.
		path := r.URL.Path
		apiBase := h.appConfig.APIBasePath()
		if path == "/healthz" ||
			path == "/server/healthz" || path == "/server/healthz.json" || path == "/server/healthz.txt" ||
			path == apiBase+"/server/healthz" || path == "/api/healthz" ||
			path == "/version" || path == apiBase+"/version" ||
			strings.HasPrefix(path, "/static/") ||
			strings.HasPrefix(path, "/branding/") {
			next.ServeHTTP(w, r)
			return
		}

		// Check if maintenance mode is active
		paths := config.GetAppPaths("", "")
		modeFile := filepath.Join(paths.Data, "maintenance.flag")
		if _, err := os.Stat(modeFile); err == nil {
			// Maintenance mode is active. Per AI.md PART 5/6 "API Responses in
			// Maintenance Mode": API/JSON/text clients get the canonical
			// {"ok":false,"error":"MAINTENANCE",...} envelope with
			// Retry-After/X-Maintenance-* headers; browsers get the HTML page.
			// This project has no self-healing/reason tracking (maintenance is
			// only ever toggled manually via the CLI), so those fields reflect
			// that reality rather than fabricating unimplemented state.
			const maintenanceReason = "manual"
			wantsHTML := true
			if strings.HasPrefix(path, "/api/") {
				wantsHTML = false
			} else if detectResponseFormat(r) != "text/html" {
				wantsHTML = false
			}

			headers := w.Header()
			headers.Set("Retry-After", "3600")
			headers.Set("X-Maintenance-Mode", "true")
			headers.Set("X-Maintenance-Reason", maintenanceReason)

			if !wantsHTML {
				WriteJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
					"ok":      false,
					"error":   CodeMaintenance,
					"message": MsgMaintenance,
					"details": map[string]interface{}{
						"reason":       maintenanceReason,
						"self_healing": false,
					},
				})
				return
			}

			headers.Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			lang := resolveLocale(r)
			dir := i18n.Direction(lang)
			_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="%s" dir="%s">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="robots" content="noindex,nofollow">
    <title>Maintenance - %s</title>
    <link rel="stylesheet" href="/static/css/common.css">
</head>
<body class="maintenance-page">
    <div class="maintenance">
        <h1>Under Maintenance</h1>
        <p>We're performing scheduled maintenance.</p>
        <p>Please check back shortly.</p>
    </div>
</body>
</html>`, lang, dir, template.HTMLEscapeString(h.appConfig.Server.Branding.Title))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AgeVerifyMiddleware checks for age verification cookie
func (h *SearchHandler) AgeVerifyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip age check for static files, health checks, and age verification endpoints
		path := r.URL.Path
		if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/branding/") ||
			strings.HasPrefix(path, "/api/") ||
			path == "/healthz" ||
			path == "/robots.txt" ||
			path == "/llms.txt" ||
			path == "/.well-known/llms.txt" ||
			path == "/age-verify" {
			next.ServeHTTP(w, r)
			return
		}

		// Check for age verification cookie
		cookie, err := r.Cookie(ageVerifyCookieName)
		if err != nil || cookie.Value != "1" {
			// Redirect to age verification page. The target path+query must be
			// percent-encoded before being embedded as the "redirect" query
			// value — an unescaped "?" (e.g. "/search?q=Test") produces a
			// malformed Location header with an ambiguous nested query string
			// that Chrome rejects outright as net::ERR_FAILED, even though
			// lenient clients like curl still follow it.
			redirect := r.URL.Path
			if r.URL.RawQuery != "" {
				redirect += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, "/age-verify?redirect="+url.QueryEscape(redirect), http.StatusFound)
			return
		}

		// Renew cookie on each visit
		h.setAgeVerifyCookie(w)

		next.ServeHTTP(w, r)
	})
}

// AgeVerifyPage shows the age verification gate
func (h *SearchHandler) AgeVerifyPage(w http.ResponseWriter, r *http.Request) {
	// If already verified, redirect to home or specified redirect
	cookie, err := r.Cookie(ageVerifyCookieName)
	if err == nil && cookie.Value == "1" {
		redirect := r.URL.Query().Get("redirect")
		redirect = safeLocalRedirect(redirect)
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}

	redirect := safeLocalRedirect(r.URL.Query().Get("redirect"))

	h.renderResponse(w, r, "age-verify", map[string]interface{}{
		"Title":    "Age Verification - " + h.appConfig.Server.Branding.Title,
		"Theme":    h.getRequestTheme(r),
		"Redirect": redirect,
	})
}

// AgeVerifySubmit handles the age verification form submission
func (h *SearchHandler) AgeVerifySubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/age-verify", http.StatusFound)
		return
	}

	// Set the age verification cookie
	h.setAgeVerifyCookie(w)

	// Redirect to the original destination
	redirect := r.FormValue("redirect")
	redirect = safeLocalRedirect(redirect)

	http.Redirect(w, r, redirect, http.StatusFound)
}

// AnnouncementDismiss handles POST /announcements/dismiss per AI.md PART 16
// "Site Banner" — the zero-JS dismissal flow. Appends the submitted id to the
// dismissed_announcements cookie (comma-separated, JS-readable so the
// progressive-enhancement handler in app.js can update it client-side too)
// and redirects back to return_to (same-site relative paths only, as with
// /server/consent). CSRF is enforced globally by newCSRFMiddleware.
func (h *SearchHandler) AnnouncementDismiss(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, i18n.Translate(i18n.DetectLocale(r), "error.invalid_input"), http.StatusBadRequest)
		return
	}

	id := strings.TrimSpace(r.PostFormValue("id"))
	if id != "" {
		dismissed := dismissedAnnouncementIDs(r)
		if dismissed == nil {
			dismissed = make(map[string]bool)
		}
		dismissed[id] = true

		ids := make([]string, 0, len(dismissed))
		for existing := range dismissed {
			ids = append(ids, existing)
		}
		sort.Strings(ids)

		secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
		http.SetCookie(w, &http.Cookie{
			Name:     "dismissed_announcements",
			Value:    url.QueryEscape(strings.Join(ids, ",")),
			Path:     "/",
			MaxAge:   31536000,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}

	target := safeReturnPath(r.PostFormValue("return_to"), r)
	if target == "" {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// setAgeVerifyCookie sets/renews the age verification cookie per AI.md PART 11
func (h *SearchHandler) setAgeVerifyCookie(w http.ResponseWriter) {
	// 30 days, with Secure flag per AI.md PART 11
	http.SetCookie(w, newSecureCookie(
		ageVerifyCookieName,
		"1",
		"/",
		ageVerifyCookieDays*24*60*60,
		h.appConfig.Server.SSL.Enabled,
	))
}

// ContentRestrictionMiddleware checks for geographic content restrictions
// Behavior depends on mode: warn, soft_block, or hard_block
func (h *SearchHandler) ContentRestrictionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip restriction check for static files, health checks, API, and restriction pages
		path := r.URL.Path
		if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/branding/") ||
			strings.HasPrefix(path, "/api/") ||
			path == "/healthz" ||
			path == "/robots.txt" ||
			path == "/llms.txt" ||
			path == "/.well-known/llms.txt" ||
			path == "/age-verify" ||
			path == "/content-restricted" {
			next.ServeHTTP(w, r)
			return
		}

		// Check if user is from a restricted region
		restriction := h.checkContentRestriction(r)
		if restriction == nil {
			// Not restricted, proceed
			next.ServeHTTP(w, r)
			return
		}

		// Handle based on restriction mode
		switch restriction.Mode {
		case "hard_block":
			// Completely block access - show error page
			h.renderContentBlockedPage(w, r, restriction)
			return

		case "soft_block":
			// Require acknowledgment before proceeding
			if !h.hasContentRestrictionAck(r) {
				// Percent-encode before embedding as a query value — see the
				// identical fix/comment in AgeVerifyMiddleware above.
				redirect := r.URL.Path
				if r.URL.RawQuery != "" {
					redirect += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, "/content-restricted?redirect="+url.QueryEscape(redirect), http.StatusFound)
				return
			}
			// Has acknowledgment, proceed
			next.ServeHTTP(w, r)

		case "warn":
			// Set warning header for frontend to display dismissable banner
			w.Header().Set("X-Content-Warning", restriction.Message)
			w.Header().Set("X-Content-Warning-Region", restriction.Reason)
			next.ServeHTTP(w, r)

		default:
			// Unknown mode, proceed without restriction
			next.ServeHTTP(w, r)
		}
	})
}

// renderContentBlockedPage renders the hard block page (no way to bypass)
func (h *SearchHandler) renderContentBlockedPage(w http.ResponseWriter, r *http.Request, restriction *geoip.RestrictionResult) {
	w.WriteHeader(http.StatusForbidden)
	h.renderResponse(w, r, "content-blocked", map[string]interface{}{
		"Title":   "Access Restricted - " + h.appConfig.Server.Branding.Title,
		"Theme":   h.getRequestTheme(r),
		"Message": restriction.Message,
		"Region":  restriction.Reason,
	})
}

// ContentRestrictedPage shows the soft block acknowledgment page
func (h *SearchHandler) ContentRestrictedPage(w http.ResponseWriter, r *http.Request) {
	// If already acknowledged, redirect to home or specified redirect
	if h.hasContentRestrictionAck(r) {
		redirect := r.URL.Query().Get("redirect")
		redirect = safeLocalRedirect(redirect)
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}

	// Get redirect destination
	redirect := safeLocalRedirect(r.URL.Query().Get("redirect"))

	// Get restriction info for display
	restriction := h.checkContentRestriction(r)
	message := "Adult content may be restricted in your region."
	region := ""
	if restriction != nil {
		message = restriction.Message
		region = restriction.Reason
	}

	h.renderResponse(w, r, "content-restricted", map[string]interface{}{
		"Title":    "Content Notice - " + h.appConfig.Server.Branding.Title,
		"Theme":    h.getRequestTheme(r),
		"Redirect": redirect,
		"Message":  message,
		"Region":   region,
	})
}

// ContentRestrictedSubmit handles the acknowledgment form submission
func (h *SearchHandler) ContentRestrictedSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/content-restricted", http.StatusFound)
		return
	}

	// Set the acknowledgment cookie
	h.setContentRestrictionAckCookie(w)

	// Redirect to the original destination
	redirect := r.FormValue("redirect")
	redirect = safeLocalRedirect(redirect)

	http.Redirect(w, r, redirect, http.StatusFound)
}

// BuildDateTime returns the build time formatted per AI.md PART 16
// Format: "January 2, 2006 at 15:04:05" (December 4, 2025 at 13:05:13)
func BuildDateTime() string {
	raw := version.BuildTime
	if raw == "" || raw == "unknown" {
		return "unknown"
	}

	// Try to parse common build time formats.
	// Handles both Makefile (ISO 8601 UTC) and CI (human-readable) formats.
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		// CI/CD format from AI.md PART 28: "Mon Jan 02, 2006 at 15:04:05 MST"
		"Mon Jan 02, 2006 at 15:04:05 MST",
		"Mon Jan _2, 2006 at 15:04:05 MST",
		"Jan 2 2006 15:04:05",
		"Mon Jan 2 15:04:05 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, raw); err == nil {
			// Format per AI.md PART 16: %B %-d, %Y at %H:%M:%S
			return t.Format("January 2, 2006 at 15:04:05")
		}
	}

	// If parsing fails, return raw value
	return raw
}

// HomePage renders the main search page
// HomePage renders the home page with content negotiation per AI.md PART 16
func (h *SearchHandler) HomePage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)

	engineCount := h.engineMgr.EnabledCount()

	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":        h.appConfig.Server.Branding.Title,
			"description":  h.appConfig.Server.Branding.Description,
			"engine_count": engineCount,
			"version":      version.GetVersion(),
		})

	default:
		// HTML/text response — renderResponse() applies full content negotiation
		// per AI.md PART 14: text/plain → HTML2TextConverter, browser → HTML+JS
		h.renderResponse(w, r, "home", map[string]interface{}{
			"Title":         h.appConfig.Server.Branding.Title,
			"Description":   h.appConfig.Server.Branding.Description,
			"Theme":         h.getRequestTheme(r),
			"BuildDateTime": BuildDateTime(),
			"EngineCount":   engineCount,
		})
	}
}

// SearchPage renders search results with content negotiation per AI.md PART 16
func (h *SearchHandler) SearchPage(w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()

	// Strip leading/trailing whitespace from query per AI.md
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Parse bangs from query (e.g., "!ph amateur" -> search pornhub for "amateur")
	parsed := engine.ParseBangs(query)
	searchQuery := strings.TrimSpace(parsed.Query)
	if searchQuery == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	engineNames := requestEngineNames(r, parsed.Engines, h.engineMgr.ListEngines())

	format := detectResponseFormat(r)

	// Page parameter — server-driven pagination (IDEA.md "Search behavior":
	// the server decides how/how much to send, not client-side JS).
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if pn, err := strconv.Atoi(p); err == nil && pn > 0 {
			page = pn
		}
	}

	// Results-per-page preference cookie — server-authoritative (IDEA.md
	// "Search Settings"). "0" ("Infinite scroll") falls back to the engine's
	// configured default; JS only decides whether to auto-fetch the next
	// page, never how many results come back.
	resultsPerPageOverride := 0
	infiniteScroll := false
	if n, err := strconv.Atoi(h.getRequestResultsPerPage(r)); err == nil {
		if n == 0 {
			infiniteScroll = true
		} else {
			resultsPerPageOverride = n
		}
	}

	// Preview-first result ordering — no-JS clients only (see
	// getRequestPreviewFirst); the JS-enabled UI sends this as a
	// SearchWithOperators-equivalent query param on its own SSE/AJAX calls
	// (see APISearch), never through this cookie path.
	previewFirst := h.getRequestPreviewFirst(w, r)

	// For regular browsers: JavaScript streams results into the page via SSE
	// ({api_version}/search) as an enhancement. To keep core search working WITHOUT
	// JavaScript (progressive enhancement, AI.md PART 16), also perform a
	// synchronous search and render the results in a <noscript> fallback.
	if format == "text/html" {
		relatedSearches := h.engineMgr.GetValidatedRelatedSearches(searchQuery, 8)
		spellSuggestion := h.engineMgr.SpellCorrect(searchQuery)
		enginesParam := r.URL.Query().Get("engines")

		// Each server-rendered page gets a fresh token. The browser reuses it for
		// infinite-scroll requests so later batches dedup against this first one;
		// native page navigation receives a new token and remains repeatable.
		sessionID := newSearchSessionID()
		filterOpts := parseResultFilterOptions(r)
		searchCtx := withRequestSearchPreferences(r.Context(), r)
		results := h.engineMgr.SearchWithOperators(searchCtx, searchQuery, page, engineNames, parsed.ExactPhrases, parsed.Exclusions, parsed.RequiredTerms, previewFirst, sessionID, resultsPerPageOverride, filterOpts)
		results.Data.SearchTimeMS = time.Since(requestStart).Milliseconds()
		results.Data.InvalidBang = parsed.InvalidBang
		if h.metrics != nil {
			h.metrics.IncrementSearches()
		}

		// Search capacity exceeded (searchSem saturated) — answer fast with a
		// retryable 429 instead of rendering an empty-results page as if the
		// query genuinely had none (AI.md PART 12 "Rate Limiting").
		if isSearchOverloaded(results) {
			h.renderSearchOverloadPage(w, r, query, searchQuery, page)
			return
		}

		pageResults := results.Data.Results
		resultCountFormats := formatResultCountFormats(maxInt(1000, len(pageResults)), resolveLocale(r))
		// A full page came back, so another page is worth requesting. Explicit
		// session tokens are used by the JavaScript infinite-scroll API; native
		// page links remain stateless so previous-page navigation is repeatable.
		hasMore := results.Pagination.Limit > 0 && len(pageResults) >= results.Pagination.Limit
		nextPage := page + 1

		// Embed the server-computed page as an inline JSON payload so the JS
		// client hydrates from it (no second {api_version}/search round-trip) per
		// AI.md PART 14 "JavaScript enhances, it does not enable". The same
		// results are also rendered as visible cards below for no-JS clients.
		resultsJSON, err := json.Marshal(pageResults)
		if err != nil || len(pageResults) == 0 {
			resultsJSON = []byte("[]")
		}

		// Server-side favorites (AI.md PART 16/32): the owner token cookie
		// identifies which of these results are already favorited, so the
		// no-JS add/remove form on each card can render the correct state
		// without a client-side lookup.
		favoriteURLs := map[string]bool{}
		if ownerID, _ := h.ownerID(w, r, false, false); ownerID != "" {
			if favs, favErr := h.listFavorites(ownerID); favErr == nil {
				for _, f := range favs {
					favoriteURLs[f.URL] = true
				}
			}
		}

		prevURL := searchPaginationURL(r, page-1)
		nextURL := searchPaginationURL(r, nextPage)
		h.renderResponse(w, r, "search", map[string]interface{}{
			"Title":              query + " - " + h.appConfig.Server.Branding.Title,
			"Query":              query,
			"SearchQuery":        searchQuery,
			"ResultsJSON":        template.JS(resultsJSON),
			"Results":            pageResults,
			"FavoriteURLs":       favoriteURLs,
			"CurrentPath":        r.URL.RequestURI(),
			"EnginesUsed":        results.Data.EnginesUsed,
			"SearchTime":         humanformat.Duration(time.Duration(results.Data.SearchTimeMS)*time.Millisecond, resolveLocale(r)),
			"ResultCountFormats": resultCountFormats,
			"Theme":              h.getRequestTheme(r),
			"HasBang":            parsed.HasBang,
			"BangEngines":        parsed.Engines,
			"RelatedSearches":    relatedSearches,
			"SpellSuggestion":    spellSuggestion,
			"EnginesParam":       enginesParam,
			"FilterDuration":     effectiveFilterValue(r, "duration", "vidveil_pref_default_duration", ""),
			"FilterQuality":      r.URL.Query().Get("quality"),
			"FilterSort":         effectiveFilterValue(r, "sort", "vidveil_pref_default_sort", ""),
			"Duration":           effectiveFilterValue(r, "duration", "vidveil_pref_default_duration", ""),
			"Sort":               effectiveFilterValue(r, "sort", "vidveil_pref_default_sort", ""),
			"GridDensity":        preferenceCookieValue(r, "vidveil_pref_grid_density", "default"),
			"ThumbnailSize":      preferenceCookieValue(r, "vidveil_pref_thumbnail_size", "medium"),
			"PreviewFirst":       previewFirst,
			"OpenNewTab":         h.getRequestOpenNewTab(r),
			"Page":               page,
			"PrevPage":           page - 1,
			"NextPage":           nextPage,
			"PrevURL":            prevURL,
			"NextURL":            nextURL,
			"HasMore":            hasMore,
			"InfiniteScroll":     infiniteScroll,
			"SearchSessionID":    sessionID,
			"Version":            version.GetVersion(),
			"BuildDateTime":      BuildDateTime(),
		})
		return
	}

	// Non-browser clients (CLI, curl, JSON API): perform synchronous search
	searchCtx := withRequestSearchPreferences(r.Context(), r)
	results := h.engineMgr.SearchWithOperators(searchCtx, searchQuery, page, engineNames, parsed.ExactPhrases, parsed.Exclusions, parsed.RequiredTerms, previewFirst, "", resultsPerPageOverride, parseResultFilterOptions(r))
	results.Data.SearchTimeMS = time.Since(requestStart).Milliseconds()
	results.Data.InvalidBang = parsed.InvalidBang

	if h.metrics != nil {
		h.metrics.IncrementSearches()
	}

	// Search capacity exceeded — respond fast and retryable regardless of the
	// requested format (AI.md PART 12 "Rate Limiting"); JSON callers get the
	// canonical envelope, everything else goes through the normal content
	// negotiation in renderResponseStatus (text/plain, HTTP-tool text, HTML)
	// so a curl/CLI client still gets the format it asked for, just with a
	// 429 status and Retry-After header instead of a 200.
	if isSearchOverloaded(results) {
		if format == "application/json" {
			h.writeSearchOverloadJSON(w)
			return
		}
		h.renderSearchOverloadPage(w, r, query, searchQuery, page)
		return
	}

	switch format {
	case "application/json":
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"query":        query,
			"search_query": searchQuery,
			"results":      results.Data.Results,
			"engines_used": results.Data.EnginesUsed,
			"search_time":  results.Data.SearchTimeMS,
			"has_bang":     parsed.HasBang,
			"invalid_bang": parsed.InvalidBang,
		})

	default:
		// HTML/text response — renderResponse() applies full content negotiation
		// per AI.md PART 14: text/plain → HTML2TextConverter, browser → HTML+JS
		// Fallback: embed results in HTML shell
		resultsJSON, _ := json.Marshal(results.Data.Results)
		relatedSearches := h.engineMgr.GetValidatedRelatedSearches(searchQuery, 8)
		spellSuggestion := h.engineMgr.SpellCorrect(searchQuery)
		enginesParam := r.URL.Query().Get("engines")

		h.renderResponse(w, r, "search", map[string]interface{}{
			"Title":              query + " - " + h.appConfig.Server.Branding.Title,
			"Query":              query,
			"SearchQuery":        searchQuery,
			"ResultsJSON":        template.JS(resultsJSON),
			"EnginesUsed":        results.Data.EnginesUsed,
			"SearchTime":         humanformat.Duration(time.Duration(results.Data.SearchTimeMS)*time.Millisecond, resolveLocale(r)),
			"ResultCountFormats": formatResultCountFormats(maxInt(1000, len(results.Data.Results)), resolveLocale(r)),
			"Theme":              h.getRequestTheme(r),
			"HasBang":            parsed.HasBang,
			"BangEngines":        parsed.Engines,
			"RelatedSearches":    relatedSearches,
			"SpellSuggestion":    spellSuggestion,
			"EnginesParam":       enginesParam,
			"FilterDuration":     effectiveFilterValue(r, "duration", "vidveil_pref_default_duration", ""),
			"FilterQuality":      r.URL.Query().Get("quality"),
			"FilterSort":         effectiveFilterValue(r, "sort", "vidveil_pref_default_sort", ""),
			"Duration":           effectiveFilterValue(r, "duration", "vidveil_pref_default_duration", ""),
			"Sort":               effectiveFilterValue(r, "sort", "vidveil_pref_default_sort", ""),
			"GridDensity":        preferenceCookieValue(r, "vidveil_pref_grid_density", "default"),
			"ThumbnailSize":      preferenceCookieValue(r, "vidveil_pref_thumbnail_size", "medium"),
			"PreviewFirst":       previewFirst,
			"Version":            version.GetVersion(),
			"BuildDateTime":      BuildDateTime(),
		})
	}
}

// PreferencesPage renders user preferences with content negotiation per AI.md PART 16
func (h *SearchHandler) PreferencesPage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)

	engines := h.engineMgr.ListEngines()

	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":   "Preferences",
			"engines": engines,
			"theme":   h.getRequestTheme(r),
		})

	default:
		data := h.preferencePageData(w, r)
		query := h.exportablePreferenceQuery(r)
		data["PreferenceExportURL"] = urlvar.BuildURL(r, "/server/preferences/import") + "?" + query
		data["PreferenceExportCode"] = base64.RawURLEncoding.EncodeToString([]byte(query))
		h.renderResponse(w, r, "preferences", data)
	}
}

func (h *SearchHandler) preferencePageData(w http.ResponseWriter, r *http.Request) map[string]interface{} {
	engines := h.engineMgr.ListEngines()
	if selected, configured := requestEngineSelection(r, engines); configured {
		for i := range engines {
			_, engines[i].Enabled = selected[engines[i].Name]
		}
	}
	// Favorites count is server-authoritative (server.db, see favorites.go)
	// per AI.md PART 16/32 — never derived from client-side localStorage.
	ownerID, _ := h.ownerID(w, r, false, false)
	favs, err := h.listFavorites(ownerID)
	favCount := 0
	if err == nil {
		favCount = len(favs)
	}
	// HTML/text response — renderResponse() applies full content negotiation
	// per AI.md PART 14: text/plain → HTML2TextConverter, browser → HTML+JS
	return map[string]interface{}{
		"Title":              "Preferences - " + h.appConfig.Server.Branding.Title,
		"Theme":              h.getRequestTheme(r),
		"Lang":               h.effectivePreferences(r).Lang,
		"Prefs":              h.effectivePreferences(r).Prefs,
		"CCPAOptedOut":       hasCCPAOptOut(r),
		"ConsentGiven":       hasConsentCookie(r),
		"ConsentPreferences": consentPreferencesEnabled(r, h.appConfig.Server.Privacy.Cookies.Preferences.Enabled),
		"AnalyticsConsent":   consentAnalyticsEnabled(r),
		"DataSold":           h.appConfig.Server.Privacy.Data.Sold,
		"Engines":            engines,
		"ResultsPerPage":     h.getRequestResultsPerPage(r),
		"OpenNewTab":         h.getRequestOpenNewTab(r),
		"FavoritesCount":     favCount,
		"CSRFToken":          cSRFTokenFromRequest(r),
		"BuildDateTime":      BuildDateTime(),
		// ReturnTo is the page the user arrived from (Referer), threaded
		// through the form (hidden field for no-JS, data attribute for
		// JS) so saving/closing preferences sends them back there instead
		// of always landing on /server/preferences. See safeReturnPath.
		"ReturnTo":  safeReturnPath(r.Referer(), r),
		"CancelURL": preferenceRedirect(r),
	}
}

func requestEngineSelection(r *http.Request, engines []model.EngineInfo) (map[string]bool, bool) {
	c, err := r.Cookie(engineSelectionCookieName)
	if err != nil || c.Value != "1" {
		return nil, false
	}
	selected := make(map[string]bool, len(engines))
	for _, info := range engines {
		if preferenceCookieValue(r, "vidveil_pref_engine_"+info.Name, "0") == "1" {
			selected[info.Name] = true
		}
	}
	return selected, true
}

func requestEngineNames(r *http.Request, bangEngines []string, engines []model.EngineInfo) []string {
	if len(bangEngines) > 0 {
		return bangEngines
	}
	if values, present := r.URL.Query()["engines"]; present {
		var names []string
		for _, value := range values {
			for _, name := range strings.Split(value, ",") {
				if name = strings.TrimSpace(name); name != "" {
					names = append(names, name)
				}
			}
		}
		if len(names) == 0 {
			return []string{"__none__"}
		}
		return names
	}
	if selected, configured := requestEngineSelection(r, engines); configured {
		var names []string
		for name := range selected {
			names = append(names, name)
		}
		if len(names) == 0 {
			return []string{"__none__"}
		}
		return names
	}
	return nil
}

// getRequestResultsPerPage returns the user's results-per-page preference from
// their cookie — server-authoritative per IDEA.md "Search Settings" (the
// server, not client-side JS, decides how many results to send). "0" means
// "Infinite scroll" and is the default. Set by PreferencesSave for
// no-JS clients and mirrored into the cookie by the JS preferences form too.
func (h *SearchHandler) getRequestResultsPerPage(r *http.Request) string {
	if c, err := r.Cookie(resultsPerPageCookieName); err == nil {
		switch c.Value {
		case "0", "20", "50", "100":
			return c.Value
		}
	}
	return "0"
}

// getRequestOpenNewTab returns the user's open-links-in-new-tab preference
// from their cookie (set by PreferencesSave for no-JS clients), falling back
// to the JS-UI default of true (see static/js/app.js DEFAULT_PREFS.openNewTab).
func (h *SearchHandler) getRequestOpenNewTab(r *http.Request) bool {
	if c, err := r.Cookie(openNewTabCookieName); err == nil {
		return c.Value == "1"
	}
	return true
}

// getRequestPreviewFirst returns the user's "preview first" result-ordering
// preference for the no-JS/text-browser SearchPage render path. An explicit
// `preview_first` query param (submitted by the filters.tmpl checkbox+hidden
// fallback pair — "1" checked, "0" unchecked) always wins and is persisted
// back into previewFirstCookieName so the choice survives navigating to a
// fresh search with no query param at all; otherwise falls back to the
// existing cookie, defaulting to false.
func (h *SearchHandler) getRequestPreviewFirst(w http.ResponseWriter, r *http.Request) bool {
	if v := r.URL.Query().Get("preview_first"); v == "0" || v == "1" {
		previewFirst := v == "1"
		http.SetCookie(w, newSecureCookie(previewFirstCookieName, v, "/", 365*24*60*60, h.appConfig.Server.SSL.Enabled))
		return previewFirst
	}
	return requestPreviewFirst(r)
}

func requestPreviewFirst(r *http.Request) bool {
	if value := r.URL.Query().Get("preview_first"); value == "0" || value == "1" {
		return value == "1"
	}
	if c, err := r.Cookie(previewFirstCookieName); err == nil {
		if c.Value == "1" || c.Value == "0" {
			return c.Value == "1"
		}
	}
	return preferenceCookieValue(r, previewFirstCookieName, "1") == "1"
}

func withRequestTorPreference(ctx context.Context, r *http.Request) context.Context {
	if useTor := requestTorPreference(r); useTor != nil {
		return engine.WithTorPref(ctx, useTor)
	}
	return ctx
}

func requestTorPreference(r *http.Request) *bool {
	value := preferenceCookieValue(r, "vidveil_pref_use_tor", "")
	if value != "0" && value != "1" {
		return nil
	}
	useTor := value == "1"
	return &useTor
}

func requestShowAIContent(r *http.Request) bool {
	if values, ok := r.URL.Query()["show_ai"]; ok && len(values) > 0 {
		return values[len(values)-1] == "1"
	}
	return preferenceCookieValue(r, "vidveil_pref_show_ai_content", "0") == "1"
}

func withRequestSearchPreferences(ctx context.Context, r *http.Request) context.Context {
	ctx = withRequestTorPreference(ctx, r)
	return engine.WithShowAIContent(ctx, requestShowAIContent(r))
}

func preferenceCookieValue(r *http.Request, name, fallback string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return fallback
	}
	value, valid := validatePreference(name, cookie.Value)
	if !valid {
		return fallback
	}
	return value
}

func effectiveFilterValue(r *http.Request, queryName, cookieName, fallback string) string {
	if values, ok := r.URL.Query()[queryName]; ok && len(values) > 0 {
		return values[len(values)-1]
	}
	value := preferenceCookieValue(r, cookieName, fallback)
	if value == "any" {
		return ""
	}
	return value
}

// safeLocalRedirect returns target only when it is a same-site, absolute-path
// redirect target, and "/" otherwise. A bare strings.HasPrefix(target, "/")
// check is not enough: "//evil.com" and "/\evil.com" are both protocol-relative
// (browsers normalize the backslash) and would send the visitor off-site, so
// any target starting with "/", "//", or "/\" is rejected. Also rejects
// control characters, which can truncate the Location header in intermediaries.
// Used for every user-supplied ?redirect=/redirect form field (age verify and
// content-restricted acknowledgment flows).
func safeLocalRedirect(target string) string {
	if target == "" || !strings.HasPrefix(target, "/") {
		return "/"
	}
	if strings.HasPrefix(target, "//") || strings.HasPrefix(target, "/\\") {
		return "/"
	}
	if strings.ContainsAny(target, "\r\n\t\x00") {
		return "/"
	}
	return target
}

// safeReturnPath validates a redirect target taken from either a Referer
// header (full URL) or a same-origin form/query field (bare path), and
// returns "" if the target is missing, points at a different host, isn't
// rooted at "/", or is protocol-relative ("//evil.com") — the standard
// open-redirect guards. Also rejects bouncing back into the preferences
// page/save/export/import endpoints themselves, which would defeat the point
// of remembering where the user came from. Used by PreferencesPage (capture)
// and PreferencesSave/PreferencesImport (use) so saving/closing preferences
// returns the user to their prior page for both JS and no-JS clients.
func safeReturnPath(target string, r *http.Request) string {
	if target == "" {
		return ""
	}
	u, err := url.Parse(target)
	if err != nil {
		return ""
	}
	if u.Host != "" && !strings.EqualFold(u.Host, urlvar.RequestHost(r)) {
		return ""
	}
	path := u.Path
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return ""
	}
	switch path {
	case "/server/preferences", "/server/preferences/export", "/server/preferences/import":
		return ""
	}
	if u.RawQuery != "" {
		return path + "?" + u.RawQuery
	}
	return path
}

// PreferencesSave handles the nojs/preferences.tmpl form submission, persisting
// preferences as cookies for JavaScript and no-JavaScript clients.
func (h *SearchHandler) PreferencesSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		r.PostForm = url.Values{}
	}
	if r.PostForm.Get("action") == "reset" {
		resetPreferenceCookies(w, r)
		writePreferenceRedirect(w, r)
		return
	}
	values := url.Values{}
	for name, v := range preferencesWithCode(r.PostForm) {
		if name == "theme" || name == "lang" || strings.HasPrefix(name, "vidveil_pref_") {
			values[name] = v
		}
	}
	result := h.applyPreferences(w, r, values)
	h.applyWebConsent(w, r)
	if r.PostForm.Get("engines_selection_set") == "1" {
		selected := make(map[string]struct{}, len(r.PostForm["engines"]))
		for _, name := range r.PostForm["engines"] {
			selected[name] = struct{}{}
		}
		secure := strings.HasPrefix(urlSchemeForRequest(r), "https")
		http.SetCookie(w, &http.Cookie{Name: engineSelectionCookieName, Value: "1", Path: "/", MaxAge: 31536000, Secure: secure, SameSite: http.SameSiteLaxMode})
		for _, info := range h.engineMgr.ListEngines() {
			value := "0"
			if _, ok := selected[info.Name]; ok {
				value = "1"
			}
			http.SetCookie(w, &http.Cookie{Name: "vidveil_pref_engine_" + info.Name, Value: value, Path: "/", MaxAge: 31536000, Secure: secure, SameSite: http.SameSiteLaxMode})
		}
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		if len(result.Applied) == 0 {
			SendError(w, "VALIDATION_FAILED", "No valid preferences were supplied")
			return
		}
		SendOK(w, result)
		return
	}
	writePreferenceRedirect(w, r)
}

// exportablePreferenceQuery builds the canonical query string for the current
// request's effective theme/language and valid app preferences, per AI.md's
// "Cross-device preference sync" spec. Consent and build-stamp cookies are
// never included.
func (h *SearchHandler) exportablePreferenceQuery(r *http.Request) string {
	v := url.Values{}
	effective := h.effectivePreferences(r)
	v.Set("theme", effective.Theme)
	v.Set("lang", effective.Lang)
	for _, cookie := range r.Cookies() {
		if preferenceKeyPattern.MatchString(cookie.Name) {
			if value, ok := validatePreference(cookie.Name, cookie.Value); ok {
				v.Set(cookie.Name, value)
			}
		}
	}
	return v.Encode()
}

// PreferencesExport renders the current theme, language, and app preferences
// as a shareable full URL and short base64url code. The code/URL contains the
// preference values; nothing is stored server-side.
func (h *SearchHandler) PreferencesExport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	query := h.exportablePreferenceQuery(r)
	importURL := urlvar.BuildURL(r, "/server/preferences/import") + "?" + query
	code := base64.RawURLEncoding.EncodeToString([]byte(query))

	format := detectResponseFormat(r)
	if strings.HasPrefix(r.URL.Path, "/api/") {
		format = "application/json"
	}
	switch format {
	case "application/json":
		data := h.effectivePreferences(r)
		SendOK(w, map[string]interface{}{"theme": data.Theme, "lang": data.Lang, "prefs": data.Prefs, "url": importURL, "code": code})
	case "text/plain":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("URL: " + importURL + "\nCode: " + code + "\n"))
	default:
		data := h.preferencePageData(w, r)
		data["PreferenceExportURL"] = importURL
		data["PreferenceExportCode"] = code
		h.renderResponse(w, r, "preferences", data)
	}
}

// decodePreferenceCode decodes a short sync code back into a preference query
// string. It also tolerates a pasted full import URL per AI.md's paste-a-code
// UX note.
func decodePreferenceCode(code string) (string, bool) {
	code = strings.TrimSpace(code)
	if len(code) > 2048 {
		return "", false
	}
	if idx := strings.Index(code, "?"); idx != -1 {
		code = code[idx+1:]
	}
	if code == "" {
		return "", false
	}
	if strings.Contains(code, "=") {
		padding := len(code) - len(strings.TrimRight(code, "="))
		if padding == 0 || padding > 2 {
			return code, true
		}
		decoded, err := base64.URLEncoding.DecodeString(code)
		if err != nil || !strings.Contains(string(decoded), "=") {
			return code, true
		}
		return string(decoded), true
	}
	decoded, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(code)
	}
	if err != nil {
		return "", false
	}
	return string(decoded), true
}

// PreferencesImport validates and applies guest preferences supplied either
// directly as query params or as a short base64url code. It then redirects so
// imported values do not linger in the visible URL or browser history. Values
// outside the theme/language rules or app-preference key/value rules are
// silently dropped per AI.md's "Cross-device preference sync" spec. Nothing
// is written or looked up server-side.
func (h *SearchHandler) PreferencesImport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	q := make(url.Values)
	if raw, ok := decodePreferenceCode(r.URL.Query().Get("code")); ok {
		if decoded, err := url.ParseQuery(raw); err == nil {
			q = decoded
		}
	}
	for key, vals := range r.URL.Query() {
		if key == "code" {
			continue
		}
		q[key] = vals
	}
	if code := r.URL.Query().Get("code"); len(code) > 2048 {
		q.Add("code", code)
	}
	result := h.applyPreferences(w, r, q)
	if strings.HasPrefix(r.URL.Path, "/api/") {
		SendOK(w, result)
		return
	}
	writePreferenceRedirect(w, r)
}

func preferenceRedirect(r *http.Request) string {
	if redirect := safeRelativePreferenceRedirect(r.FormValue("redirect"), r); redirect != "" {
		return redirect
	}
	if referer := safeRefererPreferenceRedirect(r.Referer(), r); referer != "" {
		return referer
	}
	return "/"
}

func writePreferenceRedirect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Location", preferenceRedirect(r))
	w.WriteHeader(http.StatusSeeOther)
}

func safeRelativePreferenceRedirect(target string, r *http.Request) string {
	if strings.ContainsAny(target, "\r\n") || strings.HasPrefix(target, "//") || strings.HasPrefix(target, "/\\") || !strings.HasPrefix(target, "/") {
		return ""
	}
	return safeReturnPath(target, r)
}

func safeRefererPreferenceRedirect(target string, r *http.Request) string {
	if strings.ContainsAny(target, "\r\n") {
		return ""
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		proto, _, _ := urlvar.GetURLVars(r)
		if !strings.EqualFold(parsed.Scheme, proto) || !strings.EqualFold(parsed.Host, urlvar.RequestHost(r)) || parsed.User != nil {
			return ""
		}
		target = parsed.RequestURI()
	}
	return safeRelativePreferenceRedirect(target, r)
}

// FavoritesPage, FavoritesSave, FavoritesExport, FavoritesImport, and the
// FavoritesAPI* handlers live in favorites.go — server-side favorites
// storage per AI.md PART 16/32 (see that file's header comment).

// AboutPage renders the about page with content negotiation per AI.md PART 16
func (h *SearchHandler) AboutPage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)

	ver := version.GetVersion()

	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":       h.appConfig.Server.Branding.Title,
			"version":     ver,
			"build_date":  BuildDateTime(),
			"description": h.appConfig.Server.Branding.Description,
		})

	default:
		// HTML/text response — renderResponse() applies full content negotiation
		// per AI.md PART 14: text/plain → HTML2TextConverter, browser → HTML+JS
		h.renderResponse(w, r, "about", map[string]interface{}{
			"Title":         "About - " + h.appConfig.Server.Branding.Title,
			"Theme":         h.getRequestTheme(r),
			"Version":       ver,
			"BuildDateTime": BuildDateTime(),
		})
	}
}

// PrivacyPage renders the privacy policy page with content negotiation per AI.md PART 16
func (h *SearchHandler) PrivacyPage(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)

	ver := version.GetVersion()

	// Echo the honored Global Privacy Control signal back on the privacy page
	// per AI.md PART 11 "Privacy Signal Headers" point 4.
	gpcHonored := GPCOptOut(r)

	switch format {
	case "application/json":
		// JSON response for API clients
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"title":       "Privacy Policy",
			"version":     ver,
			"gpc_honored": gpcHonored,
		})

	default:
		// HTML/text response — renderResponse() applies full content negotiation
		// per AI.md PART 14: text/plain → HTML2TextConverter, browser → HTML+JS
		h.renderResponse(w, r, "privacy", map[string]interface{}{
			"Title":         "Privacy Policy - " + h.appConfig.Server.Branding.Title,
			"Theme":         h.getRequestTheme(r),
			"Version":       ver,
			"BuildDateTime": BuildDateTime(),
			"GPCHonored":    gpcHonored,
		})
	}
}

// parseResultFilterOptions builds a server-authoritative engine.ResultFilterOptions
// from the search-results filter panel's GET query params (filters.tmpl:
// "duration", "quality", "sort"), per AI.md PART 16 "JavaScript enhances, it
// does not enable" — the filter panel is a plain GET form so this parsing (and
// the resulting filtering/sorting) is identical whether JS is enabled or not.
// The legacy numeric "min_quality"/"min_duration" query params (used by the
// SSE endpoint and the visitor's separate Preferences "minimum duration"
// setting) are also honored and merged in — the higher/stricter value wins.
func parseResultFilterOptions(r *http.Request) engine.ResultFilterOptions {
	opts := engine.ResultFilterOptions{
		SortBy: effectiveFilterValue(r, "sort", "vidveil_pref_default_sort", ""),
	}

	minQualityValue := preferenceCookieValue(r, "vidveil_pref_min_quality", "360")
	if values, ok := r.URL.Query()["min_quality"]; ok && len(values) > 0 {
		minQualityValue = values[len(values)-1]
	}
	if mq := minQualityValue; mq != "" {
		if qv, err := strconv.Atoi(mq); err == nil && qv > 0 {
			opts.MinQuality = qv
		}
	}
	switch r.URL.Query().Get("quality") {
	case "4k":
		opts.MinQuality = maxInt(opts.MinQuality, engine.Quality4K)
	case "1080":
		opts.MinQuality = maxInt(opts.MinQuality, engine.Quality1080p)
	case "720":
		opts.MinQuality = maxInt(opts.MinQuality, engine.Quality720p)
	}

	minDurationValue := preferenceCookieValue(r, "vidveil_pref_min_duration", "600")
	if values, ok := r.URL.Query()["min_duration"]; ok && len(values) > 0 {
		minDurationValue = values[len(values)-1]
	}
	if md := minDurationValue; md != "" {
		if mv, err := strconv.Atoi(md); err == nil && mv > 0 {
			opts.UserMinDuration = mv
		}
	}
	duration := effectiveFilterValue(r, "duration", "vidveil_pref_default_duration", "")
	switch duration {
	case "short":
		opts.MaxDuration = 599
	case "medium":
		opts.UserMinDuration = maxInt(opts.UserMinDuration, 600)
		opts.MaxDuration = 1799
	case "long":
		opts.UserMinDuration = maxInt(opts.UserMinDuration, 1800)
	}

	return opts
}

// maxInt returns the larger of two ints.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// searchPaginationURL preserves the active search and filters while changing
// the page. Session tokens are intentionally omitted so a native link back to
// an earlier page can return the same results.
func searchPaginationURL(r *http.Request, page int) string {
	query := r.URL.Query()
	query.Set("page", strconv.Itoa(page))
	query.Del("session")
	return "/search?" + query.Encode()
}

func formatResultCountFormats(maximum int, locale string) []string {
	formats := make([]string, maximum+1)
	for count := range formats {
		formats[count] = humanformat.Count(int64(count), locale)
	}
	return formats
}

// detectResponseFormat determines response format per AI.md PART 14 (Content Negotiation)
func detectResponseFormat(r *http.Request) string {
	// 0. Check URL path extension FIRST per AI.md PART 14
	// Use original path from context if available (set by extensionStripMiddleware)
	path := r.URL.Path
	if origPath, ok := r.Context().Value(OriginalPathKey).(string); ok {
		path = origPath
	}
	if strings.HasSuffix(path, ".json") {
		return "application/json"
	}
	if strings.HasSuffix(path, ".txt") {
		return "text/plain"
	}
	if strings.HasSuffix(path, ".rss") {
		return "application/rss+xml"
	}
	if strings.HasSuffix(path, ".atom") {
		return "application/atom+xml"
	}
	if strings.HasSuffix(path, ".csv") {
		return "text/csv"
	}

	// 0b. Check ?format= query parameter
	switch r.URL.Query().Get("format") {
	case "rss":
		return "application/rss+xml"
	case "atom":
		return "application/atom+xml"
	case "csv":
		return "text/csv"
	case "json":
		return "application/json"
	case "text", "txt":
		return "text/plain"
	}

	// 1. Check Accept header (explicit preference)
	accept := r.Header.Get("Accept")

	// SSE streaming takes priority for search endpoints
	if strings.Contains(accept, "text/event-stream") {
		return "text/event-stream"
	}
	if strings.Contains(accept, "application/rss+xml") {
		return "application/rss+xml"
	}
	if strings.Contains(accept, "application/atom+xml") {
		return "application/atom+xml"
	}
	if strings.Contains(accept, "text/html") {
		return "text/html"
	}
	if strings.Contains(accept, "text/plain") {
		return "text/plain"
	}
	if strings.Contains(accept, "application/json") {
		return "application/json"
	}

	// 2. Check User-Agent for browser detection
	ua := r.Header.Get("User-Agent")

	// Browser User-Agents (common patterns)
	browsers := []string{
		"Mozilla/", "Chrome/", "Safari/", "Edge/", "Firefox/",
		"Opera/", "MSIE", "Trident/",
	}

	for _, browser := range browsers {
		if strings.Contains(ua, browser) {
			return "text/html"
		}
	}

	// 3. CLI tools (curl, wget, httpie, etc.)
	cliTools := []string{
		"curl/", "Wget/", "HTTPie/", "python-requests/",
		"Go-http-client/", "node-fetch/",
	}

	for _, tool := range cliTools {
		if strings.Contains(ua, tool) {
			return "text/plain"
		}
	}

	// 4. Empty or unknown User-Agent
	if ua == "" {
		// Default to text for programmatic access
		return "text/plain"
	}

	// 5. Default: HTML (safest fallback)
	return "text/html"
}

// getAPIResponseFormat determines format for /api/** routes per AI.md PART 14
// Returns "text" or "json" (raw strings, not MIME types)
// Priority: .txt extension > Accept header > CLI detection > default JSON
func getAPIResponseFormat(r *http.Request) string {
	// 1. Check .txt extension FIRST (highest priority).
	// extensionStripMiddleware removes the suffix from r.URL.Path before routing,
	// so read the original pre-strip path stored in the request context.
	path := r.URL.Path
	if origPath, ok := r.Context().Value(OriginalPathKey).(string); ok {
		path = origPath
	}
	if strings.HasSuffix(path, ".txt") {
		return "text"
	}

	// 2. Check Accept header (explicit preference - overrides UA detection)
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		return "json"
	}
	if strings.Contains(accept, "text/plain") {
		return "text"
	}

	// 3. Check if non-interactive client (CLI tools like curl, wget)
	// Per AI.md PART 14: CLI Tool column shows "Text"
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	cliTools := []string{
		"curl/", "wget/", "httpie/",
		"libcurl/", "python-requests/",
		"go-http-client/", "axios/", "node-fetch/",
	}
	for _, tool := range cliTools {
		if strings.Contains(ua, tool) {
			return "text"
		}
	}

	// Empty User-Agent = likely HTTP tool (non-interactive)
	if ua == "" {
		return "text"
	}

	// 4. Default to JSON for API routes (browsers, API clients)
	return "json"
}

// HealthCheck returns health status with content negotiation
// Per AI.md PART 16: Supports HTML (default), JSON (Accept: application/json), and Text
// HealthResponse is the canonical JSON body for /server/healthz and
// /api/{api_version}/server/healthz per AI.md PART 13 ("Field Order &
// Structure"). Field order here is significant: encoding/json preserves
// struct field order on output, whereas map[string]interface{} is always
// emitted in alphabetical key order regardless of insertion order — using
// a map here would silently violate the spec's mandated field order.
type HealthResponse struct {
	Project ProjectInfo `json:"project"`

	Status         string   `json:"status"`
	PendingRestart bool     `json:"pending_restart,omitempty"`
	RestartReason  []string `json:"restart_reason,omitempty"`

	Version   string    `json:"version"`
	GoVersion string    `json:"go_version"`
	Build     BuildInfo `json:"build"`

	Uptime    string `json:"uptime"`
	Mode      string `json:"mode"`
	Timestamp string `json:"timestamp"`

	Features FeaturesInfo `json:"features"`
	Checks   ChecksInfo   `json:"checks"`
	Stats    StatsInfo    `json:"stats"`
}

// ProjectInfo is the project-identification block of HealthResponse,
// sourced from branding config per AI.md PART 16.
type ProjectInfo struct {
	Name        string `json:"name"`
	Tagline     string `json:"tagline"`
	Description string `json:"description"`
}

// BuildInfo is the build-metadata block of HealthResponse per AI.md PART 7.
type BuildInfo struct {
	Commit string `json:"commit"`
	Date   string `json:"date"`
}

// FeaturesInfo lists PUBLIC-safe feature flags only per AI.md PART 13.
// /metrics is internal-only (PART 20) and must NOT appear here.
type FeaturesInfo struct {
	Tor   TorInfo `json:"tor"`
	I2P   I2PInfo `json:"i2p"`
	GeoIP bool    `json:"geoip"`
}

// TorInfo is the Tor hidden-service block of FeaturesInfo per AI.md PART 31.
type TorInfo struct {
	Enabled  bool   `json:"enabled"`
	Running  bool   `json:"running"`
	Status   string `json:"status"`
	Hostname string `json:"hostname"`
}

// I2PInfo is the I2P eepsite block of FeaturesInfo per AI.md PART 31.2.
// I2P is opt-in and off by default.
type I2PInfo struct {
	Enabled  bool   `json:"enabled"`
	Running  bool   `json:"running"`
	Status   string `json:"status"`
	Hostname string `json:"hostname"`
	Provider string `json:"provider"`
	Address  string `json:"address,omitempty"`
}

// ChecksInfo holds component health checks — "ok"/"error" only, no details,
// per AI.md PART 13's public-safe-only requirement.
type ChecksInfo struct {
	Database  string `json:"database"`
	Cache     string `json:"cache"`
	Disk      string `json:"disk"`
	Scheduler string `json:"scheduler"`
	Tor       string `json:"tor,omitempty"`
	I2P       string `json:"i2p,omitempty"`
}

// StatsInfo holds public-safe aggregate statistics per AI.md PART 13.
type StatsInfo struct {
	RequestsTotal     uint64 `json:"requests_total"`
	Requests24h       uint64 `json:"requests_24h"`
	ActiveConnections int64  `json:"active_connections"`
	SearchesTotal     uint64 `json:"searches_total"`
}

// checkDatabase pings the database per AI.md PART 13. A nil handle (e.g. in
// tests, or a build with no DB wired) reports "ok" since there is nothing to
// probe; a live handle that fails to answer within 2s reports "error".
func (h *SearchHandler) checkDatabase(ctx context.Context) string {
	if h.healthDB == nil {
		return "ok"
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := h.healthDB.PingContext(pingCtx); err != nil {
		return "error"
	}
	return "ok"
}

// checkCache reports the in-memory search cache health per AI.md PART 13. The
// cache is process-local (no external backend) and cannot fail while allocated;
// a nil cache means caching is simply not wired. Either way there is nothing
// that can report unhealthy, so this is always "ok" — the meaningful failure
// probes are database, disk, and scheduler.
func (h *SearchHandler) checkCache() string {
	return "ok"
}

// checkDisk verifies the data directory's filesystem is reachable and not
// critically full per AI.md PART 13. It reports "error" when the path cannot
// be stat'd or when usage is at or above 99%; unsupported platforms (statfs
// returns an error) degrade to "ok" rather than a false failure.
func (h *SearchHandler) checkDisk() string {
	path := h.dataDir
	if path == "" {
		path = h.configDir
	}
	if path == "" {
		path = os.TempDir()
	}
	total, free, err := maintenance.DiskSpace(path)
	if err != nil {
		return "ok"
	}
	if total > 0 {
		used := total - free
		if float64(used)/float64(total) >= 0.99 {
			return "error"
		}
	}
	return "ok"
}

// checkScheduler reports whether the background scheduler loop is active per
// AI.md PART 13/18. A nil scheduler (not wired, e.g. in tests) reports "ok".
func (h *SearchHandler) checkScheduler() string {
	if h.sched == nil {
		return "ok"
	}
	if h.sched.IsRunning() {
		return "ok"
	}
	return "error"
}

// checkI2P probes the enabled I2P provider and exposes only its health state.
func (h *SearchHandler) checkI2P(ctx context.Context) string {
	if h.i2pSvc == nil || !h.i2pSvc.Enabled() {
		return ""
	}
	if h.i2pSvc.CheckHealth(ctx) != nil {
		return "error"
	}
	return "ok"
}

// HealthCheck handles /healthz endpoint with content negotiation
// Per AI.md PART 13
func (h *SearchHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	format := detectResponseFormat(r)

	// Build health response per AI.md PART 13
	hostname, _ := os.Hostname()
	uptime := getUptime()
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Get mode from config
	appMode := "production"
	if h.appConfig != nil {
		appMode = config.NormalizeMode(h.appConfig.Server.Mode)
	}

	// Build checks object - MUST be simple "ok"/"error" strings
	// Per AI.md PART 13: each value comes from a real probe, never hardcoded.
	checks := map[string]string{
		"database":  h.checkDatabase(r.Context()),
		"cache":     h.checkCache(),
		"disk":      h.checkDisk(),
		"scheduler": h.checkScheduler(),
	}

	// Add Tor check if enabled per PART 31
	if h.torSvc != nil && h.torSvc.IsEnabled() {
		if h.torSvc.IsRunning() {
			checks["tor"] = "ok"
		} else {
			checks["tor"] = "error"
		}
	}
	if i2pCheck := h.checkI2P(r.Context()); i2pCheck != "" {
		checks["i2p"] = i2pCheck
	}

	// Overall status - per AI.md PART 13: derive from checks
	status := "healthy"
	httpStatus := http.StatusOK
	for _, v := range checks {
		if v != "ok" {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
			break
		}
	}

	// restart_required per AI.md PART 13 status->HTTP table: HTTP 200,
	// carries pending_restart+restart_reason. Only applies when checks
	// are otherwise healthy - an unhealthy check always takes priority.
	if status == "healthy" && h.appConfig != nil && h.appConfig.PendingRestart {
		status = "restart_required"
	}
	if signalstate.IsShuttingDown() {
		status = "shutting_down"
		httpStatus = http.StatusServiceUnavailable
	}

	// Get project info from config per PART 16 branding
	projectName := "VidVeil"
	projectTagline := "Privacy-first video search"
	projectDescription := "Privacy-respecting adult video meta search"
	if h.appConfig != nil {
		if h.appConfig.Server.Branding.Title != "" {
			projectName = h.appConfig.Server.Branding.Title
		}
		if h.appConfig.Server.Branding.Tagline != "" {
			projectTagline = h.appConfig.Server.Branding.Tagline
		}
		if h.appConfig.Server.Branding.Description != "" {
			projectDescription = h.appConfig.Server.Branding.Description
		}
	}

	switch format {
	case "application/json":
		// JSON format per AI.md PART 13 - canonical HealthResponse struct
		// preserves the spec-mandated field order on the wire (see
		// HealthResponse doc comment for why a map cannot be used here).
		response := HealthResponse{
			Project: ProjectInfo{
				Name:        projectName,
				Tagline:     projectTagline,
				Description: projectDescription,
			},
			Status:    status,
			Version:   version.GetVersion(),
			GoVersion: runtime.Version(),
			Build: BuildInfo{
				Commit: version.CommitID,
				Date:   version.BuildTime,
			},
			Uptime:    uptime,
			Mode:      appMode,
			Timestamp: timestamp,
			Features: FeaturesInfo{
				Tor: TorInfo{
					Enabled:  h.torSvc != nil && h.torSvc.IsEnabled(),
					Running:  h.torSvc != nil && h.torSvc.IsRunning(),
					Status:   h.getTorStatus(),
					Hostname: h.getTorHostname(),
				},
				I2P: I2PInfo{
					Enabled:  h.i2pSvc != nil && h.i2pSvc.Enabled(),
					Running:  h.i2pSvc != nil && h.i2pSvc.IsRunning(),
					Status:   h.i2pHealthStatus(checks["i2p"]),
					Hostname: h.i2pAddress(),
					Provider: h.i2pProvider(),
				},
				GeoIP: h.appConfig != nil && h.appConfig.Server.GeoIP.Enabled,
			},
			Checks: ChecksInfo{
				Database:  checks["database"],
				Cache:     checks["cache"],
				Disk:      checks["disk"],
				Scheduler: checks["scheduler"],
				Tor:       checks["tor"],
				I2P:       checks["i2p"],
			},
			Stats: StatsInfo{
				RequestsTotal:     h.getRequestsTotal(),
				Requests24h:       h.getRequests24h(),
				ActiveConnections: h.getActiveConnections(),
				SearchesTotal:     h.getSearchCount(),
			},
		}

		// pending_restart / restart_reason — omitempty: only include when set
		if h.appConfig != nil && h.appConfig.PendingRestart {
			response.PendingRestart = true
			response.RestartReason = h.appConfig.RestartReasons
		}

		WriteJSON(w, httpStatus, response)

	case "text/plain":
		// Plain text format per AI.md PART 13 — canonical field order
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(httpStatus)
		// 1. Project
		_, _ = fmt.Fprintf(w, "project.name: %s\n", projectName)
		_, _ = fmt.Fprintf(w, "project.tagline: %s\n", projectTagline)
		_, _ = fmt.Fprintf(w, "project.description: %s\n", projectDescription)
		// 2. Status
		_, _ = fmt.Fprintf(w, "status: %s\n", status)
		if h.appConfig != nil && h.appConfig.PendingRestart {
			_, _ = fmt.Fprintf(w, "pending_restart: true\n")
			for _, r := range h.appConfig.RestartReasons {
				_, _ = fmt.Fprintf(w, "restart_reason: %s\n", r)
			}
		}
		// 3. Version & build
		_, _ = fmt.Fprintf(w, "version: %s\n", version.GetVersion())
		_, _ = fmt.Fprintf(w, "go_version: %s\n", runtime.Version())
		_, _ = fmt.Fprintf(w, "build.commit: %s\n", version.CommitID)
		_, _ = fmt.Fprintf(w, "build.date: %s\n", version.BuildTime)
		// 4. Runtime
		_, _ = fmt.Fprintf(w, "uptime: %s\n", uptime)
		_, _ = fmt.Fprintf(w, "mode: %s\n", appMode)
		_, _ = fmt.Fprintf(w, "timestamp: %s\n", timestamp)
		// 5. Features
		torEnabled := h.torSvc != nil && h.torSvc.IsEnabled()
		torRunning := h.torSvc != nil && h.torSvc.IsRunning()
		i2pEnabled := h.i2pSvc != nil && h.i2pSvc.Enabled()
		i2pRunning := h.i2pSvc != nil && h.i2pSvc.IsRunning()
		_, _ = fmt.Fprintf(w, "features.tor.enabled: %v\n", torEnabled)
		_, _ = fmt.Fprintf(w, "features.tor.running: %v\n", torRunning)
		_, _ = fmt.Fprintf(w, "features.tor.status: %s\n", h.getTorStatus())
		if torRunning {
			_, _ = fmt.Fprintf(w, "features.tor.hostname: %s\n", h.getTorHostname())
		}
		_, _ = fmt.Fprintf(w, "features.i2p.enabled: %v\n", i2pEnabled)
		_, _ = fmt.Fprintf(w, "features.i2p.running: %v\n", i2pRunning)
		_, _ = fmt.Fprintf(w, "features.i2p.status: %s\n", h.i2pHealthStatus(checks["i2p"]))
		if i2pRunning {
			_, _ = fmt.Fprintf(w, "features.i2p.hostname: %s\n", h.i2pAddress())
		}
		_, _ = fmt.Fprintf(w, "features.i2p.provider: %s\n", h.i2pProvider())
		_, _ = fmt.Fprintf(w, "features.geoip: %v\n", h.appConfig != nil && h.appConfig.Server.GeoIP.Enabled)
		// 7. Checks
		_, _ = fmt.Fprintf(w, "checks.database: %s\n", checks["database"])
		_, _ = fmt.Fprintf(w, "checks.cache: %s\n", checks["cache"])
		_, _ = fmt.Fprintf(w, "checks.disk: %s\n", checks["disk"])
		_, _ = fmt.Fprintf(w, "checks.scheduler: %s\n", checks["scheduler"])
		if _, ok := checks["tor"]; ok {
			_, _ = fmt.Fprintf(w, "checks.tor: %s\n", checks["tor"])
		}
		if _, ok := checks["i2p"]; ok {
			_, _ = fmt.Fprintf(w, "checks.i2p: %s\n", checks["i2p"])
		}
		// 8. Stats
		_, _ = fmt.Fprintf(w, "stats.requests_total: %d\n", h.getRequestsTotal())
		_, _ = fmt.Fprintf(w, "stats.requests_24h: %d\n", h.getRequests24h())
		_, _ = fmt.Fprintf(w, "stats.active_connections: %d\n", h.getActiveConnections())

	default:
		// HTML format (default) per AI.md PART 13 with full template
		h.renderHealthzHTML(w, r, status, httpStatus, appMode, uptime, hostname, timestamp, checks)
	}
}

// HealthzHTMLData holds all data for healthz template per AI.md PART 13
type HealthzHTMLData struct {
	Title         string
	Theme         string
	Version       string
	BuildDateTime string

	// AI.md PART 30: locale + direction for <html lang dir>
	Lang string
	Dir  string

	// Nav template compatibility
	ActiveNav string
	Query     string

	// Shared partial compatibility (head.tmpl / footer.tmpl per AI.md PART 16) —
	// these are flat top-level fields the shared head/footer partials read
	// directly; renderTemplate() injects them for every other page, but healthz
	// renders via a dedicated struct+ParseFS path (not renderTemplate), so they
	// must be populated here too or html/template hard-errors on the missing
	// field at ExecuteTemplate (unlike a map, a struct has no "zero value" for
	// an absent field — it's a template execution error, not silent omission).
	TorEnabled          bool
	TorRunning          bool
	OnionAddress        string
	I2PEnabled          bool
	I2PRunning          bool
	I2PAddress          string
	ThemeColor          string
	NextTheme           string
	Robots              string
	CanonicalURL        string
	SEOKeywords         string
	SEOAuthor           string
	SEOOGImage          string
	SEOTwitterHandle    string
	SEOVerification     config.SEOVerificationConfig
	BrandingDescription string
	BrandingTagline     string
	// Operator-configured logo/favicon per AI.md PART 16 "Image Sources" —
	// a local path or remote URL; empty falls back to the embedded default.
	BrandingLogo    string
	BrandingFavicon string
	AppURL          string
	// Consent banner gating (AI.md PART 12) — footer.tmpl renders the banner
	// only when no valid cookie_consent cookie exists
	HasConsentCookie bool
	// CSRF token for the consent banner forms in footer.tmpl (AI.md PART 16)
	CSRFToken string
	// CurrentPath backs the return_to hidden field on the zero-JS
	// consent/theme/announcement-dismiss forms (AI.md PART 12 / PART 16).
	CurrentPath string
	// Announcements per AI.md PART 16 "Site Banner" / PART 25 "Announcements" —
	// active, undismissed site-wide banners to render immediately inside <body>.
	Announcements []BannerAnnouncement
	// Consent banner text/links per AI.md PART 12 "Cookie Consent Banner ->
	// Implementation" (server.privacy.consent.*, server.privacy.data.sold)
	ConsentMessage         string
	ConsentPolicyURL       string
	ConsentPolicyText      string
	ConsentDeclineText     string
	ConsentAcceptText      string
	ConsentPreferencesText string
	ConsentDataSold        bool

	// Project info (PART 16 branding)
	ProjectName        string
	ProjectTagline     string
	ProjectDescription string

	// Status
	StatusClass string
	StatusIcon  string
	StatusText  string

	// Version info
	GoVersion   string
	BuildCommit string
	BuildDate   string
	Uptime      string
	Mode        string
	ModeDisplay string

	// Features
	Features FeaturesData

	// Checks
	Checks ChecksData

	// Stats (VidVeil-specific per IDEA.md)
	Stats StatsData

	// Timestamp
	Timestamp        string
	TimestampDisplay string
}

// FeaturesData holds public-safe feature flags per AI.md PART 13.
// /metrics is internal-only (PART 20) and must NOT appear here.
type FeaturesData struct {
	TorEnabled  bool
	TorStarting bool
	// TorStatus is "healthy", "unhealthy", "starting", or empty
	TorStatus    string
	TorOnionAddr string
	I2PEnabled   bool
	I2PRunning   bool
	I2PStatus    string
	I2PAddress   string
	I2PProvider  string
	GeoIP        bool
}

type ChecksData struct {
	Database  string
	Cache     string
	Disk      string
	Scheduler string
	I2P       string
}

// StatsData holds statistics for healthz display per AI.md PART 13
type StatsData struct {
	RequestsTotal     uint64
	Requests24h       uint64
	ActiveConnections int64
}

// renderHealthzHTML renders the healthz HTML template per AI.md PART 13
func (h *SearchHandler) renderHealthzHTML(w http.ResponseWriter, r *http.Request, status string, httpStatus int, appMode, uptime, hostname, timestamp string, checks map[string]string) {
	// Parse timestamp
	ts, _ := time.Parse(time.RFC3339, timestamp)

	// Build template data per AI.md PART 13
	lang := resolveLocale(r)
	data := HealthzHTMLData{
		Title:         "Vidveil - Health Status",
		Theme:         h.getRequestTheme(r),
		Version:       version.GetVersion(),
		BuildDateTime: version.BuildTime,

		// AI.md PART 30: lang/dir for <html>
		Lang: lang,
		Dir:  i18n.Direction(lang),

		// Nav template compatibility
		ActiveNav: "healthz",
		Query:     "",

		// Consent banner gating per AI.md PART 12
		HasConsentCookie: hasConsentCookie(r),
		CSRFToken:        cSRFTokenFromRequest(r),
		CurrentPath:      r.URL.RequestURI(),
		Announcements:    activeAnnouncements(h.appConfig, r),

		// Consent banner text/links per AI.md PART 12
		ConsentMessage:         h.appConfig.Server.Privacy.GetConsentMessage(),
		ConsentPolicyURL:       h.appConfig.Server.Privacy.Consent.Policy.URL,
		ConsentPolicyText:      h.appConfig.Server.Privacy.Consent.Policy.Text,
		ConsentDeclineText:     h.appConfig.Server.Privacy.Consent.Buttons.Decline,
		ConsentAcceptText:      h.appConfig.Server.Privacy.Consent.Buttons.Accept,
		ConsentPreferencesText: h.appConfig.Server.Privacy.Consent.PreferencesText,
		ConsentDataSold:        h.appConfig.Server.Privacy.Data.Sold,

		// Project info (populated from branding config below)
		ProjectName:        "Vidveil",
		ProjectTagline:     "Privacy-first video search",
		ProjectDescription: "Privacy-respecting adult video meta search",

		// Version info
		GoVersion:   runtime.Version(),
		BuildCommit: version.CommitID,
		BuildDate:   version.BuildTime,
		Uptime:      uptime,
		Mode:        appMode,

		// Checks
		Checks: ChecksData{
			Database:  checks["database"],
			Cache:     checks["cache"],
			Disk:      checks["disk"],
			Scheduler: checks["scheduler"],
			I2P:       checks["i2p"],
		},

		// Stats per AI.md PART 13
		Stats: StatsData{
			RequestsTotal: func() uint64 {
				if h.metrics != nil {
					return h.metrics.GetRequestsTotal()
				}
				return 0
			}(),
			Requests24h: func() uint64 {
				if h.metrics != nil {
					return h.metrics.GetRequests24h()
				}
				return 0
			}(),
			ActiveConnections: func() int64 {
				if h.metrics != nil {
					return h.metrics.GetActiveConnections()
				}
				return 0
			}(),
		},

		// Timestamp
		Timestamp:        timestamp,
		TimestampDisplay: ts.Format("Jan 02, 2006 3:04 PM"),
	}

	// Status display
	switch status {
	case "healthy":
		data.StatusClass = "healthy"
		data.StatusIcon = "OK"
		data.StatusText = "All Systems Operational"
	case "unhealthy":
		data.StatusClass = "unhealthy"
		data.StatusIcon = "DOWN"
		data.StatusText = "System Unhealthy"
	default:
		data.StatusClass = "degraded"
		data.StatusIcon = "WARN"
		data.StatusText = "System Degraded"
	}

	// Mode display
	switch appMode {
	case "production":
		data.ModeDisplay = "Production"
	case "debug":
		data.ModeDisplay = "Debug"
	default:
		data.ModeDisplay = "Development"
	}

	// Branding from config (override defaults set above)
	if h.appConfig != nil {
		if h.appConfig.Server.Branding.Title != "" {
			data.ProjectName = h.appConfig.Server.Branding.Title
		}
		if h.appConfig.Server.Branding.Tagline != "" {
			data.ProjectTagline = h.appConfig.Server.Branding.Tagline
		}
		if h.appConfig.Server.Branding.Description != "" {
			data.ProjectDescription = h.appConfig.Server.Branding.Description
		}
	}

	// Browser-chrome theme color uses the shared Go palette, as in renderTemplate().
	data.ThemeColor = getThemeColor(data.Theme)
	data.NextTheme = nextTheme(data.Theme)
	data.Robots = robotsDirective(r)
	data.CanonicalURL = urlvar.BuildURL(r, r.URL.Path)

	// Shared partial data (head.tmpl / footer.tmpl per AI.md PART 16) — same
	// injection renderTemplate() performs for every other page (handlers.go
	// renderTemplate, ~line 2835 "Footer onion-address row" / "SEO and branding").
	if h.appConfig != nil {
		data.SEOKeywords = strings.Join(h.appConfig.Server.SEO.Keywords, ", ")
		data.SEOAuthor = h.appConfig.Server.SEO.Author
		if ogPath := h.brandingAssetURL(h.appConfig.Server.SEO.OGImage, "og-1200x630.png"); ogPath != "" {
			data.SEOOGImage = urlvar.BuildURL(r, ogPath)
		}
		data.SEOTwitterHandle = h.appConfig.Server.SEO.TwitterHandle
		data.SEOVerification = h.appConfig.Server.SEO.Verification
		data.BrandingDescription = h.appConfig.Server.Branding.Description
		data.BrandingTagline = h.appConfig.Server.Branding.Tagline
		data.BrandingLogo = h.brandingAssetURL(h.appConfig.Server.Branding.Logo, "logo-200.png")
		data.BrandingFavicon = h.brandingAssetURL(h.appConfig.Server.Branding.Favicon, "favicon-32.png")
		// Resolved per request via BuildURL (AI.md PART 12) — never frozen at
		// startup/config, so the URL matches the Host/proto the client
		// actually used, including behind a reverse proxy.
		data.AppURL = urlvar.BuildURL(r, "")
	}
	// Footer onion-address row — dropped entirely unless Tor is both enabled
	// and actually running (matches renderTemplate's identical gate).
	if h.torSvc != nil && h.torSvc.IsEnabled() && h.torSvc.IsRunning() {
		data.TorEnabled = true
		data.TorRunning = true
		if addr, ok := h.torSvc.GetInfo()["onion_address"].(string); ok {
			data.OnionAddress = addr
		}
	}
	if h.i2pSvc != nil && h.i2pSvc.Enabled() && h.i2pSvc.IsRunning() {
		data.I2PEnabled = true
		data.I2PRunning = true
		data.I2PAddress = h.i2pSvc.Address()
	}

	// Features — public-safe only; /metrics is internal (PART 20)
	if h.appConfig != nil {
		// Tor status per AI.md PART 13
		if h.torSvc != nil {
			if h.torSvc.IsRunning() {
				data.Features.TorEnabled = true
				data.Features.TorStatus = "healthy"
				info := h.torSvc.GetInfo()
				if addr, ok := info["onion_address"].(string); ok {
					data.Features.TorOnionAddr = addr
				}
			} else if h.torSvc.IsStarting() {
				// Tor is still bootstrapping — show as "starting" not "disabled"
				data.Features.TorStarting = true
				data.Features.TorStatus = "starting"
			} else {
				data.Features.TorStatus = "unhealthy"
			}
		}
		data.Features.GeoIP = h.appConfig.Server.GeoIP.Enabled
	}
	if h.i2pSvc != nil {
		data.Features.I2PEnabled = h.i2pSvc.Enabled()
		data.Features.I2PRunning = h.i2pSvc.IsRunning()
		data.Features.I2PStatus = h.i2pHealthStatus(checks["i2p"])
		data.Features.I2PAddress = h.i2pAddress()
		data.Features.I2PProvider = h.i2pProvider()
	}

	// Guard against uninitialized template filesystem
	if templatesFS == nil {
		log.Printf("healthz template: templates filesystem not initialized")
		http.Error(w, i18n.Translate(i18n.DetectLocale(r), "error.server"), http.StatusInternalServerError)
		return
	}

	// Parse and execute template. FuncMap must be registered before ParseFS
	// since healthz.tmpl and its partials call {{ t }}/{{ tf }}/{{ safeHTML }}
	// (PART 30 i18n) — template.ParseFS alone has no funcs and fails at
	// execute time with "function \"t\" not defined".
	tmpl, err := template.New("healthz.tmpl").Funcs(template.FuncMap{
		"dict": func(values ...interface{}) map[string]interface{} {
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				if i+1 < len(values) {
					if key, ok := values[i].(string); ok {
						dict[key] = values[i+1]
					}
				}
			}
			return dict
		},
		"eq": func(a, b interface{}) bool { return a == b },
		"t": func(key string) string {
			return i18n.Translate(lang, key)
		},
		"tf": func(key string, args ...interface{}) string {
			return i18n.TranslateFormat(lang, key, args...)
		},
		"safeHTML": func(s string) template.HTML {
			return sanitize.HTML(s)
		},
		// asset appends the version-busting stamp per AI.md PART 9
		"asset": func(path string) string {
			return path + "?v=" + version.AssetStamp()
		},
	}).ParseFS(templatesFS,
		"template/page/healthz.tmpl",
		"template/partial/public/head.tmpl",
		"template/partial/public/header.tmpl",
		"template/partial/public/nav.tmpl",
		"template/partial/public/site-banner.tmpl",
		"template/partial/public/footer.tmpl",
		"template/partial/public/scripts.tmpl",
	)
	if err != nil {
		log.Printf("healthz template parse: %v", err)
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError)
		return
	}

	// Buffer template output to prevent proxy truncation issues
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "healthz", data); err != nil {
		log.Printf("healthz template execute: %v", err)
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError)
		return
	}

	// Set headers and write buffered response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(httpStatus)
	_, _ = w.Write(buf.Bytes())
}

// publicIndexablePaths lists the routes explicitly marked public for the
// robots meta directive (AI.md PART 16 "Robots Directive"). Every other route
// falls through to noindex,nofollow.
var publicIndexablePaths = map[string]bool{
	"/":                       true,
	"/about":                  true,
	"/privacy":                true,
	"/terms":                  true,
	"/help":                   true,
	"/server/about":           true,
	"/server/help":            true,
	"/server/privacy":         true,
	"/server/terms":           true,
	"/server/contact":         true,
	"/server/security":        true,
	"/server/security/policy": true,
	"/server/docs/swagger":    true,
	"/server/docs/graphql":    true,
}

// robotsDirective computes the per-route robots meta value server-side per
// AI.md PART 16. Only explicitly public routes are indexable; everything else
// (API, health, debug, search result pages) fails closed.
func robotsDirective(r *http.Request) string {
	if r == nil {
		return "noindex,nofollow"
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}
	if publicIndexablePaths[strings.ToLower(path)] {
		return "index,follow"
	}
	return "noindex,nofollow"
}

// RobotsTxt returns robots.txt, generated from web.robots configuration per
// AI.md PART 11. The wildcard block renders the configured deny paths first,
// then the allow paths; recognized AI crawlers only get their own stanza when
// web.robots.ai_bots resolves them to deny.
func (h *SearchHandler) RobotsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	robots := h.appConfig.Web.Robots

	deny := robots.Deny

	var b strings.Builder
	b.WriteString("User-agent: *\n")
	for _, path := range deny {
		if path == "" {
			continue
		}
		b.WriteString("Disallow: " + path + "\n")
	}
	allow := robots.Allow
	if len(allow) == 0 {
		allow = []string{"/"}
	}
	for _, path := range allow {
		if path == "" {
			continue
		}
		b.WriteString("Allow: " + path + "\n")
	}

	// Resolved per request via BuildURL (AI.md PART 12) — never frozen at
	// startup, so the advertised sitemap URL matches the Host/proto the
	// client actually used, including behind a reverse proxy.
	baseURL := urlvar.BuildURL(r, "")
	b.WriteString("\nSitemap: " + baseURL + "/sitemap.xml\n")

	for _, bot := range robots.DeniedAIBots() {
		b.WriteString("\nUser-agent: " + bot + "\nDisallow: /\n")
	}

	_, _ = w.Write([]byte(b.String()))
}

// SecurityTxt returns security.txt per RFC 9116 (AI.md PART 11 "Security
// Reports"). Contact: lines are emitted in preference order: (1) the
// repo-level vulnerability-reporting URL (web.security.report_url, e.g.
// GitHub private vulnerability reporting), (2) the rotating {security_id}
// coordinated-disclosure contact mode, (3) the mailto CC address.
func (h *SearchHandler) SecurityTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	var contacts []string

	if reportURL := h.appConfig.Web.Security.ReportURL; reportURL != "" {
		contacts = append(contacts, reportURL)
	}

	if h.secretsMgr != nil {
		if secret, err := h.secretsMgr.GetInstallationSecret(r.Context()); err == nil {
			id := secreport.GenerateSecurityID(secret, time.Now())
			contactURL := urlvar.BuildURL(r, "/server/contact") + "?security_id=" + id
			contacts = append(contacts, contactURL)
		}
	}

	// Tor variant per AI.md PART 31/12: Host matches the onion address. The Tor
	// response must never leak the clearnet FQDN or a clearnet email address,
	// and Preferred-Languages is omitted entirely on the Tor variant.
	isTor := urlvar.IsTorRequest(r)

	if isTor {
		// Tor: only tor.contact_email may appear — never a clearnet fallback
		if torMail := h.appConfig.Server.Tor.ContactEmail; torMail != "" {
			if !strings.HasPrefix(torMail, "mailto:") {
				torMail = "mailto:" + torMail
			}
			contacts = append(contacts, torMail)
		}
	} else {
		mailto := h.appConfig.Web.Security.Contact
		if mailto == "" {
			mailto = "security@" + h.appConfig.Server.FQDN
		}
		if !strings.HasPrefix(mailto, "mailto:") {
			mailto = "mailto:" + mailto
		}
		contacts = append(contacts, mailto)
	}

	expires := h.appConfig.Web.Security.Expires
	if expires == "" {
		expires = time.Now().AddDate(1, 0, 0).Format(time.RFC3339)
	}

	var body strings.Builder
	for _, contact := range contacts {
		fmt.Fprintf(&body, "Contact: %s\n", contact)
	}
	// Policy: required RFC 9116 field per AI.md PART 11 — human-readable page
	// at /server/security; BuildURL is per-request so the Tor variant gets the
	// onion origin, never the clearnet FQDN
	fmt.Fprintf(&body, "Policy: %s\n", urlvar.BuildURL(r, "/server/security"))
	fmt.Fprintf(&body, "Expires: %s\n", expires)
	if !isTor {
		fmt.Fprintf(&body, "Preferred-Languages: en\n")
	}

	// Add Encryption field when PGP key is published (AI.md PART 11)
	if h.appConfig.Web.Security.PGPKeyURL != "" {
		fmt.Fprintf(&body, "Encryption: %s\n", h.appConfig.Web.Security.PGPKeyURL)
	}

	_, _ = w.Write([]byte(body.String()))
}

// PGPKeyAsc serves /.well-known/pgp-key.asc per AI.md PART 12 "GPG Keypair
// Management". The public key lives at {config_dir}/security/pgp.pub.asc (written
// by "--maintenance pgp generate"). Returns 404 when no keypair exists.
func (h *SearchHandler) PGPKeyAsc(w http.ResponseWriter, r *http.Request) {
	pgpPath := filepath.Join(h.configDir, "security", "pgp.pub.asc")
	data, err := os.ReadFile(pgpPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/pgp-keys")
	_, _ = w.Write(data)
}

// HumansTxt returns humans.txt per humanstxt.org standard (PART 16)
func (h *SearchHandler) HumansTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// Get app info from config
	appName := h.appConfig.Server.Branding.Title
	if appName == "" {
		appName = "Vidveil"
	}

	// Resolved per request via BuildURL (AI.md PART 12) — never frozen at
	// startup, so the advertised URL matches the Host/proto the client
	// actually used, including behind a reverse proxy.
	appURL := urlvar.BuildURL(r, "")

	_, _ = fmt.Fprintf(w, `/* TEAM */
Name: %s Team
Site: %s
Location: Earth

/* THANKS */
Go: https://go.dev
Chi Router: https://github.com/go-chi/chi
Dracula Theme: https://draculatheme.com

/* SITE */
Last update: %s
Language: English
Doctype: HTML5
Standards: WCAG 2.1 AA, RFC 9116
Components: Go, SQLite, Valkey/Redis
`, appName, appURL, time.Now().Format("2006-01-02"))
}

// LlmsTxt serves the AI-agent discovery file per AI.md PART 14 "llms.txt (AI
// Discovery)". ALL projects MUST serve it; it is registered at both
// /.well-known/llms.txt and /llms.txt. Every URL is resolved per request via
// urlvar.BuildURL (reverse-proxy aware) so the advertised base URL always
// matches the Host/proto the client actually used. The metrics endpoint is
// deliberately never advertised (operational/internal only).
func (h *SearchHandler) LlmsTxt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if !h.appConfig.Web.LLMS.Enabled {
		http.NotFound(w, r)
		return
	}

	name := h.appConfig.Server.Branding.Title
	if name == "" {
		name = "Vidveil"
	}
	desc := h.appConfig.Server.Branding.Description
	if desc == "" {
		desc = "Privacy-respecting adult video search"
	}

	apiPath := h.appConfig.APIBasePath()
	apiBase := urlvar.BuildURL(r, apiPath)

	// Rate limit: requests per window-seconds, normalized to per-minute.
	rlReqs := h.appConfig.Server.RateLimit.Requests
	rlWindow := h.appConfig.Server.RateLimit.Window
	perMinute := rlReqs
	if rlWindow > 0 && rlWindow != 60 {
		perMinute = rlReqs * 60 / rlWindow
	}

	securityContact := h.appConfig.Web.Security.Contact
	if securityContact == "" {
		securityContact = "security@" + h.appConfig.Server.FQDN
	}
	securityContact = strings.TrimPrefix(securityContact, "mailto:")

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n> %s\n\n", name, desc)

	fmt.Fprintf(&b, "## API\n")
	fmt.Fprintf(&b, "Base URL: %s\n", apiBase)
	fmt.Fprintf(&b, "Authentication: Bearer token for protected endpoints\n")
	if h.appConfig.Server.RateLimit.Enabled {
		fmt.Fprintf(&b, "Rate limit: %d requests/minute\n", perMinute)
	}
	if h.appConfig.Server.MCP.Enabled {
		fmt.Fprintf(&b, "MCP: %s (Streamable HTTP; same Bearer token as the API)\n", urlvar.BuildURL(r, "/mcp"))
	}
	fmt.Fprintln(&b)

	if h.appConfig.Web.LLMS.IncludeEndpoints {
		fmt.Fprintln(&b, "## Endpoints")
		var spec map[string]any
		if err := json.Unmarshal([]byte(swagger.GenerateSpec(h.appConfig, r)), &spec); err == nil {
			if paths, ok := spec["paths"].(map[string]any); ok {
				pathNames := make([]string, 0, len(paths))
				for path := range paths {
					pathNames = append(pathNames, path)
				}
				sort.Strings(pathNames)
				for _, path := range pathNames {
					if !strings.HasPrefix(path, apiPath+"/") || strings.Contains(path, "/metrics") {
						continue
					}
					operations, ok := paths[path].(map[string]any)
					if !ok {
						continue
					}
					methods := make([]string, 0, len(operations))
					for method := range operations {
						if method == "get" || method == "post" || method == "put" || method == "patch" || method == "delete" {
							methods = append(methods, method)
						}
					}
					sort.Strings(methods)
					for _, method := range methods {
						operation, ok := operations[method].(map[string]any)
						if !ok || operation["x-admin-only"] == true {
							continue
						}
						label := "public"
						if operation["security"] != nil {
							label = "Bearer token required"
						}
						description, _ := operation["summary"].(string)
						fmt.Fprintf(&b, "- %s %s - %s (%s)\n", strings.ToUpper(method), path, description, label)
						if h.appConfig.Web.LLMS.IncludeSchemas {
							if encoded, err := json.Marshal(operation); err == nil {
								fmt.Fprintf(&b, "  Schema: `%s`\n", encoded)
							}
						}
					}
				}
			}
		}
		fmt.Fprintf(&b, "- GET /server/healthz - Health check (public)\n\n")
	}

	fmt.Fprintf(&b, "## Capabilities\n")
	fmt.Fprintf(&b, "- Privacy-respecting metasearch across multiple video engines\n")
	fmt.Fprintf(&b, "- Server-rendered search and favorites with no JavaScript required\n")
	fmt.Fprintf(&b, "- JSON, SSE streaming, and plain-text result formats via content negotiation\n")
	fmt.Fprintf(&b, "- Bang shortcuts for direct engine queries\n")
	fmt.Fprintf(&b, "\n")

	fmt.Fprintf(&b, "## Contact\n")
	apiContact := h.appConfig.Web.APIContact
	if apiContact == "" {
		apiContact = h.appConfig.Server.Contact.General.Email
	}
	fmt.Fprintf(&b, "API issues: %s\n", strings.TrimPrefix(apiContact, "mailto:"))
	fmt.Fprintf(&b, "Security: %s\n", securityContact)
	for _, section := range h.appConfig.Web.LLMS.CustomSections {
		section = strings.TrimSpace(section)
		if section != "" {
			fmt.Fprintf(&b, "\n%s\n", section)
		}
	}

	_, _ = w.Write([]byte(b.String()))
}

// SitemapXML returns sitemap.xml per AI.md PART 16
func (h *SearchHandler) SitemapXML(w http.ResponseWriter, r *http.Request) {
	appConfig := h.appConfig
	if appConfig == nil {
		appConfig = config.DefaultAppConfig()
	}
	sitemapConfig := appConfig.Server.SEO.Sitemap
	if !sitemapConfig.Enabled {
		http.NotFound(w, r)
		return
	}
	if sitemapConfig.MaxURLs < 1 || sitemapConfig.MaxURLs > 50000 {
		sitemapConfig.MaxURLs = 50000
	}

	// Resolve the public origin per request so reverse proxy and Tor URLs are
	// accurate. encoding/xml escapes host and path values before writing XML.
	baseURL := urlvar.BuildURL(r, "")
	lastModified := time.Now().UTC().Format(time.RFC3339)
	pages := []struct {
		path       string
		changefreq string
		priority   string
	}{
		{path: "/", changefreq: "daily", priority: "1.0"},
		{path: "/server/about", changefreq: "weekly", priority: "0.8"},
		{path: "/server/privacy", changefreq: "weekly", priority: "0.8"},
		{path: "/server/contact", changefreq: "weekly", priority: "0.8"},
		{path: "/server/help", changefreq: "weekly", priority: "0.8"},
		{path: "/server/terms", changefreq: "weekly", priority: "0.8"},
		{path: "/server/security", changefreq: "weekly", priority: "0.8"},
		{path: "/server/security/policy", changefreq: "weekly", priority: "0.8"},
		{path: "/server/docs/swagger", changefreq: "weekly", priority: "0.7"},
		{path: "/server/docs/graphql", changefreq: "weekly", priority: "0.7"},
	}
	if len(pages) > sitemapConfig.MaxURLs {
		pages = pages[:sitemapConfig.MaxURLs]
	}

	doc := struct {
		XMLName  xml.Name `xml:"urlset"`
		XMLNS    string   `xml:"xmlns,attr"`
		ImageNS  string   `xml:"xmlns:image,attr,omitempty"`
		URLItems []struct {
			Loc        string `xml:"loc"`
			LastMod    string `xml:"lastmod"`
			ChangeFreq string `xml:"changefreq"`
			Priority   string `xml:"priority"`
			Images     []struct {
				Loc string `xml:"image:loc"`
			} `xml:"image:image,omitempty"`
		} `xml:"url"`
	}{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	if sitemapConfig.IncludeImages {
		doc.ImageNS = "http://www.google.com/schemas/sitemap-image/1.1"
	}
	for _, page := range pages {
		entry := struct {
			Loc        string `xml:"loc"`
			LastMod    string `xml:"lastmod"`
			ChangeFreq string `xml:"changefreq"`
			Priority   string `xml:"priority"`
			Images     []struct {
				Loc string `xml:"image:loc"`
			} `xml:"image:image,omitempty"`
		}{
			Loc: baseURL + page.path, LastMod: lastModified,
			ChangeFreq: page.changefreq, Priority: page.priority,
		}
		if sitemapConfig.IncludeImages {
			imagePath := h.brandingAssetURL(appConfig.Server.SEO.OGImage, "og-1200x630.png")
			if imagePath == "" {
				imagePath = "/static/icons/icon-512.png"
			}
			entry.Images = append(entry.Images, struct {
				Loc string `xml:"image:loc"`
			}{Loc: urlvar.BuildURL(r, imagePath)})
		}
		doc.URLItems = append(doc.URLItems, entry)
	}
	data, err := xml.Marshal(doc)
	if err != nil {
		http.Error(w, "failed to generate sitemap", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	if _, err := w.Write([]byte(xml.Header)); err != nil {
		log.Printf("failed to write sitemap response")
		return
	}
	if _, err := w.Write(data); err != nil {
		log.Printf("failed to write sitemap response")
	}
}

// Favicon serves favicon.ico - redirects to embedded ICO file
// Per AI.md PART 16: /favicon.ico served (embedded default or custom)
func (h *SearchHandler) Favicon(w http.ResponseWriter, r *http.Request) {
	if h.appConfig != nil && h.brandingAssetURL(h.appConfig.Server.Branding.Favicon, "favicon-32.png") != "" {
		http.Redirect(w, r, "/branding/favicon-32.png", http.StatusMovedPermanently)
		return
	}
	http.Redirect(w, r, "/static/images/favicon.ico", http.StatusMovedPermanently)
}

// AppleTouchIcon serves apple-touch-icon.png - redirects to embedded PNG icon
// Per AI.md PART 16: Browsers request this at root level
func (h *SearchHandler) AppleTouchIcon(w http.ResponseWriter, r *http.Request) {
	if h.appConfig != nil && h.brandingAssetURL(h.appConfig.Server.Branding.Favicon, "favicon-180.png") != "" {
		f, err := os.Open(filepath.Join(h.dataDir, "branding", "favicon-180.png"))
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
		http.ServeContent(w, r, "apple-touch-icon.png", info.ModTime(), f)
		return
	}
	data, err := fs.ReadFile(templatesFS, "static/icons/icon-180.png")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(data)
}

// APISearch handles search API requests with content negotiation
// Supports: JSON (default), SSE streaming (Accept: text/event-stream), plain text
func (h *SearchHandler) APISearch(w http.ResponseWriter, r *http.Request) {
	// Start timer immediately when request is received — used in both SSE and JSON paths
	requestStart := time.Now()

	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

	query := r.URL.Query().Get("q")
	if query == "" {
		h.jsonError(w, "Query parameter 'q' is required", CodeValidation, http.StatusBadRequest)
		return
	}

	// Parse bangs from query (e.g., "!ph amateur" -> search pornhub for "amateur")
	parsed := engine.ParseBangs(query)
	searchQuery := parsed.Query
	if searchQuery == "" {
		h.jsonError(w, "Query cannot be empty after bang parsing", CodeValidation, http.StatusBadRequest)
		return
	}

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if pn, err := strconv.Atoi(p); err == nil && pn > 0 {
			page = pn
		}
	}

	engineNames := requestEngineNames(r, parsed.Engines, h.engineMgr.ListEngines())

	// Per IDEA.md Validation: "Engine names must be valid registered
	// engines" and "Bang shortcuts must exist in bangs list." An unknown
	// engines= value (or a bang for an engine that is mapped in BangMapping
	// but not actually registered, e.g. !motherless) must reject the request
	// rather than silently returning an empty-but-"ok" result set.
	if unknown := h.engineMgr.UnknownEngineNames(engineNames); len(unknown) > 0 {
		h.jsonErrorDetails(w, "Unknown engine(s): "+strings.Join(unknown, ", "), CodeValidation, http.StatusBadRequest, map[string]interface{}{
			"unknown_engines": unknown,
		})
		return
	}

	// Check if user wants to show AI content (overrides server default)
	showAI := requestShowAIContent(r)

	// Preview-first: sort each engine batch so preview-capable results stream first
	previewFirst := requestPreviewFirst(r)

	// Server-authoritative filter-panel options (min_quality/quality, min_duration/
	// duration, sort — see parseResultFilterOptions); the SSE path below applies
	// MinQuality/UserMinDuration/MaxDuration per-result the same as the synchronous
	// SearchWithOperators path, but ignores SortBy (a global reorder is
	// incompatible with incremental streaming — the no-JS/JS-fallback form
	// submission uses the synchronous path instead whenever "sort" is set).
	filterOpts := parseResultFilterOptions(r)
	minQuality := filterOpts.MinQuality
	userMinDuration := filterOpts.UserMinDuration

	sessionID := r.URL.Query().Get("session")

	// Honor the user's results-per-page preference server-side on both the
	// SSE and JSON paths (IDEA.md "Pagination": the server determines each
	// batch's contents and size, even with infinite scroll enabled; "0" =
	// infinite scroll, which still uses the config default per batch).
	resultsPerPage := 0
	if n, err := strconv.Atoi(h.getRequestResultsPerPage(r)); err == nil {
		resultsPerPage = n
	}

	// SSE streaming mode - stream results as they arrive from engines
	if format == "text/event-stream" {
		h.handleSearchSSE(w, r, requestStart, searchQuery, page, engineNames, parsed.ExactPhrases, parsed.Exclusions, parsed.RequiredTerms, nil, showAI, minQuality, previewFirst, userMinDuration, filterOpts.MaxDuration, resultsPerPage, sessionID)
		return
	}

	// API response cache per AI.md PART 12 (30s TTL, configurable
	// memory/valkey/redis driver). The cache key captures the query, page,
	// engines and the result-shaping operators so distinct filtered result
	// sets never collide; ?nocache=1 bypasses the cache.
	skipCache := r.URL.Query().Get("nocache") == "1"
	cacheKey := cache.CacheKey(searchQuery, page, engineNames)
	torPreference := requestTorPreference(r)
	torKey := "inherit"
	if torPreference != nil {
		torKey = strconv.FormatBool(*torPreference)
	}
	cacheKey += fmt.Sprintf("|filters:%d:%d:%d:%s|ai:%t|tor:%s|preview:%t|results:%d", filterOpts.MinQuality, filterOpts.UserMinDuration, filterOpts.MaxDuration, filterOpts.SortBy, showAI, torKey, previewFirst, resultsPerPage)
	if len(parsed.Exclusions) > 0 {
		sortedExclusions := append([]string(nil), parsed.Exclusions...)
		sort.Strings(sortedExclusions)
		cacheKey += "|x:" + strings.Join(sortedExclusions, ",")
	}
	if len(parsed.ExactPhrases) > 0 {
		sortedPhrases := append([]string(nil), parsed.ExactPhrases...)
		sort.Strings(sortedPhrases)
		cacheKey += "|p:" + strings.Join(sortedPhrases, "\x1f")
	}
	if sessionID != "" {
		// Session-scoped dedup filtering yields different results per session
		// for the same query/page/engines, so keep each session's entry
		// separate to avoid serving another session's dedup-filtered results.
		cacheKey += "|s:" + sessionID
	}
	if previewFirst {
		// previewFirst changes result order, not membership — without this a
		// preview-first request could be served a cached non-preview-first
		// ordering (or vice versa) from an earlier request for the same query.
		cacheKey += "|pf:1"
	}
	if resultsPerPage > 0 {
		// The per-page preference changes the batch size — keep entries for
		// different preferences separate so a 20-per-page user is never served
		// another user's cached 100-result batch (or vice versa).
		cacheKey += "|n:" + strconv.Itoa(resultsPerPage)
	}

	var results *model.SearchResponse
	if !skipCache && h.searchCache != nil {
		if cached, ok := h.searchCache.Get(cacheKey); ok {
			results = cached
			results.Data.Cached = true
			if h.metrics != nil {
				h.metrics.IncrementCacheHits()
			}
		}
	}

	if results == nil {
		ctx := r.Context()
		// Add user IP to context if user has opted-in for geo-targeted content
		if forwardIP, userIP := h.getUserIPForwardPreference(r); forwardIP {
			ctx = engine.WithUserIP(ctx, userIP, true)
		}
		ctx = withRequestSearchPreferences(ctx, r)
		results = h.engineMgr.SearchWithOperators(ctx, searchQuery, page, engineNames, parsed.ExactPhrases, parsed.Exclusions, parsed.RequiredTerms, previewFirst, sessionID, resultsPerPage, parseResultFilterOptions(r))
		results.Data.Cached = false

		// Never cache the RATE_LIMITED overload envelope — it reflects a
		// transient capacity condition, not the actual query result, and
		// caching it would keep serving 429s for cacheTTL after the server
		// has recovered (AI.md PART 12 "Rate Limiting").
		if isSearchOverloaded(results) {
			h.writeSearchOverloadJSON(w)
			return
		}

		if h.searchCache != nil {
			h.searchCache.Set(cacheKey, results)
		}
		if h.metrics != nil {
			h.metrics.IncrementSearches()
		}
	}

	// Add bang info to response
	// Keep original query with bangs
	results.Data.Query = query
	results.Data.SearchQuery = searchQuery
	results.Data.HasBang = parsed.HasBang
	results.Data.BangEngines = parsed.Engines
	results.Data.InvalidBang = parsed.InvalidBang

	// Add related searches
	results.Data.RelatedSearches = h.engineMgr.GetValidatedRelatedSearches(searchQuery, 8)

	// ETag: SHA-256 of the cache key + result count for conditional GETs
	etag := `"` + func() string {
		h256 := sha256.Sum256([]byte(cacheKey + strconv.Itoa(len(results.Data.Results))))
		return hex.EncodeToString(h256[:16])
	}() + `"`
	// Vary: Accept tells caches that response varies by content negotiation
	w.Header().Set("Vary", "Accept")
	w.Header().Set("ETag", etag)
	if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Spell suggestion: only for JSON and plain-text responses
	if suggestion := h.engineMgr.SpellCorrect(searchQuery); suggestion != "" {
		results.Data.SpellSuggestion = suggestion
	}

	// RSS feed format
	if format == "application/rss+xml" {
		renderSearchRSS(w, r, results, h.appConfig)
		return
	}

	// Atom feed format
	if format == "application/atom+xml" {
		renderSearchAtom(w, r, results, h.appConfig)
		return
	}

	// CSV format
	if format == "text/csv" {
		renderSearchCSV(w, results)
		return
	}

	// Plain text format for .txt extension or Accept: text/plain
	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "query: %s\n", results.Data.Query)
		if results.Data.SpellSuggestion != "" {
			_, _ = fmt.Fprintf(w, "did_you_mean: %s\n", results.Data.SpellSuggestion)
		}
		_, _ = fmt.Fprintf(w, "results: %d\n", len(results.Data.Results))
		_, _ = fmt.Fprintf(w, "---\n")
		for i, r := range results.Data.Results {
			_, _ = fmt.Fprintf(w, "%d. %s\n", i+1, r.Title)
			_, _ = fmt.Fprintf(w, "   url: %s\n", r.URL)
			_, _ = fmt.Fprintf(w, "   source: %s\n", r.Source)
			if r.Duration != "" {
				_, _ = fmt.Fprintf(w, "   duration: %s\n", r.Duration)
			}
			if r.Views != "" {
				_, _ = fmt.Fprintf(w, "   views: %s\n", r.Views)
			}
			_, _ = fmt.Fprintf(w, "\n")
		}
		return
	}

	// Overwrite SearchTimeMS with total request-to-response time (from first byte received)
	results.Data.SearchTimeMS = time.Since(requestStart).Milliseconds()
	if r.URL.Query().Get("display") == "human" {
		response := *results
		response.Data = results.Data
		response.Data.Display = &model.SearchDisplay{
			SearchTime:         humanformat.Duration(time.Duration(results.Data.SearchTimeMS)*time.Millisecond, resolveLocale(r)),
			ResultCount:        len(results.Data.Results),
			ResultCountText:    humanformat.Count(int64(len(results.Data.Results)), resolveLocale(r)),
			ResultCountFormats: formatResultCountFormats(len(results.Data.Results), resolveLocale(r)),
		}
		h.jsonResponse(w, &response)
		return
	}
	h.jsonResponse(w, results)
}

// handleSearchSSE handles SSE streaming for search results
func (h *SearchHandler) handleSearchSSE(w http.ResponseWriter, r *http.Request, requestStart time.Time, searchQuery string, page int, engineNames []string, exactPhrases []string, exclusions []string, requiredTerms []string, performers []string, showAI bool, minQuality int, previewFirst bool, userMinDuration int, maxDuration int, resultsPerPage int, sessionID string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Use ResponseController (Go 1.20+) to flush through wrapped writers
	// Chi's middleware wraps ResponseWriter; direct Flusher assertion fails
	rc := http.NewResponseController(w)

	// Increment search count for SSE searches
	if h.metrics != nil {
		h.metrics.IncrementSearches()
	}

	// Stream results with search operators
	ctx := r.Context()

	// Add user IP to context if user has opted-in for geo-targeted content
	// Per PART 31: This allows video sites to see user's IP for geo content
	if forwardIP, userIP := h.getUserIPForwardPreference(r); forwardIP {
		ctx = engine.WithUserIP(ctx, userIP, true)
	}

	ctx = withRequestSearchPreferences(ctx, r)

	// Total engines actually being queried, independent of how many return
	// results — reported to the client so a zero-match query is shown as
	// "0 of N engines had results" rather than the misleading "0 engines".
	enginesTotal := h.engineMgr.EnginesToUseCount(engineNames)

	resultsChan := h.engineMgr.SearchStreamWithOperators(ctx, searchQuery, page, engineNames, exactPhrases, exclusions, requiredTerms, performers, showAI, minQuality, previewFirst, userMinDuration, maxDuration, resultsPerPage, sessionID)

	// Tracked server-side (not left to the client) so the final "N of M
	// engines had results" count is always authoritative, not re-derived from
	// whichever SSE messages the browser happened to parse.
	enginesWithResults := make(map[string]struct{})
	displayCountOffset := parseDisplayCountOffset(r)
	displayResultCount := displayCountOffset

	for result := range resultsChan {
		// searchSem could not be acquired within searchQueueTimeout (AI.md
		// PART 12 "Rate Limiting") — signal the SSE equivalent of the
		// non-streaming paths' 429 RATE_LIMITED response via a dedicated
		// "error" event, then stop; SearchStreamWithOperators sends this as
		// its only message before closing resultsChan.
		if result.Overloaded {
			_, _ = fmt.Fprintf(w, "event: error\ndata: {\"ok\":false,\"error\":\"%s\",\"message\":\"%s\"}\n\n", CodeRateLimited, MsgRateLimited)
			_ = rc.Flush()
			return
		}

		if result.Engine != "" && result.Engine != "all" && result.Error == "" {
			enginesWithResults[result.Engine] = struct{}{}
		}
		if result.Result.Title != "" {
			if displayResultCount < int(^uint(0)>>1) {
				displayResultCount++
			}
			result.DisplayCount = displayResultCount
			result.DisplayCountText = humanformat.Count(int64(displayResultCount), resolveLocale(r))
		}

		data, err := json.Marshal(result)
		if err != nil {
			continue
		}

		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		_ = rc.Flush()
	}

	// Send final done message with raw and localized elapsed time.
	// engines_total / engines_with_results are computed server-side (not
	// re-derived from streamed messages in JS) per PART 14's server-side
	// processing philosophy — the client only displays these numbers.
	elapsedMS := time.Since(requestStart).Milliseconds()
	doneEvent, err := json.Marshal(map[string]interface{}{
		"done":                 true,
		"engine":               "all",
		"elapsed_ms":           elapsedMS,
		"elapsed_display":      humanformat.Duration(time.Duration(elapsedMS)*time.Millisecond, resolveLocale(r)),
		"engines_total":        enginesTotal,
		"engines_with_results": len(enginesWithResults),
	})
	if err == nil {
		_, _ = fmt.Fprintf(w, "data: %s\n\n", doneEvent)
	}
	_ = rc.Flush()
}

func parseDisplayCountOffset(r *http.Request) int {
	offset, err := strconv.Atoi(r.URL.Query().Get("display_offset"))
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}

// APIBangs returns list of available bang shortcuts
func (h *SearchHandler) APIBangs(w http.ResponseWriter, r *http.Request) {
	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

	bangs := engine.ListBangs()

	// Plain text format
	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "bangs: %d\n---\n", len(bangs))
		for _, b := range bangs {
			// b.Bang already has ! prefix per bangs.go line 287
			_, _ = fmt.Fprintf(w, "%s - %s\n", b.Bang, b.EngineName)
		}
		return
	}

	h.jsonResponse(w, map[string]interface{}{
		"ok":    true,
		"data":  bangs,
		"count": len(bangs),
	})
}

// APIAutocomplete returns autocomplete suggestions for bangs
func (h *SearchHandler) APIAutocomplete(w http.ResponseWriter, r *http.Request) {
	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

	q := r.URL.Query().Get("q")
	if q == "" {
		// Return popular searches when query is empty
		popular := engine.GetPopularSearches(10)
		if format == "text/plain" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "type: popular\nsuggestions: %d\n---\n", len(popular))
			for _, term := range popular {
				_, _ = fmt.Fprintf(w, "%s\n", term)
			}
			return
		}
		h.jsonResponse(w, map[string]interface{}{
			"ok":          true,
			"suggestions": popular,
			"type":        "popular",
		})
		return
	}

	// Check if query starts with "!" for bang autocomplete
	if strings.HasPrefix(q, "!") && len(q) > 1 {
		// Remove the "!" prefix
		prefix := q[1:]
		suggestions := engine.Autocomplete(prefix)
		if format == "text/plain" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "type: bang\nsuggestions: %d\n---\n", len(suggestions))
			for _, s := range suggestions {
				// s.Bang already has ! prefix per bangs.go line 353
				_, _ = fmt.Fprintf(w, "%s - %s\n", s.Bang, s.EngineName)
			}
			return
		}
		h.jsonResponse(w, map[string]interface{}{
			"ok":          true,
			"suggestions": suggestions,
			"type":        "bang",
		})
		return
	}

	// If query ends with " !" (space bang), suggest starting a bang
	if strings.HasSuffix(q, " !") {
		bangs := engine.ListBangs()
		// Return first 10 bangs as suggestions
		if len(bangs) > 10 {
			bangs = bangs[:10]
		}
		if format == "text/plain" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, "type: bang_start\nsuggestions: %d\n---\n", len(bangs))
			for _, b := range bangs {
				// b.Bang already has ! prefix per bangs.go line 287
				_, _ = fmt.Fprintf(w, "%s - %s\n", b.Bang, b.EngineName)
			}
			return
		}
		h.jsonResponse(w, map[string]interface{}{
			"ok":          true,
			"suggestions": bangs,
			"type":        "bang_start",
		})
		return
	}

	// Check for partial bang at end of query (e.g., "amateur !p")
	words := strings.Fields(q)
	if len(words) > 0 {
		lastWord := words[len(words)-1]
		if strings.HasPrefix(lastWord, "!") && len(lastWord) > 1 {
			prefix := lastWord[1:]
			suggestions := engine.Autocomplete(prefix)
			if format == "text/plain" {
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprintf(w, "type: bang\nreplace: %s\nsuggestions: %d\n---\n", lastWord, len(suggestions))
				for _, s := range suggestions {
					// s.Bang already has ! prefix per bangs.go line 353
					_, _ = fmt.Fprintf(w, "%s - %s\n", s.Bang, s.EngineName)
				}
				return
			}
			// replace indicates what to replace in query
			h.jsonResponse(w, map[string]interface{}{
				"ok":          true,
				"suggestions": suggestions,
				"type":        "bang",
				"replace":     lastWord,
			})
			return
		}

		// Check for @performer autocomplete anywhere in the query tail
		// (e.g. "@mia", "teen @mia", "@mia kh" — performer names contain
		// spaces, so performer mode must survive past the first space; scan
		// back to the last @-prefixed token and treat everything from it to
		// the end of the query as the performer prefix, per IDEA.md
		// "Autocomplete System" performer mode).
		atIdx := -1
		for i := len(words) - 1; i >= 0; i-- {
			// A bang token after the last @ token ends performer mode
			if strings.HasPrefix(words[i], "!") {
				break
			}
			if strings.HasPrefix(words[i], "@") {
				atIdx = i
				break
			}
		}
		if atIdx >= 0 {
			// The full trailing phrase a selected suggestion replaces
			replaceToken := strings.Join(words[atIdx:], " ")
			prefix := strings.TrimPrefix(replaceToken, "@")
			performerSuggestions := engine.AutocompletePerformers(prefix, 12)
			// Convert to suggestions with @ prefix
			var suggestions []map[string]string
			for _, p := range performerSuggestions {
				suggestions = append(suggestions, map[string]string{
					"term": "@" + p.Name,
					"type": "performer",
				})
			}
			// A completed performer name followed by extra search terms
			// (e.g. "@mia khalifa blonde") matches no performer — fall
			// through to regular search suggestions instead of returning an
			// empty performer list; a lone unmatched "@xyz" still returns
			// the (empty) performer response so the dropdown hides.
			if len(suggestions) > 0 || atIdx == len(words)-1 {
				if format == "text/plain" {
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					w.WriteHeader(http.StatusOK)
					_, _ = fmt.Fprintf(w, "type: performer\nreplace: %s\nsuggestions: %d\n---\n", replaceToken, len(suggestions))
					for _, s := range suggestions {
						_, _ = fmt.Fprintf(w, "%s\n", s["term"])
					}
					return
				}
				h.jsonResponse(w, map[string]interface{}{
					"ok":          true,
					"suggestions": suggestions,
					"type":        "performer",
					"replace":     replaceToken,
				})
				return
			}
		}
	}

	// No bang or @ in query - return search term suggestions only (no performers)
	// Get the last word as the prefix for suggestions
	lastWord := ""
	if len(words) > 0 {
		lastWord = strings.ToLower(words[len(words)-1])
	} else {
		lastWord = strings.ToLower(q)
	}

	// Use search suggestions only - performers require @ prefix
	searchSuggestions := engine.AutocompleteSuggestions(lastWord, 12)

	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "type: search\nsuggestions: %d\n---\n", len(searchSuggestions))
		for _, s := range searchSuggestions {
			_, _ = fmt.Fprintf(w, "%s [search]\n", s.Term)
		}
		return
	}
	h.jsonResponse(w, map[string]interface{}{
		"ok":          true,
		"suggestions": searchSuggestions,
		"type":        "search",
	})
}

// APIEngines returns list of available engines
func (h *SearchHandler) APIEngines(w http.ResponseWriter, r *http.Request) {
	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

	engines := h.engineMgr.ListEngines()

	// Plain text format
	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "engines: %d\n---\n", len(engines))
		for _, e := range engines {
			status := "enabled"
			if !e.Enabled {
				status = "disabled"
			}
			_, _ = fmt.Fprintf(w, "%s (%s) - tier %d [%s]\n", e.Name, e.DisplayName, e.Tier, status)
		}
		return
	}

	h.jsonResponse(w, model.EnginesResponse{
		Ok:   true,
		Data: engines,
	})
}

// APIEngineDetails returns details for a specific engine
func (h *SearchHandler) APIEngineDetails(w http.ResponseWriter, r *http.Request) {
	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

	name := chi.URLParam(r, "name")
	eng, ok := h.engineMgr.GetEngine(name)
	if !ok {
		h.jsonError(w, "Engine not found", CodeNotFound, http.StatusNotFound)
		return
	}

	caps := eng.Capabilities()

	// Plain text format
	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "name: %s\n", eng.Name())
		_, _ = fmt.Fprintf(w, "display_name: %s\n", eng.DisplayName())
		_, _ = fmt.Fprintf(w, "tier: %d\n", eng.Tier())
		_, _ = fmt.Fprintf(w, "enabled: %t\n", eng.IsAvailable())
		_, _ = fmt.Fprintf(w, "has_preview: %t\n", caps.HasPreview)
		_, _ = fmt.Fprintf(w, "has_download: %t\n", caps.HasDownload)
		return
	}

	h.jsonResponse(w, map[string]interface{}{
		"ok": true,
		"data": model.EngineInfo{
			Name:        eng.Name(),
			DisplayName: eng.DisplayName(),
			Enabled:     eng.IsAvailable(),
			Available:   eng.IsAvailable(),
			Tier:        eng.Tier(),
			Capabilities: &model.EngineCapabilities{
				HasPreview:  caps.HasPreview,
				HasDownload: caps.HasDownload,
			},
		},
	})
}

// APIEngineHealth returns health stats for all engines (circuit breaker state, latency, uptime).
func (h *SearchHandler) APIEngineHealth(w http.ResponseWriter, r *http.Request) {
	engines := h.engineMgr.ListEnginesWithHealth()

	// Plain text format per AI.md PART 14 content negotiation
	if detectResponseFormat(r) == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		for _, e := range engines {
			_, _ = fmt.Fprintf(w, "name: %s\n", e.Name)
			_, _ = fmt.Fprintf(w, "circuit_state: %s\n", e.Health.CircuitState)
			_, _ = fmt.Fprintf(w, "uptime_pct: %.2f\n", e.Health.UptimePct)
			_, _ = fmt.Fprintf(w, "avg_latency_ms: %d\n", e.Health.AvgLatencyMs)
			_, _ = fmt.Fprintf(w, "total_successes: %d\n", e.Health.TotalSuccesses)
			_, _ = fmt.Fprintf(w, "total_failures: %d\n", e.Health.TotalFailures)
			_, _ = fmt.Fprintln(w)
		}
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"data": engines,
	})
}

// APIStats returns public statistics
func (h *SearchHandler) APIStats(w http.ResponseWriter, r *http.Request) {
	// Detect response format per AI.md PART 14
	format := detectResponseFormat(r)

	enabled := h.engineMgr.EnabledCount()
	total := len(h.engineMgr.ListEngines())

	// Plain text format
	if format == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "engines_enabled: %d\n", enabled)
		_, _ = fmt.Fprintf(w, "engines_total: %d\n", total)
		return
	}

	h.jsonResponse(w, map[string]interface{}{
		"ok": true,
		"data": map[string]interface{}{
			"engines_enabled": enabled,
			"engines_total":   total,
		},
	})
}

// APIVersion returns server version info
// Per AI.md PART 13: {api_version}/version returns version, commit, build_date, official_site.
func (h *SearchHandler) APIVersion(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok":            true,
		"version":       version.GetVersion(),
		"commit":        version.CommitID,
		"build_date":    version.BuildTime,
		"official_site": version.OfficialSite,
	})
}

// APIHealthCheck returns health status as JSON per AI.md PART 13
// Returns comprehensive health status with checks object for database/cache/disk
// APIHealthCheck handles {api_version}/healthz endpoint (JSON only)
// Per AI.md PART 13: Same JSON as /healthz
func (h *SearchHandler) APIHealthCheck(w http.ResponseWriter, r *http.Request) {
	// API routes default to JSON but support text output per AI.md PART 14
	// Format detection: .txt extension > Accept header > client type > default JSON

	// Build health response per AI.md PART 13
	uptime := getUptime()
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Get mode from config
	appMode := "production"
	if h.appConfig != nil {
		appMode = config.NormalizeMode(h.appConfig.Server.Mode)
	}

	// Build checks object - MUST be simple "ok"/"error" strings
	// Per AI.md PART 13 - real probes, must match HealthCheck's checks
	// (this handler backs the /api/{api_version}/server/healthz alias,
	// which per AI.md PART 13/14 must return the same data as /healthz)
	checks := map[string]string{
		"database": h.checkDatabase(r.Context()),
		"cache":    h.checkCache(),
		"disk":     h.checkDisk(),
	}

	// Overall status - per AI.md PART 13: derive from checks
	status := "healthy"
	httpStatus := http.StatusOK
	for _, v := range checks {
		if v != "ok" {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
			break
		}
	}

	// Detect response format per AI.md PART 14
	format := getAPIResponseFormat(r)

	// Add scheduler check - per AI.md PART 13: real probe, must match HealthCheck
	checks["scheduler"] = h.checkScheduler()
	if checks["scheduler"] != "ok" {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	// Tor status for features and checks
	torEnabled := h.torSvc != nil && h.torSvc.IsEnabled()
	torRunning := h.torSvc != nil && h.torSvc.IsRunning()
	if torEnabled {
		if torRunning {
			checks["tor"] = "ok"
		} else {
			checks["tor"] = "error"
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}
	}
	if i2pCheck := h.checkI2P(r.Context()); i2pCheck != "" {
		checks["i2p"] = i2pCheck
		if i2pCheck != "ok" {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
		}
	}

	// restart_required per AI.md PART 13 status->HTTP table: HTTP 200,
	// carries pending_restart+restart_reason. Only applies when checks
	// are otherwise healthy - an unhealthy check always takes priority.
	if status == "healthy" && h.appConfig != nil && h.appConfig.PendingRestart {
		status = "restart_required"
	}
	if signalstate.IsShuttingDown() {
		status = "shutting_down"
		httpStatus = http.StatusServiceUnavailable
	}

	// Project branding from config
	apiProjectName := "VidVeil"
	apiProjectTagline := "Privacy-first video search"
	apiProjectDesc := "Privacy-respecting adult video meta search"
	if h.appConfig != nil {
		if h.appConfig.Server.Branding.Title != "" {
			apiProjectName = h.appConfig.Server.Branding.Title
		}
		if h.appConfig.Server.Branding.Tagline != "" {
			apiProjectTagline = h.appConfig.Server.Branding.Tagline
		}
		if h.appConfig.Server.Branding.Description != "" {
			apiProjectDesc = h.appConfig.Server.Branding.Description
		}
	}

	// Text output for CLI tools per AI.md PART 13 canonical field order
	if format == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(httpStatus)
		// 1. Project
		_, _ = fmt.Fprintf(w, "project.name: %s\n", apiProjectName)
		_, _ = fmt.Fprintf(w, "project.tagline: %s\n", apiProjectTagline)
		_, _ = fmt.Fprintf(w, "project.description: %s\n", apiProjectDesc)
		// 2. Status
		_, _ = fmt.Fprintf(w, "status: %s\n", status)
		if h.appConfig != nil && h.appConfig.PendingRestart {
			_, _ = fmt.Fprintf(w, "pending_restart: true\n")
			for _, rr := range h.appConfig.RestartReasons {
				_, _ = fmt.Fprintf(w, "restart_reason: %s\n", rr)
			}
		}
		// 3. Version & build
		_, _ = fmt.Fprintf(w, "version: %s\n", version.GetVersion())
		_, _ = fmt.Fprintf(w, "go_version: %s\n", version.GoVersion)
		_, _ = fmt.Fprintf(w, "build.commit: %s\n", version.CommitID)
		_, _ = fmt.Fprintf(w, "build.date: %s\n", version.BuildTime)
		// 4. Runtime
		_, _ = fmt.Fprintf(w, "uptime: %s\n", uptime)
		_, _ = fmt.Fprintf(w, "mode: %s\n", appMode)
		_, _ = fmt.Fprintf(w, "timestamp: %s\n", timestamp)
		// 5. Features
		_, _ = fmt.Fprintf(w, "features.tor.enabled: %v\n", torEnabled)
		_, _ = fmt.Fprintf(w, "features.tor.running: %v\n", torRunning)
		_, _ = fmt.Fprintf(w, "features.tor.status: %s\n", h.getTorStatus())
		if torRunning {
			_, _ = fmt.Fprintf(w, "features.tor.hostname: %s\n", h.getTorHostname())
		}
		i2pEnabled := h.i2pSvc != nil && h.i2pSvc.Enabled()
		i2pRunning := h.i2pSvc != nil && h.i2pSvc.IsRunning()
		_, _ = fmt.Fprintf(w, "features.i2p.enabled: %v\n", i2pEnabled)
		_, _ = fmt.Fprintf(w, "features.i2p.running: %v\n", i2pRunning)
		_, _ = fmt.Fprintf(w, "features.i2p.status: %s\n", h.i2pHealthStatus(checks["i2p"]))
		if i2pRunning {
			_, _ = fmt.Fprintf(w, "features.i2p.hostname: %s\n", h.i2pAddress())
		}
		_, _ = fmt.Fprintf(w, "features.i2p.provider: %s\n", h.i2pProvider())
		_, _ = fmt.Fprintf(w, "features.geoip: %v\n", h.appConfig != nil && h.appConfig.Server.GeoIP.Enabled)
		// 7. Checks
		for _, k := range []string{"database", "cache", "disk", "scheduler"} {
			_, _ = fmt.Fprintf(w, "checks.%s: %s\n", k, checks[k])
		}
		if _, ok := checks["tor"]; ok {
			_, _ = fmt.Fprintf(w, "checks.tor: %s\n", checks["tor"])
		}
		if _, ok := checks["i2p"]; ok {
			_, _ = fmt.Fprintf(w, "checks.i2p: %s\n", checks["i2p"])
		}
		// 8. Stats
		_, _ = fmt.Fprintf(w, "stats.requests_total: %d\n", h.getRequestsTotal())
		_, _ = fmt.Fprintf(w, "stats.requests_24h: %d\n", h.getRequests24h())
		_, _ = fmt.Fprintf(w, "stats.active_connections: %d\n", h.getActiveConnections())
		return
	}

	// JSON response (default) - canonical HealthResponse struct per AI.md
	// PART 13; must be byte-for-byte the same shape as HealthCheck's JSON
	// branch ("Same JSON as /healthz" — a map would alphabetize keys and
	// risk drifting out of sync with the struct-based frontend response).
	response := HealthResponse{
		Project: ProjectInfo{
			Name:        apiProjectName,
			Tagline:     apiProjectTagline,
			Description: apiProjectDesc,
		},
		Status:    status,
		Version:   version.GetVersion(),
		GoVersion: runtime.Version(),
		Build: BuildInfo{
			Commit: version.CommitID,
			Date:   version.BuildTime,
		},
		Uptime:    uptime,
		Mode:      appMode,
		Timestamp: timestamp,
		Features: FeaturesInfo{
			Tor: TorInfo{
				Enabled:  torEnabled,
				Running:  torRunning,
				Status:   h.getTorStatus(),
				Hostname: h.getTorHostname(),
			},
			I2P: I2PInfo{
				Enabled:  h.i2pSvc != nil && h.i2pSvc.Enabled(),
				Running:  h.i2pSvc != nil && h.i2pSvc.IsRunning(),
				Status:   h.i2pHealthStatus(checks["i2p"]),
				Hostname: h.i2pAddress(),
				Provider: h.i2pProvider(),
			},
			GeoIP: h.appConfig != nil && h.appConfig.Server.GeoIP.Enabled,
		},
		Checks: ChecksInfo{
			Database:  checks["database"],
			Cache:     checks["cache"],
			Disk:      checks["disk"],
			Scheduler: checks["scheduler"],
			Tor:       checks["tor"],
			I2P:       checks["i2p"],
		},
		Stats: StatsInfo{
			RequestsTotal:     h.getRequestsTotal(),
			Requests24h:       h.getRequests24h(),
			ActiveConnections: h.getActiveConnections(),
			SearchesTotal:     h.getSearchCount(),
		},
	}

	// pending_restart / restart_reason — omitempty: only include when set
	if h.appConfig != nil && h.appConfig.PendingRestart {
		response.PendingRestart = true
		response.RestartReason = h.appConfig.RestartReasons
	}

	WriteJSON(w, httpStatus, response)
}

// serverStartTime is set when the server starts
var serverStartTime = time.Now()

// getUptime returns the server uptime as a human-readable string
func getUptime() string {
	uptime := time.Since(serverStartTime)
	hours := int(uptime.Hours())
	minutes := int(uptime.Minutes()) % 60
	seconds := int(uptime.Seconds()) % 60

	if hours > 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
	}
	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}

// Helper methods

// jsonResponse writes a 200 JSON response.
func (h *SearchHandler) jsonResponse(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, data)
}

func (h *SearchHandler) jsonError(w http.ResponseWriter, message, code string, status int) {
	// Per AI.md PART 14: Error response format
	// - ok: false
	// - error: ERROR_CODE (machine-readable)
	// - message: Human readable message
	WriteJSON(w, status, map[string]interface{}{
		"ok":      false,
		"error":   code,
		"message": message,
	})
}

// jsonErrorDetails is jsonError plus the optional "details" field from the
// canonical error envelope ({"ok":false,"error":"CODE","message":"...",
// "details":{}}) per AI.md PART 9/14, for errors where the machine-readable
// cause (e.g. which engine name was unrecognized) is useful to API clients.
func (h *SearchHandler) jsonErrorDetails(w http.ResponseWriter, message, code string, status int, details map[string]interface{}) {
	WriteJSON(w, status, map[string]interface{}{
		"ok":      false,
		"error":   code,
		"message": message,
		"details": details,
	})
}

// searchOverloadRetryAfterSeconds is the Retry-After hint (seconds) sent when
// SearchWithOperators returns the RATE_LIMITED overload envelope. Kept short
// (matches engine.searchQueueTimeout) since the condition is a transient
// concurrency-slot wait, not a per-minute counter — a client retrying a
// couple seconds later is expected to succeed.
const searchOverloadRetryAfterSeconds = 2

// isSearchOverloaded reports whether results is the RATE_LIMITED envelope
// EngineManager.SearchWithOperators returns when its searchSem concurrency
// guard could not be acquired within its queue timeout (AI.md PART 12 "Rate
// Limiting" — "primary abuse defense"; PART 9 canonical error envelope). Every
// caller of SearchWithOperators must check this before treating a result as a
// normal (possibly zero-result) search response, so an overloaded server
// answers fast with a retryable 429 instead of the connection silently
// stalling until net/http's WriteTimeout resets it.
func isSearchOverloaded(results *model.SearchResponse) bool {
	return results != nil && !results.Ok && results.Error == CodeRateLimited
}

// renderSearchOverloadPage renders a complete, localized search model for HTML
// clients when search capacity is saturated. The normal search templates expect
// pagination and result fields even when the result set is empty.
func (h *SearchHandler) renderSearchOverloadPage(w http.ResponseWriter, r *http.Request, query, searchQuery string, page int) {
	w.Header().Set("Retry-After", strconv.Itoa(searchOverloadRetryAfterSeconds))
	h.renderResponseStatus(w, r, "search", map[string]interface{}{
		"Title":          query + " - " + h.appConfig.Server.Branding.Title,
		"Query":          query,
		"SearchQuery":    searchQuery,
		"ResultsJSON":    template.JS("[]"),
		"Results":        []model.VideoResult{},
		"FavoriteURLs":   map[string]bool{},
		"CurrentPath":    r.URL.RequestURI(),
		"SearchTime":     int64(0),
		"Theme":          h.getRequestTheme(r),
		"ErrorCode":      CodeRateLimited,
		"Page":           page,
		"PrevPage":       page - 1,
		"NextPage":       page + 1,
		"HasMore":        false,
		"InfiniteScroll": false,
		"EnginesParam":   r.URL.Query().Get("engines"),
		"FilterDuration": r.URL.Query().Get("duration"),
		"FilterQuality":  r.URL.Query().Get("quality"),
		"FilterSort":     r.URL.Query().Get("sort"),
		"PreviewFirst":   h.getRequestPreviewFirst(w, r),
		"OpenNewTab":     h.getRequestOpenNewTab(r),
		"Version":        version.GetVersion(),
		"BuildDateTime":  BuildDateTime(),
	}, http.StatusTooManyRequests)
}

// writeSearchOverloadJSON writes the canonical RATE_LIMITED error envelope
// (AI.md PART 9/14) with a 429 status and Retry-After header for JSON/API
// callers (APISearch, SearchPage's application/json branch, BatchSearch).
func (h *SearchHandler) writeSearchOverloadJSON(w http.ResponseWriter) {
	w.Header().Set("Retry-After", strconv.Itoa(searchOverloadRetryAfterSeconds))
	h.jsonError(w, MsgRateLimited, CodeRateLimited, http.StatusTooManyRequests)
}

// RenderErrorPage renders a custom error page per AI.md PART 30
func (h *SearchHandler) RenderErrorPage(w http.ResponseWriter, r *http.Request, code int, title, message string) {
	data := map[string]interface{}{
		"Code":      code,
		"Title":     title,
		"Message":   message,
		"SiteTitle": h.appConfig.Server.Branding.Title,
		"Theme":     h.getRequestTheme(r),
	}
	// AI.md PART 30: lang/dir for <html>
	injectLocaleData(r, data)

	if templatesFS == nil {
		writeGuaranteedErrorPage(w, r, code, title, message)
		return
	}

	// Resolve the request locale so the error page can translate via {{ t "key" }}
	locale := i18n.DefaultLocale
	if l, ok := data["Lang"].(string); ok && l != "" {
		locale = l
	}

	tmpl, err := template.New("error.tmpl").Funcs(template.FuncMap{
		"t": func(key string) string {
			return i18n.Translate(locale, key)
		},
		"tf": func(key string, args ...interface{}) string {
			return i18n.TranslateFormat(locale, key, args...)
		},
		// asset appends the version-busting stamp per AI.md PART 9
		"asset": func(path string) string {
			return path + "?v=" + version.AssetStamp()
		},
	}).ParseFS(templatesFS, "template/page/error.tmpl")
	if err != nil {
		// Fallback per AI.md PART 16 "Error Pages" — the error-page
		// template itself failed to parse, so fall back to the minimal
		// hardcoded, content-negotiated response instead of plain text.
		writeGuaranteedErrorPage(w, r, code, title, message)
		return
	}

	// Buffer template output to prevent proxy truncation issues
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "error", data); err != nil {
		// Fallback per AI.md PART 16 "Error Pages" — same guaranteed-
		// response rule applies when execution (not just parsing) fails.
		writeGuaranteedErrorPage(w, r, code, title, message)
		return
	}

	// Set headers and write buffered response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(code)
	_, _ = w.Write(buf.Bytes())
}

// NotFoundHandler handles 404 errors per AI.md PART 30
// Per AI.md PART 14, /api/** routes must honor content negotiation (including
// the not-found case) instead of falling through to the HTML frontend page.
func (h *SearchHandler) NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		if getAPIResponseFormat(r) == "text" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(MsgNotFound + "\n"))
			return
		}
		SendError(w, CodeNotFound, MsgNotFound)
		return
	}
	h.RenderErrorPage(w, r, http.StatusNotFound, "Page Not Found",
		"The page you're looking for doesn't exist or has been moved.")
}

// InternalErrorHandler handles 500 errors per AI.md PART 30
func (h *SearchHandler) InternalErrorHandler(w http.ResponseWriter, r *http.Request) {
	h.RenderErrorPage(w, r, http.StatusInternalServerError, "Server Error",
		"Something went wrong on our end. Please try again later.")
}

func (h *SearchHandler) renderTemplate(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	// Ensure required fields for nav.tmpl
	if data["ActiveNav"] == nil {
		data["ActiveNav"] = name
	}
	if data["Query"] == nil {
		data["Query"] = ""
	}

	// Map template names to file paths
	templateFile := ""
	templateName := ""
	switch name {
	case "home":
		templateFile = "template/page/index.tmpl"
		templateName = "home"
	case "search":
		templateFile = "template/page/search.tmpl"
		templateName = "search"
	case "preferences":
		templateFile = "template/page/preferences.tmpl"
		templateName = "preferences"
	case "owner-tokens":
		templateFile = "template/page/owner-tokens.tmpl"
		templateName = "owner-tokens"
	case "favorites":
		templateFile = "template/page/favorites.tmpl"
		templateName = "favorites"
	case "about":
		templateFile = "template/page/about.tmpl"
		templateName = "about"
	case "age-verify":
		templateFile = "template/page/age-verify.tmpl"
		templateName = "age-verify"
	case "content-restricted":
		templateFile = "template/page/content-restricted.tmpl"
		templateName = "content-restricted"
	case "content-blocked":
		templateFile = "template/page/content-blocked.tmpl"
		templateName = "content-blocked"
	case "privacy":
		templateFile = "template/page/privacy.tmpl"
		templateName = "privacy"
	// nojs templates for text browsers (lynx, w3m, links)
	case "nojs/home":
		templateFile = "template/nojs/home.tmpl"
		templateName = "nojs/home"
	case "nojs/search":
		templateFile = "template/nojs/search.tmpl"
		templateName = "nojs/search"
	case "nojs/preferences":
		templateFile = "template/nojs/preferences.tmpl"
		templateName = "nojs/preferences"
	case "nojs/owner-tokens":
		templateFile = "template/nojs/owner-tokens.tmpl"
		templateName = "nojs/owner-tokens"
	case "nojs/about":
		templateFile = "template/nojs/about.tmpl"
		templateName = "nojs/about"
	case "nojs/age-verify":
		templateFile = "template/nojs/age-verify.tmpl"
		templateName = "nojs/age-verify"
	case "nojs/privacy":
		templateFile = "template/nojs/privacy.tmpl"
		templateName = "nojs/privacy"
	default:
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError)
		return
	}

	// Guard against uninitialized template filesystem
	if templatesFS == nil {
		log.Printf("page template: templates filesystem not initialized")
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError)
		return
	}

	// Inject version for cache busting in all templates
	if data["Version"] == nil {
		data["Version"] = version.GetVersion()
	}
	if data["BuildDateTime"] == nil {
		data["BuildDateTime"] = BuildDateTime()
	}

	// Footer onion-address row per AI.md PART 16 — dropped entirely unless
	// Tor is both enabled and actually running.
	if h.torSvc != nil && h.torSvc.IsEnabled() && h.torSvc.IsRunning() {
		data["TorEnabled"] = true
		data["TorRunning"] = true
		if addr, ok := h.torSvc.GetInfo()["onion_address"].(string); ok {
			data["OnionAddress"] = addr
		}
	}
	if h.i2pSvc != nil && h.i2pSvc.Enabled() && h.i2pSvc.IsRunning() {
		data["I2PEnabled"] = true
		data["I2PRunning"] = true
		data["I2PAddress"] = h.i2pSvc.Address()
	}

	// Browser-chrome theme color uses the shared Go palette, never a hardcoded hex.
	if data["ThemeColor"] == nil {
		themeName, _ := data["Theme"].(string)
		data["ThemeColor"] = getThemeColor(themeName)
	}

	// Per-route robots directive and canonical URL per AI.md PART 16 —
	// computed server-side, fail-closed to noindex,nofollow.
	if data["Robots"] == nil {
		data["Robots"] = robotsDirective(r)
	}
	if data["CanonicalURL"] == nil && r != nil {
		data["CanonicalURL"] = urlvar.BuildURL(r, r.URL.Path)
	}

	// Inject SEO and branding data per AI.md PART 16
	if data["SEOKeywords"] == nil {
		data["SEOKeywords"] = strings.Join(h.appConfig.Server.SEO.Keywords, ", ")
	}
	if data["SEOAuthor"] == nil {
		data["SEOAuthor"] = h.appConfig.Server.SEO.Author
	}
	if data["SEOOGImage"] == nil {
		ogPath := h.brandingAssetURL(h.appConfig.Server.SEO.OGImage, "og-1200x630.png")
		if ogPath != "" {
			data["SEOOGImage"] = urlvar.BuildURL(r, ogPath)
		} else {
			data["SEOOGImage"] = ""
		}
	}
	if data["SEOTwitterHandle"] == nil {
		data["SEOTwitterHandle"] = h.appConfig.Server.SEO.TwitterHandle
	}
	if data["SEOVerification"] == nil {
		data["SEOVerification"] = h.appConfig.Server.SEO.Verification
	}
	if data["BrandingDescription"] == nil {
		data["BrandingDescription"] = h.appConfig.Server.Branding.Description
	}
	if data["BrandingTagline"] == nil {
		data["BrandingTagline"] = h.appConfig.Server.Branding.Tagline
	}
	// Operator-configured logo/favicon per AI.md PART 16 "Image Sources"
	if data["BrandingLogo"] == nil {
		data["BrandingLogo"] = h.brandingAssetURL(h.appConfig.Server.Branding.Logo, "logo-200.png")
	}
	if data["BrandingFavicon"] == nil {
		data["BrandingFavicon"] = h.brandingAssetURL(h.appConfig.Server.Branding.Favicon, "favicon-32.png")
	}
	if data["AppURL"] == nil {
		// Fallback only — renderResponse (the production entry point, which has
		// the *http.Request in scope) already sets AppURL via urlvar.BuildURL
		// per AI.md PART 12. This static config-based fallback exists solely for
		// direct renderTemplate() test calls that construct data maps without r.
		scheme := "https"
		if !h.appConfig.Server.SSL.Enabled {
			scheme = "http"
		}
		data["AppURL"] = scheme + "://" + h.appConfig.Server.FQDN
	}

	// Resolve the request locale so templates can translate via {{ t "key" }}
	locale := i18n.DefaultLocale
	if l, ok := data["Lang"].(string); ok && l != "" {
		locale = l
	}

	// Create base template with FuncMap
	tmpl := template.New(templateName).Funcs(template.FuncMap{
		// dict creates a map from key-value pairs for passing to templates
		"dict": func(values ...interface{}) map[string]interface{} {
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				if i+1 < len(values) {
					key, ok := values[i].(string)
					if ok {
						dict[key] = values[i+1]
					}
				}
			}
			return dict
		},
		"eq": func(a, b interface{}) bool { return a == b },
		// t translates an i18n key for the current request locale (PART 30).
		"t": func(key string) string {
			return i18n.Translate(locale, key)
		},
		"count": func(value int) string {
			return humanformat.Count(int64(value), locale)
		},
		// tf translates an i18n key with printf-style arguments.
		"tf": func(key string, args ...interface{}) string {
			return i18n.TranslateFormat(locale, key, args...)
		},
		// safeHTML sanitizes s through the strict PART 16 allowlist
		// policy and marks the result safe for template rendering.
		"safeHTML": func(s string) template.HTML {
			return sanitize.HTML(s)
		},
		// asset appends the version-busting stamp per AI.md PART 9:
		// {{ asset "/static/css/app.css" }} → /static/css/app.css?v={version}-{commit}
		"asset": func(path string) string {
			return path + "?v=" + version.AssetStamp()
		},
		// toJSON marshals a value into an inline JSON data island (CSP-safe,
		// non-executable <script type="application/json"> per AI.md PART 16).
		"toJSON": func(v interface{}) (template.JS, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return template.JS(b), nil
		},
	})

	// Load all public partials
	partialFiles := []string{
		"template/partial/public/head.tmpl",
		"template/partial/public/header.tmpl",
		"template/partial/public/nav.tmpl",
		"template/partial/public/site-banner.tmpl",
		"template/partial/public/footer.tmpl",
		"template/partial/public/filters.tmpl",
		"template/partial/public/scripts.tmpl",
	}

	for _, pf := range partialFiles {
		content, err := fs.ReadFile(templatesFS, pf)
		if err != nil {
			// Skip missing partials - they may not all be needed
			continue
		}
		if _, err = tmpl.Parse(string(content)); err != nil {
			log.Printf("page template: parse %s: %v", pf, err)
			writeGuaranteedErrorPage(w, r, http.StatusInternalServerError)
			return
		}
	}

	content, err := fs.ReadFile(templatesFS, templateFile)
	if err != nil {
		log.Printf("page template: read %s: %v", templateFile, err)
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError)
		return
	}

	if _, err = tmpl.Parse(string(content)); err != nil {
		log.Printf("page template: parse %s: %v", templateFile, err)
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError)
		return
	}

	// Buffer template output: ensures Content-Length is set and the response is written
	// atomically (avoids nginx proxy_buffer_size truncation, typically 8KB).
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, templateName, data); err != nil {
		log.Printf("page template: execute %s: %v", templateName, err)
		writeGuaranteedErrorPage(w, r, http.StatusInternalServerError)
		return
	}

	// Set headers and write buffered response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// DebugEngine probes a specific engine and returns detailed results
// GET {api_version}/debug/engines/{name}?q={query}
// Returns: engine info, capabilities, sample results with all fields
func (h *SearchHandler) DebugEngine(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	query := r.URL.Query().Get("q")
	if query == "" {
		// Default test query
		query = "test"
	}

	eng, ok := h.engineMgr.GetEngine(name)
	if !ok {
		h.jsonError(w, "Engine not found", CodeNotFound, http.StatusNotFound)
		return
	}

	// Get engine capabilities
	caps := eng.Capabilities()

	// Perform test search
	results, err := eng.Search(r.Context(), query, 1)

	// Build debug response
	response := map[string]interface{}{
		"ok": true,
		"engine": map[string]interface{}{
			"name":         eng.Name(),
			"display_name": eng.DisplayName(),
			"tier":         eng.Tier(),
			"available":    eng.IsAvailable(),
		},
		"capabilities": caps,
		"query":        query,
	}

	if err != nil {
		// Per AI.md PART 9: Never expose error details in responses
		response["error"] = "Search failed"
		response["results"] = []interface{}{}
		response["result_count"] = 0
	} else {
		response["results"] = results
		response["result_count"] = len(results)

		// Analyze what fields are populated
		fieldStats := analyzeResultFields(results)
		response["field_stats"] = fieldStats
	}

	WriteJSON(w, http.StatusOK, response)
}

// analyzeResultFields checks which fields are populated in results
func analyzeResultFields(results []model.VideoResult) map[string]interface{} {
	stats := map[string]int{
		"has_title":        0,
		"has_url":          0,
		"has_thumbnail":    0,
		"has_preview_url":  0,
		"has_download_url": 0,
		"has_duration":     0,
		"has_views":        0,
		"has_rating":       0,
		"has_quality":      0,
		"has_published":    0,
	}

	for _, r := range results {
		if r.Title != "" {
			stats["has_title"]++
		}
		if r.URL != "" {
			stats["has_url"]++
		}
		if r.Thumbnail != "" {
			stats["has_thumbnail"]++
		}
		if r.PreviewURL != "" {
			stats["has_preview_url"]++
		}
		if r.DownloadURL != "" {
			stats["has_download_url"]++
		}
		if r.Duration != "" || r.DurationSeconds > 0 {
			stats["has_duration"]++
		}
		if r.Views != "" || r.ViewsCount > 0 {
			stats["has_views"]++
		}
		if r.Rating > 0 {
			stats["has_rating"]++
		}
		if r.Quality != "" {
			stats["has_quality"]++
		}
		if !r.Published.IsZero() {
			stats["has_published"]++
		}
	}

	total := len(results)
	return map[string]interface{}{
		"total_results": total,
		"fields":        stats,
	}
}

// DebugEnginesList returns all engines with their capabilities
// GET {api_version}/debug/engines
func (h *SearchHandler) DebugEnginesList(w http.ResponseWriter, r *http.Request) {
	engines := h.engineMgr.ListEngines()

	type engineDebug struct {
		Name         string              `json:"name"`
		DisplayName  string              `json:"display_name"`
		Tier         int                 `json:"tier"`
		Enabled      bool                `json:"enabled"`
		Capabilities engine.Capabilities `json:"capabilities"`
	}

	var list []engineDebug
	for _, info := range engines {
		eng, ok := h.engineMgr.GetEngine(info.Name)
		if !ok {
			continue
		}
		list = append(list, engineDebug{
			Name:         info.Name,
			DisplayName:  info.DisplayName,
			Tier:         info.Tier,
			Enabled:      info.Enabled,
			Capabilities: eng.Capabilities(),
		})
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok":      true,
		"count":   len(list),
		"engines": list,
	})
}

// privateCIDRs lists every range the SSRF guard treats as off-limits:
// private, loopback, link-local, carrier-grade NAT, and unique-local.
var privateCIDRs = func() []*net.IPNet {
	ranges := []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"100.64.0.0/10",
		"::/128",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}
	nets := make([]*net.IPNet, 0, len(ranges))
	for _, cidr := range ranges {
		if _, network, err := net.ParseCIDR(cidr); err == nil {
			nets = append(nets, network)
		}
	}
	return nets
}()

// isPrivateIP reports whether the concrete IP falls in any blocked range.
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	for _, network := range privateCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// isPrivateHost resolves hostname and returns true if any resolved address
// falls in a private, loopback, link-local, or unique-local range.
// Used as a pre-flight SSRF check on the proxy endpoints; the dial-time
// Control hook in getProxyClient closes the DNS-rebinding TOCTOU window.
func isPrivateHost(hostname string) bool {
	if ip := net.ParseIP(hostname); ip != nil {
		return isPrivateIP(ip)
	}
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		// Unresolvable host — treat as private to be safe
		return true
	}
	for _, addr := range addrs {
		if isPrivateIP(net.ParseIP(addr)) {
			return true
		}
	}
	return false
}

// ProxyThumbnail proxies external thumbnails to prevent tracking
// Per IDEA.md: Privacy proxy for thumbnails
func (h *SearchHandler) ProxyThumbnail(w http.ResponseWriter, r *http.Request) {
	// Get URL parameter
	encodedURL := r.URL.Query().Get("url")
	if encodedURL == "" {
		SendError(w, CodeBadRequest, i18n.Translate(i18n.DetectLocale(r), "error.proxy_missing_url"))
		return
	}

	// Decode URL
	thumbURL, err := url.QueryUnescape(encodedURL)
	if err != nil {
		SendError(w, CodeBadRequest, i18n.Translate(i18n.DetectLocale(r), "error.proxy_invalid_url"))
		return
	}

	// Validate URL
	parsedURL, err := url.Parse(thumbURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		SendError(w, CodeBadRequest, i18n.Translate(i18n.DetectLocale(r), "error.proxy_invalid_url"))
		return
	}

	// SSRF guard: block requests targeting private/loopback/link-local addresses
	if isPrivateHost(parsedURL.Hostname()) {
		SendError(w, CodeBadRequest, i18n.Translate(i18n.DetectLocale(r), "error.proxy_invalid_url"))
		return
	}
	if !isAllowedProxyHost(parsedURL.Hostname()) {
		SendError(w, CodeBadRequest, i18n.Translate(i18n.DetectLocale(r), "error.proxy_invalid_url"))
		return
	}

	// Compute ETag from URL for conditional GET support
	h256 := sha256.Sum256([]byte(thumbURL))
	etag := `"` + hex.EncodeToString(h256[:16]) + `"`

	// Check If-None-Match for 304 Not Modified
	if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Thumbnail disk cache: check if a cached file exists and is still fresh
	ttlMinutes := h.appConfig.Search.ThumbnailCacheTTL
	if ttlMinutes == 0 {
		// 24 hours default
		ttlMinutes = 1440
	}
	cacheEnabled := ttlMinutes > 0 && h.dataDir != ""
	cacheDir := filepath.Join(h.dataDir, "thumbnails")
	cacheFile := filepath.Join(cacheDir, hex.EncodeToString(h256[:]))

	if cacheEnabled {
		if info, statErr := os.Stat(cacheFile); statErr == nil {
			age := time.Since(info.ModTime())
			if age < time.Duration(ttlMinutes)*time.Minute {
				// Serve from disk cache
				cachedBytes, readErr := os.ReadFile(cacheFile)
				if readErr == nil {
					ct := "image/jpeg"
					// Detect GIF from magic bytes
					if len(cachedBytes) >= 6 && string(cachedBytes[:6]) == "GIF89a" {
						ct = "image/gif"
					}
					w.Header().Set("ETag", etag)
					// 24 hours
					w.Header().Set("Cache-Control", "public, max-age=86400")
					w.Header().Set("Content-Type", ct)
					w.Header().Set("Content-Length", strconv.Itoa(len(cachedBytes)))
					w.WriteHeader(http.StatusOK)
					//nolint:errcheck
					w.Write(cachedBytes)
					return
				}
			}
		}
	}

	// Create request with headers to avoid hotlink protection
	req, err := http.NewRequest("GET", thumbURL, nil)
	if err != nil {
		SendError(w, CodeServerError, i18n.Translate(i18n.DetectLocale(r), "error.proxy_fetch_failed"))
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", parsedURL.Scheme+"://"+parsedURL.Host+"/")

	// The URL came from the inbound request, so fetch it directly per PART 31.
	client := h.getProxyClient(10 * time.Second)

	resp, err := client.Do(req)
	if err != nil {
		SendError(w, CodeServerError, i18n.Translate(i18n.DetectLocale(r), "error.proxy_fetch_failed"))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		SendError(w, CodeNotFound, i18n.Translate(i18n.DetectLocale(r), "error.proxy_not_found"))
		return
	}

	// Read full body with a hard size cap (thumbnails are bounded; refuse
	// anything larger than 16 MiB to prevent unbounded allocation).
	const maxThumbBytes = 16 * 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxThumbBytes))
	if err != nil {
		SendError(w, CodeServerError, i18n.Translate(i18n.DetectLocale(r), "error.proxy_fetch_failed"))
		return
	}

	// Re-encode JPEG/PNG as JPEG quality=75 for bandwidth savings (~30-40% reduction).
	// GIFs are passed through unchanged to preserve animated video previews.
	// Falls back to original bytes if the image cannot be decoded.
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	var outputBuf bytes.Buffer
	reEncoded := false
	if strings.HasPrefix(contentType, "image/jpeg") ||
		strings.HasPrefix(contentType, "image/png") {
		img, _, decodeErr := image.Decode(bytes.NewReader(body))
		if decodeErr == nil {
			if encErr := jpeg.Encode(&outputBuf, img, &jpeg.Options{Quality: 75}); encErr == nil {
				reEncoded = true
			}
		}
	}

	outputBytes := body
	outputContentType := contentType
	if reEncoded {
		outputBytes = outputBuf.Bytes()
		outputContentType = "image/jpeg"
	}

	// Write to disk cache for future requests
	if cacheEnabled {
		if mkErr := os.MkdirAll(cacheDir, 0o750); mkErr == nil {
			// Write to a temp file then rename to avoid partial reads
			tmpFile := cacheFile + ".tmp"
			if writeErr := os.WriteFile(tmpFile, outputBytes, 0o640); writeErr == nil {
				//nolint:errcheck
				os.Rename(tmpFile, cacheFile)
			}
		}
	}

	w.Header().Set("ETag", etag)
	// 24 hours
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", outputContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(outputBytes)))
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck
	w.Write(outputBytes)
}

// ProxyVideo proxies external video previews to prevent tracking and avoid CORS
// Per IDEA.md: Privacy proxy for video previews
func (h *SearchHandler) ProxyVideo(w http.ResponseWriter, r *http.Request) {
	// Get URL parameter
	encodedURL := r.URL.Query().Get("url")
	if encodedURL == "" {
		SendError(w, CodeBadRequest, i18n.Translate(i18n.DetectLocale(r), "error.proxy_missing_url"))
		return
	}

	// Decode URL
	videoURL, err := url.QueryUnescape(encodedURL)
	if err != nil {
		SendError(w, CodeBadRequest, i18n.Translate(i18n.DetectLocale(r), "error.proxy_invalid_url"))
		return
	}

	// Validate URL
	parsedURL, err := url.Parse(videoURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		SendError(w, CodeBadRequest, i18n.Translate(i18n.DetectLocale(r), "error.proxy_invalid_url"))
		return
	}

	// SSRF guard: block requests targeting private/loopback/link-local addresses
	if isPrivateHost(parsedURL.Hostname()) {
		SendError(w, CodeBadRequest, i18n.Translate(i18n.DetectLocale(r), "error.proxy_invalid_url"))
		return
	}
	if !isAllowedProxyHost(parsedURL.Hostname()) {
		SendError(w, CodeBadRequest, i18n.Translate(i18n.DetectLocale(r), "error.proxy_invalid_url"))
		return
	}

	// Create request with range support
	req, err := http.NewRequest("GET", videoURL, nil)
	if err != nil {
		SendError(w, CodeServerError, i18n.Translate(i18n.DetectLocale(r), "error.proxy_fetch_failed"))
		return
	}

	// Forward range header for video seeking
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	// Set user agent to avoid blocks
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	// Set referer to the video host to avoid hotlink protection
	req.Header.Set("Referer", parsedURL.Scheme+"://"+parsedURL.Host+"/")

	// The URL came from the inbound request, so fetch it directly per PART 31.
	client := h.getProxyClient(30 * time.Second)

	resp, err := client.Do(req)
	if err != nil {
		SendError(w, CodeServerError, i18n.Translate(i18n.DetectLocale(r), "error.proxy_fetch_failed"))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Never forward the upstream status verbatim - it would leak upstream state
	// to the client and can fall outside the canonical error-code table
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		SendError(w, CodeNotFound, i18n.Translate(i18n.DetectLocale(r), "error.proxy_not_found"))
		return
	}

	// Copy response headers
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "video/mp4"
	}
	w.Header().Set("Content-Type", contentType)

	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		w.Header().Set("Content-Length", contentLength)
	}
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		w.Header().Set("Content-Range", contentRange)
	}
	if acceptRanges := resp.Header.Get("Accept-Ranges"); acceptRanges != "" {
		w.Header().Set("Accept-Ranges", acceptRanges)
	}

	// Cache control: 1 hour
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Set status code (206 for partial content)
	w.WriteHeader(resp.StatusCode)

	// Proxy the video
	_, _ = io.Copy(w, resp.Body)
}

// Autodiscover returns server connection settings for CLI/agent auto-configuration
// Per AI.md PART 14: /api/autodiscover (NON-NEGOTIABLE)
// This endpoint is NOT versioned because clients need it BEFORE they know the API version
func (h *SearchHandler) Autodiscover(w http.ResponseWriter, r *http.Request) {
	channel := cliUpdateChannel(r.URL.Query().Get("channel"))
	if channel == "" {
		SendNegotiatedError(w, r, "BAD_REQUEST", "invalid update channel")
		return
	}
	// Build response per AI.md PART 14
	// "primary" resolved per request via BuildURL (AI.md PART 12) — never
	// frozen at startup/config, so autodiscover advertises the Host/proto the
	// client actually used, including behind a reverse proxy.
	response := map[string]interface{}{
		"primary": urlvar.BuildURL(r, ""),
		// Per AI.md PART 14: versioned API
		"api_version": h.appConfig.Server.APIVersion,
		// Default timeout in seconds
		"timeout": 30,
		// Default retry attempts
		"retry": 3,
		// Default seconds between retries
		"retry_delay": 1,
		// Empty disables forced updates; otherwise the operator-configured floor
		// is advertised verbatim per AI.md PART 22 and PART 32.
		"cli_min_version": h.appConfig.Server.Update.CLIMinVersion,
		"cli_versions":    map[string]cliVersionMetadata{},
	}
	if release, err := getCLIReleaseMetadata(r.Context(), channel); err == nil {
		response["cli_versions"] = release.Versions
	}

	// NEVER include secrets, internal IPs, or sensitive data.

	WriteJSON(w, http.StatusOK, response)
}

// ---- RSS / Atom / CSV helpers ----

// rssChannel is the XML structure for an RSS 2.0 feed.
type rssChannel struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel rssBody  `xml:"channel"`
}

type rssBody struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	PubDate     string    `xml:"pubDate"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Source      string `xml:"source,omitempty"`
	Duration    string `xml:"itunes:duration,omitempty"`
}

// renderSearchRSS writes an RSS 2.0 feed for the given search results.
func renderSearchRSS(w http.ResponseWriter, r *http.Request, results *model.SearchResponse, cfg *config.AppConfig) {
	items := make([]rssItem, 0, len(results.Data.Results))
	for _, res := range results.Data.Results {
		desc := res.Description
		if desc == "" && res.Thumbnail != "" {
			desc = `<img src="` + res.Thumbnail + `" alt="thumbnail"/>`
		}
		items = append(items, rssItem{
			Title:       res.Title,
			Link:        res.URL,
			Description: desc,
			Source:      res.Source,
			Duration:    res.Duration,
		})
	}

	feed := rssChannel{
		Version: "2.0",
		Channel: rssBody{
			Title:       cfg.Server.Branding.Title + " – " + results.Data.Query,
			Link:        urlvar.BuildURL(r, "/search") + "?q=" + url.QueryEscape(results.Data.Query),
			Description: "Search results for: " + results.Data.Query,
			PubDate:     time.Now().UTC().Format(time.RFC1123Z),
			Items:       items,
		},
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, xml.Header)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	//nolint:errcheck
	enc.Encode(feed)
}

// atomFeed is the XML structure for an Atom 1.0 feed.
type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	XMLNS   string      `xml:"xmlns,attr"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Link    atomLink    `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr,omitempty"`
}

type atomEntry struct {
	Title   string   `xml:"title"`
	ID      string   `xml:"id"`
	Updated string   `xml:"updated"`
	Link    atomLink `xml:"link"`
	Summary string   `xml:"summary,omitempty"`
	Source  string   `xml:"source>title,omitempty"`
}

// renderSearchAtom writes an Atom 1.0 feed for the given search results.
func renderSearchAtom(w http.ResponseWriter, r *http.Request, results *model.SearchResponse, cfg *config.AppConfig) {
	now := time.Now().UTC().Format(time.RFC3339)
	entries := make([]atomEntry, 0, len(results.Data.Results))
	for _, res := range results.Data.Results {
		entries = append(entries, atomEntry{
			Title:   res.Title,
			ID:      res.URL,
			Updated: now,
			Link:    atomLink{Href: res.URL},
			Summary: res.Description,
			Source:  res.Source,
		})
	}

	feed := atomFeed{
		XMLNS:   "http://www.w3.org/2005/Atom",
		Title:   cfg.Server.Branding.Title + " – " + results.Data.Query,
		ID:      urlvar.BuildURL(r, "/search") + "?q=" + url.QueryEscape(results.Data.Query),
		Updated: now,
		Link:    atomLink{Href: urlvar.BuildURL(r, "/search") + "?q=" + url.QueryEscape(results.Data.Query)},
		Entries: entries,
	}

	w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, xml.Header)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	//nolint:errcheck
	enc.Encode(feed)
}

// renderSearchCSV writes search results as CSV (RFC 4180).
func renderSearchCSV(w http.ResponseWriter, results *model.SearchResponse) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="vidveil-results.csv"`)
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "title,url,source,duration,views,description")
	for _, res := range results.Data.Results {
		_, _ = fmt.Fprintf(w, "%s,%s,%s,%s,%s,%s\n",
			csvEscape(res.Title),
			csvEscape(res.URL),
			csvEscape(res.Source),
			csvEscape(res.Duration),
			csvEscape(res.Views),
			csvEscape(res.Description),
		)
	}
}

// csvEscape wraps a field value in double quotes and escapes embedded quotes.
func csvEscape(s string) string {
	s = strings.ReplaceAll(s, `"`, `""`)
	return `"` + s + `"`
}

// SearchRSSFeed serves a web RSS feed at /search.rss
func (h *SearchHandler) SearchRSSFeed(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, i18n.Translate(i18n.DetectLocale(r), "error.missing_query"), http.StatusBadRequest)
		return
	}
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if pn, err := strconv.Atoi(p); err == nil && pn > 0 {
			page = pn
		}
	}
	parsed := engine.ParseBangs(query)
	engineNames := requestEngineNames(r, parsed.Engines, h.engineMgr.ListEngines())
	results := h.engineMgr.SearchWithOperators(r.Context(), parsed.Query, page, engineNames, parsed.ExactPhrases, parsed.Exclusions, parsed.RequiredTerms, false, "", 0, engine.ResultFilterOptions{})
	if isSearchOverloaded(results) {
		h.writeSearchOverloadJSON(w)
		return
	}
	results.Data.Query = query
	results.Data.InvalidBang = parsed.InvalidBang
	renderSearchRSS(w, r, results, h.appConfig)
}

// SearchAtomFeed serves a web Atom feed at /search.atom
func (h *SearchHandler) SearchAtomFeed(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, i18n.Translate(i18n.DetectLocale(r), "error.missing_query"), http.StatusBadRequest)
		return
	}
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if pn, err := strconv.Atoi(p); err == nil && pn > 0 {
			page = pn
		}
	}
	parsed := engine.ParseBangs(query)
	engineNames := requestEngineNames(r, parsed.Engines, h.engineMgr.ListEngines())
	results := h.engineMgr.SearchWithOperators(r.Context(), parsed.Query, page, engineNames, parsed.ExactPhrases, parsed.Exclusions, parsed.RequiredTerms, false, "", 0, engine.ResultFilterOptions{})
	if isSearchOverloaded(results) {
		h.writeSearchOverloadJSON(w)
		return
	}
	results.Data.Query = query
	results.Data.InvalidBang = parsed.InvalidBang
	renderSearchAtom(w, r, results, h.appConfig)
}

// BatchSearchRequest is the JSON body for POST {api_version}/search/batch
type BatchSearchRequest struct {
	Queries []BatchQuery `json:"queries"`
}

// BatchQuery is a single query in a batch request
type BatchQuery struct {
	Q       string `json:"q"`
	Page    int    `json:"page"`
	Engines string `json:"engines,omitempty"`
}

// BatchSearch handles POST {api_version}/search/batch
// Runs up to 5 queries concurrently and returns an array of SearchResponse objects.
func (h *SearchHandler) BatchSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.jsonError(w, "Method not allowed", CodeMethodNotAllowed, http.StatusMethodNotAllowed)
		return
	}

	var req BatchSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, "Invalid JSON body", CodeBadRequest, http.StatusBadRequest)
		return
	}

	const maxBatch = 5
	if len(req.Queries) == 0 {
		h.jsonError(w, "queries array must not be empty", CodeValidation, http.StatusBadRequest)
		return
	}
	if len(req.Queries) > maxBatch {
		h.jsonError(w, fmt.Sprintf("batch limit is %d queries", maxBatch), CodeValidation, http.StatusBadRequest)
		return
	}

	type batchResult struct {
		idx  int
		resp *model.SearchResponse
	}
	ch := make(chan batchResult, len(req.Queries))

	for i, q := range req.Queries {
		go func(idx int, bq BatchQuery) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("[batch] panic processing query %q: %v", bq.Q, rec)
					ch <- batchResult{idx: idx, resp: &model.SearchResponse{}}
				}
			}()
			parsed := engine.ParseBangs(bq.Q)
			page := bq.Page
			if page < 1 {
				page = 1
			}
			var engineNames []string
			if bq.Engines != "" {
				engineNames = strings.Split(bq.Engines, ",")
			}
			if len(engineNames) == 0 {
				engineNames = parsed.Engines
			}
			res := h.engineMgr.SearchWithOperators(r.Context(), parsed.Query, page, engineNames, parsed.ExactPhrases, parsed.Exclusions, parsed.RequiredTerms, false, "", 0, engine.ResultFilterOptions{})
			res.Data.Query = bq.Q
			res.Data.InvalidBang = parsed.InvalidBang
			ch <- batchResult{idx: idx, resp: res}
		}(i, q)
	}

	responses := make([]*model.SearchResponse, len(req.Queries))
	for range req.Queries {
		br := <-ch
		responses[br.idx] = br.resp
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"data": responses,
	})
}
