// SPDX-License-Identifier: MIT
package server

import (
	"context"
	"net/http"

	"github.com/apimgr/vidveil/src/server/handler"
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
				s.logger.Audit(
					"compliance.gpc_honored",
					"anonymous", "visitor", extractClientIP(r), "honored",
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
