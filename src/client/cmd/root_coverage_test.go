// SPDX-License-Identifier: MIT
package cmd

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseCLITimeoutSeconds(t *testing.T) {
	testCases := []struct {
		name        string
		input       interface{}
		wantSeconds int
		wantErr     bool
	}{
		{name: "nil", input: nil, wantSeconds: 0, wantErr: false},
		{name: "int zero", input: int(0), wantSeconds: 0, wantErr: false},
		{name: "int negative", input: int(-5), wantSeconds: 0, wantErr: false},
		{name: "int positive", input: int(30), wantSeconds: 30, wantErr: false},
		{name: "int64 positive", input: int64(10), wantSeconds: 10, wantErr: false},
		{name: "int64 negative", input: int64(-1), wantSeconds: 0, wantErr: false},
		{name: "float64 positive truncated", input: float64(45.9), wantSeconds: 45, wantErr: false},
		{name: "float64 zero", input: float64(0), wantSeconds: 0, wantErr: false},
		{name: "string empty", input: "", wantSeconds: 0, wantErr: false},
		{name: "string whitespace", input: "  ", wantSeconds: 0, wantErr: false},
		{name: "string duration 30s", input: "30s", wantSeconds: 30, wantErr: false},
		{name: "string duration 2m", input: "2m", wantSeconds: 120, wantErr: false},
		{name: "string negative duration", input: "-1s", wantSeconds: 0, wantErr: false},
		{name: "string plain integer", input: "45", wantSeconds: 45, wantErr: false},
		{name: "string invalid", input: "invalid", wantSeconds: 0, wantErr: true},
		{name: "bool unsupported type", input: true, wantSeconds: 0, wantErr: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotSeconds, gotErr := ParseCLITimeoutSeconds(testCase.input)
			if (gotErr != nil) != testCase.wantErr {
				t.Fatalf("error = %v, wantErr = %v", gotErr, testCase.wantErr)
			}
			if gotSeconds != testCase.wantSeconds {
				t.Fatalf("seconds = %d, want %d", gotSeconds, testCase.wantSeconds)
			}
		})
	}
}

func TestFormatCLITimeoutDuration(t *testing.T) {
	testCases := []struct {
		name    string
		input   int
		want    string
	}{
		{name: "zero returns empty", input: 0, want: ""},
		{name: "negative returns empty", input: -1, want: ""},
		{name: "30 seconds", input: 30, want: "30s"},
		{name: "120 seconds", input: 120, want: "120s"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got := FormatCLITimeoutDuration(testCase.input)
			if got != testCase.want {
				t.Fatalf("duration = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestCLIServerConfigYAMLRoundTrip(t *testing.T) {
	t.Run("round trip preserves timeout", func(t *testing.T) {
		original := CLIServerConfig{
			Address: "https://example.com",
			Timeout: 30,
		}

		data, err := yaml.Marshal(original)
		if err != nil {
			t.Fatalf("marshaling config: %v", err)
		}

		var decoded CLIServerConfig
		if err := yaml.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshaling config: %v", err)
		}

		if decoded.Timeout != 30 {
			t.Fatalf("timeout after round trip = %d, want %d", decoded.Timeout, 30)
		}
	})

	t.Run("marshal uses primary key not address", func(t *testing.T) {
		cfg := CLIServerConfig{
			Address: "https://example.com",
			Timeout: 30,
		}

		data, err := yaml.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshaling config: %v", err)
		}

		yamlText := string(data)
		if !contains(yamlText, "primary:") {
			t.Fatalf("marshaled yaml missing 'primary' key:\n%s", yamlText)
		}
		if contains(yamlText, "address:") {
			t.Fatalf("marshaled yaml must not contain legacy 'address' key:\n%s", yamlText)
		}
	})

	t.Run("unmarshal legacy address key fills Address field", func(t *testing.T) {
		legacyYAML := "address: https://legacy.example.com\ntimeout: 10s\n"

		var cfg CLIServerConfig
		if err := yaml.Unmarshal([]byte(legacyYAML), &cfg); err != nil {
			t.Fatalf("unmarshaling legacy config: %v", err)
		}

		if cfg.Address != "https://legacy.example.com" {
			t.Fatalf("address = %q, want %q", cfg.Address, "https://legacy.example.com")
		}
	})
}

// contains reports whether substr appears within s.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

func TestValidateCLIServerURL(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty string", input: "", wantErr: true},
		{name: "no scheme", input: "not-a-url", wantErr: true},
		{name: "wrong scheme ftp", input: "ftp://example.com", wantErr: true},
		{name: "http with no host", input: "http://", wantErr: true},
		{name: "valid http", input: "http://example.com", wantErr: false},
		{name: "valid https with port and path", input: "https://example.com:8080/path", wantErr: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotErr := ValidateCLIServerURL(testCase.input)
			if (gotErr != nil) != testCase.wantErr {
				t.Fatalf("error = %v, wantErr = %v", gotErr, testCase.wantErr)
			}
		})
	}
}

