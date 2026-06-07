// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for update.go functions.
// Uses a mock http.RoundTripper to intercept http.Get calls so that
// fetchLatestCLIStableRelease, fetchAllCLIReleases, downloadCLIBinary,
// and verifyCLIChecksum can be exercised without real network access.
// Tests also cover SetCLIUpdateBranch, GetCLIUpdateBranch, runCLIUpdateBranch,
// CheckCLIUpdate, runCLIUpdateCheck, runCLIUpdateApply, and replaceCLIBinary.
package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── mock transport ────────────────────────────────────────────────────────────

// redirectTransport redirects all outbound HTTP requests to a local httptest.Server.
type redirectTransport struct {
	srv      *httptest.Server
	original http.RoundTripper
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	testBase, _ := url.Parse(t.srv.URL)
	cloned.URL.Scheme = testBase.Scheme
	cloned.URL.Host = testBase.Host
	return t.original.RoundTrip(cloned)
}

// installMockTransport replaces http.DefaultTransport for the duration of the test.
func installMockTransport(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = &redirectTransport{srv: srv, original: orig}
	t.Cleanup(func() { http.DefaultTransport = orig })
}

// ── GitHub API mock servers ───────────────────────────────────────────────────

const mockLatestReleaseJSON = `{
  "tag_name": "v99.0.0",
  "html_url": "https://github.com/apimgr/vidveil/releases/tag/v99.0.0",
  "body": "Test release",
  "published_at": "2024-01-01T00:00:00Z",
  "assets": []
}`

const mockLatestReleaseSameVersionJSON = `{
  "tag_name": "dev",
  "html_url": "https://github.com/apimgr/vidveil/releases/tag/dev",
  "body": "Same version",
  "published_at": "2024-01-01T00:00:00Z",
  "assets": []
}`

const mockAllReleasesJSON = `[
  {
    "tag_name": "v1.0.0-beta",
    "html_url": "https://github.com/apimgr/vidveil/releases/tag/v1.0.0-beta",
    "body": "Beta release",
    "published_at": "2024-01-02T00:00:00Z",
    "assets": []
  },
  {
    "tag_name": "202401010000",
    "html_url": "https://github.com/apimgr/vidveil/releases/tag/202401010000",
    "body": "Daily release",
    "published_at": "2024-01-01T00:00:00Z",
    "assets": []
  },
  {
    "tag_name": "v1.0.0",
    "html_url": "https://github.com/apimgr/vidveil/releases/tag/v1.0.0",
    "body": "Stable release",
    "published_at": "2024-01-01T00:00:00Z",
    "assets": []
  }
]`

func newGitHubMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			_, _ = w.Write([]byte(mockLatestReleaseJSON))
		case strings.HasSuffix(r.URL.Path, "/releases"):
			_, _ = w.Write([]byte(mockAllReleasesJSON))
		default:
			// serve a tiny binary for download tests
			_, _ = w.Write([]byte("binarydata"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newSameVersionGitHubServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockLatestReleaseSameVersionJSON))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ── fetchLatestCLIStableRelease ───────────────────────────────────────────────

func TestFetchLatestCLIStableRelease_Success(t *testing.T) {
	srv := newGitHubMockServer(t)
	installMockTransport(t, srv)

	release, err := fetchLatestCLIStableRelease()
	if err != nil {
		t.Fatalf("fetchLatestCLIStableRelease: %v", err)
	}
	if release.TagName == "" {
		t.Error("fetchLatestCLIStableRelease: empty TagName")
	}
}

func TestFetchLatestCLIStableRelease_ServerError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()
	installMockTransport(t, srv)

	if _, err := fetchLatestCLIStableRelease(); err == nil {
		t.Error("fetchLatestCLIStableRelease server error: expected error")
	}
}

func TestFetchLatestCLIStableRelease_BadJSON_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()
	installMockTransport(t, srv)

	if _, err := fetchLatestCLIStableRelease(); err == nil {
		t.Error("fetchLatestCLIStableRelease bad JSON: expected error")
	}
}

// ── fetchAllCLIReleases ───────────────────────────────────────────────────────

func TestFetchAllCLIReleases_Success(t *testing.T) {
	srv := newGitHubMockServer(t)
	installMockTransport(t, srv)

	releases, err := fetchAllCLIReleases()
	if err != nil {
		t.Fatalf("fetchAllCLIReleases: %v", err)
	}
	if len(releases) == 0 {
		t.Error("fetchAllCLIReleases: expected releases")
	}
}

func TestFetchAllCLIReleases_ServerError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()
	installMockTransport(t, srv)

	if _, err := fetchAllCLIReleases(); err == nil {
		t.Error("fetchAllCLIReleases server error: expected error")
	}
}

