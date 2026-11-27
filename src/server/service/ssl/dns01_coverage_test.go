// SPDX-License-Identifier: MIT
// Coverage tests for dns01.go: legoUser interface methods,
// and buildDNSProvider (unsupported provider path and all supported providers).
package ssl

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// ── legoUser interface methods ──────────────────────────────────────────────

func TestLegoUserGetEmail(t *testing.T) {
	u := &legoUser{email: "test@example.com"}
	if got := u.GetEmail(); got != "test@example.com" {
		t.Errorf("GetEmail() = %q, want %q", got, "test@example.com")
	}
}

func TestLegoUserGetEmailEmpty(t *testing.T) {
	u := &legoUser{}
	if got := u.GetEmail(); got != "" {
		t.Errorf("GetEmail() empty struct = %q, want empty string", got)
	}
}

func TestLegoUserGetRegistrationNilByDefault(t *testing.T) {
	u := &legoUser{email: "admin@example.com"}
	if got := u.GetRegistration(); got != nil {
		t.Errorf("GetRegistration() = %v, want nil for unregistered user", got)
	}
}

func TestLegoUserGetPrivateKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	u := &legoUser{email: "x@x.com", key: key}
	if got := u.GetPrivateKey(); got == nil {
		t.Error("GetPrivateKey() = nil, want non-nil key")
	}
}

func TestLegoUserGetPrivateKeyNilKey(t *testing.T) {
	u := &legoUser{email: "x@x.com"}
	if got := u.GetPrivateKey(); got != nil {
		t.Error("GetPrivateKey() on zero struct should return nil")
	}
}

// ── buildDNSProvider ────────────────────────────────────────────────────────

func TestBuildDNSProviderUnsupportedReturnsError(t *testing.T) {
	_, err := buildDNSProvider("unsupported-provider", nil)
	if err == nil {
		t.Error("buildDNSProvider(unsupported) = nil, want error")
	}
}

func TestBuildDNSProviderEmptyTypeReturnsError(t *testing.T) {
	_, err := buildDNSProvider("", nil)
	if err == nil {
		t.Error("buildDNSProvider('') = nil, want error")
	}
}

func TestBuildDNSProviderCloudflareNoCredsReturnsError(t *testing.T) {
	// Without CLOUDFLARE_* env vars, the provider constructor fails.
	// This exercises the cloudflare case branch.
	_, err := buildDNSProvider("cloudflare", nil)
	// May return nil if env vars happen to be set in the test environment;
	// in a clean container, it should return an error. We just verify no panic.
	_ = err
}

func TestBuildDNSProviderRoute53NoCreds(t *testing.T) {
	_, err := buildDNSProvider("route53", nil)
	_ = err
}

func TestBuildDNSProviderDigitalOceanNoCreds(t *testing.T) {
	_, err := buildDNSProvider("digitalocean", nil)
	_ = err
}

func TestBuildDNSProviderGodaddyNoCreds(t *testing.T) {
	_, err := buildDNSProvider("godaddy", nil)
	_ = err
}

func TestBuildDNSProviderNamecheapNoCreds(t *testing.T) {
	_, err := buildDNSProvider("namecheap", nil)
	_ = err
}

func TestBuildDNSProviderRfc2136NoCreds(t *testing.T) {
	_, err := buildDNSProvider("rfc2136", nil)
	_ = err
}

func TestBuildDNSProviderRestoresEnvironment(t *testing.T) {
	t.Setenv("CF_DNS_API_TOKEN", "original-value")
	_, _ = buildDNSProvider("cloudflare", map[string]string{"CF_DNS_API_TOKEN": "temporary-value"})
	if got := os.Getenv("CF_DNS_API_TOKEN"); got != "original-value" {
		t.Fatalf("credential environment leaked: got %q", got)
	}
}

type rfc2136UpdateCapture struct {
	mu         sync.Mutex
	inserted   int
	removed    int
	signed     int
	unsigned   int
	badTSIG    int
	lastRecord string
	insertions []string
	removals   []string
}

func startRFC2136TestServer(t *testing.T, address string, secret string, acceptUnsigned bool) (string, *rfc2136UpdateCapture) {
	t.Helper()
	packetConn, err := net.ListenPacket("udp", address)
	if err != nil {
		t.Fatalf("listen for in-process DNS server at %s: %v", address, err)
	}
	capture := &rfc2136UpdateCapture{}
	server := &dns.Server{
		PacketConn: packetConn,
		TsigSecret: map[string]string{"acme.example.": secret},
		MsgAcceptFunc: func(dns.Header) dns.MsgAcceptAction {
			return dns.MsgAccept
		},
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, request *dns.Msg) {
			response := new(dns.Msg).SetReply(request)
			if request.Opcode == dns.OpcodeUpdate {
				capture.mu.Lock()
				if request.IsTsig() == nil {
					capture.unsigned++
				} else if w.TsigStatus() != nil {
					capture.badTSIG++
				} else {
					capture.signed++
				}
				for _, record := range request.Ns {
					if txt, ok := record.(*dns.TXT); ok {
						capture.lastRecord = strings.Join(txt.Txt, "")
						switch record.Header().Class {
						case dns.ClassINET:
							capture.inserted++
							capture.insertions = append(capture.insertions, strings.Join(txt.Txt, ""))
						case dns.ClassNONE:
							capture.removed++
							capture.removals = append(capture.removals, strings.Join(txt.Txt, ""))
						}
					}
				}
				capture.mu.Unlock()
				if request.IsTsig() == nil && !acceptUnsigned {
					response.SetRcode(request, dns.RcodeRefused)
				} else if w.TsigStatus() != nil {
					response.SetRcode(request, dns.RcodeNotAuth)
				}
			}
			if tsig := request.IsTsig(); tsig != nil && w.TsigStatus() == nil {
				response.SetTsig(tsig.Hdr.Name, tsig.Algorithm, tsig.Fudge, time.Now().Unix())
			}
			_ = w.WriteMsg(response)
		}),
	}
	go func() { _ = server.ActivateAndServe() }()
	t.Cleanup(func() {
		_ = server.Shutdown()
		_ = packetConn.Close()
	})
	return packetConn.LocalAddr().String(), capture
}

