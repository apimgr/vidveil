// SPDX-License-Identifier: MIT
// AI.md PART 13: URL Variables & Reverse Proxy Headers
package urlvars

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Config holds URL detection configuration per AI.md
type Config struct {
	Learning     bool          `yaml:"learning" json:"learning"`
	MinSamples   int           `yaml:"min_samples" json:"min_samples"`
	SampleWindow time.Duration `yaml:"sample_window" json:"sample_window"`
	LogChanges   bool          `yaml:"log_changes" json:"log_changes"`
	LiveReload   bool          `yaml:"live_reload" json:"live_reload"`
}

// DefaultConfig returns sane defaults per AI.md
func DefaultConfig() Config {
	return Config{
		Learning:     true,
		MinSamples:   3,
		SampleWindow: 5 * time.Minute,
		LogChanges:   true,
		LiveReload:   true,
	}
}

// domainObservation tracks domain observations for learning
type domainObservation struct {
	domain    string
	count     int
	firstSeen time.Time
	lastSeen  time.Time
}

// Resolver handles URL variable resolution per AI.md PART 13
type Resolver struct {
	mu           sync.RWMutex
	config       Config
	observations map[string]*domainObservation
	baseDomain   string
	wildcard     string
	logger       func(format string, args ...interface{})
}

// New creates a new URL resolver
func New(cfg Config) *Resolver {
	return &Resolver{
		config:       cfg,
		observations: make(map[string]*domainObservation),
	}
}

// SetLogger sets the logger function
func (r *Resolver) SetLogger(logger func(format string, args ...interface{})) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger = logger
}

// log logs a message if logger is set and LogChanges is enabled
func (r *Resolver) log(format string, args ...interface{}) {
	if r.logger != nil && r.config.LogChanges {
		r.logger(format, args...)
	}
}

// GetURLVars returns resolved URL variables from request per AI.md
// Checks reverse proxy headers first, triggers live reload on detection
// Port is empty string for 80/443 (always stripped)
func (r *Resolver) GetURLVars(req *http.Request) (proto, fqdn, port string) {
	proto = r.resolveProto(req)
	fqdn = r.resolveFQDN(req)
	port = r.resolvePort(req, proto)

	// Record observation for learning
	if r.config.Learning && fqdn != "localhost" {
		r.recordObservation(fqdn)
	}

	return
}

// resolveProto resolves protocol per AI.md priority order
func (r *Resolver) resolveProto(req *http.Request) string {
	// Priority 1: X-Forwarded-Proto
	if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
		return strings.ToLower(proto)
	}

	// Priority 2: X-Forwarded-Ssl
	if ssl := req.Header.Get("X-Forwarded-Ssl"); strings.EqualFold(ssl, "on") {
		return "https"
	}

	// Priority 3: X-Url-Scheme
	if scheme := req.Header.Get("X-Url-Scheme"); scheme != "" {
		return strings.ToLower(scheme)
	}

	// Priority 4: TLS on connection
	if req.TLS != nil {
		return "https"
	}

	// Priority 5: Default
	return "http"
}

// resolveFQDN resolves FQDN per AI.md priority order
func (r *Resolver) resolveFQDN(req *http.Request) string {
	// Priority 1: Reverse Proxy Headers
	if host := req.Header.Get("X-Forwarded-Host"); host != "" {
		// Strip port if present
		if h, _, err := net.SplitHostPort(host); err == nil {
			return h
		}
		return host
	}
	if host := req.Header.Get("X-Real-Host"); host != "" {
		if h, _, err := net.SplitHostPort(host); err == nil {
			return h
		}
		return host
	}
	if host := req.Header.Get("X-Original-Host"); host != "" {
		if h, _, err := net.SplitHostPort(host); err == nil {
			return h
		}
		return host
	}

	// Priority 2: DOMAIN env var (first in comma-separated list)
	if domain := os.Getenv("DOMAIN"); domain != "" {
		parts := strings.Split(domain, ",")
		if len(parts) > 0 && parts[0] != "" {
			return strings.TrimSpace(parts[0])
		}
	}

	// Priority 3: os.Hostname()
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hostname
	}

	// Priority 4: $HOSTNAME env var
	if hostname := os.Getenv("HOSTNAME"); hostname != "" {
		return hostname
	}

	// Priority 5 & 6: Public IP detection
	if publicIP := getPublicIP(); publicIP != "" {
		return publicIP
	}

	// Priority 7: localhost
	return "localhost"
}

// resolvePort resolves port per AI.md priority order
// Returns empty string for 80/443 (port stripping)
func (r *Resolver) resolvePort(req *http.Request, proto string) string {
	var port string

	// Priority 1: X-Forwarded-Port
	if p := req.Header.Get("X-Forwarded-Port"); p != "" {
		port = p
	} else if _, p, err := net.SplitHostPort(req.Host); err == nil && p != "" {
		// Priority 2: Host header port
		port = p
	} else if addr := req.Context().Value(http.LocalAddrContextKey); addr != nil {
		// Priority 3: Server listen port
		if tcpAddr, ok := addr.(*net.TCPAddr); ok {
			port = strings.TrimPrefix(strings.TrimPrefix(tcpAddr.String(), "[::]:"), ":")
		}
	} else {
		// Priority 4: Proto default
		if proto == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	// Port stripping: 80 and 443 are NEVER included per AI.md
	if port == "80" || port == "443" {
		return ""
	}

	return port
}

// getPublicIP returns first public IP (IPv6 preferred, then IPv4)
func getPublicIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	var ipv6, ipv4 string

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		ip := ipNet.IP

		// Skip loopback, link-local, private
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
			continue
		}

		// Must be global unicast
		if !ip.IsGlobalUnicast() {
			continue
		}

		// Prefer IPv6
		if ip.To4() == nil {
			if ipv6 == "" {
				ipv6 = ip.String()
			}
		} else {
			if ipv4 == "" {
				ipv4 = ip.String()
			}
		}
	}

	// IPv6 preferred per AI.md
	if ipv6 != "" {
		return ipv6
	}
	return ipv4
}

