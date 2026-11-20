// SPDX-License-Identifier: MIT
// AI.md PART 16: Contact form antispam layers.
package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/apimgr/vidveil/src/server/service/metric"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

const (
	contactBodyLimit       = 256 << 10
	contactStateLimit      = 10000
	contactNonceCacheLimit = 50000
)

var (
	contactLinkPattern   = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://[^\s<>()]+|www\.[a-z0-9.-]+|\b(?:[a-z0-9-]+\.)+[a-z]{2,}\b)`)
	contactEmailPattern  = regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
	contactMarkupPattern = regexp.MustCompile(`(?is)<\s*/?\s*[a-z!][^>\s]*(?:[^>]*)>?|\[(?:/?(?:url|img|b)|/)|&(?:lt|gt|#0*60|#0*62);`)
	contactReplayCache   = struct {
		sync.Mutex
		used map[string]time.Time
	}{used: make(map[string]time.Time)}
)

type contactWindow struct {
	start time.Time
	count int
}
type contactDedup struct {
	digest string
	seen   time.Time
}

func (h *ServerHandler) contactRateError(r *http.Request) string {
	return h.checkContactRate(r, true)
}

func (h *ServerHandler) checkContactRate(r *http.Request, count bool) string {
	cfg := h.appConfig.Server.Pages.Contact.Antispam
	window, err := time.ParseDuration(cfg.Window)
	if err != nil || window <= 0 {
		window = time.Hour
	}
	ip := urlvar.ResolveClientIP(r)
	isOverlay := urlvar.GlobalResolver().IsOverlayRequest(r)
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	now := time.Now()
	h.contactMu.Lock()
	defer h.contactMu.Unlock()
	if h.contactCounts == nil {
		h.contactCounts = make(map[string]contactWindow)
	}
	if h.contactBans == nil {
		h.contactBans = make(map[string]time.Time)
	}
	if !isOverlay {
		if until := h.contactBans[ip]; until.After(now) {
			return "banned"
		} else if !until.IsZero() {
			delete(h.contactBans, ip)
		}
	}
	for key, current := range h.contactCounts {
		if now.Sub(current.start) >= window {
			delete(h.contactCounts, key)
		}
	}
	for ip, until := range h.contactBans {
		if !until.After(now) {
			delete(h.contactBans, ip)
		}
	}
	limits := []struct {
		key    string
		max    int
		reason string
	}{{"ip:" + ip, cfg.PerIP, "rate_ip"}, {"email:" + email, cfg.PerEmail, "rate_email"}, {"global", cfg.Global, "rate_global"}}
	for _, limit := range limits {
		if limit.reason == "rate_ip" && isOverlay {
			continue
		}
		if limit.max <= 0 {
			continue
		}
		current := h.contactCounts[limit.key]
		if current.count >= limit.max {
			return limit.reason
		}
	}
	for _, limit := range limits {
		if !count {
			break
		}
		if limit.max <= 0 || (limit.reason == "rate_ip" && isOverlay) {
			continue
		}
		current := h.contactCounts[limit.key]
		if current.start.IsZero() || now.Sub(current.start) >= window {
			if current.start.IsZero() && len(h.contactCounts) >= contactStateLimit {
				evictOldestContactWindow(h.contactCounts)
			}
			current = contactWindow{start: now}
		}
		current.count++
		h.contactCounts[limit.key] = current
	}
	return ""
}

func (h *ServerHandler) releaseContactRate(r *http.Request) {
	ip := urlvar.ResolveClientIP(r)
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	isOverlay := urlvar.GlobalResolver().IsOverlayRequest(r)
	h.contactMu.Lock()
	defer h.contactMu.Unlock()
	for _, key := range []string{"ip:" + ip, "email:" + email, "global"} {
		if key == "ip:"+ip && isOverlay {
			continue
		}
		current := h.contactCounts[key]
		if current.count > 1 {
			current.count--
		} else {
			delete(h.contactCounts, key)
		}
	}
}

func (h *ServerHandler) contactIsBanned(r *http.Request) bool {
	if urlvar.GlobalResolver().IsOverlayRequest(r) {
		return false
	}
	ip := urlvar.ResolveClientIP(r)
	now := time.Now()
	h.contactMu.Lock()
	defer h.contactMu.Unlock()
	until := h.contactBans[ip]
	if until.After(now) {
		return true
	}
	if !until.IsZero() {
		delete(h.contactBans, ip)
	}
	return false
}

