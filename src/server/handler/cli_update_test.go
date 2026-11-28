// SPDX-License-Identifier: MIT
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/paths"
	"github.com/go-chi/chi/v5"
)

type cliAuditCapture struct {
	event   string
	actorID string
	actorIP string
	result  string
	details map[string]interface{}
}

func (capture *cliAuditCapture) Audit(event, actorID, actorType, actorIP, result string, details map[string]interface{}) {
	capture.event = event
	capture.actorID = actorID
	capture.actorIP = actorIP
	capture.result = result
	capture.details = details
}

func writeCLITestAssets(t *testing.T, dir string, binary []byte, checksum string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	name := cliAssetName("linux-amd64")
	if err := os.WriteFile(filepath.Join(dir, name), binary, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version.txt"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sha256.txt"), []byte(fmt.Sprintf("%s  %s\n", checksum, name)), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCLIVersionsRequireMatchingLocalChecksum(t *testing.T) {
	dir := t.TempDir()
	binary := []byte("local cli release")
	digest := sha256.Sum256(binary)
	writeCLITestAssets(t, dir, binary, hex.EncodeToString(digest[:]))
	versions := cliVersionsInDirectory(dir)
	entry, ok := versions["linux-amd64"]
	if !ok || entry.Version != "1.2.3" || entry.SHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("verified metadata = %+v, want local version and checksum", versions)
	}
	writeCLITestAssets(t, dir, binary, strings.Repeat("0", sha256.Size*2))
	if got := cliVersionsInDirectory(dir); len(got) != 0 {
		t.Fatalf("mismatched checksum published metadata: %+v", got)
	}
}

func TestCLIVersionsMissingDirectoryIsEmpty(t *testing.T) {
	if got := cliVersionsInDirectory(filepath.Join(t.TempDir(), "missing")); len(got) != 0 {
		t.Fatalf("missing directory metadata = %+v, want empty", got)
	}
}

func TestCLIVersionsRefreshWhenProvisionedDirectoryChanges(t *testing.T) {
	dir := t.TempDir()
	binary := []byte("local cli release")
	digest := sha256.Sum256(binary)
	writeCLITestAssets(t, dir, binary, hex.EncodeToString(digest[:]))
	cfg := config.DefaultAppConfig()
	cfg.CLI.BinaryDownload.Dir = dir
	h := NewSearchHandler(cfg, nil)
	audit := &cliAuditCapture{}
	h.SetOwnerTokenAuditLogger(audit)
	if got := h.cliVersions()["linux-amd64"].Version; got != "1.2.3" {
		t.Fatalf("initial version = %q, want 1.2.3", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "version.txt"), []byte("2.0.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := h.cliVersions()["linux-amd64"].Version; got != "2.0.0" {
		t.Fatalf("refreshed version = %q, want 2.0.0", got)
	}
}

func TestCLIVersionsRejectMalformedVersionAndChecksum(t *testing.T) {
	dir := t.TempDir()
	binary := []byte("local cli release")
	digest := sha256.Sum256(binary)
	writeCLITestAssets(t, dir, binary, hex.EncodeToString(digest[:]))
	if err := os.WriteFile(filepath.Join(dir, "version.txt"), []byte("1.2.3 extra\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := cliVersionsInDirectory(dir); len(got) != 0 {
		t.Fatalf("malformed version published metadata: %+v", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "version.txt"), []byte("1.2.3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sha256.txt"), []byte(strings.Repeat("z", 64)+"  "+cliAssetName("linux-amd64")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := cliVersionsInDirectory(dir); len(got) != 0 {
		t.Fatalf("malformed checksum published metadata: %+v", got)
	}
}

func TestCLIBinaryDownloadServesOnlyVerifiedLocalAssets(t *testing.T) {
	dir := t.TempDir()
	binary := []byte("verified executable bytes")
	digest := sha256.Sum256(binary)
	writeCLITestAssets(t, dir, binary, hex.EncodeToString(digest[:]))
	cfg := config.DefaultAppConfig()
	cfg.CLI.BinaryDownload.Dir = dir
	h := NewSearchHandler(cfg, nil)
	audit := &cliAuditCapture{}
	h.SetOwnerTokenAuditLogger(audit)

	router := chi.NewRouter()
	router.Get("/api/v1/cli/binaries/{filename}", h.CLIBinaryDownload)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/cli/binaries/"+cliAssetName("linux-amd64"), nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != string(binary) {
		t.Fatalf("verified download = %d %q, want local binary", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", response.Header().Get("Content-Type"))
	}
	if audit.event != "cli.update_started" || audit.result != "success" || audit.details["os_arch"] != "linux-amd64" || audit.details["version"] != "1.2.3" {
		t.Errorf("download audit = %+v, want cli.update_started with platform and version", audit)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/v1/cli/binaries/../../outside", nil)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("invalid download status = %d, want 404", response.Code)
	}
}

func TestCLIAssetNamesCoverSupportedPlatforms(t *testing.T) {
	if len(cliPlatforms) != 8 {
		t.Fatalf("supported platform count = %d, want all 8", len(cliPlatforms))
	}
	for _, platform := range cliPlatforms {
		name := cliAssetName(platform)
		if _, ok := cliPlatformFromRouteName(name); !ok {
			t.Errorf("platform asset %q is not recognized", name)
		}
	}
	if _, ok := cliPlatformFromRouteName(paths.ProjectName + "-cli-linux-riscv64"); ok {
		t.Fatal("unsupported architecture was accepted")
	}
}