// ── fetchLatestCLIBetaRelease ─────────────────────────────────────────────────

func TestFetchLatestCLIBetaRelease_Success(t *testing.T) {
	srv := newGitHubMockServer(t)
	installMockTransport(t, srv)

	release, err := fetchLatestCLIBetaRelease()
	if err != nil {
		t.Fatalf("fetchLatestCLIBetaRelease: %v", err)
	}
	if !strings.Contains(strings.ToLower(release.TagName), "-beta") {
		t.Errorf("fetchLatestCLIBetaRelease: TagName %q does not contain -beta", release.TagName)
	}
}

func TestFetchLatestCLIBetaRelease_NoBeta_ReturnsError(t *testing.T) {
	// Server returns releases with no beta tag
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"tag_name":"v1.0.0","html_url":"","assets":[]}]`))
	}))
	defer srv.Close()
	installMockTransport(t, srv)

	if _, err := fetchLatestCLIBetaRelease(); err == nil {
		t.Error("fetchLatestCLIBetaRelease no-beta: expected error")
	}
}

// ── fetchLatestCLIDailyRelease ────────────────────────────────────────────────

func TestFetchLatestCLIDailyRelease_Success(t *testing.T) {
	srv := newGitHubMockServer(t)
	installMockTransport(t, srv)

	release, err := fetchLatestCLIDailyRelease()
	if err != nil {
		t.Fatalf("fetchLatestCLIDailyRelease: %v", err)
	}
	if len(release.TagName) != 12 {
		t.Errorf("fetchLatestCLIDailyRelease: TagName %q has unexpected length", release.TagName)
	}
}

func TestFetchLatestCLIDailyRelease_NoDaily_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"tag_name":"v1.0.0","html_url":"","assets":[]}]`))
	}))
	defer srv.Close()
	installMockTransport(t, srv)

	if _, err := fetchLatestCLIDailyRelease(); err == nil {
		t.Error("fetchLatestCLIDailyRelease no-daily: expected error")
	}
}

// ── downloadCLIBinary ─────────────────────────────────────────────────────────

func TestDownloadCLIBinary_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fake binary content"))
	}))
	defer srv.Close()

	tmpPath, err := downloadCLIBinary(srv.URL + "/fake-binary")
	if err != nil {
		t.Fatalf("downloadCLIBinary: %v", err)
	}
	defer os.Remove(tmpPath)

	if _, err := os.Stat(tmpPath); err != nil {
		t.Errorf("downloadCLIBinary: temp file not found: %v", err)
	}
}

func TestDownloadCLIBinary_ServerError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	if _, err := downloadCLIBinary(srv.URL + "/nope"); err == nil {
		t.Error("downloadCLIBinary server error: expected error")
	}
}

// ── verifyCLIChecksum ─────────────────────────────────────────────────────────

func TestVerifyCLIChecksum_CorrectChecksum_NoPanic(t *testing.T) {
	// Create a temp file with known content
	tmp := t.TempDir()
	binaryPath := filepath.Join(tmp, "fake-binary")
	content := []byte("fake binary content for checksum test")
	if err := os.WriteFile(binaryPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Compute the expected SHA-256
	sum := sha256.Sum256(content)
	expected := hex.EncodeToString(sum[:])

	// Serve the checksum file
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s  fake-binary\n", expected)
	}))
	defer srv.Close()

	if err := verifyCLIChecksum(binaryPath, srv.URL+"/checksum"); err != nil {
		t.Errorf("verifyCLIChecksum correct checksum: %v", err)
	}
}

func TestVerifyCLIChecksum_WrongChecksum_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	binaryPath := filepath.Join(tmp, "fake-binary")
	if err := os.WriteFile(binaryPath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "0000000000000000000000000000000000000000000000000000000000000000  binary\n")
	}))
	defer srv.Close()

	if err := verifyCLIChecksum(binaryPath, srv.URL+"/checksum"); err == nil {
		t.Error("verifyCLIChecksum wrong checksum: expected error")
	}
}

func TestVerifyCLIChecksum_ServerError_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	binaryPath := filepath.Join(tmp, "fake-binary")
	if err := os.WriteFile(binaryPath, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	if err := verifyCLIChecksum(binaryPath, srv.URL+"/checksum"); err == nil {
		t.Error("verifyCLIChecksum server error: expected error")
	}
}

// ── replaceCLIBinary ──────────────────────────────────────────────────────────

func TestReplaceCLIBinary_Success(t *testing.T) {
	tmp := t.TempDir()
	execPath := filepath.Join(tmp, "current-binary")
	newBinaryPath := filepath.Join(tmp, "new-binary")

	if err := os.WriteFile(execPath, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBinaryPath, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := replaceCLIBinary(execPath, newBinaryPath); err != nil {
		t.Errorf("replaceCLIBinary: %v", err)
	}

	content, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Errorf("replaceCLIBinary: content = %q, want %q", content, "new")
	}
}