// recordObservation records a domain observation for learning
func (r *Resolver) recordObservation(domain string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Clean old observations outside sample window
	for d, obs := range r.observations {
		if now.Sub(obs.lastSeen) > r.config.SampleWindow {
			delete(r.observations, d)
		}
	}

	// Record or update observation
	if obs, ok := r.observations[domain]; ok {
		obs.count++
		obs.lastSeen = now
	} else {
		r.observations[domain] = &domainObservation{
			domain:    domain,
			count:     1,
			firstSeen: now,
			lastSeen:  now,
		}
	}

	// Check if we can infer patterns
	r.inferPatterns()
}

// inferPatterns infers base domain and wildcard from observations
func (r *Resolver) inferPatterns() {
	if len(r.observations) < r.config.MinSamples {
		return
	}

	// Group by base domain (extract TLD+1)
	domainCounts := make(map[string]int)
	for _, obs := range r.observations {
		base := extractBaseDomain(obs.domain)
		domainCounts[base] += obs.count
	}

	// Find most common base domain
	var mostCommon string
	var maxCount int
	for base, count := range domainCounts {
		if count > maxCount {
			maxCount = count
			mostCommon = base
		}
	}

	// Check for wildcard pattern (multiple subdomains of same base)
	subdomains := 0
	for _, obs := range r.observations {
		base := extractBaseDomain(obs.domain)
		if base == mostCommon && obs.domain != base {
			subdomains++
		}
	}

	oldBase := r.baseDomain
	oldWildcard := r.wildcard

	r.baseDomain = mostCommon

	if subdomains >= 2 {
		r.wildcard = "*." + mostCommon
	}

	// Log changes if enabled
	if r.config.LogChanges {
		if oldBase != r.baseDomain {
			r.log("URL detection: base domain changed from %s to %s", oldBase, r.baseDomain)
		}
		if oldWildcard != r.wildcard && r.wildcard != "" {
			r.log("URL detection: wildcard inferred as %s", r.wildcard)
		}
	}
}

// extractBaseDomain extracts base domain (TLD+1) from hostname
func extractBaseDomain(hostname string) string {
	parts := strings.Split(hostname, ".")
	if len(parts) <= 2 {
		return hostname
	}
	// Return last two parts (e.g., example.com from www.example.com)
	return strings.Join(parts[len(parts)-2:], ".")
}

// BuildURL constructs full URL with automatic port stripping per AI.md
// :80 and :443 are NEVER included
func (r *Resolver) BuildURL(req *http.Request, path string) string {
	proto, fqdn, port := r.GetURLVars(req)
	if port == "" {
		return proto + "://" + fqdn + path
	}
	return proto + "://" + fqdn + ":" + port + path
}

// GetBaseDomain returns inferred base domain from learning
// Returns: "myapp.com" even if accessed via "www.myapp.com"
func (r *Resolver) GetBaseDomain() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.baseDomain
}

// GetWildcardDomain returns inferred wildcard if detected
// Returns: "*.myapp.com" or empty if no wildcard pattern
func (r *Resolver) GetWildcardDomain() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.wildcard
}

// GetAllDomains returns all domains from DOMAIN env var
func GetAllDomains() []string {
	domain := os.Getenv("DOMAIN")
	if domain == "" {
		return nil
	}

	parts := strings.Split(domain, ",")
	domains := make([]string, 0, len(parts))
	for _, p := range parts {
		if d := strings.TrimSpace(p); d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

// Global resolver instance
var (
	globalResolver *Resolver
	globalOnce     sync.Once
)

// Global returns the global resolver instance
func Global() *Resolver {
	globalOnce.Do(func() {
		globalResolver = New(DefaultConfig())
	})
	return globalResolver
}

// GetURLVars is a convenience function using global resolver
func GetURLVars(req *http.Request) (proto, fqdn, port string) {
	return Global().GetURLVars(req)
}

// BuildURL is a convenience function using global resolver
func BuildURL(req *http.Request, path string) string {
	return Global().BuildURL(req, path)
}

// GetBaseDomain is a convenience function using global resolver
func GetBaseDomain() string {
	return Global().GetBaseDomain()
}

// GetWildcardDomain is a convenience function using global resolver
func GetWildcardDomain() string {
	return Global().GetWildcardDomain()
}

// Middleware returns HTTP middleware that sets X-Resolved-* headers for templates
func (r *Resolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		proto, fqdn, port := r.GetURLVars(req)
		// Set resolved values as headers for downstream handlers
		req.Header.Set("X-Resolved-Proto", proto)
		req.Header.Set("X-Resolved-Host", fqdn)
		if port != "" {
			req.Header.Set("X-Resolved-Port", port)
		}
		// Set full base URL for convenience
		baseURL := proto + "://" + fqdn
		if port != "" {
			baseURL += ":" + port
		}
		req.Header.Set("X-Resolved-BaseURL", baseURL)
		next.ServeHTTP(w, req)
	})
}
