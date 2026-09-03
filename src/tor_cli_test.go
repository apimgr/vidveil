// SPDX-License-Identifier: MIT
// Coverage tests for the tor CLI helpers in tor_cli.go.
// Mutating subcommands (restart/regenerate/vanity/import-keys) now reach the
// server-owned Tor process over the internal loopback control channel
// (AI.md PART 31 "CLI-to-running-server control channel") instead of
// touching Tor state directly, so each is exercised both against a fake
// control-channel HTTP server and via its "no running server" error path.
package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
)

// captureStderr redirects os.Stderr to a buffer for the duration of f,
// returning the captured output.
func captureStderr(f func()) string {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	orig := os.Stderr
	os.Stderr = w
	f()
	w.Close()
	os.Stderr = orig
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, readErr := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if readErr != nil {
			break
		}
	}
	r.Close()
	return string(buf)
}

// startFakeControlServer starts a real HTTP listener bound to 127.0.0.1 on an
// OS-assigned port, serving mux, and writes a matching server.yml under a
// fresh temp configDir/dataDir pair so resolveServerAddr(configDir, dataDir)
// resolves straight to it. The listener is closed automatically at test end.
func startFakeControlServer(t *testing.T, mux *http.ServeMux) (configDir, dataDir string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
		_ = ln.Close()
	})

	port := ln.Addr().(*net.TCPAddr).Port
	base := t.TempDir()
	configDir = filepath.Join(base, "config")
	dataDir = filepath.Join(base, "data")

	cfg, configPath, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Server.Port = strconv.Itoa(port)
	if err := config.SaveAppConfig(cfg, configPath); err != nil {
		t.Fatal(err)
	}
	return configDir, dataDir
}

func jsonOK(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "data": data})
}

func jsonError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "SERVER_ERROR", "message": message})
}

// ── torDirs ───────────────────────────────────────────────────────────────────

func TestTorDirs_JoinsTorAndSite(t *testing.T) {
	base := t.TempDir()
	torDir, siteDir := torDirs(filepath.Join(base, "config"), filepath.Join(base, "data"))
	if torDir != filepath.Join(base, "data", "tor") {
		t.Errorf("torDirs: torDir = %q, want %q", torDir, filepath.Join(base, "data", "tor"))
	}
	if siteDir != filepath.Join(torDir, "site") {
		t.Errorf("torDirs: siteDir = %q, want %q", siteDir, filepath.Join(torDir, "site"))
	}
}

// ── readHostnameFile ──────────────────────────────────────────────────────────

func TestReadHostnameFile_Missing(t *testing.T) {
	if got := readHostnameFile(t.TempDir()); got != "" {
		t.Errorf("readHostnameFile on missing file = %q, want empty", got)
	}
}

func TestReadHostnameFile_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hostname"), []byte("abcd1234.onion\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := readHostnameFile(dir); got != "abcd1234.onion" {
		t.Errorf("readHostnameFile = %q, want %q", got, "abcd1234.onion")
	}
}

// ── printTorHelp ──────────────────────────────────────────────────────────────

func TestPrintTorHelp_ListsAllSubcommands(t *testing.T) {
	out := captureStdout(printTorHelp)
	// Per AI.md PART 31 CLI table: all seven operations must be documented
	for _, cmd := range []string{"tor status", "tor validate", "tor restart", "tor regenerate", "tor vanity start", "tor vanity apply", "tor import-keys"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("printTorHelp: missing %q", cmd)
		}
	}
}

// ── handleTorCommand ──────────────────────────────────────────────────────────

func TestHandleTorCommand_NoArgs(t *testing.T) {
	var code int
	captureStdout(func() { code = handleTorCommand(nil, "", "") })
	if code != 1 {
		t.Errorf("handleTorCommand(nil) = %d, want 1", code)
	}
}

func TestHandleTorCommand_Help(t *testing.T) {
	var code int
	out := captureStdout(func() { code = handleTorCommand([]string{"help"}, "", "") })
	if code != 0 {
		t.Errorf("handleTorCommand(help) = %d, want 0", code)
	}
	if !strings.Contains(out, "Tor Hidden Service Commands") {
		t.Error("handleTorCommand(help): missing help header")
	}
}

