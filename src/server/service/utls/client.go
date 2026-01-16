// SPDX-License-Identifier: MIT
package utls

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"time"

	utls "github.com/refraction-networking/utls"
)

// UTLSClient provides HTTP client with spoofed TLS fingerprint
type UTLSClient struct {
	httpClient *http.Client
}

// NewUTLSClient creates a new uTLS client that mimics Chrome's TLS fingerprint
func NewUTLSClient(timeout time.Duration) *UTLSClient {
	jar, _ := cookiejar.New(nil)

	transport := &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialTLS(ctx, network, addr)
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &UTLSClient{
		httpClient: &http.Client{
			Timeout:   timeout,
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
		},
	}
}

// HTTPClient returns the underlying http.Client
func (c *UTLSClient) HTTPClient() *http.Client {
	return c.httpClient
}

// dialTLS creates a TLS connection with Chrome's fingerprint
func dialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	// Parse host from address
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	// Create TCP connection
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	// Create uTLS connection with Chrome fingerprint
	tlsConfig := &utls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
	}

	// Use Chrome 120 fingerprint - most common modern browser
	utlsConn := utls.UClient(conn, tlsConfig, utls.HelloChrome_120)

	// Perform handshake
	if err := utlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	return utlsConn, nil
}

// RoundTripper returns an http.RoundTripper with Chrome TLS fingerprint
func NewRoundTripper(timeout time.Duration) http.RoundTripper {
	return &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialTLS(ctx, network, addr)
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		TLSHandshakeTimeout: 10 * time.Second,
	}
}

// CreateHTTPClientWithFingerprint creates an HTTP client with browser TLS fingerprint
func CreateHTTPClientWithFingerprint(timeout time.Duration, fingerprint string) *http.Client {
	jar, _ := cookiejar.New(nil)

	var helloID utls.ClientHelloID
	switch fingerprint {
	case "chrome":
		helloID = utls.HelloChrome_120
	case "firefox":
		helloID = utls.HelloFirefox_120
	case "edge":
		helloID = utls.HelloEdge_106
	case "safari":
		helloID = utls.HelloSafari_16_0
	case "random":
		helloID = utls.HelloRandomized
	default:
		helloID = utls.HelloChrome_120
	}

	transport := &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialTLSWithFingerprint(ctx, network, addr, helloID)
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		TLSHandshakeTimeout: 10 * time.Second,
		// Also set regular TLS config for non-HTTPS connections
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
		},
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			for key, val := range via[0].Header {
				if _, ok := req.Header[key]; !ok {
					req.Header[key] = val
				}
			}
			return nil
		},
	}
}

// dialTLSWithFingerprint creates a TLS connection with specified fingerprint
func dialTLSWithFingerprint(ctx context.Context, network, addr string, helloID utls.ClientHelloID) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	tlsConfig := &utls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
	}

	utlsConn := utls.UClient(conn, tlsConfig, helloID)

	if err := utlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	return utlsConn, nil
}