func TestReplaceCLIBinary_MissingTarget_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	execPath := filepath.Join(tmp, "nonexistent-binary")
	newBinaryPath := filepath.Join(tmp, "new-binary")
	if err := os.WriteFile(newBinaryPath, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := replaceCLIBinary(execPath, newBinaryPath); err == nil {
		t.Error("replaceCLIBinary missing target: expected error")
	}
}

// ── SetCLIUpdateBranch / GetCLIUpdateBranch ───────────────────────────────────

func TestSetCLIUpdateBranch_Stable_WritesFile(t *testing.T) {
	if err := SetCLIUpdateBranch("stable"); err != nil {
		t.Errorf("SetCLIUpdateBranch stable: %v", err)
	}
}

func TestSetCLIUpdateBranch_Beta_WritesFile(t *testing.T) {
	if err := SetCLIUpdateBranch("beta"); err != nil {
		t.Errorf("SetCLIUpdateBranch beta: %v", err)
	}
}

func TestGetCLIUpdateBranch_AfterSet_ReturnsCorrectBranch(t *testing.T) {
	if err := SetCLIUpdateBranch("beta"); err != nil {
		t.Fatalf("SetCLIUpdateBranch: %v", err)
	}
	got := GetCLIUpdateBranch()
	if got != "beta" {
		t.Errorf("GetCLIUpdateBranch after set beta = %q, want %q", got, "beta")
	}
	// Restore stable
	_ = SetCLIUpdateBranch("stable")
}

func TestGetCLIUpdateBranch_MissingFile_ReturnsStable(t *testing.T) {
	// Temporarily rename the branch file if it exists
	origBranchFile := filepath.Join(paths_configDirForTest(), CLIUpdateBranchFile)
	tmpName := origBranchFile + ".bak"
	_ = os.Rename(origBranchFile, tmpName)
	t.Cleanup(func() { _ = os.Rename(tmpName, origBranchFile) })

	got := GetCLIUpdateBranch()
	if got != "stable" {
		t.Errorf("GetCLIUpdateBranch no file = %q, want stable", got)
	}
}

// paths_configDirForTest returns the same directory that SetCLIUpdateBranch uses.
func paths_configDirForTest() string {
	// We need to import client/paths but it's the same package trick isn't available here.
	// Instead, exercise GetCLIUpdateBranch to determine where the file lives.
	// This helper is only used for file manipulation in tests.
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "apimgr", "vidveil-cli")
}

// ── runCLIUpdateBranch ────────────────────────────────────────────────────────

func TestRunCLIUpdateBranch_ValidBranch_NoPanic(t *testing.T) {
	if err := runCLIUpdateBranch("daily"); err != nil {
		t.Errorf("runCLIUpdateBranch daily: %v", err)
	}
	// Restore
	_ = SetCLIUpdateBranch("stable")
}

func TestRunCLIUpdateBranch_InvalidBranch_ReturnsError(t *testing.T) {
	if err := runCLIUpdateBranch("invalid-branch"); err == nil {
		t.Error("runCLIUpdateBranch invalid: expected error")
	}
}

func TestRunCLIUpdateBranch_EmptyBranch_ReturnsError(t *testing.T) {
	if err := runCLIUpdateBranch(""); err == nil {
		t.Error("runCLIUpdateBranch empty: expected error")
	}
}

// ── CheckCLIUpdate ────────────────────────────────────────────────────────────

func TestCheckCLIUpdate_SameVersion_UpdateNotAvailable(t *testing.T) {
	srv := newSameVersionGitHubServer(t)
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("stable")

	info, err := CheckCLIUpdate()
	if err != nil {
		t.Fatalf("CheckCLIUpdate same version: %v", err)
	}
	if info.UpdateAvailable {
		t.Error("CheckCLIUpdate same version: UpdateAvailable should be false")
	}
}

func TestCheckCLIUpdate_NewVersion_UpdateAvailable(t *testing.T) {
	srv := newGitHubMockServer(t)
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("stable")

	info, err := CheckCLIUpdate()
	if err != nil {
		t.Fatalf("CheckCLIUpdate new version: %v", err)
	}
	if !info.UpdateAvailable {
		t.Errorf("CheckCLIUpdate new version: UpdateAvailable = false, want true (latest=%s, current=%s)", info.LatestVersion, info.CurrentVersion)
	}
}

func TestCheckCLIUpdate_BetaBranch_NoPanic(t *testing.T) {
	srv := newGitHubMockServer(t)
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("beta")
	t.Cleanup(func() { _ = SetCLIUpdateBranch("stable") })

	_, err := CheckCLIUpdate()
	if err != nil {
		t.Logf("CheckCLIUpdate beta: error (acceptable): %v", err)
	}
}

