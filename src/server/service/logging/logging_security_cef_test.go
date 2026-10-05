// SPDX-License-Identifier: MIT
// Tests for the CEF security log format and the raw-text log line rule
// required by AI.md PART 11.
package logging

import (
	"bytes"
	"strings"
	"testing"
)

// TestAppLoggerSecurityCEFFormat verifies the "cef" format is rendered as a
// Common Event Format header per AI.md PART 11, not silently falling through
// to the fail2ban default.
func TestAppLoggerSecurityCEFFormat(t *testing.T) {
	var buf bytes.Buffer
	l := newInMemoryLogger(LevelDebug, &buf)
	l.appConfig.Server.Logs.Security.Format = "cef"

	l.Security("brute_force", "10.0.0.1", map[string]interface{}{"attempts": 5})

	out := buf.String()
	if !strings.HasPrefix(out, "CEF:0|") {
		t.Fatalf("Security() cef format = %q, want a CEF:0| header", out)
	}
	if !strings.Contains(out, "brute_force") {
		t.Errorf("Security() cef output missing event name: %s", out)
	}
	// The peer IP is masked by MaskIP, so assert on the src extension's presence
	// rather than the raw address.
	if !strings.Contains(out, "src=") {
		t.Errorf("Security() cef output missing src extension: %s", out)
	}
}

// TestCEFEscapeNeutralizesPipes guards the header: a pipe in any value would
// otherwise forge a new CEF field boundary.
func TestCEFEscapeNeutralizesPipes(t *testing.T) {
	if got := cefEscape("a|b"); got != "a_b" {
		t.Errorf("cefEscape() = %q, want %q", got, "a_b")
	}
	if got := cefEscape("a\x1b[31mb"); strings.ContainsRune(got, 0x1b) {
		t.Errorf("cefEscape() left an escape byte in %q", got)
	}
}

// TestCEFExtensionsUsesSanitizedValues verifies detail fields pass through
// SanitizeLogFields, so PII is masked in CEF output too.
func TestCEFExtensionsUsesSanitizedValues(t *testing.T) {
	ext := cefExtensions("10.0.0.1", map[string]interface{}{
		"email":       "user@example.com",
		"remote_addr": "192.168.1.50",
	})
	if !strings.Contains(ext, "src=10.0.0.1") {
		t.Errorf("cefExtensions() missing src: %s", ext)
	}
	if strings.Contains(ext, "user@example.com") {
		t.Errorf("cefExtensions() leaked an unmasked email: %s", ext)
	}
	// The peer IP must not survive unmasked in the extension list.
	if strings.Contains(ext, "192.168.1.50") {
		t.Errorf("cefExtensions() leaked an unmasked peer IP: %s", ext)
	}
	if !strings.Contains(ext, "remote_addr=") {
		t.Errorf("cefExtensions() dropped the remote_addr key: %s", ext)
	}
}

// TestStripLogControlRemovesEscapes verifies ANSI/control bytes are stripped
// per the AI.md PART 11 raw-text rule for every log file.
func TestStripLogControlRemovesEscapes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"CSI colour", "before\x1b[31mred\x1b[0mafter", "beforeredafter"},
		{"OSC title", "a\x1b]0;window title\x07b", "ab"},
		{"bare control", "a\x00b\x7fc", "abc"},
		{"tab dropped", "a\tb", "ab"},
		{"clean text unchanged", "plain text 123", "plain text 123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripLogControl(tt.in); got != tt.want {
				t.Errorf("stripLogControl(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestAppLoggerAccessStripsEscapes verifies the strip is applied at the write
// boundary, so a crafted request path cannot inject cursor codes into a file an
// operator tails in a terminal.
func TestAppLoggerAccessStripsEscapes(t *testing.T) {
	var buf bytes.Buffer
	l := newInMemoryLogger(LevelDebug, &buf)

	l.Access("GET", "/\x1b[2J\x1b[H/evil", "HTTP/1.1", "127.0.0.1:9000", "", "go-test/1.0", 404, 0)

	if strings.ContainsRune(buf.String(), 0x1b) {
		t.Errorf("Access() wrote an escape byte into the log: %q", buf.String())
	}
}

// TestAppLoggerSecurityStripsEscapes covers the same boundary for security.log.
func TestAppLoggerSecurityStripsEscapes(t *testing.T) {
	var buf bytes.Buffer
	l := newInMemoryLogger(LevelDebug, &buf)

	l.Security("evt\x1b[2J", "10.0.0.1", map[string]interface{}{"k": "v\x07"})

	if strings.ContainsRune(buf.String(), 0x1b) {
		t.Errorf("Security() wrote an escape byte into the log: %q", buf.String())
	}
}
