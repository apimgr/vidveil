// SPDX-License-Identifier: MIT
// AI.md PART 11: deterministic coverage for the per-format branches of the
// Access, Security, and Auth loggers. Each format is selected via appConfig and
// verified through an in-memory buffer, so the tests need no server or network.
package logging

import (
	"bytes"
	"strings"
	"testing"
)

// newFormatLogger builds an in-memory logger and applies the given log formats
// so every switch branch of Access/Security/Auth can be exercised.
func newFormatLogger(buf *bytes.Buffer, access, security, auth string) *AppLogger {
	l := newInMemoryLogger(LevelDebug, buf)
	l.appConfig.Server.Logs.Access.Format = access
	l.appConfig.Server.Logs.Security.Format = security
	l.appConfig.Server.Logs.Auth.Format = auth
	return l
}

// TestAccessFormatBranches covers apache (default), nginx, and json.
func TestAccessFormatBranches(t *testing.T) {
	for _, f := range []string{"apache", "nginx", "json", "unknown"} {
		var buf bytes.Buffer
		l := newFormatLogger(&buf, f, "fail2ban", "syslog")
		l.Access("GET", "/x", "HTTP/1.1", "1.2.3.4", "ref", "curl", 200, 42)
		if buf.Len() == 0 {
			t.Errorf("Access format %q produced no output", f)
		}
	}
}

// TestSecurityFormatBranches covers fail2ban, syslog, json, text, and default.
func TestSecurityFormatBranches(t *testing.T) {
	for _, f := range []string{"fail2ban", "syslog", "json", "text", "weird"} {
		var buf bytes.Buffer
		l := newFormatLogger(&buf, "apache", f, "syslog")
		l.Security("brute_force", "9.9.9.9", map[string]interface{}{"tries": 5})
		out := buf.String()
		if out == "" {
			t.Errorf("Security format %q produced no output", f)
		}
		if !strings.Contains(out, "\n") {
			t.Errorf("Security format %q missing trailing newline", f)
		}
	}
}

// TestAuthFormatBranches covers syslog (default) and json.
func TestAuthFormatBranches(t *testing.T) {
	for _, f := range []string{"syslog", "json"} {
		var buf bytes.Buffer
		l := newFormatLogger(&buf, "apache", "fail2ban", f)
		l.Auth("alice", "5.5.5.5", "fail", "invalid_credentials")
		if buf.Len() == 0 {
			t.Errorf("Auth format %q produced no output", f)
		}
	}
}
