// SPDX-License-Identifier: MIT
package handler

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/config"
	paths "github.com/apimgr/vidveil/src/paths"
	"github.com/go-chi/chi/v5"
)

type cliVersionMetadata struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

type cliBinarySnapshot struct {
	Metadata cliVersionMetadata
	Size     int64
	ModTime  time.Time
}

var cliPlatforms = []string{
	"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64",
	"windows-amd64", "windows-arm64", "freebsd-amd64", "freebsd-arm64",
}

func (h *SearchHandler) cliBinaryDir() string {
	dataDir := h.dataDir
	if dataDir == "" {
		dataDir = config.GetAppPaths("", "").Data
	}
	dir := strings.TrimSpace(h.appConfig.CLI.BinaryDownload.Dir)
	if dir == "" {
		dir = filepath.Join(dataDir, "cli")
	} else {
		dir = strings.ReplaceAll(dir, "{data_dir}", dataDir)
	}
	return filepath.Clean(dir)
}

func cliAssetName(platform string) string {
	name := paths.ProjectName + "-cli-" + platform
	if strings.HasPrefix(platform, "windows-") {
		name += ".exe"
	}
	return name
}

func cliPlatformFromRouteName(filename string) (string, bool) {
	for _, platform := range cliPlatforms {
		if filename == cliAssetName(platform) {
			return platform, true
		}
	}
	return "", false
}

func cliDirectorySignature(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "missing", nil
		}
		return "", err
	}
	var signature strings.Builder
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&signature, "%s:%d:%d:%d|", entry.Name(), info.Size(), info.ModTime().UnixNano(), info.Mode())
	}
	return signature.String(), nil
}

func scanCLIVersionsInDirectory(dir string) map[string]cliBinarySnapshot {
	versions := make(map[string]cliBinarySnapshot)
	versionBytes, err := os.ReadFile(filepath.Join(dir, "version.txt"))
	if err != nil {
		return versions
	}
	versionName := strings.TrimSpace(string(versionBytes))
	if versionName == "" || strings.ContainsAny(versionName, " \t\r\n") {
		return versions
	}
	checksumsFile, err := os.Open(filepath.Join(dir, "sha256.txt"))
	if err != nil {
		return versions
	}
	defer func() { _ = checksumsFile.Close() }()
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(io.LimitReader(checksumsFile, 1<<20))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || len(fields[0]) != sha256.Size*2 {
			continue
		}
		decoded, err := hex.DecodeString(fields[0])
		if err != nil || len(decoded) != sha256.Size {
			continue
		}
		if _, ok := cliPlatformFromRouteName(filepath.Base(fields[1])); ok {
			checksums[filepath.Base(fields[1])] = strings.ToLower(fields[0])
		}
	}
	if scanner.Err() != nil {
		return versions
	}
	for _, platform := range cliPlatforms {
		filename := cliAssetName(platform)
		path := filepath.Join(dir, filename)
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		actual := hex.EncodeToString(hash.Sum(nil))
		if expected := checksums[filename]; expected != "" && actual == expected {
			versions[platform] = cliBinarySnapshot{
				Metadata: cliVersionMetadata{Version: versionName, SHA256: actual},
				Size:     info.Size(),
				ModTime:  info.ModTime(),
			}
		} else if expected != "" {
			log.Printf("WARN cli binary checksum mismatch: %s", filename)
		}
	}
	return versions
}

func cliVersionsInDirectory(dir string) map[string]cliVersionMetadata {
	snapshots := scanCLIVersionsInDirectory(dir)
	versions := make(map[string]cliVersionMetadata, len(snapshots))
	for platform, snapshot := range snapshots {
		versions[platform] = snapshot.Metadata
	}
	return versions
}

func (h *SearchHandler) refreshCLIVersions(force bool) {
	dir := h.cliBinaryDir()
	signature, err := cliDirectorySignature(dir)
	if err != nil {
		log.Printf("WARN cli binary directory could not be inspected: %v", err)
		h.cliVersionsMu.Lock()
		h.cliVersionsCache = make(map[string]cliBinarySnapshot)
		h.cliVersionsSig = ""
		h.cliVersionsMu.Unlock()
		return
	}
	h.cliVersionsMu.Lock()
	defer h.cliVersionsMu.Unlock()
	if !force && h.cliVersionsCache != nil && signature == h.cliVersionsSig {
		return
	}
	h.cliVersionsCache = scanCLIVersionsInDirectory(dir)
	h.cliVersionsSig = signature
}

func (h *SearchHandler) cliVersionSnapshot(platform string) (cliBinarySnapshot, bool) {
	h.refreshCLIVersions(false)
	h.cliVersionsMu.Lock()
	defer h.cliVersionsMu.Unlock()
	snapshot, ok := h.cliVersionsCache[platform]
	return snapshot, ok
}

func (h *SearchHandler) cliVersions() map[string]cliVersionMetadata {
	h.refreshCLIVersions(false)
	h.cliVersionsMu.Lock()
	defer h.cliVersionsMu.Unlock()
	versions := make(map[string]cliVersionMetadata, len(h.cliVersionsCache))
	for platform, snapshot := range h.cliVersionsCache {
		versions[platform] = snapshot.Metadata
	}
	return versions
}

// CLIBinaryDownload streams an operator-provisioned, checksum-verified CLI release file.
func (h *SearchHandler) CLIBinaryDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		SendNegotiatedError(w, r, "BAD_REQUEST", "GET is required")
		return
	}
	filename := chi.URLParam(r, "filename")
	if _, ok := cliPlatformFromRouteName(filename); !ok {
		SendNegotiatedError(w, r, "NOT_FOUND", "CLI binary not found")
		return
	}
	platform, _ := cliPlatformFromRouteName(filename)
	snapshot, ok := h.cliVersionSnapshot(platform)
	if !ok {
		SendNegotiatedError(w, r, "NOT_FOUND", "CLI binary is not available for this platform")
		return
	}
	path := filepath.Join(h.cliBinaryDir(), filename)
	file, err := os.Open(path)
	if err != nil {
		SendNegotiatedError(w, r, "NOT_FOUND", "CLI binary is not available for this platform")
		return
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		SendNegotiatedError(w, r, "NOT_FOUND", "CLI binary is not available for this platform")
		return
	}
	if info.Size() != snapshot.Size || !info.ModTime().Equal(snapshot.ModTime) {
		h.refreshCLIVersions(true)
		SendNegotiatedError(w, r, "NOT_FOUND", "CLI binary is not available for this platform")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	w.Header().Set("Content-Length", fmt.Sprint(info.Size()))
	if h.ownerAudit != nil {
		clientIP := getClientIP(r)
		h.ownerAudit.Audit("cli.update_started", "cli", "client", clientIP, "success", map[string]interface{}{
			"os_arch": platform, "version": snapshot.Metadata.Version, "client_ip": clientIP,
		})
	}
	http.ServeContent(w, r, filename, info.ModTime(), file)
}
