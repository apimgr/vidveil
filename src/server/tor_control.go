// SPDX-License-Identifier: MIT
package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// TorControl is the interface the server uses to expose Tor process
// mutation/inspection to the loopback-only internal control channel.
// Per AI.md PART 31 "CLI-to-running-server control channel": the server
// binary is the sole owner of the embedded Tor process, so a separately
// invoked CLI subcommand reaches it over HTTP instead of touching Tor
// state directly. This is intentionally a separate, additive interface
// from handler.TorStatusChecker (which is shared across ServerHandler,
// SearchHandler, and this Server for unrelated Onion-Location/outbound
// routing concerns) so broadening it here never touches those consumers.
type TorControl interface {
	IsEnabled() bool
	IsRunning() bool
	IsStarting() bool
	GetStatus() tor.TorServiceStatus
	GetStatusString() string
	GetUptime() string
	GetOnionAddress() string
	GetInfo() map[string]interface{}
	RegenerateAddress() error
	GenerateVanityAddress(prefix string) error
	GetVanityStatus() *tor.VanityStatus
	CancelVanityGeneration()
	ApplyVanityAddress() error
	ImportKeys(secretKey []byte) error
	Restart(ctx context.Context) error
	TestConnection() *tor.TestConnectionResult
}

// SetTorControl wires the live, process-owned Tor service instance into
// the internal loopback-only control channel. Separate from SetTorService
// (handler.TorStatusChecker), which feeds unrelated handler concerns.
func (s *Server) SetTorControl(t TorControl) {
	s.torControl = t
}

// isLoopbackPeer reports whether the request's immediate TCP peer is
// 127.0.0.1/::1. It MUST use the raw, unrewritten r.RemoteAddr — never
// extractClientIP/urlvar.ResolveClientIP, which are proxy-header-aware and
// would let a spoofed forwarded-for chain self-authorize (AI.md PART 12).
func isLoopbackPeer(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr without a port (e.g. in some test harnesses) — treat
		// the whole value as the host.
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// requireLoopback wraps an internal control handler so any request whose
// TCP peer is not loopback gets a 404 (never 403 — the endpoint must not
// be discoverable, per AI.md PART 31).
func requireLoopback(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackPeer(r) {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

// registerTorControlRoutes mounts the internal, loopback-gated Tor control
// channel used exclusively by the `{project_name} tor ...` CLI subcommand
// (src/tor_cli.go) to reach the server-owned Tor process. Unlike
// registerDebugRoutes, these routes are ALWAYS registered — gating happens
// per-request via requireLoopback, not via conditional registration —
// since the CLI must be able to reach them whenever the server is up.
// Never under /api/{api_version}/**, never in OpenAPI/GraphQL/well-known,
// never advertised via FeaturesInfo (AI.md PART 31).
func (s *Server) registerTorControlRoutes(r chi.Router) {
	r.Route("/server/tor", func(r chi.Router) {
		r.Get("/status", requireLoopback(s.handleTorControlStatus))
		r.Post("/validate", requireLoopback(s.handleTorControlValidate))
		r.Post("/restart", requireLoopback(s.handleTorControlRestart))
		r.Post("/regenerate", requireLoopback(s.handleTorControlRegenerate))
		r.Post("/vanity/start", requireLoopback(s.handleTorControlVanityStart))
		r.Post("/vanity/apply", requireLoopback(s.handleTorControlVanityApply))
		r.Post("/import-keys", requireLoopback(s.handleTorControlImportKeys))
	})
}

func (s *Server) handleTorControlStatus(w http.ResponseWriter, r *http.Request) {
	if s.torControl == nil {
		handler.SendError(w, "NOT_FOUND", "tor control not available")
		return
	}
	handler.SendOK(w, map[string]interface{}{
		"enabled":       s.torControl.IsEnabled(),
		"running":       s.torControl.IsRunning(),
		"starting":      s.torControl.IsStarting(),
		"status":        string(s.torControl.GetStatus()),
		"status_string": s.torControl.GetStatusString(),
		"uptime":        s.torControl.GetUptime(),
		"onion_address": s.torControl.GetOnionAddress(),
		"info":          s.torControl.GetInfo(),
	})
}

func (s *Server) handleTorControlValidate(w http.ResponseWriter, r *http.Request) {
	if s.torControl == nil {
		handler.SendError(w, "NOT_FOUND", "tor control not available")
		return
	}
	result := s.torControl.TestConnection()
	handler.SendOK(w, result)
}

func (s *Server) handleTorControlRestart(w http.ResponseWriter, r *http.Request) {
	if s.torControl == nil {
		handler.SendError(w, "NOT_FOUND", "tor control not available")
		return
	}
	if err := s.torControl.Restart(r.Context()); err != nil {
		handler.SendError(w, "SERVER_ERROR", err.Error())
		return
	}
	handler.SendOK(w, map[string]interface{}{"message": "tor restarted"})
}

func (s *Server) handleTorControlRegenerate(w http.ResponseWriter, r *http.Request) {
	if s.torControl == nil {
		handler.SendError(w, "NOT_FOUND", "tor control not available")
		return
	}
	if err := s.torControl.RegenerateAddress(); err != nil {
		handler.SendError(w, "SERVER_ERROR", err.Error())
		return
	}
	handler.SendOK(w, map[string]interface{}{
		"onion_address": s.torControl.GetOnionAddress(),
	})
}

type torVanityStartRequest struct {
	Prefix string `json:"prefix"`
}

func (s *Server) handleTorControlVanityStart(w http.ResponseWriter, r *http.Request) {
	if s.torControl == nil {
		handler.SendError(w, "NOT_FOUND", "tor control not available")
		return
	}
	var req torVanityStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Prefix == "" {
		handler.SendError(w, "VALIDATION_FAILED", "prefix is required")
		return
	}
	if err := s.torControl.GenerateVanityAddress(req.Prefix); err != nil {
		handler.SendError(w, "SERVER_ERROR", err.Error())
		return
	}
	handler.SendOK(w, map[string]interface{}{"message": "vanity generation started"})
}

func (s *Server) handleTorControlVanityApply(w http.ResponseWriter, r *http.Request) {
	if s.torControl == nil {
		handler.SendError(w, "NOT_FOUND", "tor control not available")
		return
	}
	if err := s.torControl.ApplyVanityAddress(); err != nil {
		handler.SendError(w, "SERVER_ERROR", err.Error())
		return
	}
	handler.SendOK(w, map[string]interface{}{
		"onion_address": s.torControl.GetOnionAddress(),
	})
}

type torImportKeysRequest struct {
	SecretKey []byte `json:"secret_key"`
}

func (s *Server) handleTorControlImportKeys(w http.ResponseWriter, r *http.Request) {
	if s.torControl == nil {
		handler.SendError(w, "NOT_FOUND", "tor control not available")
		return
	}
	var req torImportKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.SecretKey) == 0 {
		handler.SendError(w, "VALIDATION_FAILED", "secret_key is required")
		return
	}
	if err := s.torControl.ImportKeys(req.SecretKey); err != nil {
		handler.SendError(w, "SERVER_ERROR", err.Error())
		return
	}
	handler.SendOK(w, map[string]interface{}{
		"onion_address": s.torControl.GetOnionAddress(),
	})
}