func (h *ServerHandler) duplicateContact(r *http.Request) bool {
	cfg := h.appConfig.Server.Pages.Contact.Antispam
	if cfg.DuplicateWindowHours <= 0 {
		return false
	}
	input := normalizeContact(r.FormValue("email")) + "\x00" + normalizeContact(r.FormValue("subject")) + "\x00" + normalizeContact(r.FormValue("message"))
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
	now := time.Now()
	h.contactMu.Lock()
	defer h.contactMu.Unlock()
	for _, item := range h.contactSeen {
		if item.digest == digest && now.Sub(item.seen) <= time.Duration(cfg.DuplicateWindowHours)*time.Hour {
			return true
		}
	}
	h.contactSeen = append(h.contactSeen, contactDedup{digest: digest, seen: now})
	if len(h.contactSeen) > 10000 {
		h.contactSeen = append([]contactDedup(nil), h.contactSeen[len(h.contactSeen)-10000:]...)
	}
	return false
}

func (h *ServerHandler) forgetContactDuplicate(r *http.Request) {
	input := normalizeContact(r.FormValue("email")) + "\x00" + normalizeContact(r.FormValue("subject")) + "\x00" + normalizeContact(r.FormValue("message"))
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(input)))
	h.contactMu.Lock()
	defer h.contactMu.Unlock()
	for i := len(h.contactSeen) - 1; i >= 0; i-- {
		if h.contactSeen[i].digest == digest {
			h.contactSeen = append(h.contactSeen[:i], h.contactSeen[i+1:]...)
			return
		}
	}
}

func (h *ServerHandler) contactFormFailure(w http.ResponseWriter, r *http.Request, reason string, api bool) {
	if h.contactBlock(r, reason) {
		reason = "banned"
	}
	status429 := reason == "rate_ip" || reason == "rate_email" || reason == "rate_global" || reason == "banned"
	if status429 {
		w.Header().Set("Retry-After", "3600")
	}
	if api {
		if reason == "fill_time" {
			WriteJSON(w, http.StatusBadRequest, APIResponse{OK: false, Error: CodeValidation, Message: "Please wait a moment and submit again", Details: map[string]interface{}{"field": "form_token", "form_token": h.issueContactFormToken(r)}})
		} else if reason == "captcha" {
			WriteJSON(w, http.StatusBadRequest, APIResponse{OK: false, Error: CodeValidation, Message: "Captcha is invalid or expired", Details: map[string]interface{}{"field": "captcha", "captcha": h.apiCaptchaDescriptor(r)}})
		} else if status429 {
			SendError(w, CodeRateLimited, "Too many submissions. Please try again later.")
		} else {
			SendError(w, CodeValidation, "Your message could not be sent")
		}
		return
	}
	if status429 {
		http.Error(w, "Too many submissions. Please try again later.", http.StatusTooManyRequests)
	} else if reason == "fill_time" {
		w = withResponseStatus(w, http.StatusBadRequest)
		h.renderContactFailure(w, r, "Please wait a moment and submit again")
	} else if reason == "captcha" {
		w = withResponseStatus(w, http.StatusBadRequest)
		h.renderContactFailure(w, r, "Please complete the captcha and try again.")
	} else {
		w = withResponseStatus(w, http.StatusBadRequest)
		h.renderContactFailure(w, r, "Your message could not be sent")
	}
}

func writeContactBanned(w http.ResponseWriter, api bool) {
	w.Header().Set("Retry-After", "3600")
	if api {
		SendError(w, CodeRateLimited, "Too many submissions. Please try again later.")
		return
	}
	http.Error(w, "Too many submissions. Please try again later.", http.StatusTooManyRequests)
}

func (h *ServerHandler) issueContactFormToken(r *http.Request) string {
	if h.secretsMgr == nil {
		return ""
	}
	secret, err := h.secretsMgr.GetInstallationSecret(r.Context())
	if err != nil || len(secret) == 0 {
		return ""
	}
	issued := time.Now().Unix()
	nonce, err := newContactNonce()
	if err != nil {
		return ""
	}
	message := fmt.Sprintf("contact-form|%d|%s", issued, nonce)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(message))
	return fmt.Sprintf("%d.%s.%s", issued, nonce, hex.EncodeToString(mac.Sum(nil)))
}

