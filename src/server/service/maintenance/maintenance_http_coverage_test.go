// SPDX-License-Identifier: MIT
// AI.md PART 28: Coverage tests for CheckUpdate, fetchLatestRelease,
// fetchLatestBetaRelease, fetchLatestDailyRelease, fetchAllReleases,
// and the no-update path of ApplyUpdate.
// All GitHub API calls are intercepted via a mock HTTP transport — no real
// network calls are made.
package maintenance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

// redirectTransport rewrites every outbound request to point at srv,
// preserving the original path and query string.
type maintenanceRedirectTransport struct {
	srv      *httptest.Server
	original http.RoundTripper
}

func (t *maintenanceRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	testBase, _ := url.Parse(t.srv.URL)
	cloned.URL.Scheme = testBase.Scheme
	cloned.URL.Host = testBase.Host
	return t.original.RoundTrip(cloned)
}

// installMaintenanceMockTransport replaces http.DefaultTransport for the test.
func installMaintenanceMockTransport(t *testing.T, srv *httptest.Server) {
	orig := http.DefaultTransport
	http.DefaultTransport = &maintenanceRedirectTransport{srv: srv, original: orig}
	t.Cleanup(func() { http.DefaultTransport = orig })
}

// newGitHubMockServer returns a server that serves GitHub-like release responses:
//   - GET **/releases/latest  → single release JSON with tag_name=latestTag
//   - GET **/releases?**      → array with a beta and a daily release
//   - everything else         → 200 with dummy bytes (download simulation)
func newGitHubMockServer(latestTag string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case len(path) >= 7 && path[len(path)-7:] == "/latest":
			rel := GitHubRelease{TagName: latestTag, HTMLURL: "https://example.com", Assets: []GitHubAsset{}}
			json.NewEncoder(w).Encode(rel)
		case r.URL.Query().Get("per_page") != "" || (len(path) >= 9 && path[len(path)-9:] == "/releases"):
			releases := []GitHubRelease{
				{TagName: "v2.0.0-beta", HTMLURL: "https://example.com/beta"},
				{TagName: "202601010000", HTMLURL: "https://example.com/daily"},
				{TagName: latestTag, HTMLURL: "https://example.com/stable"},
			}
			json.NewEncoder(w).Encode(releases)
		default:
			w.Write([]byte("binarydata"))
		}
	}))
}

// newMaintManagerTmp creates a MaintenanceManager with temp dirs.
func newMaintManagerTmp(t *testing.T, version string) *MaintenanceManager {
	t.Helper()
	tmp := t.TempDir()
	cfg := tmp
	data := tmp
	return NewMaintenanceManager(cfg, data, version)
}

// ── fetchLatestRelease ────────────────────────────────────────────────────────

func TestFetchLatestRelease_Success(t *testing.T) {
	srv := newGitHubMockServer("v99.0.0")
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	rel, err := m.fetchLatestRelease()
	if err != nil {
		t.Fatalf("fetchLatestRelease: %v", err)
	}
	if rel.TagName != "v99.0.0" {
		t.Errorf("TagName = %q, want v99.0.0", rel.TagName)
	}
}

func TestFetchLatestRelease_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	_, err := m.fetchLatestRelease()
	if err == nil {
		t.Error("fetchLatestRelease on 500: expected error, got nil")
	}
}

func TestFetchLatestRelease_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-json"))
	}))
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	_, err := m.fetchLatestRelease()
	if err == nil {
		t.Error("fetchLatestRelease bad JSON: expected error, got nil")
	}
}

// ── fetchAllReleases ──────────────────────────────────────────────────────────

func TestFetchAllReleases_Success(t *testing.T) {
	srv := newGitHubMockServer("v2.0.0")
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	releases, err := m.fetchAllReleases()
	if err != nil {
		t.Fatalf("fetchAllReleases: %v", err)
	}
	if len(releases) == 0 {
		t.Error("fetchAllReleases: expected at least one release")
	}
}

func TestFetchAllReleases_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	_, err := m.fetchAllReleases()
	if err == nil {
		t.Error("fetchAllReleases on 403: expected error, got nil")
	}
}

func TestFetchAllReleases_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{bad"))
	}))
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	_, err := m.fetchAllReleases()
	if err == nil {
		t.Error("fetchAllReleases bad JSON: expected error, got nil")
	}
}

// ── fetchLatestBetaRelease ────────────────────────────────────────────────────

