// SPDX-License-Identifier: MIT
package engines

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/models"
	"github.com/apimgr/vidveil/src/services/tor"
	"github.com/apimgr/vidveil/src/services/utls"
)

// Feature represents optional engine capabilities
type Feature int

const (
	FeaturePagination Feature = iota
	FeatureSorting
	FeatureFiltering
	FeatureThumbnailPreview
)

// Engine interface defines what a search engine must implement
type Engine interface {
	Name() string
	DisplayName() string
	Search(ctx context.Context, query string, page int) ([]models.Result, error)
	IsAvailable() bool
	SupportsFeature(feature Feature) bool
	Tier() int
}

// ConfigurableEngine interface for engines that support configuration
type ConfigurableEngine interface {
	Engine
	SetEnabled(enabled bool)
	SetUseTor(useTor bool)
}

// BaseEngine provides common functionality for all engines
type BaseEngine struct {
	name           string
	displayName    string
	baseURL        string
	tier           int
	enabled        bool
	timeout        time.Duration
	useTor         bool
	useSpoofedTLS  bool
	httpClient     *http.Client
	spoofedClient  *http.Client
	torClient      *tor.Client
}

// NewBaseEngine creates a new base engine
func NewBaseEngine(name, displayName, baseURL string, tier int, cfg *config.Config, torClient *tor.Client) *BaseEngine {
	timeout := time.Duration(cfg.Search.EngineTimeout) * time.Second
	return &BaseEngine{
		name:          name,
		displayName:   displayName,
		baseURL:       baseURL,
		tier:          tier,
		enabled:       true,
		timeout:       timeout,
		useTor:        false,
		useSpoofedTLS: cfg.Search.SpoofTLS, // Use config setting
		httpClient:    createHTTPClient(cfg.Search.EngineTimeout),
		spoofedClient: utls.CreateHTTPClientWithFingerprint(timeout, "chrome"),
		torClient:     torClient,
	}
}

// Name returns the engine identifier
func (e *BaseEngine) Name() string {
	return e.name
}

// DisplayName returns the human-readable name
func (e *BaseEngine) DisplayName() string {
	return e.displayName
}

// Tier returns the engine tier (1=major, 2=popular, 3=additional)
func (e *BaseEngine) Tier() int {
	return e.tier
}

// IsAvailable checks if the engine is currently working
func (e *BaseEngine) IsAvailable() bool {
	return e.enabled
}

// SetEnabled sets the enabled state
func (e *BaseEngine) SetEnabled(enabled bool) {
	e.enabled = enabled
}

// SetUseTor sets whether to use Tor for requests
func (e *BaseEngine) SetUseTor(useTor bool) {
	e.useTor = useTor
}

// SetUseSpoofedTLS sets whether to use spoofed TLS fingerprint for Cloudflare bypass
func (e *BaseEngine) SetUseSpoofedTLS(use bool) {
	e.useSpoofedTLS = use
}

// GetClient returns the appropriate HTTP client
func (e *BaseEngine) GetClient() *http.Client {
	if e.useTor && e.torClient != nil {
		return e.torClient.HTTPClient()
	}
	if e.useSpoofedTLS && e.spoofedClient != nil {
		return e.spoofedClient
	}
	return e.httpClient
}

// RequestModifier is a function that can modify a request before it's sent
type RequestModifier func(*http.Request)

// MakeRequest performs an HTTP request with proper headers
func (e *BaseEngine) MakeRequest(ctx context.Context, reqURL string) (*http.Response, error) {
	return e.MakeRequestWithMod(ctx, reqURL, nil)
}

// MakeRequestWithMod performs an HTTP request with optional modifier
func (e *BaseEngine) MakeRequestWithMod(ctx context.Context, reqURL string, mod RequestModifier) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	// Set comprehensive browser-like headers to help bypass Cloudflare and similar protections
	ua := getRandomUserAgent()
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="120", "Not_A Brand";v="24", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	// Apply custom modifier if provided
	if mod != nil {
		mod(req)
	}

	client := e.GetClient()
	return client.Do(req)
}