func (h *ServerHandler) validContactFormToken(r *http.Request) bool {
	return h.contactFormTokenFailure(r) == ""
}

func (h *ServerHandler) contactFormTokenFailure(r *http.Request) string {
	parts := strings.Split(r.FormValue("form_token"), ".")
	if len(parts) != 3 || len(parts[2]) != 64 || !validContactNonce(parts[1]) {
		return "fill_time"
	}
	issued, err := strconvParseInt(parts[0])
	if err != nil || h.secretsMgr == nil {
		return "fill_time"
	}
	secret, err := h.secretsMgr.GetInstallationSecret(r.Context())
	if err != nil || len(secret) == 0 {
		return "fill_time"
	}
	provided, err := hex.DecodeString(parts[2])
	if err != nil {
		return "fill_time"
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(fmt.Sprintf("contact-form|%d|%s", issued, parts[1])))
	now := time.Now()
	age := now.Unix() - issued
	cfg := h.appConfig.Server.Pages.Contact.Antispam
	if !hmac.Equal(provided, mac.Sum(nil)) || cfg.MinFillSeconds > 0 && age < int64(cfg.MinFillSeconds) || cfg.MaxAgeSeconds > 0 && age > int64(cfg.MaxAgeSeconds) {
		return "fill_time"
	}
	expires := time.Unix(issued, 0).Add(time.Duration(cfg.MaxAgeSeconds) * time.Second)
	if cfg.MaxAgeSeconds <= 0 {
		expires = now.Add(time.Hour)
	}
	if !useContactNonce("form:"+parts[1], expires, now) {
		return "replay"
	}
	return ""
}

func (h *ServerHandler) issueContactReviewToken(r *http.Request, flags string) string {
	if h.secretsMgr == nil || flags != "" && flags != "d" {
		return ""
	}
	secret, err := h.secretsMgr.GetInstallationSecret(r.Context())
	if err != nil || len(secret) == 0 {
		return ""
	}
	issued := time.Now().Unix()
	nonce, err := newContactNonce()
	if err != nil {
		return ""
	}
	fieldHash := contactReviewFieldHash(r)
	message := fmt.Sprintf("contact-review|%d|%s|%s|%s", issued, nonce, flags, fieldHash)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(message))
	return fmt.Sprintf("%d.%s.%s.%s", issued, nonce, flags, hex.EncodeToString(mac.Sum(nil)))
}

func (h *ServerHandler) validContactReviewToken(r *http.Request) (string, bool) {
	parts := strings.Split(r.FormValue("review_token"), ".")
	if len(parts) != 4 || len(parts[3]) != 64 || !validContactNonce(parts[1]) || parts[2] != "" && parts[2] != "d" || h.secretsMgr == nil {
		return "", false
	}
	issued, err := strconvParseInt(parts[0])
	if err != nil {
		return "", false
	}
	secret, err := h.secretsMgr.GetInstallationSecret(r.Context())
	if err != nil || len(secret) == 0 {
		return "", false
	}
	provided, err := hex.DecodeString(parts[3])
	if err != nil {
		return "", false
	}
	fieldHash := contactReviewFieldHash(r)
	message := fmt.Sprintf("contact-review|%d|%s|%s|%s", issued, parts[1], parts[2], fieldHash)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(message))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return "", false
	}
	now := time.Now()
	age := now.Unix() - issued
	cfg := h.appConfig.Server.Pages.Contact.Antispam
	if cfg.ReviewMaxSeconds > 0 && age > int64(cfg.ReviewMaxSeconds) || age < 0 {
		return "", false
	}
	expires := time.Unix(issued, 0).Add(time.Duration(cfg.ReviewMaxSeconds) * time.Second)
	if cfg.ReviewMaxSeconds <= 0 {
		expires = now.Add(15 * time.Minute)
	}
	if !useContactNonce("review:"+parts[1], expires, now) {
		return "", false
	}
	if age < int64(cfg.ReviewMinSeconds) {
		return "", false
	}
	return parts[2], true
}

func newContactNonce() (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(nonce[:]), nil
}

func validContactNonce(value string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil && len(decoded) == 16
}

