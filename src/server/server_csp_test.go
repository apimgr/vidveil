// SPDX-License-Identifier: MIT
package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

func TestBuildCSPHeaderUpgradesInsecureRequestsOnlyForHTTPS(t *testing.T) {
	tests := []struct {
		name    string
		request string
		ssl     bool
		want    bool
	}{
		{name: "plain HTTP", request: "http://vidveil.example/", want: false},
		{name: "HTTPS request", request: "https://vidveil.example/", want: true},
		{name: "SSL enabled", request: "http://vidveil.example/", ssl: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{appConfig: &config.AppConfig{Server: config.ServerConfig{
				SSL: config.SSLConfig{Enabled: tt.ssl},
			}}}
			policy := s.buildCSPHeader(httptest.NewRequest("GET", tt.request, nil))
			got := strings.Contains(policy, "upgrade-insecure-requests;")
			if got != tt.want {
				t.Errorf("upgrade-insecure-requests present = %t, want %t; policy=%q", got, tt.want, policy)
			}
		})
	}
}
