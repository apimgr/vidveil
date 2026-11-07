package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

type captchaChallenge struct {
	answerHash [32]byte
	expires    time.Time
}

type captchaRateWindow struct {
	started time.Time
	count   int
}

func (h *ServerHandler) captchaType(r *http.Request) string {
	t := strings.ToLower(strings.TrimSpace(h.appConfig.Server.Pages.Contact.Captcha))
	if urlvar.GlobalResolver().IsOverlayRequest(r) || isTextBrowser(r) || t == "" {
		return "simple"
	}
	return t
}

func (h *ServerHandler) newSimpleCaptcha(r *http.Request) (string, string) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", ""
	}
	a, err := rand.Int(rand.Reader, big.NewInt(9))
	if err != nil {
		return "", ""
	}
	b, err := rand.Int(rand.Reader, big.NewInt(9))
	if err != nil {
		return "", ""
	}
	left, right := int(a.Int64())+1, int(b.Int64())+1
	id := hex.EncodeToString(token[:])
	answer := sha256.Sum256([]byte(strconv.Itoa(left + right)))
	now := time.Now()
	h.captchaMu.Lock()
	if h.captchaAnswers == nil {
		h.captchaAnswers = make(map[string]captchaChallenge)
	}
	for key, value := range h.captchaAnswers {
		if now.After(value.expires) {
			delete(h.captchaAnswers, key)
		}
	}
	if len(h.captchaAnswers) < 10000 {
		h.captchaAnswers[id] = captchaChallenge{answerHash: answer, expires: now.Add(10 * time.Minute)}
	} else {
		id = ""
	}
	h.captchaMu.Unlock()
	if id == "" {
		return "", ""
	}
	return id, strconv.Itoa(left) + " + " + strconv.Itoa(right) + " = ?"
}

func (h *ServerHandler) verifyCaptcha(r *http.Request) bool {
	switch h.captchaType(r) {
	case "simple":
		return h.verifySimpleCaptcha(r)
	case "recaptcha":
		return h.verifyRemoteCaptcha(r, "https://www.google.com/recaptcha/api/siteverify", "g-recaptcha-response", true)
	case "hcaptcha":
		return h.verifyRemoteCaptcha(r, "https://api.hcaptcha.com/siteverify", "h-captcha-response", false)
	default:
		return false
	}
}

func (h *ServerHandler) verifySimpleCaptcha(r *http.Request) bool {
	now := time.Now()
	clientIP := urlvar.ResolveClientIP(r)
	h.captchaMu.Lock()
	if h.captchaAttempts == nil {
		h.captchaAttempts = make(map[string]captchaRateWindow)
	}
	for ip, current := range h.captchaAttempts {
		if now.Sub(current.started) >= 10*time.Minute {
			delete(h.captchaAttempts, ip)
		}
	}
	window := h.captchaAttempts[clientIP]
	if now.Sub(window.started) >= 10*time.Minute {
		window = captchaRateWindow{started: now}
	}
	window.count++
	h.captchaAttempts[clientIP] = window
	if window.count > 10 {
		h.captchaMu.Unlock()
		return false
	}
	id := r.FormValue("captcha_id")
	challenge, ok := h.captchaAnswers[id]
	delete(h.captchaAnswers, id)
	h.captchaMu.Unlock()
	if !ok || now.After(challenge.expires) {
		return false
	}
	supplied := sha256.Sum256([]byte(strings.TrimSpace(r.FormValue("captcha_answer"))))
	return subtle.ConstantTimeCompare(challenge.answerHash[:], supplied[:]) == 1
}

func (h *ServerHandler) verifyRemoteCaptcha(r *http.Request, endpoint, responseField string, includeRemoteIP bool) bool {
	cfg := h.appConfig.Server.Pages.Contact
	response := strings.TrimSpace(r.FormValue(responseField))
	if response == "" || cfg.CaptchaSecretKey == "" {
		return false
	}
	form := url.Values{"secret": {cfg.CaptchaSecretKey}, "response": {response}}
	if includeRemoteIP {
		form.Set("remoteip", urlvar.ResolveClientIP(r))
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var result struct {
		Success bool `json:"success"`
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&result) == nil && result.Success
}

func (h *ServerHandler) captchaTemplateData(r *http.Request) map[string]interface{} {
	typeName := h.captchaType(r)
	data := map[string]interface{}{"CaptchaType": typeName}
	switch typeName {
	case "simple":
		id, question := h.newSimpleCaptcha(r)
		data["CaptchaID"] = id
		data["CaptchaQuestion"] = question
	case "recaptcha", "hcaptcha":
		data["CaptchaSiteKey"] = h.appConfig.Server.Pages.Contact.CaptchaSiteKey
	}
	return data
}

func CaptchaCSPOrigins(cfg config.ServerContactPagesConfig, r *http.Request) (scripts, frames, connections []string) {
	if r != nil && (urlvar.GlobalResolver().IsOverlayRequest(r) || isTextBrowser(r)) {
		return nil, nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Captcha)) {
	case "recaptcha":
		return []string{"https://www.google.com", "https://www.gstatic.com"}, []string{"https://www.google.com"}, []string{"https://www.google.com"}
	case "hcaptcha":
		return []string{"https://hcaptcha.com", "https://*.hcaptcha.com"}, []string{"https://hcaptcha.com", "https://*.hcaptcha.com"}, []string{"https://hcaptcha.com", "https://*.hcaptcha.com"}
	default:
		return nil, nil, nil
	}
}
