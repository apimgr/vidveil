// SPDX-License-Identifier: MIT
package logging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// --- parseSize ---

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"50MB", 52428800},
		{"100KB", 102400},
		{"2GB", 2147483648},
		{"1024B", 1024},
		{"", 0},
		{"invalid", 0},
		// Case-insensitive: lower-case should work via ToUpper
		{"50mb", 52428800},
		{"100kb", 102400},
		{"1gb", 1073741824},
		// Whitespace trimming
		{"  10MB  ", 10485760},
		// Zero value
		{"0MB", 0},
		// Single byte
		{"1B", 1},
	}
	for _, tt := range tests {
		got := parseSize(tt.input)
		if got != tt.expected {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// --- parseInterval ---

func TestParseInterval(t *testing.T) {
	tests := []struct {
		input    string
		expected RotationInterval
	}{
		{"hourly", RotationHourly},
		{"daily", RotationDaily},
		{"weekly", RotationWeekly},
		{"monthly", RotationMonthly},
		{"", RotationNone},
		{"invalid", RotationNone},
		// Case-insensitive
		{"DAILY", RotationDaily},
		{"Weekly", RotationWeekly},
		// Whitespace trimming
		{"  hourly  ", RotationHourly},
	}
	for _, tt := range tests {
		got := parseInterval(tt.input)
		if got != tt.expected {
			t.Errorf("parseInterval(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

// --- MaskEmail ---

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user@example.com", "u***@e***.com"},
		{"", ""},
		// No @ sign → "***"
		{"notanemail", "***"},
		// Single-char local part
		{"a@b.com", "a***@b***.com"},
		// Subdomain domain: last TLD kept, first char of primary domain kept
		{"x@mail.example.org", "x***@m***.org"},
		// Multiple @ signs → treated as not 2 parts (Split returns >2 parts for that case)
		// Standard two-part email
		{"admin@vidveil.io", "a***@v***.io"},
	}
	for _, tt := range tests {
		got := MaskEmail(tt.input)
		if got != tt.expected {
			t.Errorf("MaskEmail(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- MaskUsername ---

func TestMaskUsername(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"johndoe", "joh***"},
		{"ab", "a***"},
		{"", ""},
		{"a", "a***"},
		{"abc", "a***"},
		{"abcd", "abc***"},
		// Longer name
		{"administrator", "adm***"},
	}
	for _, tt := range tests {
		got := MaskUsername(tt.input)
		if got != tt.expected {
			t.Errorf("MaskUsername(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- MaskIP ---

func TestMaskIP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"192.168.1.100", "192.168.xxx.xxx"},
		{"10.0.0.1", "10.0.xxx.xxx"},
		{"", ""},
		// IPv6 with >= 4 colon-separated parts
		{"2001:db8::1:2:3:4", "2001:db8:xxxx:xxxx:..."},
		{"fe80:0:0:0:1:2:3:4", "fe80:0:xxxx:xxxx:..."},
	}
	for _, tt := range tests {
		got := MaskIP(tt.input)
		if got != tt.expected {
			t.Errorf("MaskIP(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- SanitizeLogFields ---

func TestSanitizeLogFields(t *testing.T) {
	// nil map must return nil
	if got := SanitizeLogFields(nil); got != nil {
		t.Errorf("SanitizeLogFields(nil) = %v, want nil", got)
	}

	tests := []struct {
		name   string
		input  map[string]interface{}
		checks func(t *testing.T, out map[string]interface{})
	}{
		{
			name:  "email masked",
			input: map[string]interface{}{"email": "user@example.com"},
			checks: func(t *testing.T, out map[string]interface{}) {
				if got := out["email"]; got != "u***@e***.com" {
					t.Errorf("email = %q, want %q", got, "u***@e***.com")
				}
			},
		},
		{
			name:  "password redacted",
			input: map[string]interface{}{"password": "secret"},
			checks: func(t *testing.T, out map[string]interface{}) {
				if got := out["password"]; got != "[REDACTED]" {
					t.Errorf("password = %q, want %q", got, "[REDACTED]")
				}
			},
		},
		{
			name:  "username masked",
			input: map[string]interface{}{"username": "johndoe"},
			checks: func(t *testing.T, out map[string]interface{}) {
				if got := out["username"]; got != "joh***" {
					t.Errorf("username = %q, want %q", got, "joh***")
				}
			},
		},
		{
			name:  "ip masked",
			input: map[string]interface{}{"ip": "192.168.1.1"},
			checks: func(t *testing.T, out map[string]interface{}) {
				if got := out["ip"]; got != "192.168.xxx.xxx" {
					t.Errorf("ip = %q, want %q", got, "192.168.xxx.xxx")
				}
			},
		},
		{
			name:  "token redacted",
			input: map[string]interface{}{"token": "abc"},
			checks: func(t *testing.T, out map[string]interface{}) {
				if got := out["token"]; got != "[REDACTED]" {
					t.Errorf("token = %q, want %q", got, "[REDACTED]")
				}
			},
		},
		{
			name:  "api_key redacted",
			input: map[string]interface{}{"api_key": "abc"},
			checks: func(t *testing.T, out map[string]interface{}) {
				if got := out["api_key"]; got != "[REDACTED]" {
					t.Errorf("api_key = %q, want %q", got, "[REDACTED]")
				}
			},
		},
		{
			name:  "unknown key passes through",
			input: map[string]interface{}{"other": "value"},
			checks: func(t *testing.T, out map[string]interface{}) {
				if got := out["other"]; got != "value" {
					t.Errorf("other = %q, want %q", got, "value")
				}
			},
		},
		{
			name:  "email non-string becomes ***",
			input: map[string]interface{}{"email": 12345},
			checks: func(t *testing.T, out map[string]interface{}) {
				if got := out["email"]; got != "***" {
					t.Errorf("email (non-string) = %q, want %q", got, "***")
				}
			},
		},
		{
			name:  "secret redacted",
			input: map[string]interface{}{"secret": "mysecret"},
			checks: func(t *testing.T, out map[string]interface{}) {
				if got := out["secret"]; got != "[REDACTED]" {
					t.Errorf("secret = %q, want %q", got, "[REDACTED]")
				}
			},
		},
		{
			name:  "multiple fields all sanitized correctly",
			input: map[string]interface{}{"email": "a@b.com", "other": "keep"},
			checks: func(t *testing.T, out map[string]interface{}) {
				if got := out["email"]; got != "a***@b***.com" {
					t.Errorf("email = %q, want %q", got, "a***@b***.com")
				}
				if got := out["other"]; got != "keep" {
					t.Errorf("other = %q, want %q", got, "keep")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := SanitizeLogFields(tt.input)
			tt.checks(t, out)
		})
	}
}

// Verify SanitizeLogFields does not mutate the original map
func TestSanitizeLogFieldsDoesNotMutateInput(t *testing.T) {
	input := map[string]interface{}{"password": "original"}
	_ = SanitizeLogFields(input)
	if input["password"] != "original" {
		t.Error("SanitizeLogFields mutated the input map")
	}
}

// --- Level.String ---

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.expected {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.expected)
		}
	}
}

// --- auditCategory ---

func TestAuditCategory(t *testing.T) {
	tests := []struct {
		event    string
		expected string
	}{
		{"admin.login", "authentication"},
		{"user.logout", "authentication"},
		{"config.update", "configuration"},
		{"security.block", "security"},
		{"token.create", "tokens"},
		{"backup.start", "backup"},
		{"server.restart", "server"},
		{"cluster.join", "other"},
		{"oidc.login", "authentication"},
		{"ldap.sync", "authentication"},
		{"branding.update", "configuration"},
		{"ssl.renew", "configuration"},
		{"scheduler.run", "server"},
		{"org.create", "organization"},
		{"unknown.event", "other"},
		// No dot: prefix is the full string
		{"plainstring", "other"},
	}
	for _, tt := range tests {
		got := auditCategory(tt.event)
		if got != tt.expected {
			t.Errorf("auditCategory(%q) = %q, want %q", tt.event, got, tt.expected)
		}
	}
}

// --- auditSeverity ---

func TestAuditSeverity(t *testing.T) {
	tests := []struct {
		event    string
		result   string
		expected string
	}{
		// failure + brute_force → critical
		{"login.brute_force", "failure", "critical"},
		// failure + suspicious → critical
		{"auth.suspicious", "failure", "critical"},
		// failure without special markers → warn
		{"admin.login", "failure", "warn"},
		{"config.update", "failure", "warn"},
		// success + maintenance_entered → critical
		{"maintenance_entered.xyz", "success", "critical"},
		// success + brute_force event → critical
		{"brute_force.reset", "success", "critical"},
		// success + ip_blocked → warn
		{"ip_blocked.abc", "success", "warn"},
		// success + country_blocked → warn
		{"country_blocked.region", "success", "warn"},
		// success + normal event → info
		{"config.update", "success", "info"},
		{"admin.login", "success", "info"},
	}
	for _, tt := range tests {
		got := auditSeverity(tt.event, tt.result)
		if got != tt.expected {
			t.Errorf("auditSeverity(%q, %q) = %q, want %q", tt.event, tt.result, got, tt.expected)
		}
	}
}

// --- parseRotationString ---

func TestParseRotationString(t *testing.T) {
	// Empty string → defaults
	t.Run("empty uses defaults", func(t *testing.T) {
		got := parseRotationString("")
		if got.MaxSize != "50MB" {
			t.Errorf("MaxSize = %q, want %q", got.MaxSize, "50MB")
		}
		if got.Interval != "" {
			t.Errorf("Interval = %q, want %q", got.Interval, "")
		}
		if got.Compress {
			t.Error("Compress should be false by default")
		}
	})

	t.Run("weekly,50MB", func(t *testing.T) {
		got := parseRotationString("weekly,50MB")
		if got.Interval != "weekly" {
			t.Errorf("Interval = %q, want %q", got.Interval, "weekly")
		}
		if got.MaxSize != "50MB" {
			t.Errorf("MaxSize = %q, want %q", got.MaxSize, "50MB")
		}
	})

	t.Run("daily,compress", func(t *testing.T) {
		got := parseRotationString("daily,compress")
		if got.Interval != "daily" {
			t.Errorf("Interval = %q, want %q", got.Interval, "daily")
		}
		if !got.Compress {
			t.Error("Compress should be true")
		}
	})

	t.Run("100KB only", func(t *testing.T) {
		got := parseRotationString("100KB")
		if got.MaxSize != "100KB" {
			t.Errorf("MaxSize = %q, want %q", got.MaxSize, "100KB")
		}
		if got.Interval != "" {
			t.Errorf("Interval = %q, want empty", got.Interval)
		}
	})

	t.Run("gzip,daily,10MB", func(t *testing.T) {
		got := parseRotationString("gzip,daily,10MB")
		if !got.Compress {
			t.Error("Compress should be true for gzip")
		}
		if got.Interval != "daily" {
			t.Errorf("Interval = %q, want %q", got.Interval, "daily")
		}
		if got.MaxSize != "10MB" {
			t.Errorf("MaxSize = %q, want %q", got.MaxSize, "10MB")
		}
	})
}

// --- parseKeepString ---

func TestParseKeepString(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"5", 5},
		{"invalid", 0},
		{"  3  ", 3},
		{"0", 0},
		{"100", 100},
		{"-1", -1},
	}
	for _, tt := range tests {
		got := parseKeepString(tt.input)
		if got != tt.expected {
			t.Errorf("parseKeepString(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// --- NewRotatingFile ---

func TestNewRotatingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	cfg := RotationConfig{
		MaxSize:  "10MB",
		Interval: "",
		Compress: false,
		Keep:     0,
	}

	rf, err := NewRotatingFile(path, cfg)
	if err != nil {
		t.Fatalf("NewRotatingFile() error: %v", err)
	}
	defer rf.Close()

	// File must exist after creation
	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file does not exist after NewRotatingFile: %v", err)
	}

	// Write data and verify it lands in the file
	payload := []byte("hello log\n")
	n, err := rf.Write(payload)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != len(payload) {
		t.Errorf("Write() n = %d, want %d", n, len(payload))
	}

	// Close must succeed
	if err := rf.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}

	// File contents must contain the written payload
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("file contents = %q, want %q", string(got), string(payload))
	}
}

// needsRotation returns false for a fresh file well under maxSize
func TestNewRotatingFileNeedsRotationFalseInitially(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nrot.log")

	cfg := RotationConfig{
		MaxSize:  "10MB",
		Interval: "",
		Compress: false,
		Keep:     0,
	}

	rf, err := NewRotatingFile(path, cfg)
	if err != nil {
		t.Fatalf("NewRotatingFile() error: %v", err)
	}
	defer rf.Close()

	if rf.needsRotation() {
		t.Error("needsRotation() = true on fresh file, want false")
	}
}

// needsRotation returns true once currentSize >= maxSize
func TestNewRotatingFileNeedsRotationTrueWhenFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "full.log")

	// maxSize = 1 byte; writing 2 bytes must trigger rotation need
	cfg := RotationConfig{
		MaxSize:  "1B",
		Interval: "",
		Compress: false,
		Keep:     0,
	}

	rf, err := NewRotatingFile(path, cfg)
	if err != nil {
		t.Fatalf("NewRotatingFile() error: %v", err)
	}
	defer rf.Close()

	// Bypass the public Write so we set currentSize directly without triggering rotate
	rf.currentSize = rf.maxSize

	if !rf.needsRotation() {
		t.Error("needsRotation() = false when currentSize >= maxSize, want true")
	}
}

// Writing exactly at the threshold triggers rotation on the next Write
func TestRotatingFileRotatesOnSizeThreshold(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rottest.log")

	cfg := RotationConfig{
		// maxSize 1 byte: first write of 2 bytes will rotate
		MaxSize:  "1B",
		Interval: "",
		Compress: false,
		Keep:     0,
	}

	rf, err := NewRotatingFile(path, cfg)
	if err != nil {
		t.Fatalf("NewRotatingFile() error: %v", err)
	}
	defer rf.Close()

	// Write enough data to exceed maxSize; the call must succeed
	_, err = rf.Write([]byte("ab"))
	if err != nil {
		t.Errorf("Write() after threshold error: %v", err)
	}
}

// --- NewAppLogger ---

func TestNewAppLogger(t *testing.T) {
	// DefaultAppConfig has all log files disabled by default, so no file I/O occurs
	cfg := config.DefaultAppConfig()

	logger, err := NewAppLogger(cfg)
	if err != nil {
		t.Fatalf("NewAppLogger() error: %v", err)
	}
	if logger == nil {
		t.Fatal("NewAppLogger() returned nil logger")
	}
	logger.Close()
}

// NewAppLogger with a path that is a file (not a directory) blocks dir creation
func TestNewAppLoggerBadPath(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file where a directory would be needed
	blocker := filepath.Join(dir, "notadir")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg := config.DefaultAppConfig()
	cfg.Server.Logs.Debug.Enabled = true
	// Path requires "notadir" to be a directory, but it is a file
	cfg.Server.Logs.Debug.Filename = filepath.Join(blocker, "debug.log")

	_, err := NewAppLogger(cfg)
	if err == nil {
		t.Error("NewAppLogger() expected error for bad path, got nil")
	}
}

// NewAppLogger log-level parsing: verify level is assigned correctly
func TestNewAppLoggerLevelParsing(t *testing.T) {
	levels := []struct {
		levelStr string
		expected Level
	}{
		{"debug", LevelDebug},
		{"info", LevelInfo},
		{"warn", LevelWarn},
		{"error", LevelError},
		// Unknown falls back to info
		{"", LevelInfo},
		{"verbose", LevelInfo},
	}

	for _, tt := range levels {
		cfg := config.DefaultAppConfig()
		cfg.Server.Logs.Level = tt.levelStr

		logger, err := NewAppLogger(cfg)
		if err != nil {
			t.Fatalf("NewAppLogger(%q) error: %v", tt.levelStr, err)
		}
		if logger.level != tt.expected {
			t.Errorf("level %q: logger.level = %v, want %v", tt.levelStr, logger.level, tt.expected)
		}
		logger.Close()
	}
}