func TestHandleTorCommand_Unknown(t *testing.T) {
	var code int
	captureStdout(func() { code = handleTorCommand([]string{"bogus"}, "", "") })
	if code != 1 {
		t.Errorf("handleTorCommand(bogus) = %d, want 1", code)
	}
}

func TestHandleTorCommand_VanityMissingSubcommand(t *testing.T) {
	if code := handleTorCommand([]string{"vanity"}, "", ""); code != 1 {
		t.Errorf("handleTorCommand(vanity) = %d, want 1", code)
	}
}

func TestHandleTorCommand_VanityStartMissingPrefix(t *testing.T) {
	if code := handleTorCommand([]string{"vanity", "start"}, "", ""); code != 1 {
		t.Errorf("handleTorCommand(vanity start) = %d, want 1", code)
	}
}

func TestHandleTorCommand_VanityUnknown(t *testing.T) {
	if code := handleTorCommand([]string{"vanity", "bogus"}, "", ""); code != 1 {
		t.Errorf("handleTorCommand(vanity bogus) = %d, want 1", code)
	}
}

func TestHandleTorCommand_ImportKeysMissingPath(t *testing.T) {
	if code := handleTorCommand([]string{"import-keys"}, "", ""); code != 1 {
		t.Errorf("handleTorCommand(import-keys) = %d, want 1", code)
	}
}

// ── torStatus (server not running, disk fallback) ─────────────────────────────

func TestTorStatus_ServerStoppedWithHostname(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")
	siteDir := filepath.Join(dataDir, "tor", "site")
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "hostname"), []byte("teststatus.onion\n"), 0600); err != nil {
		t.Fatal(err)
	}

	var code int
	out := captureStdout(func() { code = torStatus(configDir, dataDir) })
	if code != 0 {
		t.Errorf("torStatus = %d, want 0", code)
	}
	if !strings.Contains(out, "teststatus.onion") {
		t.Errorf("torStatus: missing stored address, got %q", out)
	}
}

func TestTorStatus_ServerStoppedNoKeys(t *testing.T) {
	base := t.TempDir()
	var code int
	out := captureStdout(func() {
		code = torStatus(filepath.Join(base, "config"), filepath.Join(base, "data"))
	})
	if code != 0 {
		t.Errorf("torStatus = %d, want 0", code)
	}
	if !strings.Contains(out, "server not running") {
		t.Errorf("torStatus: missing stopped notice, got %q", out)
	}
}

func TestTorStatus_ServerRunning(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/server/tor/status", func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]interface{}{
			"running":       true,
			"starting":      false,
			"enabled":       true,
			"status_string": "connected",
			"onion_address": "running123.onion",
		})
	})
	configDir, dataDir := startFakeControlServer(t, mux)

	var code int
	out := captureStdout(func() { code = torStatus(configDir, dataDir) })
	if code != 0 {
		t.Errorf("torStatus = %d, want 0", code)
	}
	if !strings.Contains(out, "Connected") || !strings.Contains(out, "running123.onion") {
		t.Errorf("torStatus: missing live status/address, got %q", out)
	}
}

// ── torValidate ───────────────────────────────────────────────────────────────

func TestTorValidate_FreshDirs(t *testing.T) {
	base := t.TempDir()
	var code int
	out := captureStdout(func() {
		code = torValidate(filepath.Join(base, "config"), filepath.Join(base, "data"))
	})
	if code != 0 {
		t.Errorf("torValidate = %d, want 0 (missing tor binary is a warning, not an error)", code)
	}
	if !strings.Contains(out, "Config:") {
		t.Errorf("torValidate: missing config line, got %q", out)
	}
}

func TestTorValidate_SecretKeyWithoutHostname(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")
	siteDir := filepath.Join(dataDir, "tor", "site")
	if err := os.MkdirAll(siteDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "hs_ed25519_secret_key"), []byte("stub"), 0600); err != nil {
		t.Fatal(err)
	}

	var code int
	captureStdout(func() { code = torValidate(configDir, dataDir) })
	if code != 1 {
		t.Errorf("torValidate with orphaned secret key = %d, want 1", code)
	}
}

