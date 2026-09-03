// SPDX-License-Identifier: MIT
// Coverage tests for the internal loopback-only Tor control channel
// (AI.md PART 31 "CLI-to-running-server control channel"): isLoopbackPeer,
// requireLoopback, route registration, and every handler in tor_control.go,
// including the torControl == nil not-configured path.
package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/server/service/tor"
)

// fakeTorControl is a minimal, fully scriptable TorControl implementation
// used to exercise every handler branch without a real Tor process.
type fakeTorControl struct {
	enabled      bool
	running      bool
	starting     bool
	status       tor.TorServiceStatus
	statusString string
	uptime       string
	onionAddress string
	info         map[string]interface{}
	vanityStatus *tor.VanityStatus
	testResult   *tor.TestConnectionResult

	regenerateErr error
	vanityErr     error
	applyErr      error
	importErr     error
	restartErr    error

	cancelCalled     bool
	lastVanityPrefix string
	lastImportedKey  []byte
}

func (f *fakeTorControl) IsEnabled() bool                           { return f.enabled }
func (f *fakeTorControl) IsRunning() bool                           { return f.running }
func (f *fakeTorControl) IsStarting() bool                          { return f.starting }
func (f *fakeTorControl) GetStatus() tor.TorServiceStatus           { return f.status }
func (f *fakeTorControl) GetStatusString() string                   { return f.statusString }
func (f *fakeTorControl) GetUptime() string                         { return f.uptime }
func (f *fakeTorControl) GetOnionAddress() string                   { return f.onionAddress }
func (f *fakeTorControl) GetInfo() map[string]interface{}           { return f.info }
func (f *fakeTorControl) GetVanityStatus() *tor.VanityStatus        { return f.vanityStatus }
func (f *fakeTorControl) TestConnection() *tor.TestConnectionResult { return f.testResult }
func (f *fakeTorControl) CancelVanityGeneration()                   { f.cancelCalled = true }

func (f *fakeTorControl) RegenerateAddress() error {
	if f.regenerateErr != nil {
		return f.regenerateErr
	}
	f.onionAddress = "regenerated.onion"
	return nil
}

func (f *fakeTorControl) GenerateVanityAddress(prefix string) error {
	f.lastVanityPrefix = prefix
	return f.vanityErr
}

func (f *fakeTorControl) ApplyVanityAddress() error {
	if f.applyErr != nil {
		return f.applyErr
	}
	f.onionAddress = "applied.onion"
	return nil
}

func (f *fakeTorControl) ImportKeys(secretKey []byte) error {
	f.lastImportedKey = secretKey
	if f.importErr != nil {
		return f.importErr
	}
	f.onionAddress = "imported.onion"
	return nil
}

func (f *fakeTorControl) Restart(ctx context.Context) error { return f.restartErr }

// newTestServerWithTorControl builds a bare Server with only the fields the
// Tor control handlers touch, plus a mounted chi router carrying the routes.
func newTestServerWithTorControl(tc TorControl) (*Server, *chi.Mux) {
	s := &Server{torControl: tc}
	r := chi.NewRouter()
	s.registerTorControlRoutes(r)
	return s, r
}

func doLoopback(r *chi.Mux, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:54321"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func doNonLoopback(r *chi.Mux, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "203.0.113.7:54321"
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// ── isLoopbackPeer ────────────────────────────────────────────────────────────

func TestIsLoopbackPeer_IPv4Loopback(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	if !isLoopbackPeer(r) {
		t.Error("isLoopbackPeer(127.0.0.1) = false, want true")
	}
}

func TestIsLoopbackPeer_IPv6Loopback(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "[::1]:12345"
	if !isLoopbackPeer(r) {
		t.Error("isLoopbackPeer(::1) = false, want true")
	}
}

func TestIsLoopbackPeer_RemoteHost(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "203.0.113.7:12345"
	if isLoopbackPeer(r) {
		t.Error("isLoopbackPeer(203.0.113.7) = true, want false")
	}
}

func TestIsLoopbackPeer_NoPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1"
	if !isLoopbackPeer(r) {
		t.Error("isLoopbackPeer(127.0.0.1, no port) = false, want true")
	}
}