// AddCookies is a helper to add cookies to a request
func AddCookies(cookies map[string]string) RequestModifier {
	return func(req *http.Request) {
		for name, value := range cookies {
			req.AddCookie(&http.Cookie{Name: name, Value: value})
		}
	}
}

// BuildSearchURL builds the search URL with query and page
func (e *BaseEngine) BuildSearchURL(path string, query string, page int) string {
	return fmt.Sprintf("%s%s", e.baseURL, strings.ReplaceAll(strings.ReplaceAll(path, "{query}", url.QueryEscape(query)), "{page}", strconv.Itoa(page)))
}

// GenerateResultID generates a unique ID for a result
func GenerateResultID(url, source string) string {
	hash := sha256.Sum256([]byte(url + source))
	return hex.EncodeToString(hash[:8])
}

// ParseDuration parses various duration formats to seconds
func ParseDuration(duration string) int {
	duration = strings.TrimSpace(duration)
	if duration == "" {
		return 0
	}

	// Handle formats like "12:34" or "1:23:45"
	parts := strings.Split(duration, ":")
	switch len(parts) {
	case 2: // mm:ss
		m, _ := strconv.Atoi(parts[0])
		s, _ := strconv.Atoi(parts[1])
		return m*60 + s
	case 3: // hh:mm:ss
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		s, _ := strconv.Atoi(parts[2])
		return h*3600 + m*60 + s
	}

	// Try to parse "12 min" or "12min" format
	re := regexp.MustCompile(`(\d+)\s*min`)
	if matches := re.FindStringSubmatch(duration); len(matches) > 1 {
		m, _ := strconv.Atoi(matches[1])
		return m * 60
	}

	return 0
}

// ParseViews parses view counts like "1.2M" or "500K" to integers
func ParseViews(views string) int64 {
	views = strings.TrimSpace(strings.ToUpper(views))
	if views == "" {
		return 0
	}

	// Remove "views" suffix
	views = strings.TrimSuffix(views, " VIEWS")
	views = strings.TrimSuffix(views, "VIEWS")

	multiplier := int64(1)
	if strings.HasSuffix(views, "K") {
		multiplier = 1000
		views = strings.TrimSuffix(views, "K")
	} else if strings.HasSuffix(views, "M") {
		multiplier = 1000000
		views = strings.TrimSuffix(views, "M")
	} else if strings.HasSuffix(views, "B") {
		multiplier = 1000000000
		views = strings.TrimSuffix(views, "B")
	}

	// Parse the number
	views = strings.ReplaceAll(views, ",", "")
	views = strings.ReplaceAll(views, " ", "")

	if f, err := strconv.ParseFloat(views, 64); err == nil {
		return int64(f * float64(multiplier))
	}

	return 0
}

// createHTTPClient creates an HTTP client with timeout and browser-like TLS
func createHTTPClient(timeoutSecs int) *http.Client {
	// Create a cookie jar to persist cookies across requests
	jar, _ := cookiejar.New(nil)

	// Use a transport with browser-like TLS settings
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
			// Use cipher suites that match Chrome
			CipherSuites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			},
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &http.Client{
		Timeout:   time.Duration(timeoutSecs) * time.Second,
		Transport: transport,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			// Preserve headers on redirect
			for key, val := range via[0].Header {
				if _, ok := req.Header[key]; !ok {
					req.Header[key] = val
				}
			}
			return nil
		},
	}
}

// getRandomUserAgent returns a random user agent string
func getRandomUserAgent() string {
	userAgents := []string{
		// Edge on Windows 11 - most common modern browser
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0",
		// Chrome on Windows 11
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		// Edge on Windows 10
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0",
		// Chrome on Mac
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		// Firefox on Windows 11
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
	}
	return userAgents[time.Now().UnixNano()%int64(len(userAgents))]
}
