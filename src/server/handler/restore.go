// SPDX-License-Identifier: MIT
// AI.md PART 21: authenticated backup restore API and WebUI.
package handler

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/common/authpad"
	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/path"
	"github.com/apimgr/vidveil/src/server/service/maintenance"
)

var errRestorePasswordRequired = errors.New("encrypted backup requires password")

// RestorePage renders the one-off public restore form. The destructive POST
// requires an operator-token field and the global double-submit CSRF check.
func (h *ServerHandler) RestorePage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	h.renderServerTemplate(w, r, "server-restore", nil)
}

// RestoreSubmit handles the native multipart WebUI form. It returns the form
// with a password prompt when an encrypted archive lacks its separate password.
func (h *ServerHandler) RestoreSubmit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if err := h.limitRestoreRequestBody(w, r); err != nil {
		h.renderServerTemplate(w, r, "server-restore", map[string]interface{}{
			"HTTPStatus":   http.StatusRequestEntityTooLarge,
			"RestoreError": i18n.Translate(i18n.DetectLocale(r), "restore.failed"),
		})
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		status := http.StatusBadRequest
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			status = http.StatusRequestEntityTooLarge
		}
		h.renderServerTemplate(w, r, "server-restore", map[string]interface{}{
			"HTTPStatus":   status,
			"RestoreError": i18n.Translate(i18n.DetectLocale(r), "restore.failed"),
		})
		return
	}
	start := time.Now()
	if !h.validRestoreOperatorToken(r.FormValue("operator_token")) {
		authpad.Wait(start)
		h.renderServerTemplate(w, r, "server-restore", map[string]interface{}{
			"HTTPStatus":   http.StatusUnauthorized,
			"RestoreError": i18n.Translate(i18n.DetectLocale(r), "restore.unauthorized"),
		})
		return
	}
	if err := h.restoreUploadedBackup(r); err != nil {
		message := i18n.Translate(i18n.DetectLocale(r), "restore.failed")
		if errors.Is(err, errRestorePasswordRequired) {
			message = i18n.Translate(i18n.DetectLocale(r), "restore.password_required")
		}
		h.renderServerTemplate(w, r, "server-restore", map[string]interface{}{"HTTPStatus": http.StatusBadRequest, "RestoreError": message})
		return
	}
	h.renderServerTemplate(w, r, "server-restore", map[string]interface{}{"RestoreSuccess": i18n.Translate(i18n.DetectLocale(r), "restore.success")})
	h.restartAfterRestore()
}

// APIRestore handles POST /api/{api_version}/server/restore. The route is
// mounted behind requireOperatorToken; backup encryption uses its own password.
func (h *ServerHandler) APIRestore(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if err := h.limitRestoreRequestBody(w, r); err != nil {
		SendNegotiatedError(w, r, CodeValidation, "backup restore upload exceeds configured archive size")
		return
	}
	if err := h.restoreUploadedBackup(r); err != nil {
		if errors.Is(err, errRestorePasswordRequired) {
			SendError(w, CodeValidation, "Encrypted backup requires password")
			return
		}
		SendNegotiatedError(w, r, CodeValidation, "backup restore failed")
		return
	}
	SendOK(w, map[string]string{"message": "backup restored"})
	h.restartAfterRestore()
}

func (h *ServerHandler) validRestoreOperatorToken(provided string) bool {
	configured := strings.TrimSpace(h.appConfig.Server.Token)
	provided = strings.TrimSpace(provided)
	providedHash := sha256.Sum256([]byte(provided))
	configuredHash := sha256.Sum256([]byte(configured))
	return configured != "" && provided != "" && subtle.ConstantTimeCompare(providedHash[:], configuredHash[:]) == 1
}

func (h *ServerHandler) limitRestoreRequestBody(w http.ResponseWriter, r *http.Request) error {
	limits, err := maintenance.RestoreLimitsFromConfig(h.appConfig.Server.Restore)
	if err != nil {
		return err
	}
	const multipartOverhead = 1 << 20
	maxInt64 := uint64(^uint64(0) >> 1)
	if limits.MaxArchiveBytes > maxInt64-multipartOverhead {
		return fmt.Errorf("restore.max_archive_size leaves no room for multipart framing")
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(limits.MaxArchiveBytes+multipartOverhead))
	return nil
}

func (h *ServerHandler) restoreUploadedBackup(r *http.Request) (retErr error) {
	defer func() {
		if r.MultipartForm != nil {
			if err := r.MultipartForm.RemoveAll(); err != nil {
				retErr = errors.Join(retErr, fmt.Errorf("remove multipart temporary files: %w", err))
			}
		}
	}()
	file, header, err := r.FormFile("backup")
	if err != nil {
		return fmt.Errorf("backup file is required")
	}
	defer func() {
		if err := file.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close uploaded backup: %w", err))
		}
	}()

	password := r.FormValue("password")
	if password != strings.TrimSpace(password) {
		return maintenance.ErrBackupPasswordWhitespace
	}
	limits, err := maintenance.RestoreLimitsFromConfig(h.appConfig.Server.Restore)
	if err != nil {
		return err
	}
	if header.Size < 0 || uint64(header.Size) > limits.MaxArchiveBytes {
		return fmt.Errorf("backup archive exceeds configured maximum of %d bytes", limits.MaxArchiveBytes)
	}
	ext := ".tar.gz"
	if strings.HasSuffix(strings.ToLower(filepath.Base(header.Filename)), ".tar.gz.enc") {
		ext = ".tar.gz.enc"
		if password == "" {
			return errRestorePasswordRequired
		}
	} else if !strings.HasSuffix(strings.ToLower(filepath.Base(header.Filename)), ".tar.gz") {
		return fmt.Errorf("backup must be a .tar.gz or .tar.gz.enc archive")
	}

	tempBase := filepath.Join(os.TempDir(), path.ProjectOrg)
	if err := os.MkdirAll(tempBase, 0755); err != nil {
		return fmt.Errorf("prepare restore temporary directory: %w", err)
	}
	tempDir, err := os.MkdirTemp(tempBase, path.ProjectName+"-restore-")
	if err != nil {
		return fmt.Errorf("create restore temporary directory: %w", err)
	}
	if err := os.Chmod(tempDir, 0700); err != nil {
		return errors.Join(fmt.Errorf("protect restore temporary directory: %w", err), os.RemoveAll(tempDir))
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("remove restore temporary directory: %w", err))
		}
	}()

	backupFile := filepath.Join(tempDir, "uploaded-backup"+ext)
	out, err := os.OpenFile(backupFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create uploaded backup file: %w", err)
	}
	written, err := io.Copy(out, io.LimitReader(file, int64(limits.MaxArchiveBytes)+1))
	if err != nil {
		return errors.Join(fmt.Errorf("save uploaded backup: %w", err), out.Close())
	}
	if uint64(written) > limits.MaxArchiveBytes {
		return errors.Join(fmt.Errorf("backup archive exceeds configured maximum of %d bytes", limits.MaxArchiveBytes), out.Close())
	}
	if err := out.Sync(); err != nil {
		return errors.Join(fmt.Errorf("sync uploaded backup: %w", err), out.Close())
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close uploaded backup: %w", err)
	}

	manager := maintenance.NewMaintenanceManager(h.configDir, h.dataDir, version.GetVersion())
	manager.SetLogger(h.logger)
	manager.SetRestoreLimits(limits)
	if err := manager.RestoreWithPassword(backupFile, password); err != nil {
		return err
	}
	return nil
}