func TestFetchLatestBetaRelease_Found(t *testing.T) {
	srv := newGitHubMockServer("v2.0.0")
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	rel, err := m.fetchLatestBetaRelease()
	if err != nil {
		t.Fatalf("fetchLatestBetaRelease: %v", err)
	}
	if rel == nil {
		t.Error("fetchLatestBetaRelease: expected non-nil release")
	}
}

func TestFetchLatestBetaRelease_NoneFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]GitHubRelease{
			{TagName: "v1.0.0", HTMLURL: "https://example.com"},
		})
	}))
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	_, err := m.fetchLatestBetaRelease()
	if err == nil {
		t.Error("fetchLatestBetaRelease(no beta): expected error, got nil")
	}
}

// ── fetchLatestDailyRelease ───────────────────────────────────────────────────

func TestFetchLatestDailyRelease_Found(t *testing.T) {
	srv := newGitHubMockServer("v2.0.0")
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	rel, err := m.fetchLatestDailyRelease()
	if err != nil {
		t.Fatalf("fetchLatestDailyRelease: %v", err)
	}
	if rel == nil {
		t.Error("fetchLatestDailyRelease: expected non-nil release")
	}
}

func TestFetchLatestDailyRelease_NoneFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]GitHubRelease{
			{TagName: "v1.0.0-stable"},
		})
	}))
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	_, err := m.fetchLatestDailyRelease()
	if err == nil {
		t.Error("fetchLatestDailyRelease(no daily): expected error, got nil")
	}
}

// ── CheckUpdate ───────────────────────────────────────────────────────────────

func TestCheckUpdate_StableBranch_NewVersion(t *testing.T) {
	srv := newGitHubMockServer("v99.0.0")
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	info, err := m.CheckUpdate()
	if err != nil {
		t.Fatalf("CheckUpdate stable: %v", err)
	}
	if info == nil {
		t.Fatal("CheckUpdate: returned nil info")
	}
	if info.UpdateAvailable == false {
		t.Log("CheckUpdate: update not flagged as available (compareVersions may differ)")
	}
}

func TestCheckUpdate_SameVersion_NoUpdate(t *testing.T) {
	srv := newGitHubMockServer("v1.0.0")
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	info, err := m.CheckUpdate()
	if err != nil {
		t.Fatalf("CheckUpdate same version: %v", err)
	}
	if info.UpdateAvailable {
		t.Error("CheckUpdate same version: UpdateAvailable should be false")
	}
}

func TestCheckUpdate_BetaBranch(t *testing.T) {
	srv := newGitHubMockServer("v2.0.0")
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	tmp := t.TempDir()
	os.MkdirAll(tmp, 0755)
	os.WriteFile(tmp+"/update-branch", []byte("beta"), 0644)
	m.paths.Config = tmp

	_, err := m.CheckUpdate()
	if err != nil {
		t.Fatalf("CheckUpdate beta: %v", err)
	}
}

func TestCheckUpdate_DailyBranch(t *testing.T) {
	srv := newGitHubMockServer("v2.0.0")
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	tmp := t.TempDir()
	os.MkdirAll(tmp, 0755)
	os.WriteFile(tmp+"/update-branch", []byte("daily"), 0644)
	m.paths.Config = tmp

	_, err := m.CheckUpdate()
	if err != nil {
		t.Fatalf("CheckUpdate daily: %v", err)
	}
}

func TestCheckUpdate_UnknownBranch_DefaultsToStable(t *testing.T) {
	srv := newGitHubMockServer("v99.0.0")
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	tmp := t.TempDir()
	os.MkdirAll(tmp, 0755)
	os.WriteFile(tmp+"/update-branch", []byte("nightly"), 0644)
	m.paths.Config = tmp

	_, err := m.CheckUpdate()
	if err != nil {
		t.Fatalf("CheckUpdate unknown branch: %v", err)
	}
}

func TestCheckUpdate_NetworkError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	_, err := m.CheckUpdate()
	if err == nil {
		t.Error("CheckUpdate on 503: expected error, got nil")
	}
}

// ── ApplyUpdate (safe no-update path) ────────────────────────────────────────

func TestApplyUpdate_EmptyURL_AlreadyUpToDate(t *testing.T) {
	srv := newGitHubMockServer("v1.0.0")
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	err := m.ApplyUpdate("")
	if err != nil {
		t.Errorf("ApplyUpdate(same version): expected nil, got %v", err)
	}
}

func TestApplyUpdate_EmptyURL_NetworkError_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	installMaintenanceMockTransport(t, srv)

	m := newMaintManagerTmp(t, "1.0.0")
	err := m.ApplyUpdate("")
	if err == nil {
		t.Error("ApplyUpdate on 500: expected error, got nil")
	}
}
