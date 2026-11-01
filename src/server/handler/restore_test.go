// SPDX-License-Identifier: MIT
package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/maintenance"
)

func TestAPIRestoreRequestsRestartAfterSuccessfulRestore(t *testing.T) {
	base := t.TempDir()
	configDir := filepath.Join(base, "config")
	dataDir := filepath.Join(base, "data")
	cfg, configPath, err := config.LoadAppConfig(configDir, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Server.Token = "restore-test-token"
	if err := config.SaveAppConfig(cfg, configPath); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(base, "restore.tar.gz")
	backupManager := maintenance.NewMaintenanceManager(configDir, dataDir, "test")
	if err := backupManager.BackupWithOptions(maintenance.BackupOptions{
		Filename:    backupPath,
		IncludeData: true,
		MaxBackups:  1,
	}); err != nil {
		t.Fatalf("create test backup: %v", err)
	}

	backup, err := os.Open(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = backup.Close() }()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("backup", filepath.Base(backupPath))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(file, backup); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	h := NewServerHandler(cfg)
	h.SetConfigDir(configDir)
	h.SetDataDir(dataDir)
	restartRequested := false
	h.SetRestoreRestartHandler(func() { restartRequested = true })
	req := httptest.NewRequest(http.MethodPost, "/api/v1/server/restore", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	h.APIRestore(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if !restartRequested {
		t.Fatal("successful restore did not request a server restart")
	}
}

func TestAPIRestoreEncryptedUploadRequiresBackupPassword(t *testing.T) {
	cfg := config.DefaultAppConfig()
	h := NewServerHandler(cfg)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("backup", "server-backup.tar.gz.enc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("encrypted backup bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/server/restore", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	h.APIRestore(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["ok"] != false || response["error"] != "VALIDATION_FAILED" || response["message"] != "Encrypted backup requires password" {
		t.Fatalf("response = %#v, want PART 21 validation failure", response)
	}
}

func TestAPIRestoreRejectsBackupPasswordWithBoundaryWhitespace(t *testing.T) {
	cfg := config.DefaultAppConfig()
	h := NewServerHandler(cfg)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("backup", "server-backup.tar.gz.enc")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("encrypted backup bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("password", " secret "); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/server/restore", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	h.APIRestore(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "whitespace") || !strings.Contains(response.Body.String(), "backup restore failed") {
		t.Errorf("response should reject the password without exposing internal validation details: %s", response.Body.String())
	}
}

func TestAPIRestoreHonorsConfiguredArchiveSize(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Restore.MaxArchiveSize = "128"
	h := NewServerHandler(cfg)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("backup", "server-backup.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(bytes.Repeat([]byte("x"), 129)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/server/restore", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	h.APIRestore(response, req)
	if response.Code == http.StatusOK {
		t.Fatal("restore accepted an upload above max_archive_size")
	}
}

func TestRestoreOperatorTokenRejectsEmptyAndAcceptsConfiguredValue(t *testing.T) {
	cfg := config.DefaultAppConfig()
	cfg.Server.Token = "operator-secret"
	h := NewServerHandler(cfg)
	if h.validRestoreOperatorToken("") {
		t.Fatal("empty token accepted")
	}
	if !h.validRestoreOperatorToken("operator-secret") {
		t.Fatal("configured operator token rejected")
	}
	if h.validRestoreOperatorToken("wrong-secret") {
		t.Fatal("incorrect operator token accepted")
	}
}
