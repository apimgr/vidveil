// SPDX-License-Identifier: MIT
package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/config"
	paths "github.com/apimgr/vidveil/src/paths"
	"github.com/go-chi/chi/v5"
)

const cliReleaseMetadataCacheTTL = 5 * time.Minute

var stableCLITagPattern = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+$`)

type cliReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type cliRelease struct {
	TagName    string            `json:"tag_name"`
	Name       string            `json:"name"`
	Body       string            `json:"body"`
	Prerelease bool              `json:"prerelease"`
	Assets     []cliReleaseAsset `json:"assets"`
}

type cliVersionMetadata struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

type cliReleaseMetadata struct {
	Versions map[string]cliVersionMetadata
	Assets   map[string]string
}

type cachedCLIReleaseMetadata struct {
	metadata  cliReleaseMetadata
	expiresAt time.Time
}

var cliReleaseMetadataCache = struct {
	sync.Mutex
	byChannel map[string]cachedCLIReleaseMetadata
}{byChannel: make(map[string]cachedCLIReleaseMetadata)}

var cliReleaseHTTPClient = &http.Client{Timeout: 30 * time.Second}
var cliReleaseMetadataFetcher = fetchCLIReleaseMetadata

// CLIBinaryDownload streams a published CLI release asset. The release and
// its checksum are selected from trusted project release metadata, never from
// a caller-supplied URL.
func (h *SearchHandler) CLIBinaryDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendNegotiatedError(w, r, "BAD_REQUEST", "GET is required")
		return
	}
	filename := chi.URLParam(r, "filename")
	platform, ok := cliPlatformFromRouteName(filename)
	if !ok {
		SendNegotiatedError(w, r, "NOT_FOUND", "CLI binary not found")
		return
	}
	channel := cliUpdateChannel(r.URL.Query().Get("channel"))
	if channel == "" {
		SendNegotiatedError(w, r, "BAD_REQUEST", "invalid update channel")
		return
	}
	metadata, err := getCLIReleaseMetadata(r.Context(), channel)
	if err != nil {
		SendNegotiatedError(w, r, "SERVER_ERROR", "CLI release metadata is unavailable")
		return
	}
	assetURL := metadata.Assets[platform]
	if assetURL == "" {
		SendNegotiatedError(w, r, "NOT_FOUND", "CLI binary is not available for this platform")
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, assetURL, nil)
	if err != nil {
		SendNegotiatedError(w, r, "SERVER_ERROR", "failed to prepare CLI download")
		return
	}
	request.Header.Set("User-Agent", "vidveil-cli-release-proxy")
	response, err := cliReleaseHTTPClient.Do(request)
	if err != nil {
		SendNegotiatedError(w, r, "SERVER_ERROR", "failed to download CLI binary")
		return
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusNotFound {
		return
	}
	if response.StatusCode != http.StatusOK {
		SendNegotiatedError(w, r, "SERVER_ERROR", "upstream CLI download failed")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	if response.ContentLength >= 0 {
		w.Header().Set("Content-Length", fmt.Sprint(response.ContentLength))
	}
	if _, err := io.Copy(w, response.Body); err != nil {
		return
	}
}

func cliUpdateChannel(value string) string {
	channel, valid := config.NormalizeUpdateChannel(value)
	if !valid {
		return ""
	}
	return channel
}

func cliPlatformFromRouteName(filename string) (string, bool) {
	for _, platform := range []string{
		"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64",
		"windows-amd64", "windows-arm64", "freebsd-amd64", "freebsd-arm64",
	} {
		if filename == paths.ProjectName+"-cli-"+platform {
			return platform, true
		}
	}
	return "", false
}

func getCLIReleaseMetadata(ctx context.Context, channel string) (cliReleaseMetadata, error) {
	if channel = cliUpdateChannel(channel); channel == "" {
		return cliReleaseMetadata{}, fmt.Errorf("invalid CLI update channel")
	}
	cliReleaseMetadataCache.Lock()
	defer cliReleaseMetadataCache.Unlock()
	if cached, ok := cliReleaseMetadataCache.byChannel[channel]; ok && time.Now().Before(cached.expiresAt) {
		return cached.metadata, nil
	}
	metadata, err := cliReleaseMetadataFetcher(ctx, channel)
	if err != nil {
		return cliReleaseMetadata{}, err
	}
	cliReleaseMetadataCache.byChannel[channel] = cachedCLIReleaseMetadata{
		metadata: metadata, expiresAt: time.Now().Add(cliReleaseMetadataCacheTTL),
	}
	return metadata, nil
}

func fetchCLIReleaseMetadata(ctx context.Context, channel string) (cliReleaseMetadata, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", paths.ProjectOrg, paths.ProjectName)
	if channel != "stable" {
		endpoint = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=100", paths.ProjectOrg, paths.ProjectName)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return cliReleaseMetadata{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "vidveil-cli-release-metadata")
	response, err := cliReleaseHTTPClient.Do(request)
	if err != nil {
		return cliReleaseMetadata{}, fmt.Errorf("fetching CLI release metadata: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusNotFound {
		return cliReleaseMetadata{
			Versions: map[string]cliVersionMetadata{},
			Assets:   map[string]string{},
		}, nil
	}
	if response.StatusCode != http.StatusOK {
		return cliReleaseMetadata{}, fmt.Errorf("GitHub release API returned status %d", response.StatusCode)
	}
	var releases []cliRelease
	if channel == "stable" {
		var release cliRelease
		if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
			return cliReleaseMetadata{}, fmt.Errorf("decoding stable CLI release: %w", err)
		}
		releases = []cliRelease{release}
	} else if err := json.NewDecoder(response.Body).Decode(&releases); err != nil {
		return cliReleaseMetadata{}, fmt.Errorf("decoding CLI releases: %w", err)
	}
	var selected *cliRelease
	for index := range releases {
		if cliReleaseMatchesChannel(releases[index], channel) {
			selected = &releases[index]
			break
		}
	}
	if selected == nil {
		return cliReleaseMetadata{
			Versions: map[string]cliVersionMetadata{},
			Assets:   map[string]string{},
		}, nil
	}
	assets := make(map[string]string)
	var checksumURL string
	for _, asset := range selected.Assets {
		if asset.Name == "sha256.txt" {
			checksumURL = asset.BrowserDownloadURL
		}
		if platform, ok := cliPlatformFromAssetName(asset.Name); ok {
			assets[platform] = asset.BrowserDownloadURL
		}
	}
	if checksumURL == "" {
		return cliReleaseMetadata{}, fmt.Errorf("CLI release has no sha256.txt asset")
	}
	checksums, err := fetchCLIReleaseChecksums(ctx, checksumURL)
	if err != nil {
		return cliReleaseMetadata{}, err
	}
	versions := make(map[string]cliVersionMetadata, len(assets))
	version, err := cliReleaseVersion(*selected)
	if err != nil {
		return cliReleaseMetadata{}, err
	}
	for platform := range assets {
		assetName := cliAssetName(platform)
		checksum := checksums[assetName]
		if checksum == "" {
			delete(assets, platform)
			continue
		}
		versions[platform] = cliVersionMetadata{Version: version, SHA256: checksum}
	}
	if len(versions) == 0 {
		return cliReleaseMetadata{}, fmt.Errorf("CLI release has no supported platform assets")
	}
	return cliReleaseMetadata{Versions: versions, Assets: assets}, nil
}

func cliReleaseVersion(release cliRelease) (string, error) {
	if release.TagName != "devel" {
		return strings.TrimPrefix(release.TagName, "v"), nil
	}
	for _, candidate := range []struct {
		text   string
		prefix string
	}{
		{text: release.Name, prefix: "Devel Build "},
		{text: release.Body, prefix: "Devel build: "},
		{text: release.Name, prefix: "Daily Build "},
		{text: release.Body, prefix: "Daily build: "},
	} {
		if !strings.HasPrefix(candidate.text, candidate.prefix) {
			continue
		}
		version := strings.TrimSpace(strings.TrimPrefix(candidate.text, candidate.prefix))
		if version != "" && !strings.ContainsAny(version, " \t\r\n") {
			return version, nil
		}
	}
	return "", fmt.Errorf("devel CLI release has no commit version in its release name or body")
}

func cliReleaseMatchesChannel(release cliRelease, channel string) bool {
	stable := stableCLITagPattern.MatchString(release.TagName)
	beta := strings.HasSuffix(release.TagName, "-beta")
	devel := release.TagName == "devel"
	switch channel {
	case "stable":
		return stable && !release.Prerelease
	case "beta":
		return stable || beta
	case "devel":
		return stable || beta || devel
	default:
		return false
	}
}

func cliAssetName(platform string) string {
	name := paths.ProjectName + "-cli-" + platform
	if strings.HasPrefix(platform, "windows-") {
		name += ".exe"
	}
	return name
}

func cliPlatformFromAssetName(assetName string) (string, bool) {
	for _, platform := range []string{
		"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64",
		"windows-amd64", "windows-arm64", "freebsd-amd64", "freebsd-arm64",
	} {
		if assetName == cliAssetName(platform) {
			return platform, true
		}
	}
	return "", false
}

func fetchCLIReleaseChecksums(ctx context.Context, checksumURL string) (map[string]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "vidveil-cli-release-metadata")
	response, err := cliReleaseHTTPClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetching CLI checksums: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CLI checksum download returned status %d", response.StatusCode)
	}
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(io.LimitReader(response.Body, 1<<20))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && len(fields[0]) == 64 {
			checksums[fields[1]] = strings.ToLower(fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading CLI checksums: %w", err)
	}
	return checksums, nil
}