func TestTorValidate_LiveConnectionTest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/server/tor/validate", func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]interface{}{"connected": true, "message": "reachable"})
	})
	configDir, dataDir := startFakeControlServer(t, mux)

	var code int
	out := captureStdout(func() { code = torValidate(configDir, dataDir) })
	if code != 0 {
		t.Errorf("torValidate = %d, want 0", code)
	}
	if !strings.Contains(out, "Live connection test: reachable") {
		t.Errorf("torValidate: missing live connection test line, got %q", out)
	}
}

// ── torRestart ────────────────────────────────────────────────────────────────

func TestTorRestart_NoRunningServer(t *testing.T) {
	base := t.TempDir()
	var code int
	errOut := captureStderr(func() {
		code = torRestart(filepath.Join(base, "config"), filepath.Join(base, "data"))
	})
	if code != 1 {
		t.Errorf("torRestart without a running server = %d, want 1", code)
	}
	if !strings.Contains(errOut, errNoRunningServer) {
		t.Errorf("torRestart: stderr = %q, want to contain %q", errOut, errNoRunningServer)
	}
}

func TestTorRestart_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/server/tor/restart", func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]interface{}{"message": "tor restarted"})
	})
	configDir, dataDir := startFakeControlServer(t, mux)

	var code int
	out := captureStdout(func() { code = torRestart(configDir, dataDir) })
	if code != 0 {
		t.Errorf("torRestart = %d, want 0", code)
	}
	if !strings.Contains(out, "Tor restarted") {
		t.Errorf("torRestart: missing success message, got %q", out)
	}
}

func TestTorRestart_ServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/server/tor/restart", func(w http.ResponseWriter, r *http.Request) {
		jsonError(w, "tor is not enabled")
	})
	configDir, dataDir := startFakeControlServer(t, mux)

	var code int
	errOut := captureStderr(func() { code = torRestart(configDir, dataDir) })
	if code != 1 {
		t.Errorf("torRestart with server error = %d, want 1", code)
	}
	if !strings.Contains(errOut, "tor is not enabled") {
		t.Errorf("torRestart: stderr = %q, want to contain server error message", errOut)
	}
}

// ── torRegenerate ─────────────────────────────────────────────────────────────

func TestTorRegenerate_NoRunningServer(t *testing.T) {
	base := t.TempDir()
	var code int
	errOut := captureStderr(func() {
		code = torRegenerate(filepath.Join(base, "config"), filepath.Join(base, "data"))
	})
	if code != 1 {
		t.Errorf("torRegenerate without a running server = %d, want 1", code)
	}
	if !strings.Contains(errOut, errNoRunningServer) {
		t.Errorf("torRegenerate: stderr = %q, want to contain %q", errOut, errNoRunningServer)
	}
}

func TestTorRegenerate_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/server/tor/regenerate", func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]interface{}{"onion_address": "regenerated.onion"})
	})
	configDir, dataDir := startFakeControlServer(t, mux)

	var code int
	out := captureStdout(func() { code = torRegenerate(configDir, dataDir) })
	if code != 0 {
		t.Errorf("torRegenerate = %d, want 0", code)
	}
	if !strings.Contains(out, "regenerated.onion") {
		t.Errorf("torRegenerate: missing new address, got %q", out)
	}
}

// ── torVanityStart / torVanityApply ────────────────────────────────────────────

func TestTorVanityStart_NoRunningServer(t *testing.T) {
	base := t.TempDir()
	var code int
	errOut := captureStderr(func() {
		code = torVanityStart(filepath.Join(base, "config"), filepath.Join(base, "data"), "abc")
	})
	if code != 1 {
		t.Errorf("torVanityStart without a running server = %d, want 1", code)
	}
	if !strings.Contains(errOut, errNoRunningServer) {
		t.Errorf("torVanityStart: stderr = %q, want to contain %q", errOut, errNoRunningServer)
	}
}

