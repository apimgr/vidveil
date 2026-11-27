// SPDX-License-Identifier: MIT
package server

import (
	"context"
	"net"
	"net/http"

	"github.com/apimgr/vidveil/src/server/handler"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

// privacySignalsMiddleware honors browser-emitted privacy signals as opt-outs
// per AI.md PART 11 "Privacy Signal Headers".
//
// Sec-GPC (Global Privacy Control) is the load-bearing signal and is ALWAYS
// honored: when "Sec-GPC: 1" is received the request is flagged opt-out for its
// whole lifecycle (handler.GPCOptOutKey=true), so downstream handlers skip
// personalization, behavioral analytics, and any non-essential cookie. The
// signal is echoed to the audit log as compliance.gpc_honored so an operator
// can prove honoring.
//
// DNT (Do Not Track) is NOT honored by default — it was de-facto removed from
// mainstream browsers. Operators with EU-only audiences can opt in via
// web.honor_dnt: true, in which case "DNT: 1" is treated the same as Sec-GPC.
//
// Signals are advisory opt-outs only: never opt-ins. Absence changes nothing.
func (s *Server) privacySignalsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		optOut := false
		reason := ""

		if r.Header.Get("Sec-GPC") == "1" {
			optOut = true
			reason = "sec-gpc"
		} else if s.appConfig.Web.HonorDNT && r.Header.Get("DNT") == "1" {
			optOut = true
			reason = "dnt"
		}

		if optOut {
			ctx := context.WithValue(r.Context(), handler.GPCOptOutKey, true)
			r = r.WithContext(ctx)
			// Surface the honored signal so the compliance officer can prove it.
			if s.logger != nil {
				// Per AI.md PART 31, Tor requests key audit-log entries on the
				// exported circuit ID (or the "tor" sentinel) instead of the
				// Tor daemon's loopback address.
				clientIP := extractClientIP(r)
				if urlvar.IsTorRequest(r) {
					clientIP = urlvar.TorClientLabel(r)
				} else {
					clientIP = anonymizeAuditIP(clientIP)
				}
				s.logger.Audit(
					"compliance.gpc_honored",
					"", "anonymous", clientIP, "success",
					map[string]interface{}{
						"signal": reason,
						"path":   r.URL.Path,
						"method": r.Method,
					},
				)
			}
		}

		next.ServeHTTP(w, r)
	})
}

func anonymizeAuditIP(value string) string {
	ip := net.ParseIP(value)
	if ip == nil {
		return ""
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return net.IPv4(ipv4[0], ipv4[1], ipv4[2], 0).String()
	}
	ipv6 := ip.To16()
	if ipv6 == nil {
		return ""
	}
	for i := 6; i < len(ipv6); i++ {
		ipv6[i] = 0
	}
	return ipv6.String()
}