// ── requireLoopback / route gating ──────────────────────────────────────────

func TestRegisterTorControlRoutes_NonLoopbackIs404(t *testing.T) {
	_, r := newTestServerWithTorControl(&fakeTorControl{})
	rec := doNonLoopback(r, http.MethodGet, "/server/tor/status", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("non-loopback GET /server/tor/status = %d, want 404", rec.Code)
	}
}

func TestRegisterTorControlRoutes_LoopbackReachesHandler(t *testing.T) {
	_, r := newTestServerWithTorControl(&fakeTorControl{enabled: true})
	rec := doLoopback(r, http.MethodGet, "/server/tor/status", "")
	if rec.Code != http.StatusOK {
		t.Errorf("loopback GET /server/tor/status = %d, want 200", rec.Code)
	}
}

// ── handleTorControlStatus ───────────────────────────────────────────────────

func TestHandleTorControlStatus_NilControl(t *testing.T) {
	_, r := newTestServerWithTorControl(nil)
	rec := doLoopback(r, http.MethodGet, "/server/tor/status", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status with nil torControl = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "NOT_FOUND") {
		t.Errorf("status with nil torControl: body = %q, want NOT_FOUND", rec.Body.String())
	}
}

func TestHandleTorControlStatus_Populated(t *testing.T) {
	tc := &fakeTorControl{
		enabled:      true,
		running:      true,
		status:       tor.TorServiceStatusConnected,
		statusString: "Connected",
		onionAddress: "abc123.onion",
		info:         map[string]interface{}{"num_intro_points": 3},
	}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodGet, "/server/tor/status", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "abc123.onion") {
		t.Errorf("status body missing onion address: %q", rec.Body.String())
	}
}

// ── handleTorControlValidate ─────────────────────────────────────────────────

func TestHandleTorControlValidate_NilControl(t *testing.T) {
	_, r := newTestServerWithTorControl(nil)
	rec := doLoopback(r, http.MethodPost, "/server/tor/validate", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("validate with nil torControl = %d, want 404", rec.Code)
	}
}

func TestHandleTorControlValidate_Success(t *testing.T) {
	tc := &fakeTorControl{testResult: &tor.TestConnectionResult{Connected: true, Message: "reachable"}}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/validate", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("validate = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "reachable") {
		t.Errorf("validate body missing message: %q", rec.Body.String())
	}
}

// ── handleTorControlRestart ──────────────────────────────────────────────────

func TestHandleTorControlRestart_NilControl(t *testing.T) {
	_, r := newTestServerWithTorControl(nil)
	rec := doLoopback(r, http.MethodPost, "/server/tor/restart", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("restart with nil torControl = %d, want 404", rec.Code)
	}
}

func TestHandleTorControlRestart_Success(t *testing.T) {
	tc := &fakeTorControl{}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/restart", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("restart = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tor restarted") {
		t.Errorf("restart body missing success message: %q", rec.Body.String())
	}
}

func TestHandleTorControlRestart_Error(t *testing.T) {
	tc := &fakeTorControl{restartErr: errors.New("tor is not enabled")}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/restart", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("restart error = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "tor is not enabled") {
		t.Errorf("restart error body missing message: %q", rec.Body.String())
	}
}

// ── handleTorControlRegenerate ───────────────────────────────────────────────

func TestHandleTorControlRegenerate_NilControl(t *testing.T) {
	_, r := newTestServerWithTorControl(nil)
	rec := doLoopback(r, http.MethodPost, "/server/tor/regenerate", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("regenerate with nil torControl = %d, want 404", rec.Code)
	}
}

func TestHandleTorControlRegenerate_Success(t *testing.T) {
	tc := &fakeTorControl{}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/regenerate", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("regenerate = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "regenerated.onion") {
		t.Errorf("regenerate body missing new address: %q", rec.Body.String())
	}
}

func TestHandleTorControlRegenerate_Error(t *testing.T) {
	tc := &fakeTorControl{regenerateErr: errors.New("boom")}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/regenerate", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("regenerate error = %d, want 500", rec.Code)
	}
}

