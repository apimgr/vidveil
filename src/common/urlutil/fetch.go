// Package urlutil contains URL helpers used by server-side fetchers.
package urlutil

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// FetchRemoteImageConfig configures a bounded, HTTPS-only remote image fetch.
type FetchRemoteImageConfig struct {
	MaxSize        int64
	Timeout        time.Duration
	AllowedTypes   []string
	AllowedSchemes []string
}

// DefaultFetchRemoteImageConfig returns the AI.md PART 16 safe defaults.
func DefaultFetchRemoteImageConfig() FetchRemoteImageConfig {
	return FetchRemoteImageConfig{
		MaxSize: 10 * 1024 * 1024, Timeout: 30 * time.Second,
		AllowedTypes:   []string{"image/png", "image/jpeg", "image/gif", "image/webp", "image/x-icon"},
		AllowedSchemes: []string{"https"},
	}
}

func (c FetchRemoteImageConfig) normalized() FetchRemoteImageConfig {
	d := DefaultFetchRemoteImageConfig()
	if c.MaxSize > 0 {
		d.MaxSize = c.MaxSize
	}
	if c.Timeout > 0 {
		d.Timeout = c.Timeout
	}
	if len(c.AllowedTypes) > 0 {
		d.AllowedTypes = append([]string(nil), c.AllowedTypes...)
	}
	if len(c.AllowedSchemes) > 0 {
		d.AllowedSchemes = append([]string(nil), c.AllowedSchemes...)
	}
	return d
}

// ValidateRemoteURL checks an image URL and all of its resolved addresses.
func ValidateRemoteURL(rawURL string, cfg FetchRemoteImageConfig) error {
	cfg = cfg.normalized()
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("URL must have a host and no user information")
	}
	allowed := false
	for _, scheme := range cfg.AllowedSchemes {
		if strings.EqualFold(u.Scheme, scheme) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("scheme not allowed: %s", u.Scheme)
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return fmt.Errorf("local or internal hostnames are not allowed")
	}
	ips, err := resolveAndValidate(context.Background(), host)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return fmt.Errorf("host has no addresses")
	}
	return nil
}

func resolveAndValidate(ctx context.Context, host string) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		if !publicIP(ip) {
			return nil, fmt.Errorf("private or special IP address is not allowed")
		}
		return []netip.Addr{ip}, nil
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed: %w", err)
	}
	for _, ip := range ips {
		if !publicIP(ip) {
			return nil, fmt.Errorf("host resolves to a private or special IP address")
		}
	}
	return ips, nil
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	for _, block := range nonPublicRanges {
		if block.Contains(ip) {
			return false
		}
	}
	return true
}

var nonPublicRanges = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

// FetchRemoteImage fetches one image with DNS pinning, redirect checks, a size
// limit, and strict response MIME validation.
func FetchRemoteImage(ctx context.Context, rawURL string, cfg FetchRemoteImageConfig) ([]byte, string, error) {
	cfg = cfg.normalized()
	if err := ValidateRemoteURL(rawURL, cfg); err != nil {
		return nil, "", fmt.Errorf("URL validation failed: %w", err)
	}
	dialer := &net.Dialer{Timeout: cfg.Timeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := resolveAndValidate(ctx, host)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ip := range ips {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: cfg.Timeout, Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		if err := ValidateRemoteURL(req.URL.String(), cfg); err != nil {
			return fmt.Errorf("redirect blocked: %w", err)
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "vidveil/1.0")
	req.Header.Set("Accept", strings.Join(cfg.AllowedTypes, ", "))
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching URL: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, "", fmt.Errorf("invalid content type: %w", err)
	}
	typeAllowed := false
	for _, allowedType := range cfg.AllowedTypes {
		if strings.EqualFold(mediaType, allowedType) {
			typeAllowed = true
			break
		}
	}
	if !typeAllowed {
		return nil, "", fmt.Errorf("content type not allowed: %s", mediaType)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, cfg.MaxSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("reading response: %w", err)
	}
	if int64(len(data)) > cfg.MaxSize {
		return nil, "", fmt.Errorf("file too large (max: %d bytes)", cfg.MaxSize)
	}
	return data, mediaType, nil
}
