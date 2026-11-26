// SPDX-License-Identifier: MIT
package logging

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const zeroAuditHash = "0000000000000000000000000000000000000000000000000000000000000000"

type auditChainIssue struct {
	file   string
	line   int
	reason string
}

type auditLineHeader struct {
	Prev string `json:"prev"`
}

func auditLineHash(line []byte) string {
	hash := sha256.Sum256(line)
	return hex.EncodeToString(hash[:])
}

// AuditLogIntegrity reports the current audit file's verified chain status in
// the operator-facing compliance report without exposing entry contents.
func AuditLogIntegrity(logPath, anchorPath string) string {
	_, issue := inspectAuditChain(logPath, anchorPath)
	if issue != nil {
		return fmt.Sprintf("broken at %s:%d", issue.file, issue.line)
	}
	file, err := os.Open(logPath)
	if errors.Is(err, os.ErrNotExist) {
		return "intact (0 entries)"
	}
	if err != nil {
		return fmt.Sprintf("broken at %s:1", filepath.Base(logPath))
	}
	defer func() { _ = file.Close() }()
	entries := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	for scanner.Scan() {
		entries++
	}
	if scanner.Err() != nil {
		return fmt.Sprintf("broken at %s:%d", filepath.Base(logPath), entries+1)
	}
	return fmt.Sprintf("intact (%d entries)", entries)
}

func (l *AppLogger) initializeAuditChain() {
	last, issue := inspectAuditChain(l.auditPath, l.auditAnchor)
	l.auditPrevious = last
	l.auditIssue = issue
	l.auditGenesis = auditLogHasNoHistory(l.auditPath, l.auditAnchor)
	if output, ok := l.outputs["audit"].(*RotatingFile); ok {
		output.beforeRotate = l.prepareAuditRotation
	}
}

func (l *AppLogger) prepareAuditRotation() error {
	last, issue := inspectAuditChain(l.auditPath, l.auditAnchor)
	if issue != nil && !l.auditReportedBroken {
		l.auditIssue = issue
	}
	if last == "" {
		last = zeroAuditHash
	}
	if err := writeAuditAnchor(l.auditAnchor, last); err != nil {
		return err
	}
	l.auditPrevious = last
	return nil
}

func auditLogHasNoHistory(logPath, anchorPath string) bool {
	if _, err := os.Stat(anchorPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		return false
	}
	info, err := os.Stat(logPath)
	if err != nil || info.Size() != 0 {
		return false
	}
	rotated, err := filepath.Glob(logPath + ".*")
	return err == nil && len(rotated) == 0
}

func inspectAuditChain(logPath, anchorPath string) (string, *auditChainIssue) {
	anchor := ""
	if data, err := os.ReadFile(anchorPath); err == nil {
		anchor = strings.TrimSpace(string(data))
		decoded, decodeErr := hex.DecodeString(anchor)
		if decodeErr != nil || len(decoded) != sha256.Size || strings.ToLower(anchor) != anchor {
			return zeroAuditHash, &auditChainIssue{file: filepath.Base(anchorPath), line: 1, reason: "anchor_mismatch"}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return zeroAuditHash, &auditChainIssue{file: filepath.Base(anchorPath), line: 1, reason: "anchor_read_failed"}
	}

	file, err := os.Open(logPath)
	if errors.Is(err, os.ErrNotExist) {
		if anchor != "" {
			return anchor, nil
		}
		return zeroAuditHash, nil
	}
	if err != nil {
		return anchorOrZero(anchor), &auditChainIssue{file: filepath.Base(logPath), line: 1, reason: "read_failed"}
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	previousLineHash := ""
	lineNumber := 0
	var issue *auditChainIssue
	for scanner.Scan() {
		lineNumber++
		line := append([]byte(nil), scanner.Bytes()...)
		var header auditLineHeader
		if err := json.Unmarshal(line, &header); err != nil {
			if issue == nil {
				issue = &auditChainIssue{file: filepath.Base(logPath), line: lineNumber, reason: "invalid_entry"}
			}
		} else {
			wantPrev := previousLineHash
			if lineNumber == 1 {
				wantPrev = anchorOrZero(anchor)
				if anchor == "" && header.Prev != zeroAuditHash && issue == nil {
					issue = &auditChainIssue{file: filepath.Base(logPath), line: lineNumber, reason: "anchor_missing"}
				}
				if anchor != "" && header.Prev != anchor && issue == nil {
					issue = &auditChainIssue{file: filepath.Base(logPath), line: lineNumber, reason: "anchor_mismatch"}
				}
			}
			if header.Prev != wantPrev && issue == nil {
				issue = &auditChainIssue{file: filepath.Base(logPath), line: lineNumber, reason: "chain_mismatch"}
			}
		}
		previousLineHash = auditLineHash(line)
	}
	if err := scanner.Err(); err != nil && issue == nil {
		issue = &auditChainIssue{file: filepath.Base(logPath), line: lineNumber + 1, reason: "read_failed"}
	}
	if previousLineHash == "" {
		previousLineHash = anchorOrZero(anchor)
	}
	return previousLineHash, issue
}

func anchorOrZero(anchor string) string {
	if anchor == "" {
		return zeroAuditHash
	}
	return anchor
}

func writeAuditAnchor(path, hash string) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".audit-anchor-*")
	if err != nil {
		return fmt.Errorf("create audit anchor: %w", err)
	}
	tempPath := file.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return fmt.Errorf("protect audit anchor: %w", err)
	}
	if _, err := io.WriteString(file, hash+"\n"); err != nil {
		_ = file.Close()
		return fmt.Errorf("write audit anchor: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync audit anchor: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close audit anchor: %w", err)
	}
	if err := replaceAuditAnchor(tempPath, path); err != nil {
		return fmt.Errorf("install audit anchor: %w", err)
	}
	if err := syncAuditDirectory(dir); err != nil {
		return fmt.Errorf("sync audit directory: %w", err)
	}
	return nil
}
