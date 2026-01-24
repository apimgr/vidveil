// SPDX-License-Identifier: MIT
// AI.md PART 11: Security & Logging with Built-in Rotation
package logging

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/config"
)

// RotationInterval represents time-based rotation intervals
type RotationInterval int

const (
	RotationNone RotationInterval = iota
	RotationHourly
	RotationDaily
	RotationWeekly
	RotationMonthly
)

// RotatingFile implements io.Writer with automatic rotation
// AI.md PART 11: Built-in rotation support (no external logrotate needed)
type RotatingFile struct {
	mu   sync.Mutex
	path string
	file *os.File
	// Max size in bytes before rotation
	maxSize int64
	// Time-based rotation interval
	interval RotationInterval
	// Whether to gzip rotated files
	compress bool
	// Current file size
	currentSize int64
	// Last rotation time
	lastRotation time.Time
	// Number of rotated files to keep (0 = delete immediately)
	keepCount int
}

// RotationConfig holds rotation settings per PART 11
type RotationConfig struct {
	// e.g., "50MB", "100KB"
	MaxSize string
	// e.g., "daily", "weekly", "hourly"
	Interval string
	// Whether to gzip rotated files
	Compress bool
	// Number of rotated files to keep (0 = delete immediately)
	Keep int
}

// NewRotatingFile creates a new rotating file writer
func NewRotatingFile(path string, cfg RotationConfig) (*RotatingFile, error) {
	rf := &RotatingFile{
		path:         path,
		compress:     cfg.Compress,
		keepCount:    cfg.Keep,
		lastRotation: time.Now(),
	}

	// Parse max size (e.g., "50MB", "100KB")
	rf.maxSize = parseSize(cfg.MaxSize)
	// Default 50MB
	if rf.maxSize == 0 {
		rf.maxSize = 50 * 1024 * 1024
	}

	// Parse interval
	rf.interval = parseInterval(cfg.Interval)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// Open file
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	rf.file = f

	// Get current file size
	if info, err := f.Stat(); err == nil {
		rf.currentSize = info.Size()
	}

	return rf, nil
}

// parseSize parses size string like "50MB", "100KB" to bytes
func parseSize(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0
	}

	var multiplier int64 = 1
	if strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	} else if strings.HasSuffix(s, "KB") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	} else if strings.HasSuffix(s, "B") {
		s = strings.TrimSuffix(s, "B")
	}

	val, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return val * multiplier
}

// parseInterval parses interval string to RotationInterval
func parseInterval(s string) RotationInterval {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "hourly":
		return RotationHourly
	case "daily":
		return RotationDaily
	case "weekly":
		return RotationWeekly
	case "monthly":
		return RotationMonthly
	default:
		return RotationNone
	}
}

// Write implements io.Writer with automatic rotation check
func (rf *RotatingFile) Write(p []byte) (n int, err error) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// Check if rotation is needed (size OR time, whichever comes first per PART 11)
	if rf.needsRotation() {
		if err := rf.rotate(); err != nil {
			// Log rotation error but continue writing
			fmt.Fprintf(os.Stderr, "log rotation error: %v\n", err)
		}
	}

	n, err = rf.file.Write(p)
	rf.currentSize += int64(n)
	return n, err
}

// needsRotation checks if rotation is needed based on size or time
func (rf *RotatingFile) needsRotation() bool {
	// Size-based check
	if rf.maxSize > 0 && rf.currentSize >= rf.maxSize {
		return true
	}

	// Time-based check
	now := time.Now()
	switch rf.interval {
	case RotationHourly:
		return now.Hour() != rf.lastRotation.Hour() || now.Day() != rf.lastRotation.Day()
	case RotationDaily:
		return now.Day() != rf.lastRotation.Day() || now.Month() != rf.lastRotation.Month()
	case RotationWeekly:
		_, lastWeek := rf.lastRotation.ISOWeek()
		_, thisWeek := now.ISOWeek()
		return thisWeek != lastWeek || now.Year() != rf.lastRotation.Year()
	case RotationMonthly:
		return now.Month() != rf.lastRotation.Month() || now.Year() != rf.lastRotation.Year()
	}

	return false
}