func useContactNonce(key string, expires, now time.Time) bool {
	contactReplayCache.Lock()
	defer contactReplayCache.Unlock()
	for nonce, expiry := range contactReplayCache.used {
		if !expiry.After(now) {
			delete(contactReplayCache.used, nonce)
		}
	}
	if _, exists := contactReplayCache.used[key]; exists {
		return false
	}
	if len(contactReplayCache.used) >= contactNonceCacheLimit {
		var oldestKey string
		var oldestExpiry time.Time
		for nonce, expiry := range contactReplayCache.used {
			if oldestExpiry.IsZero() || expiry.Before(oldestExpiry) {
				oldestKey, oldestExpiry = nonce, expiry
			}
		}
		delete(contactReplayCache.used, oldestKey)
	}
	contactReplayCache.used[key] = expires
	return true
}

func contactReviewFieldHash(r *http.Request) string {
	keys := make([]string, 0, len(r.Form))
	for key := range r.Form {
		switch key {
		case "csrf_token", "form_token", "captcha_id", "captcha_answer", "g-recaptcha-response", "h-captcha-response", "captcha_response", "website", "step", "review_token", "confirmed":
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var normalized strings.Builder
	for _, key := range keys {
		values := append([]string(nil), r.Form[key]...)
		sort.Strings(values)
		normalized.WriteString(normalizeContact(key))
		normalized.WriteByte('=')
		for _, value := range values {
			normalized.WriteString(normalizeContact(value))
			normalized.WriteByte('\x00')
		}
		normalized.WriteByte('\n')
	}
	digest := sha256.Sum256([]byte(normalized.String()))
	return hex.EncodeToString(digest[:])
}

func (h *ServerHandler) contactOriginError(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	referer := strings.TrimSpace(r.Header.Get("Referer"))
	fetchSite := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))
	if origin == "" && referer == "" && fetchSite == "" {
		return true
	}
	if fetchSite != "" && !strings.EqualFold(fetchSite, "same-origin") {
		return true
	}
	provided := origin
	if provided == "" {
		provided = referer
	}
	if provided == "" {
		return false
	}
	expected := contactRequestOrigin(r)
	if expected == "" {
		return true
	}
	return !sameContactOrigin(provided, expected)
}

func contactRequestOrigin(r *http.Request) string {
	expected, err := url.Parse(urlvar.BuildURL(r, "/"))
	if err != nil {
		return ""
	}
	// Origin headers contain only scheme and authority. BuildURL also includes
	// the configured base path, so remove it before comparing origins.
	expected.Path = ""
	expected.RawPath = ""
	expected.RawQuery = ""
	expected.Fragment = ""
	return expected.String()
}

func sameContactOrigin(provided, expected string) bool {
	a, errA := url.Parse(provided)
	b, errB := url.Parse(expected)
	if errA != nil || errB != nil || a.Opaque != "" || a.User != nil || a.Hostname() == "" || b.Hostname() == "" {
		return false
	}
	if a.Path != "" && a.Path != "/" || a.RawQuery != "" || a.Fragment != "" {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(strings.TrimSuffix(a.Hostname(), "."), strings.TrimSuffix(b.Hostname(), ".")) && effectiveContactPort(a) == effectiveContactPort(b)
}

func effectiveContactPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	if strings.EqualFold(u.Scheme, "https") {
		return "443"
	}
	if strings.EqualFold(u.Scheme, "http") {
		return "80"
	}
	return ""
}

func strconvParseInt(v string) (int64, error) { return strconv.ParseInt(v, 10, 64) }

func shortContactDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])[:8]
}