func TestGetCLIAuthTokenFromEnvCoverage(t *testing.T) {
	t.Run("primary env var takes precedence", func(t *testing.T) {
		t.Setenv("VIDVEIL_TOKEN", "primary-token")
		t.Setenv("VIDVEIL_CLI_TOKEN", "fallback-token")

		got := GetCLIAuthTokenFromEnv()
		if got != "primary-token" {
			t.Fatalf("token = %q, want %q", got, "primary-token")
		}
	})

	t.Run("fallback used when primary unset", func(t *testing.T) {
		t.Setenv("VIDVEIL_TOKEN", "")
		t.Setenv("VIDVEIL_CLI_TOKEN", "fallback-token")

		got := GetCLIAuthTokenFromEnv()
		if got != "fallback-token" {
			t.Fatalf("token = %q, want %q", got, "fallback-token")
		}
	})

	t.Run("empty when both unset", func(t *testing.T) {
		t.Setenv("VIDVEIL_TOKEN", "")
		t.Setenv("VIDVEIL_CLI_TOKEN", "")

		got := GetCLIAuthTokenFromEnv()
		if got != "" {
			t.Fatalf("token = %q, want empty", got)
		}
	})
}

func TestGetCLIServerAddressFromEnvCoverage(t *testing.T) {
	t.Run("primary env var takes precedence", func(t *testing.T) {
		t.Setenv("VIDVEIL_SERVER_PRIMARY", "https://primary.example.com")
		t.Setenv("VIDVEIL_SERVER", "https://fallback.example.com")

		got := GetCLIServerAddressFromEnv()
		if got != "https://primary.example.com" {
			t.Fatalf("address = %q, want %q", got, "https://primary.example.com")
		}
	})

	t.Run("fallback used when primary unset", func(t *testing.T) {
		t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
		t.Setenv("VIDVEIL_SERVER", "https://fallback.example.com")

		got := GetCLIServerAddressFromEnv()
		if got != "https://fallback.example.com" {
			t.Fatalf("address = %q, want %q", got, "https://fallback.example.com")
		}
	})

	t.Run("empty when both unset", func(t *testing.T) {
		t.Setenv("VIDVEIL_SERVER_PRIMARY", "")
		t.Setenv("VIDVEIL_SERVER", "")

		got := GetCLIServerAddressFromEnv()
		if got != "" {
			t.Fatalf("address = %q, want empty", got)
		}
	})
}

func TestGetCLIOutputFormatFromEnv(t *testing.T) {
	t.Run("canonical env var takes precedence", func(t *testing.T) {
		t.Setenv("VIDVEIL_OUTPUT_FORMAT", "json")
		t.Setenv("VIDVEIL_FORMAT", "yaml")

		got := GetCLIOutputFormatFromEnv()
		if got != "json" {
			t.Fatalf("format = %q, want %q", got, "json")
		}
	})

	t.Run("fallback used when canonical unset", func(t *testing.T) {
		t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
		t.Setenv("VIDVEIL_FORMAT", "yaml")

		got := GetCLIOutputFormatFromEnv()
		if got != "yaml" {
			t.Fatalf("format = %q, want %q", got, "yaml")
		}
	})

	t.Run("empty when both unset", func(t *testing.T) {
		t.Setenv("VIDVEIL_OUTPUT_FORMAT", "")
		t.Setenv("VIDVEIL_FORMAT", "")

		got := GetCLIOutputFormatFromEnv()
		if got != "" {
			t.Fatalf("format = %q, want empty", got)
		}
	})
}