// rotate performs log rotation
func (rf *RotatingFile) rotate() error {
	// Close current file
	if rf.file != nil {
		rf.file.Close()
	}

	// Generate rotated filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := rf.path + "." + timestamp

	// Rename current log to rotated name
	if err := os.Rename(rf.path, rotatedPath); err != nil {
		// If rename fails, try to reopen original
		f, _ := os.OpenFile(rf.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		rf.file = f
		return err
	}

	// Compress if enabled
	if rf.compress {
		go rf.compressFile(rotatedPath)
	} else if rf.keepCount == 0 {
		// Delete immediately if not keeping rotated files (PART 11 default)
		go os.Remove(rotatedPath)
	}

	// Clean up old rotated files if keepCount > 0
	if rf.keepCount > 0 {
		go rf.cleanupOldFiles()
	}

	// Open new file
	f, err := os.OpenFile(rf.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	rf.file = f
	rf.currentSize = 0
	rf.lastRotation = time.Now()
	return nil
}

// compressFile compresses a rotated log file
func (rf *RotatingFile) compressFile(path string) {
	// Open source file
	src, err := os.Open(path)
	if err != nil {
		return
	}
	defer src.Close()

	// Create gzip file
	dst, err := os.Create(path + ".gz")
	if err != nil {
		return
	}
	defer dst.Close()

	// Write compressed data
	gz := gzip.NewWriter(dst)
	if _, err := io.Copy(gz, src); err != nil {
		gz.Close()
		os.Remove(path + ".gz")
		return
	}
	gz.Close()

	// Remove original after successful compression
	os.Remove(path)

	// If not keeping files, remove compressed too (PART 11 default)
	if rf.keepCount == 0 {
		os.Remove(path + ".gz")
	}
}

// cleanupOldFiles removes rotated files beyond keepCount
func (rf *RotatingFile) cleanupOldFiles() {
	dir := filepath.Dir(rf.path)
	base := filepath.Base(rf.path)
	pattern := base + ".*"

	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return
	}

	// Sort by modification time (oldest first)
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var files []fileInfo
	for _, match := range matches {
		if info, err := os.Stat(match); err == nil {
			files = append(files, fileInfo{path: match, modTime: info.ModTime()})
		}
	}

	// Sort by modification time
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].modTime.After(files[j].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	// Remove files beyond keepCount
	if len(files) > rf.keepCount {
		for i := 0; i < len(files)-rf.keepCount; i++ {
			os.Remove(files[i].path)
		}
	}
}

// Close closes the rotating file
func (rf *RotatingFile) Close() error {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.file != nil {
		return rf.file.Close()
	}
	return nil
}

// MaskEmail masks an email address per AI.md PART 11
// "user@example.com" -> "u***@e***.com"
func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}

	user := parts[0]
	domain := parts[1]

	// Mask user part: keep first char
	maskedUser := string(user[0]) + "***"

	// Mask domain: keep first char and TLD
	domainParts := strings.Split(domain, ".")
	if len(domainParts) >= 2 {
		maskedDomain := string(domainParts[0][0]) + "***." + domainParts[len(domainParts)-1]
		return maskedUser + "@" + maskedDomain
	}
	return maskedUser + "@***"
}

// MaskUsername masks a username per AI.md PART 11
// "johndoe" -> "joh***"
func MaskUsername(username string) string {
	if username == "" {
		return ""
	}
	if len(username) <= 3 {
		return username[:1] + "***"
	}
	return username[:3] + "***"
}

// MaskIP masks an IP address per AI.md PART 11
// "192.168.1.100" -> "192.168.xxx.xxx"
func MaskIP(ip string) string {
	if ip == "" {
		return ""
	}
	// Handle IPv4
	if parts := strings.Split(ip, "."); len(parts) == 4 {
		return parts[0] + "." + parts[1] + ".xxx.xxx"
	}
	// Handle IPv6 (simplify)
	if strings.Contains(ip, ":") {
		parts := strings.Split(ip, ":")
		if len(parts) >= 4 {
			return parts[0] + ":" + parts[1] + ":xxxx:xxxx:..."
		}
	}
	return ip[:len(ip)/2] + "***"
}

// SanitizeLogFields masks sensitive fields in log data per AI.md PART 11
func SanitizeLogFields(fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return nil
	}

	sanitized := make(map[string]interface{})
	for k, v := range fields {
		switch strings.ToLower(k) {
		case "email", "user_email", "admin_email":
			if s, ok := v.(string); ok {
				sanitized[k] = MaskEmail(s)
			} else {
				sanitized[k] = "***"
			}
		case "username", "user", "admin", "admin_username":
			if s, ok := v.(string); ok {
				sanitized[k] = MaskUsername(s)
			} else {
				sanitized[k] = "***"
			}
		case "password", "secret", "token", "api_key", "apikey":
			sanitized[k] = "[REDACTED]"
		case "ip", "remote_addr", "client_ip":
			if s, ok := v.(string); ok {
				sanitized[k] = MaskIP(s)
			} else {
				sanitized[k] = "***"
			}
		default:
			sanitized[k] = v
		}
	}
	return sanitized
}