func (h *ServerHandler) contactBlock(r *http.Request, reason string) bool {
	ip := urlvar.ResolveClientIP(r)
	isOverlay := urlvar.GlobalResolver().IsOverlayRequest(r)
	if urlvar.IsTorRequest(r) {
		ip = urlvar.TorClientLabel(r)
	}
	if h.logger != nil {
		h.logger.Security("contact.spam_blocked", ip, map[string]interface{}{"reason": reason})
	}
	metric.ContactBlockedTotal.WithLabelValues(reason).Inc()
	if isOverlay || (reason != "honeypot" && reason != "captcha" && reason != "content" && reason != "markup" && reason != "duplicate" && reason != "origin" && reason != "replay" && reason != "review") {
		return false
	}
	cfg := h.appConfig.Server.Pages.Contact.Antispam
	if cfg.Strikes <= 0 {
		return false
	}
	now := time.Now()
	window, err := time.ParseDuration(cfg.Window)
	if err != nil || window <= 0 {
		window = time.Hour
	}
	h.contactMu.Lock()
	defer h.contactMu.Unlock()
	if h.contactStrikes == nil {
		h.contactStrikes = make(map[string][]time.Time)
	}
	if h.contactBans == nil {
		h.contactBans = make(map[string]time.Time)
	}
	for existingIP, existing := range h.contactStrikes {
		kept := existing[:0]
		for _, at := range existing {
			if now.Sub(at) <= window {
				kept = append(kept, at)
			}
		}
		if len(kept) == 0 {
			delete(h.contactStrikes, existingIP)
		} else {
			h.contactStrikes[existingIP] = kept
		}
	}
	for existingIP, until := range h.contactBans {
		if !until.After(now) {
			delete(h.contactBans, existingIP)
		}
	}
	if _, exists := h.contactStrikes[ip]; !exists && len(h.contactStrikes) >= contactStateLimit {
		evictOldestContactStrikes(h.contactStrikes)
	}
	strikes := h.contactStrikes[ip][:0]
	for _, at := range h.contactStrikes[ip] {
		if now.Sub(at) <= window {
			strikes = append(strikes, at)
		}
	}
	strikes = append(strikes, now)
	h.contactStrikes[ip] = strikes
	if len(strikes) >= cfg.Strikes && cfg.BanHours > 0 {
		if _, exists := h.contactBans[ip]; !exists && len(h.contactBans) >= contactStateLimit {
			evictOldestContactBan(h.contactBans)
		}
		h.contactBans[ip] = now.Add(time.Duration(cfg.BanHours) * time.Hour)
		return true
	}
	return false
}

func evictOldestContactWindow(entries map[string]contactWindow) {
	var oldestKey string
	var oldest time.Time
	for key, entry := range entries {
		if oldest.IsZero() || entry.start.Before(oldest) {
			oldestKey, oldest = key, entry.start
		}
	}
	delete(entries, oldestKey)
}

func evictOldestContactStrikes(entries map[string][]time.Time) {
	var oldestIP string
	var oldest time.Time
	for ip, strikes := range entries {
		if len(strikes) > 0 && (oldest.IsZero() || strikes[0].Before(oldest)) {
			oldestIP, oldest = ip, strikes[0]
		}
	}
	delete(entries, oldestIP)
}

func evictOldestContactBan(entries map[string]time.Time) {
	var oldestIP string
	var oldest time.Time
	for ip, until := range entries {
		if oldest.IsZero() || until.Before(oldest) {
			oldestIP, oldest = ip, until
		}
	}
	delete(entries, oldestIP)
}

func (h *ServerHandler) contactShapeError(r *http.Request, security bool) string {
	if !utf8.ValidString(r.Form.Encode()) {
		return "shape"
	}
	allowed := map[string]bool{
		"csrf_token": true, "form_token": true, "captcha_id": true, "captcha_answer": true,
		"g-recaptcha-response": true, "h-captcha-response": true, "captcha_response": true,
		"website": true, "security_id": true, "step": true, "review_token": true,
		"confirmed": true, "name": true, "email": true, "subject": true, "message": true,
		"type": true, "reported_url": true, "reason": true, "copyrighted_work": true,
		"good_faith": true, "accuracy": true, "signature": true, "request_kind": true,
	}
	if security {
		for _, field := range []string{"researcher_gpg", "component", "component_other", "endpoint", "severity", "summary", "steps", "impact", "suggested_fix", "cve_requested", "disclosure_days", "credit_preference", "credit_handle", "disclosure_agreement"} {
			allowed[field] = true
		}
	}
	step := strings.TrimSpace(r.FormValue("step"))
	if step != "" && step != "submit" && step != "confirm" && step != "edit" {
		return "shape"
	}
	for key, values := range r.Form {
		if !allowed[key] || len(values) != 1 && key != "security_id" {
			return "shape"
		}
		for _, value := range values {
			if !utf8.ValidString(value) {
				return "shape"
			}
			for _, ch := range value {
				if unicode.Is(unicode.Cf, ch) || ch == 0 {
					return "shape"
				}
			}
		}
	}
	required := []string{"name", "email", "subject", "message", "type"}
	if security {
		required = []string{"name", "email", "component", "severity", "summary", "steps", "impact", "credit_preference", "disclosure_agreement"}
	} else {
		switch r.FormValue("type") {
		case "abuse":
			required = append(required, "reported_url", "reason")
		case "dmca":
			required = append(required, "reported_url", "copyrighted_work", "good_faith", "accuracy", "signature")
		case "privacy":
			required = append(required, "request_kind")
		default:
			return "shape"
		}
	}
	for _, key := range required {
		if strings.TrimSpace(r.FormValue(key)) == "" {
			return "shape"
		}
	}
	for _, key := range []string{"name", "email"} {
		for _, ch := range r.FormValue(key) {
			if unicode.IsControl(ch) {
				return "shape"
			}
		}
	}
	for _, ch := range r.FormValue("subject") {
		if unicode.IsControl(ch) {
			return "shape"
		}
	}
	if email := r.FormValue("email"); email != "" && (strings.ContainsAny(email, ",;<>")) {
		return "shape"
	}
	if r.FormValue("name") == "" || r.FormValue("email") == "" {
		return "shape"
	}
	_, err := mail.ParseAddress(r.FormValue("email"))
	if err != nil {
		return "shape"
	}
	return ""
}