func TestCheckCLIUpdate_DailyBranch_NoPanic(t *testing.T) {
	srv := newGitHubMockServer(t)
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("daily")
	t.Cleanup(func() { _ = SetCLIUpdateBranch("stable") })

	_, err := CheckCLIUpdate()
	if err != nil {
		t.Logf("CheckCLIUpdate daily: error (acceptable): %v", err)
	}
}

// ── runCLIUpdateCheck ─────────────────────────────────────────────────────────

func TestRunCLIUpdateCheck_SameVersion_NoPanic(t *testing.T) {
	srv := newSameVersionGitHubServer(t)
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("stable")

	if err := runCLIUpdateCheck(); err != nil {
		t.Errorf("runCLIUpdateCheck same version: %v", err)
	}
}

func TestRunCLIUpdateCheck_NewVersion_NoPanic(t *testing.T) {
	srv := newGitHubMockServer(t)
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("stable")

	if err := runCLIUpdateCheck(); err != nil {
		t.Errorf("runCLIUpdateCheck new version: %v", err)
	}
}

func TestRunCLIUpdateCheck_ServerError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("stable")

	if err := runCLIUpdateCheck(); err == nil {
		t.Error("runCLIUpdateCheck server error: expected error")
	}
}

func TestRunCLIUpdateCheck_404_NoPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("stable")

	// 404 is treated as "already up to date", not an error
	if err := runCLIUpdateCheck(); err != nil {
		t.Errorf("runCLIUpdateCheck 404: expected nil but got: %v", err)
	}
}

// ── runCLIUpdateApply ─────────────────────────────────────────────────────────

func TestRunCLIUpdateApply_SameVersion_AlreadyUpToDate(t *testing.T) {
	srv := newSameVersionGitHubServer(t)
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("stable")

	if err := runCLIUpdateApply(); err != nil {
		t.Errorf("runCLIUpdateApply same version: %v", err)
	}
}

func TestRunCLIUpdateApply_NewVersionNoAsset_ReturnsError(t *testing.T) {
	// Returns a newer version but no download asset
	srv := newGitHubMockServer(t)
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("stable")

	// The mock returns v99.0.0 with no assets[], so DownloadURL = ""
	// runCLIUpdateApply should return "no download asset" error
	err := runCLIUpdateApply()
	if err == nil {
		t.Error("runCLIUpdateApply no asset: expected error about missing download URL")
	}
}

func TestRunCLIUpdateApply_404_AlreadyUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("stable")

	if err := runCLIUpdateApply(); err != nil {
		t.Errorf("runCLIUpdateApply 404: expected nil, got: %v", err)
	}
}

// ── RunCLIUpdateCommand ───────────────────────────────────────────────────────

func TestRunCLIUpdateCommand_BranchStable_NoPanic(t *testing.T) {
	if err := RunCLIUpdateCommand([]string{"branch", "stable"}); err != nil {
		t.Errorf("RunCLIUpdateCommand branch stable: %v", err)
	}
}

func TestRunCLIUpdateCommand_BranchBeta_NoPanic(t *testing.T) {
	if err := RunCLIUpdateCommand([]string{"branch", "beta"}); err != nil {
		t.Errorf("RunCLIUpdateCommand branch beta: %v", err)
	}
	_ = SetCLIUpdateBranch("stable")
}

func TestRunCLIUpdateCommand_BranchInvalid_ReturnsError(t *testing.T) {
	if err := RunCLIUpdateCommand([]string{"branch", "invalid"}); err == nil {
		t.Error("RunCLIUpdateCommand branch invalid: expected error")
	}
}

func TestRunCLIUpdateCommand_Check_NoPanic(t *testing.T) {
	srv := newSameVersionGitHubServer(t)
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("stable")

	if err := RunCLIUpdateCommand([]string{"check"}); err != nil {
		t.Errorf("RunCLIUpdateCommand check: %v", err)
	}
}

func TestRunCLIUpdateCommand_Yes_SameVersion_NoPanic(t *testing.T) {
	srv := newSameVersionGitHubServer(t)
	installMockTransport(t, srv)
	_ = SetCLIUpdateBranch("stable")

	if err := RunCLIUpdateCommand([]string{"yes"}); err != nil {
		t.Errorf("RunCLIUpdateCommand yes same version: %v", err)
	}
}

func TestRunCLIUpdateCommand_Unknown_ReturnsError(t *testing.T) {
	if err := RunCLIUpdateCommand([]string{"bogus"}); err == nil {
		t.Error("RunCLIUpdateCommand bogus: expected error")
	}
}