// Level represents log severity
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// AppLogger handles structured logging
type AppLogger struct {
	mu        sync.Mutex
	level     Level
	outputs   map[string]io.Writer
	appConfig *config.AppConfig
}

// NewAppLogger creates a new logger
func NewAppLogger(appConfig *config.AppConfig) (*AppLogger, error) {
	l := &AppLogger{
		outputs:   make(map[string]io.Writer),
		appConfig: appConfig,
	}

	// Parse log level
	switch appConfig.Server.Logs.Level {
	case "debug":
		l.level = LevelDebug
	case "info":
		l.level = LevelInfo
	case "warn":
		l.level = LevelWarn
	case "error":
		l.level = LevelError
	default:
		l.level = LevelInfo
	}

	// Setup debug log with rotation per PART 11
	if appConfig.Server.Logs.Debug.Enabled && appConfig.Server.Logs.Debug.Filename != "" {
		keep := parseKeepString(appConfig.Server.Logs.Debug.Keep)
		if err := l.addFileOutput("debug", appConfig.Server.Logs.Debug.Filename, appConfig.Server.Logs.Debug.Rotate, keep); err != nil {
			return nil, fmt.Errorf("failed to open debug log: %w", err)
		}
	}

	// Setup access log with rotation per PART 11
	if appConfig.Server.Logs.Access.Enabled && appConfig.Server.Logs.Access.Filename != "" {
		keep := parseKeepString(appConfig.Server.Logs.Access.Keep)
		if err := l.addFileOutput("access", appConfig.Server.Logs.Access.Filename, appConfig.Server.Logs.Access.Rotate, keep); err != nil {
			return nil, fmt.Errorf("failed to open access log: %w", err)
		}
	}

	// Setup server log with rotation per PART 11
	if appConfig.Server.Logs.Server.Enabled && appConfig.Server.Logs.Server.Filename != "" {
		keep := parseKeepString(appConfig.Server.Logs.Server.Keep)
		if err := l.addFileOutput("server", appConfig.Server.Logs.Server.Filename, appConfig.Server.Logs.Server.Rotate, keep); err != nil {
			return nil, fmt.Errorf("failed to open server log: %w", err)
		}
	}

	// Setup error log with rotation per PART 11
	if appConfig.Server.Logs.Error.Enabled && appConfig.Server.Logs.Error.Filename != "" {
		keep := parseKeepString(appConfig.Server.Logs.Error.Keep)
		if err := l.addFileOutput("error", appConfig.Server.Logs.Error.Filename, appConfig.Server.Logs.Error.Rotate, keep); err != nil {
			return nil, fmt.Errorf("failed to open error log: %w", err)
		}
	}

	// Setup audit log with rotation per PART 11
	if appConfig.Server.Logs.Audit.Enabled && appConfig.Server.Logs.Audit.Filename != "" {
		keep := parseKeepString(appConfig.Server.Logs.Audit.Keep)
		if err := l.addFileOutput("audit", appConfig.Server.Logs.Audit.Filename, appConfig.Server.Logs.Audit.Rotate, keep); err != nil {
			return nil, fmt.Errorf("failed to open audit log: %w", err)
		}
	}

	// Setup security log with rotation per PART 11
	if appConfig.Server.Logs.Security.Enabled && appConfig.Server.Logs.Security.Filename != "" {
		keep := parseKeepString(appConfig.Server.Logs.Security.Keep)
		if err := l.addFileOutput("security", appConfig.Server.Logs.Security.Filename, appConfig.Server.Logs.Security.Rotate, keep); err != nil {
			return nil, fmt.Errorf("failed to open security log: %w", err)
		}
	}

	return l, nil
}

// addFileOutput adds a rotating file output per PART 11
func (l *AppLogger) addFileOutput(name, path, rotate string, keep int) error {
	// Parse rotation config from string like "weekly,50MB" or "daily" or "100MB"
	rotCfg := parseRotationString(rotate)
	rotCfg.Keep = keep

	// Create rotating file
	rf, err := NewRotatingFile(path, rotCfg)
	if err != nil {
		return err
	}

	l.outputs[name] = rf
	return nil
}

// parseRotationString parses rotation string like "weekly,50MB" per PART 11
// Supports: "weekly,50MB" = rotate on weekly OR 50MB, whichever comes first
func parseRotationString(s string) RotationConfig {
	// Default per PART 11
	cfg := RotationConfig{
		MaxSize:  "50MB",
		Interval: "",
		Compress: false,
		// Delete immediately after rotation (default per PART 11)
		Keep: 0,
	}

	if s == "" {
		return cfg
	}

	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		partUpper := strings.ToUpper(part)

		// Check if it's a size (has MB, KB, GB suffix)
		if strings.HasSuffix(partUpper, "MB") || strings.HasSuffix(partUpper, "KB") ||
			strings.HasSuffix(partUpper, "GB") || strings.HasSuffix(partUpper, "B") {
			cfg.MaxSize = part
		} else if strings.ToLower(part) == "compress" || strings.ToLower(part) == "gzip" {
			cfg.Compress = true
		} else {
			// Must be an interval (hourly, daily, weekly, monthly)
			cfg.Interval = part
		}
	}

	return cfg
}