func (h *ServerHandler) contactHoneypotFilled(r *http.Request) bool {
	return h.appConfig.Server.Pages.Contact.Antispam.Honeypot && r.FormValue("website") != ""
}

func (h *ServerHandler) contactContentError(r *http.Request) string {
	cfg := h.appConfig.Server.Pages.Contact.Antispam
	if strings.EqualFold(r.FormValue("type"), "security") {
		return ""
	}
	name, subject, message := r.FormValue("name"), r.FormValue("subject"), r.FormValue("message")
	contentFields := []string{name, subject, message, r.FormValue("reported_url"), r.FormValue("reason"), r.FormValue("copyrighted_work"), r.FormValue("signature"), r.FormValue("request_kind")}
	content := strings.Join(contentFields, " ")
	if cfg.RejectMarkup {
		decoded := content
		for i := 0; i < 3; i++ {
			decoded = html.UnescapeString(decoded)
		}
		if contactMarkupPattern.MatchString(decoded) {
			return "markup"
		}
	}
	if contactLinkPattern.MatchString(name + " " + subject) {
		return "content"
	}
	linkText := contactEmailPattern.ReplaceAllString(strings.Join(contentFields[2:], " "), "")
	links := contactLinkPattern.FindAllString(linkText, -1)
	if cfg.MaxLinks > 0 && len(links) > cfg.MaxLinks {
		return "content"
	}
	for _, match := range links {
		if !h.allowedContactLinkHost(r, contactLinkHost(match)) {
			return "content"
		}
	}
	for _, word := range cfg.BlockedWords {
		if word != "" && strings.Contains(strings.ToLower(content), strings.ToLower(word)) {
			return "content"
		}
	}
	_, address, _ := strings.Cut(strings.ToLower(r.FormValue("email")), "@")
	for _, domain := range cfg.BlockedEmailDomains {
		if strings.EqualFold(address, domain) || strings.HasSuffix(address, "."+strings.ToLower(domain)) {
			return "content"
		}
	}
	return ""
}

func contactLinkHost(match string) string {
	value := match
	if strings.HasPrefix(strings.ToLower(value), "www.") || !strings.Contains(value, "://") {
		value = "https://" + value
	}
	u, err := url.Parse(value)
	if err != nil || u.User != nil {
		return ""
	}
	return strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
}

func (h *ServerHandler) allowedContactLinkHost(r *http.Request, host string) bool {
	if host == "" {
		return false
	}
	allowed := make(map[string]bool)
	_, fqdn, _ := urlvar.GetURLVars(r)
	add := func(value string) {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			return
		}
		if strings.Contains(value, "://") {
			parsed, err := url.Parse(value)
			if err != nil {
				return
			}
			value = parsed.Hostname()
		}
		if parsed, err := url.Parse("https://" + value); err == nil && parsed.Hostname() != "" {
			allowed[strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")] = true
		}
	}
	add(fqdn)
	add(h.appConfig.Server.FQDN)
	for _, domain := range urlvar.GetAllDomains() {
		add(domain)
	}
	if h.torSvc != nil {
		if address, ok := h.torSvc.GetInfo()["onion_address"].(string); ok {
			add(address)
		}
	}
	if h.i2pSvc != nil {
		add(h.i2pSvc.Address())
	}
	return allowed[host]
}

func normalizeContact(v string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(v))), " ")
}