func TestGetCLIOutputColorFromEnv(t *testing.T) {
	t.Run("canonical env var takes precedence", func(t *testing.T) {
		t.Setenv("VIDVEIL_OUTPUT_COLOR", "always")
		t.Setenv("VIDVEIL_COLOR", "never")

		got := GetCLIOutputColorFromEnv()
		if got != "always" {
			t.Fatalf("color = %q, want %q", got, "always")
		}
	})

	t.Run("fallback used when canonical unset", func(t *testing.T) {
		t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
		t.Setenv("VIDVEIL_COLOR", "never")

		got := GetCLIOutputColorFromEnv()
		if got != "never" {
			t.Fatalf("color = %q, want %q", got, "never")
		}
	})

	t.Run("empty when both unset", func(t *testing.T) {
		t.Setenv("VIDVEIL_OUTPUT_COLOR", "")
		t.Setenv("VIDVEIL_COLOR", "")

		got := GetCLIOutputColorFromEnv()
		if got != "" {
			t.Fatalf("color = %q, want empty", got)
		}
	})
}

func TestGetCLITimeoutSecondsFromEnv(t *testing.T) {
	t.Run("both unset returns zero", func(t *testing.T) {
		t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
		t.Setenv("VIDVEIL_TIMEOUT", "")

		got, err := GetCLITimeoutSecondsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 0 {
			t.Fatalf("seconds = %d, want 0", got)
		}
	})

	t.Run("primary env var returns parsed value", func(t *testing.T) {
		t.Setenv("VIDVEIL_SERVER_TIMEOUT", "30")
		t.Setenv("VIDVEIL_TIMEOUT", "")

		got, err := GetCLITimeoutSecondsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 30 {
			t.Fatalf("seconds = %d, want 30", got)
		}
	})

	t.Run("fallback env var used when primary unset", func(t *testing.T) {
		t.Setenv("VIDVEIL_SERVER_TIMEOUT", "")
		t.Setenv("VIDVEIL_TIMEOUT", "60")

		got, err := GetCLITimeoutSecondsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 60 {
			t.Fatalf("seconds = %d, want 60", got)
		}
	})

	t.Run("invalid value returns error", func(t *testing.T) {
		t.Setenv("VIDVEIL_SERVER_TIMEOUT", "invalid")
		t.Setenv("VIDVEIL_TIMEOUT", "")

		_, err := GetCLITimeoutSecondsFromEnv()
		if err == nil {
			t.Fatal("expected error for invalid timeout value, got nil")
		}
	})

	t.Run("zero value returns error", func(t *testing.T) {
		t.Setenv("VIDVEIL_SERVER_TIMEOUT", "0")
		t.Setenv("VIDVEIL_TIMEOUT", "")

		_, err := GetCLITimeoutSecondsFromEnv()
		if err == nil {
			t.Fatal("expected error for zero timeout value, got nil")
		}
	})
}

func TestGetCLIDefaultServerAddress(t *testing.T) {
	t.Run("empty official site returns empty", func(t *testing.T) {
		orig := OfficialSite
		t.Cleanup(func() { OfficialSite = orig })
		OfficialSite = ""

		got := GetCLIDefaultServerAddress()
		if got != "" {
			t.Fatalf("default server address = %q, want empty", got)
		}
	})

	t.Run("set official site is returned", func(t *testing.T) {
		orig := OfficialSite
		t.Cleanup(func() { OfficialSite = orig })
		OfficialSite = "https://example.com"

		got := GetCLIDefaultServerAddress()
		if got != "https://example.com" {
			t.Fatalf("default server address = %q, want %q", got, "https://example.com")
		}
	})
}

func TestIsCLITUIEnabled(t *testing.T) {
	t.Run("nil config returns true", func(t *testing.T) {
		orig := cliConfig
		t.Cleanup(func() { cliConfig = orig })
		cliConfig = nil

		if !IsCLITUIEnabled() {
			t.Fatal("expected IsCLITUIEnabled to return true when cliConfig is nil")
		}
	})

	t.Run("config with TUI disabled returns false", func(t *testing.T) {
		orig := cliConfig
		t.Cleanup(func() { cliConfig = orig })
		cliConfig = &CLIConfig{}
		cliConfig.TUI.Enabled = false

		if IsCLITUIEnabled() {
			t.Fatal("expected IsCLITUIEnabled to return false when TUI.Enabled is false")
		}
	})

	t.Run("config with TUI enabled returns true", func(t *testing.T) {
		orig := cliConfig
		t.Cleanup(func() { cliConfig = orig })
		cliConfig = &CLIConfig{}
		cliConfig.TUI.Enabled = true

		if !IsCLITUIEnabled() {
			t.Fatal("expected IsCLITUIEnabled to return true when TUI.Enabled is true")
		}
	})
}

func TestCloseCLILoggingWithNilFile(t *testing.T) {
	orig := cliLogOutputFile
	t.Cleanup(func() { cliLogOutputFile = orig })
	cliLogOutputFile = nil

	CloseCLILogging()
}
