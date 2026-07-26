// SPDX-License-Identifier: MIT
// AI.md PART 15: Coverage tests for the TLS/ACME serving wiring —
// SetSSLManager, ServeTLSOn, ServeHTTPRedirectOn, and httpRedirectHandler.
package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeTLSProvider is a minimal TLSProvider for exercising the serve wiring.
type fakeTLSProvider struct {
	cert    tls.Certificate
	acmeHit *bool
}

func (f *fakeTLSProvider) GetTLSConfig() *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{f.cert}}
}

func (f *fakeTLSProvider) GetHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if f.acmeHit != nil {
			*f.acmeHit = true
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "acme-ok")
	})
}

// newSelfSignedCert builds a throwaway self-signed certificate for TLS serving.
func newSelfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func TestSetSSLManager_Attaches(t *testing.T) {
	s := newTestServer(t)
	if s.tlsProvider != nil {
		t.Fatal("tlsProvider should be nil before SetSSLManager")
	}
	p := &fakeTLSProvider{}
	s.SetSSLManager(p)
	if s.tlsProvider == nil {
		t.Fatal("SetSSLManager did not attach the provider")
	}
}

func TestServeTLSOn_NilProviderReturnsServerClosed(t *testing.T) {
	s := newTestServer(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()
	if err := s.ServeTLSOn(ln); !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("expected ErrServerClosed with nil provider, got %v", err)
	}
}

func TestServeTLSOn_ServesHTTPS(t *testing.T) {
	s := newTestServer(t)
	s.SetSSLManager(&fakeTLSProvider{cert: newSelfSignedCert(t)})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go func() { _ = s.ServeTLSOn(ln) }()
	defer func() { _ = s.Shutdown(context.Background()) }()

	addr := ln.Addr().String()
	client := &http.Client{
		Timeout:   3 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = client.Get("https://" + addr + "/health")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("HTTPS GET failed: %v", err)
	}
	_ = resp.Body.Close()
}

func TestHTTPRedirectHandler_RedirectsToHTTPS(t *testing.T) {
	s := newTestServer(t)
	s.SetSSLManager(&fakeTLSProvider{})
	h := s.httpRedirectHandler("443")

	req := httptest.NewRequest("GET", "http://example.com:80/search?q=x", nil)
	req.Host = "example.com:80"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc != "https://example.com/search?q=x" {
		t.Fatalf("unexpected redirect target (should strip :443): %q", loc)
	}
}

func TestHTTPRedirectHandler_CustomHTTPSPort(t *testing.T) {
	s := newTestServer(t)
	h := s.httpRedirectHandler("8443")

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Host = "example.com"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	loc := rr.Header().Get("Location")
	if loc != "https://example.com:8443/" {
		t.Fatalf("expected custom https port in target, got %q", loc)
	}
}

func TestHTTPRedirectHandler_RoutesACMEChallenge(t *testing.T) {
	s := newTestServer(t)
	hit := false
	s.SetSSLManager(&fakeTLSProvider{acmeHit: &hit})
	h := s.httpRedirectHandler("443")

	req := httptest.NewRequest("GET", "http://example.com/.well-known/acme-challenge/tok", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !hit {
		t.Fatal("ACME challenge was not routed to the TLS provider's HTTP handler")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from ACME handler, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "acme-ok") {
		t.Fatalf("unexpected ACME body: %q", rr.Body.String())
	}
}

func TestServeHTTPRedirectOn_ServesRedirect(t *testing.T) {
	s := newTestServer(t)
	s.SetSSLManager(&fakeTLSProvider{})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	srvErr := make(chan error, 1)
	go func() { srvErr <- s.ServeHTTPRedirectOn(ln, "443") }()

	addr := ln.Addr().String()
	client := &http.Client{
		Timeout: 3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = client.Get("http://" + addr + "/foo")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		_ = ln.Close()
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("expected 301 from redirect server, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	_ = ln.Close()
	<-srvErr
}
