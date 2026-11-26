// SPDX-License-Identifier: MIT
package banner

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestModeForWidth(t *testing.T) {
	tests := []struct {
		width int
		mode  int
	}{
		{width: 39, mode: 0},
		{width: 40, mode: 1},
		{width: 59, mode: 1},
		{width: 60, mode: 2},
		{width: 79, mode: 2},
		{width: 80, mode: 3},
	}
	for _, test := range tests {
		if got := modeForWidth(test.width); got != test.mode {
			t.Errorf("modeForWidth(%d) = %d, want %d", test.width, got, test.mode)
		}
	}
}

func TestCompactAddress(t *testing.T) {
	if got := compactAddress([]string{"http://vidveil.example:8080/path"}); got != "vidveil.example:8080" {
		t.Fatalf("compactAddress = %q, want hostname and port", got)
	}
	if got := compactAddress(nil); got != "" {
		t.Fatalf("compactAddress(nil) = %q, want empty string", got)
	}
}

func TestPrintServerStartupBannerIncludesResolvedDetails(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("NO_COLOR", "1")
	t.Setenv("COLUMNS", "80")
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	originalStdout := os.Stdout
	os.Stdout = write
	defer func() { os.Stdout = originalStdout }()

	PrintServerStartupBanner(ServerBannerInfo{
		AppName:         "VidVeil",
		Version:         "1.2.3",
		AppMode:         "production",
		StartupDatetime: "November 26, 2026 at 5:22:42 PM EST",
		URLs:            []string{"https://vidveil.example"},
		OnionAddress:    "example.onion",
		I2PAddress:      "example.b32.i2p",
		SMTPAddress:     "mail.example:25",
		SetupToken:      "adm_once",
	})
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if err := read.Close(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Listening on https://vidveil.example", "Tor: http://example.onion", "I2P: http://example.b32.i2p", "SMTP: mail.example:25", "Server started on November 26", "Setup token: adm_once"} {
		if !strings.Contains(string(output), want) {
			t.Errorf("banner output %q does not contain %q", output, want)
		}
	}
}