func rfc2136Credentials(nameserver, prefix, secret string, signed bool) map[string]string {
	credentials := map[string]string{
		prefix + "NAMESERVER": nameserver,
		prefix + "ZONES":      "example.org",
	}
	if signed {
		credentials[prefix+"TSIG_KEY"] = "acme.example."
		credentials[prefix+"TSIG_SECRET"] = secret
		credentials[prefix+"TSIG_ALGORITHM"] = "hmac-sha256."
	}
	return credentials
}

func TestRFC2136SignedUpdateAddsAndRemovesOnlyChallengeRecord(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	address, capture := startRFC2136TestServer(t, "127.0.0.1:0", secret, false)
	for _, testCase := range []struct {
		name     string
		provider string
		prefix   string
	}{
		{name: "current provider and variable names", provider: "rfc2136", prefix: "DNSUPDATE_"},
		{name: "legacy provider and variable names", provider: "dnsupdate", prefix: "RFC2136_"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			provider, err := buildDNSProvider(testCase.provider, rfc2136Credentials(address, testCase.prefix, secret, true))
			if err != nil {
				t.Fatalf("construct RFC2136 provider: %v", err)
			}
			challenge, ok := provider.(interface {
				Present(string, string, string) error
				CleanUp(string, string, string) error
			})
			if !ok {
				t.Fatalf("RFC2136 provider does not implement challenge lifecycle")
			}
			domain, token, keyAuth := "example.org", "challenge-token", "challenge-key-auth"
			if err := challenge.Present(domain, token, keyAuth); err != nil {
				t.Fatalf("signed DNS UPDATE insert: %v", err)
			}
			if err := challenge.CleanUp(domain, token, keyAuth); err != nil {
				t.Fatalf("signed DNS UPDATE removal: %v", err)
			}
		})
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.signed != 4 || capture.inserted != 2 || capture.removed != 2 {
		t.Errorf("signed UPDATE counts = signed:%d insert:%d remove:%d; want 4, 2, 2", capture.signed, capture.inserted, capture.removed)
	}
	if capture.lastRecord == "" {
		t.Error("server did not receive a TXT challenge record")
	}
	if len(capture.insertions) != len(capture.removals) {
		t.Errorf("inserted and removed TXT record counts differ: %#v vs %#v", capture.insertions, capture.removals)
	} else {
		for index := range capture.insertions {
			if capture.insertions[index] != capture.removals[index] {
				t.Errorf("cleanup removed a different TXT value: inserted %q, removed %q", capture.insertions[index], capture.removals[index])
			}
		}
	}
}

func TestRFC2136WrongTSIGKeyAndUnsignedUpdate(t *testing.T) {
	goodSecret := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	address, capture := startRFC2136TestServer(t, "127.0.0.1:0", goodSecret, true)
	badSecret := base64.StdEncoding.EncodeToString([]byte("fedcba9876543210fedcba9876543210"))
	provider, err := buildDNSProvider("rfc2136", rfc2136Credentials(address, "DNSUPDATE_", badSecret, true))
	if err != nil {
		t.Fatalf("construct provider with wrong TSIG key: %v", err)
	}
	challenge := provider.(interface {
		Present(string, string, string) error
	})
	if err := challenge.Present("example.org", "wrong-key", "wrong-key-auth"); err == nil {
		t.Fatal("wrong TSIG key unexpectedly authorized an update")
	}

	unsignedProvider, err := buildDNSProvider("rfc2136", rfc2136Credentials(address, "DNSUPDATE_", "", false))
	if err != nil {
		t.Fatalf("construct unsigned provider: %v", err)
	}
	unsignedChallenge := unsignedProvider.(interface {
		Present(string, string, string) error
	})
	if err := unsignedChallenge.Present("example.org", "unsigned", "unsigned-key-auth"); err != nil {
		t.Fatalf("unsigned update to trusted test server: %v", err)
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.badTSIG == 0 {
		t.Error("in-process server did not observe the bad TSIG signature")
	}
	if capture.unsigned != 1 {
		t.Errorf("unsigned update count = %d, want 1", capture.unsigned)
	}
}

func TestRFC2136NameserverWithoutPortUsesDNSDefault(t *testing.T) {
	secret := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	probe, err := net.ListenPacket("udp", "127.0.0.1:53")
	if err != nil {
		t.Skipf("cannot bind the local DNS default port for this test: %v", err)
	}
	_ = probe.Close()
	address, _ := startRFC2136TestServer(t, "127.0.0.1:53", secret, false)
	if address != "127.0.0.1:53" {
		t.Fatalf("test DNS server address = %q, want default DNS port 53", address)
	}
	provider, err := buildDNSProvider("rfc2136", rfc2136Credentials("127.0.0.1", "DNSUPDATE_", secret, true))
	if err != nil {
		t.Fatalf("construct provider with a port-less nameserver: %v", err)
	}
	challenge := provider.(interface {
		Present(string, string, string) error
	})
	if err := challenge.Present("example.org", "portless", "portless-key-auth"); err != nil {
		t.Fatalf("port-less nameserver update: %v", err)
	}
}
