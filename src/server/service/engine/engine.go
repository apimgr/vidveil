// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/mode"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/retry"
	"github.com/apimgr/vidveil/src/server/service/utls"
)

// Feature represents optional engine capabilities
type Feature int

const (
	FeaturePagination Feature = iota
	FeatureSorting
	FeatureFiltering
	FeatureThumbnailPreview
)

// Capabilities describes what data an engine can provide
// Per IDEA.md Engine Capability Declaration
type Capabilities struct {
	HasPreview    bool   `json:"has_preview"`     // Can provide PreviewURL
	HasDownload   bool   `json:"has_download"`    // Can provide DownloadURL
	HasDuration   bool   `json:"has_duration"`    // Can provide duration
	HasViews      bool   `json:"has_views"`       // Can provide view count
	HasRating     bool   `json:"has_rating"`      // Can provide rating
	HasQuality    bool   `json:"has_quality"`     // Can provide quality badge
	HasUploadDate bool   `json:"has_upload_date"` // Can provide upload date
	PreviewSource string `json:"preview_source"`  // e.g., "data-preview", "data-mediabook", "api"
	APIType       string `json:"api_type"`        // "api", "html", "json_extraction"
}

// SearchEngine interface defines what a search engine must implement
type SearchEngine interface {
	Name() string
	DisplayName() string
	Search(ctx context.Context, query string, page int) ([]model.VideoResult, error)
	IsAvailable() bool
	SupportsFeature(feature Feature) bool
	Tier() int
	Capabilities() Capabilities
}

// ConfigurableSearchEngine interface for engines that support configuration
type ConfigurableSearchEngine interface {
	SearchEngine
	SetEnabled(enabled bool)
}

// BaseEngine provides common functionality for all engines
// Per PART 30: Tor is ONLY for hidden service, NOT for outbound proxy
type BaseEngine struct {
	name           string
	displayName    string
	baseURL        string
	tier           int
	enabled        bool
	timeout        time.Duration
	useSpoofedTLS  bool
	appConfig      *config.AppConfig
	httpClient     *http.Client
	spoofedClient  *http.Client
	circuitBreaker *retry.CircuitBreaker
	retryConfig    *retry.RetryConfig
	capabilities   Capabilities
}

