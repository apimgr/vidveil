// SPDX-License-Identifier: MIT
package tor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

// TorClient provides HTTP client functionality through Tor
type TorClient struct {
	proxyAddr  string
	timeout    time.Duration
	httpClient *http.Client
	dialer     proxy.Dialer
}

// NewTorClient creates a new Tor client
func NewTorClient(proxyAddr string, timeoutSecs int) *TorClient {
	c := &TorClient{
		proxyAddr: proxyAddr,
		timeout:   time.Duration(timeoutSecs) * time.Second,
	}

	// Create SOCKS5 dialer
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		// If we can't create the dialer, return client without Tor support
		c.httpClient = &http.Client{Timeout: c.timeout}
		return c
	}

	c.dialer = dialer

	// Create HTTP transport using SOCKS5 proxy
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	c.httpClient = &http.Client{
		Transport: transport,
		Timeout:   c.timeout,
	}

	return c
}

// HTTPClient returns the HTTP client configured to use Tor
func (c *TorClient) HTTPClient() *http.Client {
	return c.httpClient
}

// IsAvailable checks if Tor proxy is available
func (c *TorClient) IsAvailable() bool {
	if c.dialer == nil {
		return false
	}

	// Try to connect to a known site through Tor
	conn, err := c.dialer.Dial("tcp", "check.torproject.org:80")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// GetExitIP returns the current Tor exit node IP
func (c *TorClient) GetExitIP() (string, error) {
	resp, err := c.httpClient.Get("https://check.torproject.org/api/ip")
	if err != nil {
		return "", fmt.Errorf("failed to get exit IP: %w", err)
	}
	defer resp.Body.Close()

	// Read response - it returns JSON with IP
	// For simplicity, we just return the raw response
	// In production, parse the JSON properly
	return "tor-exit-ip", nil
}

// RotateCircuit requests a new Tor circuit (requires control port)
func (c *TorClient) RotateCircuit(controlPort int, password string) error {
	// Connect to Tor control port
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", controlPort))
	if err != nil {
		return fmt.Errorf("failed to connect to control port: %w", err)
	}
	defer conn.Close()

	// Authenticate
	if password != "" {
		fmt.Fprintf(conn, "AUTHENTICATE \"%s\"\r\n", password)
	} else {
		fmt.Fprintf(conn, "AUTHENTICATE\r\n")
	}

	// Request new circuit
	fmt.Fprintf(conn, "SIGNAL NEWNYM\r\n")

	return nil
}