func TestTorVanityStart_FindsPendingAddress(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/server/tor/vanity/start", func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]interface{}{"message": "vanity generation started"})
	})
	mux.HandleFunc("/server/tor/status", func(w http.ResponseWriter, r *http.Request) {
		// Report generation already finished so the poll loop exits on its
		// first iteration (still incurs the single 2s poll interval sleep).
		jsonOK(w, map[string]interface{}{
			"info": map[string]interface{}{
				"vanity": map[string]interface{}{"active": false, "attempts": float64(42)},
			},
		})
	})
	configDir, dataDir := startFakeControlServer(t, mux)

	torDir, _ := torDirs(configDir, dataDir)
	if err := os.MkdirAll(filepath.Join(torDir, "vanity_pending"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(torDir, "vanity_pending", "hostname"), []byte("abcvanity.onion\n"), 0600); err != nil {
		t.Fatal(err)
	}

	var code int
	out := captureStdout(func() { code = torVanityStart(configDir, dataDir, "abc") })
	if code != 0 {
		t.Errorf("torVanityStart = %d, want 0", code)
	}
	if !strings.Contains(out, "abcvanity.onion") {
		t.Errorf("torVanityStart: missing pending address, got %q", out)
	}
}

func TestTorVanityApply_NoRunningServer(t *testing.T) {
	base := t.TempDir()
	var code int
	errOut := captureStderr(func() {
		code = torVanityApply(filepath.Join(base, "config"), filepath.Join(base, "data"))
	})
	if code != 1 {
		t.Errorf("torVanityApply without a running server = %d, want 1", code)
	}
	if !strings.Contains(errOut, errNoRunningServer) {
		t.Errorf("torVanityApply: stderr = %q, want to contain %q", errOut, errNoRunningServer)
	}
}

func TestTorVanityApply_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/server/tor/vanity/apply", func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]interface{}{"onion_address": "applied.onion"})
	})
	configDir, dataDir := startFakeControlServer(t, mux)

	var code int
	out := captureStdout(func() { code = torVanityApply(configDir, dataDir) })
	if code != 0 {
		t.Errorf("torVanityApply = %d, want 0", code)
	}
	if !strings.Contains(out, "applied.onion") {
		t.Errorf("torVanityApply: missing applied address, got %q", out)
	}
}

// ── torImportKeys ─────────────────────────────────────────────────────────────

func TestTorImportKeys_NoRunningServer(t *testing.T) {
	base := t.TempDir()
	var code int
	errOut := captureStderr(func() {
		code = torImportKeys(filepath.Join(base, "config"), filepath.Join(base, "data"), filepath.Join(base, "nope.key"))
	})
	if code != 1 {
		t.Errorf("torImportKeys without a running server = %d, want 1", code)
	}
	if !strings.Contains(errOut, errNoRunningServer) {
		t.Errorf("torImportKeys: stderr = %q, want to contain %q", errOut, errNoRunningServer)
	}
}

func TestTorImportKeys_MissingFile(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/server/tor/import-keys", func(w http.ResponseWriter, r *http.Request) {
		t.Error("import-keys handler must not be reached when the key file is unreadable")
	})
	configDir, dataDir := startFakeControlServer(t, mux)

	var code int
	errOut := captureStderr(func() {
		code = torImportKeys(configDir, dataDir, filepath.Join(dataDir, "nope.key"))
	})
	if code != 1 {
		t.Errorf("torImportKeys with missing file = %d, want 1", code)
	}
	if !strings.Contains(errOut, "Failed to read key file") {
		t.Errorf("torImportKeys: stderr = %q, want to contain read-failure message", errOut)
	}
}

func TestTorImportKeys_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/server/tor/import-keys", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SecretKey []byte `json:"secret_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.SecretKey) == 0 {
			jsonError(w, "secret_key is required")
			return
		}
		jsonOK(w, map[string]interface{}{"onion_address": "imported.onion"})
	})
	configDir, dataDir := startFakeControlServer(t, mux)

	keyPath := filepath.Join(dataDir, "imported.key")
	if err := os.WriteFile(keyPath, []byte("stub-secret-key-bytes"), 0600); err != nil {
		t.Fatal(err)
	}

	var code int
	out := captureStdout(func() { code = torImportKeys(configDir, dataDir, keyPath) })
	if code != 0 {
		t.Errorf("torImportKeys = %d, want 0", code)
	}
	if !strings.Contains(out, "imported.onion") {
		t.Errorf("torImportKeys: missing new address, got %q", out)
	}
}

// ── serverReachable ───────────────────────────────────────────────────────────

func TestServerReachable_EmptyAddr(t *testing.T) {
	if serverReachable("") {
		t.Error("serverReachable(\"\") = true, want false")
	}
}

func TestServerReachable_ClosedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	if serverReachable(addr) {
		t.Error("serverReachable on a closed port = true, want false")
	}
}