// NewBaseEngine creates a new base engine
func NewBaseEngine(name, displayName, baseURL string, tier int, appConfig *config.AppConfig) *BaseEngine {
	timeout := time.Duration(appConfig.Search.EngineTimeout) * time.Second

	// Create circuit breaker for this engine
	cbConfig := retry.DefaultCircuitBreakerConfig(name)
	// Open after 5 failures
	cbConfig.FailureThreshold = 5
	// Close after 2 successes in half-open
	cbConfig.SuccessThreshold = 2
	cbConfig.Timeout = 30 * time.Second

	// Create retry config for transient errors
	retryConfig := &retry.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     2 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
		RetryableErrors: []error{
			retry.ErrTemporary,
			retry.ErrTimeout,
			retry.ErrNetworkError,
			retry.ErrServerError,
		},
	}

	return &BaseEngine{
		name:           name,
		displayName:    displayName,
		baseURL:        baseURL,
		tier:           tier,
		enabled:        true,
		timeout:        timeout,
		useSpoofedTLS:  appConfig.Search.SpoofTLS,
		appConfig:      appConfig,
		httpClient:     createHTTPClient(appConfig.Search.EngineTimeout),
		spoofedClient:  utls.CreateHTTPClientWithFingerprint(timeout, "chrome"),
		circuitBreaker: retry.NewCircuitBreaker(cbConfig),
		retryConfig:    retryConfig,
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

// SetUseSpoofedTLS sets whether to use spoofed TLS fingerprint for Cloudflare bypass
func (e *BaseEngine) SetUseSpoofedTLS(use bool) {
	e.useSpoofedTLS = use
}

// Capabilities returns the engine's data capabilities
func (e *BaseEngine) Capabilities() Capabilities {
	return e.capabilities
}

// SetCapabilities sets the engine's data capabilities
func (e *BaseEngine) SetCapabilities(caps Capabilities) {
	e.capabilities = caps
}

// BaseURL returns the engine's base URL
func (e *BaseEngine) BaseURL() string {
	return e.baseURL
}

// GetClient returns the appropriate HTTP client
// Per PART 30: Tor is ONLY for hidden service, NOT for outbound proxy
func (e *BaseEngine) GetClient() *http.Client {
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
// Uses circuit breaker and retry logic for resilience
func (e *BaseEngine) MakeRequestWithMod(ctx context.Context, reqURL string, mod RequestModifier) (*http.Response, error) {
	// Check circuit breaker first
	if !e.circuitBreaker.AllowRequest() {
		return nil, retry.ErrCircuitOpen
	}

	var resp *http.Response
	var lastErr error

	// Execute with retry logic
	err := retry.ExecuteWithRetry(ctx, e.retryConfig, func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			return err
		}

		// Set browser headers using configured user agent
		req.Header.Set("User-Agent", e.GetUserAgent())
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
		req.Header.Set("Sec-Fetch-User", "?1")
		// Sec-Ch-* headers only for Chromium-based browsers
		if e.appConfig != nil && e.appConfig.Engines.UserAgent.IsChromiumBased() {
			req.Header.Set("Sec-Ch-Ua", e.appConfig.Engines.UserAgent.SecChUa())
			req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
			req.Header.Set("Sec-Ch-Ua-Platform", e.appConfig.Engines.UserAgent.SecChUaPlatform())
		} else if e.appConfig == nil {
			// Fallback to Chrome defaults
			req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`)
			req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
			req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
		}

		// Apply custom modifier if provided
		if mod != nil {
			mod(req)
		}

		client := e.GetClient()
		resp, lastErr = client.Do(req)
		if lastErr != nil {
			// Classify error for retry logic
			return classifyHTTPError(lastErr)
		}

		// Check for server errors that should trigger retry
		if resp.StatusCode >= 500 {
			// Close the body to allow retry
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return retry.ErrServerError
		}

		// Check for rate limiting
		if resp.StatusCode == 429 {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return retry.ErrRateLimit
		}

		return nil
	})

	if err != nil {
		e.circuitBreaker.RecordFailure()
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, err
	}

	e.circuitBreaker.RecordSuccess()
	return resp, nil
}

// classifyHTTPError converts an HTTP error to a retryable error type
func classifyHTTPError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check for timeout errors
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return fmt.Errorf("%w: %v", retry.ErrTimeout, err)
	}

	// Check for network errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "connection reset") {
		return fmt.Errorf("%w: %v", retry.ErrNetworkError, err)
	}

	// Check for temporary errors
	if retry.IsTemporaryError(err) {
		return fmt.Errorf("%w: %v", retry.ErrTemporary, err)
	}

	return err
}

// GetCircuitBreakerState returns the current circuit breaker state for this engine
func (e *BaseEngine) GetCircuitBreakerState() retry.CircuitBreakerState {
	return e.circuitBreaker.GetState()
}

// ResetCircuitBreaker resets the circuit breaker to closed state
func (e *BaseEngine) ResetCircuitBreaker() {
	e.circuitBreaker.Reset()
}

// IsCircuitOpen returns true if the circuit breaker is open
func (e *BaseEngine) IsCircuitOpen() bool {
	return e.circuitBreaker.GetState() == retry.CircuitBreakerStateOpen
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

// DefaultUserAgent is the fallback user agent when config is nil
// Windows 11 Chrome x86_64 - most common browser/OS combination for best compatibility
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// GetUserAgent returns the configured user agent string
// Uses search.useragent config for customizable user agent without rebuild
func (e *BaseEngine) GetUserAgent() string {
	if e.appConfig != nil {
		return e.appConfig.Engines.UserAgent.String()
	}
	return DefaultUserAgent
}

// DebugLogEngineResponse logs raw response body when --debug is enabled
// Per IDEA.md: Verbose logging helps identify site changes and extraction opportunities
// Per AI.md: Debug features tied to --debug flag
func DebugLogEngineResponse(engineName, requestURL string, body []byte) {
	if !mode.IsDebugEnabled() {
		return
	}

	// Truncate to 2000 chars as per IDEA.md spec
	bodyStr := string(body)
	if len(bodyStr) > 2000 {
		bodyStr = bodyStr[:2000] + "\n... [truncated]"
	}

	log.Printf("[DEBUG ENGINE] %s request: %s\n[DEBUG ENGINE] %s response (%d bytes):\n%s\n",
		engineName, requestURL, engineName, len(body), bodyStr)
}

// DebugLogEngineParseResult logs parsing results when --debug is enabled
// Helps identify extraction successes/failures and missing fields
func DebugLogEngineParseResult(engineName string, resultCount int, fieldStats map[string]int) {
	if !mode.IsDebugEnabled() {
		return
	}

	statsStr := ""
	for field, count := range fieldStats {
		statsStr += fmt.Sprintf(" %s=%d", field, count)
	}

	log.Printf("[DEBUG ENGINE] %s parsed %d results:%s\n", engineName, resultCount, statsStr)
}
