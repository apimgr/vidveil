// SPDX-License-Identifier: MIT
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/handler"
)

// runPrivacySignals runs the middleware against a request carrying the given
// headers and returns the honored opt-out flag seen by the downstream handler.
func runPrivacySignals(cfg *config.AppConfig, headers map[string]string) bool {
	s := newTestServerWithConfig(cfg)
	var optOut bool
	h := s.privacySignalsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		optOut = handler.GPCOptOut(r)
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.RemoteAddr = "1.2.3.4:5678"
	h.ServeHTTP(httptest.NewRecorder(), req)
	return optOut
}

func TestPrivacySignals_SecGPCHonored(t *testing.T) {
	cfg := config.DefaultAppConfig()
	if !runPrivacySignals(cfg, map[string]string{"Sec-GPC": "1"}) {
		t.Error("Sec-GPC: 1 should set the request opt-out flag")
	}
}

func TestPrivacySignals_NoSignal(t *testing.T) {
	cfg := config.DefaultAppConfig()
	if runPrivacySignals(cfg, nil) {
		t.Error("no privacy signal should leave the opt-out flag false")
	}
}

func TestPrivacySignals_DNTNotHonoredByDefault(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Web.HonorDNT = false
	if runPrivacySignals(cfg, map[string]string{"DNT": "1"}) {
		t.Error("DNT: 1 must NOT be honored when web.honor_dnt is false")
	}
}

func TestPrivacySignals_DNTHonoredWhenOptedIn(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Web.HonorDNT = true
	if !runPrivacySignals(cfg, map[string]string{"DNT": "1"}) {
		t.Error("DNT: 1 should be honored when web.honor_dnt is true")
	}
}