// parseKeepString parses keep string to number of files to keep
func parseKeepString(s string) int {
	// Delete immediately (default)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// Close closes all log files
func (l *AppLogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, w := range l.outputs {
		if rf, ok := w.(*RotatingFile); ok {
			rf.Close()
		} else if f, ok := w.(*os.File); ok {
			f.Close()
		}
	}
}

// Reopen reopens all log files (called on SIGUSR1 for log rotation per AI.md PART 8)
func (l *AppLogger) Reopen() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for name, w := range l.outputs {
		if rf, ok := w.(*RotatingFile); ok {
			rf.Reopen()
		}
		// Suppress unused variable (used for future logging)
		_ = name
	}
}

// Reopen closes and reopens the log file (for SIGUSR1 log rotation per AI.md PART 8)
func (rf *RotatingFile) Reopen() error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// Close current file
	if rf.file != nil {
		rf.file.Close()
	}

	// Reopen file
	f, err := os.OpenFile(rf.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	rf.file = f
	rf.currentSize = 0
	if info, err := f.Stat(); err == nil {
		rf.currentSize = info.Size()
	}

	return nil
}

// log writes a log entry
func (l *AppLogger) log(level Level, output string, message string, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level.String(),
		Message:   message,
		Fields:    fields,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if w, ok := l.outputs[output]; ok {
		w.Write(data)
		w.Write([]byte("\n"))
	}
}

// Debug logs a debug message
func (l *AppLogger) Debug(message string, fields map[string]interface{}) {
	l.log(LevelDebug, "debug", message, fields)
}

// Info logs an info message
func (l *AppLogger) Info(message string, fields map[string]interface{}) {
	l.log(LevelInfo, "server", message, fields)
}

// Warn logs a warning message
func (l *AppLogger) Warn(message string, fields map[string]interface{}) {
	l.log(LevelWarn, "server", message, fields)
}

// Error logs an error message
func (l *AppLogger) Error(message string, fields map[string]interface{}) {
	l.log(LevelError, "server", message, fields)
}

// Access logs an access log entry
func (l *AppLogger) Access(method, path, remoteAddr, userAgent string, status int, duration time.Duration) {
	l.log(LevelInfo, "access", "HTTP request", map[string]interface{}{
		"method":      method,
		"path":        path,
		"remote_addr": remoteAddr,
		"user_agent":  userAgent,
		"status":      status,
		"duration_ms": duration.Milliseconds(),
	})
}

// Audit logs an audit event with automatic PII masking per AI.md PART 11
func (l *AppLogger) Audit(action, user, resource string, details map[string]interface{}) {
	fields := map[string]interface{}{
		"action":   action,
		"user":     MaskUsername(user),
		"resource": resource,
	}
	for k, v := range details {
		fields[k] = v
	}
	// Sanitize sensitive fields before logging
	l.log(LevelInfo, "audit", "Audit event", SanitizeLogFields(fields))
}

// Security logs a security event with automatic PII masking per AI.md PART 11
func (l *AppLogger) Security(event, remoteAddr string, details map[string]interface{}) {
	fields := map[string]interface{}{
		"event":       event,
		"remote_addr": MaskIP(remoteAddr),
	}
	for k, v := range details {
		fields[k] = v
	}
	// Sanitize sensitive fields before logging
	l.log(LevelWarn, "security", "Security event", SanitizeLogFields(fields))
}

// AccessLogMiddleware creates middleware for access logging
type AccessLogMiddleware struct {
	logger *AppLogger
}

// NewAccessLogMiddleware creates access log middleware
func NewAccessLogMiddleware(logger *AppLogger) *AccessLogMiddleware {
	return &AccessLogMiddleware{logger: logger}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Ensure we implement http.Hijacker if the underlying ResponseWriter does
func (rw *responseWriter) Hijack() (interface{}, interface{}, error) {
	if h, ok := rw.ResponseWriter.(interface{ Hijack() (interface{}, interface{}, error) }); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("hijack not supported")
}

// Handler wraps an http.Handler with access logging
func (m *AccessLogMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		m.logger.Access(
			r.Method,
			r.URL.Path,
			r.RemoteAddr,
			r.UserAgent(),
			wrapped.status,
			time.Since(start),
		)
	})
}