// ── handleTorControlVanityStart ──────────────────────────────────────────────

func TestHandleTorControlVanityStart_NilControl(t *testing.T) {
	_, r := newTestServerWithTorControl(nil)
	rec := doLoopback(r, http.MethodPost, "/server/tor/vanity/start", `{"prefix":"abc"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("vanity start with nil torControl = %d, want 404", rec.Code)
	}
}

func TestHandleTorControlVanityStart_MissingPrefix(t *testing.T) {
	tc := &fakeTorControl{}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/vanity/start", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("vanity start missing prefix = %d, want 400", rec.Code)
	}
}

func TestHandleTorControlVanityStart_Success(t *testing.T) {
	tc := &fakeTorControl{}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/vanity/start", `{"prefix":"abc"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("vanity start = %d, want 200", rec.Code)
	}
	if tc.lastVanityPrefix != "abc" {
		t.Errorf("vanity start: prefix passed through = %q, want %q", tc.lastVanityPrefix, "abc")
	}
}

func TestHandleTorControlVanityStart_Error(t *testing.T) {
	tc := &fakeTorControl{vanityErr: errors.New("already running")}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/vanity/start", `{"prefix":"abc"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("vanity start error = %d, want 500", rec.Code)
	}
}

// ── handleTorControlVanityApply ──────────────────────────────────────────────

func TestHandleTorControlVanityApply_NilControl(t *testing.T) {
	_, r := newTestServerWithTorControl(nil)
	rec := doLoopback(r, http.MethodPost, "/server/tor/vanity/apply", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("vanity apply with nil torControl = %d, want 404", rec.Code)
	}
}

func TestHandleTorControlVanityApply_Success(t *testing.T) {
	tc := &fakeTorControl{}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/vanity/apply", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("vanity apply = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "applied.onion") {
		t.Errorf("vanity apply body missing address: %q", rec.Body.String())
	}
}

func TestHandleTorControlVanityApply_Error(t *testing.T) {
	tc := &fakeTorControl{applyErr: errors.New("no pending address")}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/vanity/apply", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("vanity apply error = %d, want 500", rec.Code)
	}
}

// ── handleTorControlImportKeys ───────────────────────────────────────────────

func TestHandleTorControlImportKeys_NilControl(t *testing.T) {
	_, r := newTestServerWithTorControl(nil)
	rec := doLoopback(r, http.MethodPost, "/server/tor/import-keys", `{"secret_key":"AAAA"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("import-keys with nil torControl = %d, want 404", rec.Code)
	}
}

func TestHandleTorControlImportKeys_MissingKey(t *testing.T) {
	tc := &fakeTorControl{}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/import-keys", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("import-keys missing key = %d, want 400", rec.Code)
	}
}

func TestHandleTorControlImportKeys_Success(t *testing.T) {
	tc := &fakeTorControl{}
	_, r := newTestServerWithTorControl(tc)
	// base64 of "hello" per encoding/json []byte marshaling rules
	rec := doLoopback(r, http.MethodPost, "/server/tor/import-keys", `{"secret_key":"aGVsbG8="}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("import-keys = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if string(tc.lastImportedKey) != "hello" {
		t.Errorf("import-keys: decoded key = %q, want %q", tc.lastImportedKey, "hello")
	}
	if !strings.Contains(rec.Body.String(), "imported.onion") {
		t.Errorf("import-keys body missing address: %q", rec.Body.String())
	}
}

func TestHandleTorControlImportKeys_Error(t *testing.T) {
	tc := &fakeTorControl{importErr: errors.New("invalid key format")}
	_, r := newTestServerWithTorControl(tc)
	rec := doLoopback(r, http.MethodPost, "/server/tor/import-keys", `{"secret_key":"aGVsbG8="}`)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("import-keys error = %d, want 500", rec.Code)
	}
}

// ── SetTorControl ─────────────────────────────────────────────────────────────

func TestSetTorControl_WiresField(t *testing.T) {
	s := &Server{}
	tc := &fakeTorControl{}
	s.SetTorControl(tc)
	if s.torControl != tc {
		t.Error("SetTorControl did not wire s.torControl")
	}
}
