// SPDX-License-Identifier: MIT
// AI.md PART 16: Contact form antispam layers.
package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/apimgr/vidveil/src/server/service/metric"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

const (
	contactBodyLimit  = 16 << 10
	contactStateLimit = 10000
)

var (
	contactLinkPattern   = regexp.MustCompile(`(?i)(?:https?://[^\s<>()]+|www\.[a-z0-9.-]+|\b(?:[a-z0-9-]+\.)+[a-z]{2,}\b)`)
	contactEmailPattern  = regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
	contactMarkupPattern = regexp.MustCompile(`(?is)<\s*/?\s*[a-z!][^>\s]*(?:[^>]*)>?|\[(?:/?(?:url|img|b)|/)|&(?:lt|gt|#0*60|#0*62);`)
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
	message := fmt.Sprintf("contact-form|%d", issued)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(message))
	return fmt.Sprintf("%d.%s", issued, hex.EncodeToString(mac.Sum(nil)))
}

func (h *ServerHandler) validContactFormToken(r *http.Request) bool {
	parts := strings.Split(r.FormValue("form_token"), ".")
	if len(parts) != 2 || len(parts[1]) != 64 {
		return false
	}
	issued, err := strconvParseInt(parts[0])
	if err != nil || h.secretsMgr == nil {
		return false
	}
	secret, err := h.secretsMgr.GetInstallationSecret(r.Context())
	if err != nil || len(secret) == 0 {
		return false
	}
	provided, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(fmt.Sprintf("contact-form|%d", issued)))
	age := time.Now().Unix() - issued
	cfg := h.appConfig.Server.Pages.Contact.Antispam
	return hmac.Equal(provided, mac.Sum(nil)) && (cfg.MinFillSeconds <= 0 || age >= int64(cfg.MinFillSeconds)) && (cfg.MaxAgeSeconds <= 0 || age <= int64(cfg.MaxAgeSeconds))
}

func strconvParseInt(v string) (int64, error) { return strconv.ParseInt(v, 10, 64) }

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
	if isOverlay || (reason != "honeypot" && reason != "captcha" && reason != "content" && reason != "markup" && reason != "duplicate") {
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
	if r.ContentLength > contactBodyLimit {
		return "shape"
	}
	if !utf8.ValidString(r.Form.Encode()) {
		return "shape"
	}
	allowed := map[string]bool{"csrf_token": true, "form_token": true, "captcha_id": true, "captcha_answer": true, "g-recaptcha-response": true, "h-captcha-response": true, "captcha_response": true, "website": true, "security_id": true}
	fields := []string{"name", "email", "subject", "message"}
	if security {
		fields = []string{"name", "email", "researcher_gpg", "component", "component_other", "endpoint", "severity", "summary", "steps", "impact", "suggested_fix", "cve_requested", "disclosure_days", "credit_preference", "credit_handle", "disclosure_agreement"}
	}
	for _, key := range fields {
		allowed[key] = true
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
	required := []string{"name", "email", "subject", "message"}
	if security {
		required = []string{"name", "email", "component", "severity", "summary", "steps", "impact", "credit_preference", "disclosure_agreement"}
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
	if !security {
		for _, ch := range r.FormValue("subject") {
			if unicode.IsControl(ch) {
				return "shape"
			}
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
	name, subject, message := r.FormValue("name"), r.FormValue("subject"), r.FormValue("message")
	if cfg.RejectMarkup && contactMarkupPattern.MatchString(html.UnescapeString(name+" "+subject+" "+message)) {
		return "markup"
	}
	linkText := contactEmailPattern.ReplaceAllString(message, "")
	if cfg.MaxLinks > 0 && len(contactLinkPattern.FindAllString(linkText, -1)) > cfg.MaxLinks {
		return "content"
	}
	if contactLinkPattern.MatchString(name + " " + subject) {
		return "content"
	}
	for _, word := range cfg.BlockedWords {
		if word != "" && strings.Contains(strings.ToLower(name+" "+subject+" "+message), strings.ToLower(word)) {
			return "content"
		}
	}
	_, address, _ := strings.Cut(strings.ToLower(r.FormValue("email")), "@")
	for _, domain := range cfg.BlockedEmailDomains {
		if strings.EqualFold(address, domain) || strings.HasSuffix(address, "."+strings.ToLower(domain)) {
			return "content"
		}
	}
	for _, match := range contactLinkPattern.FindAllString(linkText, -1) {
		u, _ := url.Parse(match)
		host := strings.ToLower(u.Hostname())
		if host == "" {
			host = strings.ToLower(strings.TrimPrefix(match, "www."))
		}
		for _, blocked := range cfg.BlockedLinkDomains {
			if host == blocked || strings.HasSuffix(host, "."+blocked) {
				return "content"
			}
		}
	}
	return ""
}

func normalizeContact(v string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(v))), " ")
}
