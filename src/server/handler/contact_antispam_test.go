// SPDX-License-Identifier: MIT
package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestContactFormTokenRejectsForgery(t *testing.T) {
	h, _ := newSecurityTestHandler(t)
	h.appConfig.Server.Pages.Contact.Antispam.MinFillSeconds = 0
	token := h.issueContactFormToken(httpRequestForContact())
	if token == "" {
		t.Fatal("expected a signed form token")
	}
	r := httpRequestForContact()
	r.Form = url.Values{"form_token": {token}}
	if !h.validContactFormToken(r) {
		t.Fatal("fresh valid token was rejected")
	}
	r.Form.Set("form_token", token+"00")
	if h.validContactFormToken(r) {
		t.Fatal("forged token was accepted")
	}
}

func TestContactFormTokenIsSingleUse(t *testing.T) {
	h, _ := newSecurityTestHandler(t)
	h.appConfig.Server.Pages.Contact.Antispam.MinFillSeconds = 0
	r := httpRequestForContact()
	token := h.issueContactFormToken(r)
	if token == "" {
		t.Fatal("expected form token")
	}
	r.Form.Set("form_token", token)
	if got := h.contactFormTokenFailure(r); got != "" {
		t.Fatalf("first token validation = %q, want empty", got)
	}
	if got := h.contactFormTokenFailure(r); got != "replay" {
		t.Fatalf("reused token validation = %q, want replay", got)
	}
}

func TestContactReviewTokenBindsFieldsAndIsSingleUse(t *testing.T) {
	h, _ := newSecurityTestHandler(t)
	h.appConfig.Server.Pages.Contact.Antispam.ReviewMinSeconds = 0
	r := httpRequestForContact()
	r.Form = url.Values{"type": {"privacy"}, "name": {"Alice"}, "email": {"alice@example.test"}, "subject": {"Privacy request"}, "message": {"Please export my data"}, "request_kind": {"export"}}
	token := h.issueContactReviewToken(r, "")
	if token == "" {
		t.Fatal("expected review token")
	}
	r.Form.Set("step", "confirm")
	r.Form.Set("review_token", token)
	r.Form.Set("confirmed", "true")
	flags, ok := h.validContactReviewToken(r)
	if !ok || flags != "" {
		t.Fatalf("valid review token = (%q, %v), want (empty, true)", flags, ok)
	}
	r.Form.Set("review_token", token)
	if _, ok := h.validContactReviewToken(r); ok {
		t.Fatal("reused review token was accepted")
	}
}

func TestContactReviewTokenRejectsChangedField(t *testing.T) {
	h, _ := newSecurityTestHandler(t)
	h.appConfig.Server.Pages.Contact.Antispam.ReviewMinSeconds = 0
	r := httpRequestForContact()
	r.Form = url.Values{"type": {"privacy"}, "name": {"Alice"}, "email": {"alice@example.test"}, "subject": {"Privacy request"}, "message": {"Please export my data"}, "request_kind": {"export"}}
	token := h.issueContactReviewToken(r, "")
	r.Form.Set("step", "confirm")
	r.Form.Set("review_token", token)
	r.Form.Set("message", "Please delete my data")
	if _, ok := h.validContactReviewToken(r); ok {
		t.Fatal("review token was accepted after a field changed")
	}
}

func TestContactOriginRequiresSameOriginEvidence(t *testing.T) {
	h := NewServerHandler(nil)
	same := httptest.NewRequest(http.MethodPost, "https://vidveil.example/server/contact", strings.NewReader(""))
	same.Header.Set("Origin", contactRequestOrigin(same))
	same.Header.Set("Sec-Fetch-Site", "same-origin")
	if h.contactOriginError(same) {
		t.Fatal("same-origin browser request was rejected")
	}
	if !sameContactOrigin("https://vidveil.example", "https://vidveil.example/") {
		t.Fatal("sameContactOrigin rejected matching origins")
	}
	cross := httptest.NewRequest(http.MethodPost, "https://vidveil.example/server/contact", strings.NewReader(""))
	cross.Header.Set("Origin", "https://attacker.example")
	if !h.contactOriginError(cross) {
		t.Fatal("cross-origin request was accepted")
	}
	missing := httptest.NewRequest(http.MethodPost, "https://vidveil.example/server/contact", strings.NewReader(""))
	if !h.contactOriginError(missing) {
		t.Fatal("request without origin evidence was accepted")
	}
}

func TestContactLinksMustUseAnApplicationHost(t *testing.T) {
	h := NewServerHandler(nil)
	h.appConfig.Server.FQDN = "vidveil.example"
	r := httptest.NewRequest(http.MethodPost, "https://vidveil.example/server/contact", nil)
	if !h.allowedContactLinkHost(r, "vidveil.example") {
		t.Fatal("application host was rejected")
	}
	if h.allowedContactLinkHost(r, "attacker.example") {
		t.Fatal("external host was accepted")
	}
}

func TestContactRateLimitAndDuplicateSuppression(t *testing.T) {
	h := NewServerHandler(nil)
	h.appConfig.Server.Pages.Contact.Antispam.PerIP = 3
	h.appConfig.Server.Pages.Contact.Antispam.PerEmail = 0
	h.appConfig.Server.Pages.Contact.Antispam.Global = 0
	form := url.Values{"email": {"person@example.test"}, "subject": {"Question"}, "message": {"Hello"}}
	for i := 0; i < 3; i++ {
		r := httpRequestForContact()
		r.Form = form
		if reason := h.contactRateError(r); reason != "" {
			t.Fatalf("submission %d unexpectedly limited: %s", i+1, reason)
		}
	}
	r := httpRequestForContact()
	r.Form = form
	if reason := h.contactRateError(r); reason != "rate_ip" {
		t.Fatalf("fourth submission reason = %q, want rate_ip", reason)
	}
	r = httpRequestForContact()
	r.Form = form
	if h.duplicateContact(r) {
		t.Fatal("first message was considered a duplicate")
	}
	if !h.duplicateContact(r) {
		t.Fatal("repeated message was not suppressed")
	}
}

func TestContactShapeRejectsHeaderInjection(t *testing.T) {
	h := NewServerHandler(nil)
	r := httpRequestForContact()
	r.Form = url.Values{"name": {"Alice"}, "email": {"alice@example.test"}, "subject": {"hello\r\nBcc: victim@example.test"}, "message": {"hello"}}
	if got := h.contactShapeError(r, false); got != "shape" {
		t.Fatalf("shape error = %q, want shape", got)
	}
}

func TestContactAbuseStateMapsRemainBounded(t *testing.T) {
	h := NewServerHandler(nil)
	cfg := &h.appConfig.Server.Pages.Contact.Antispam
	cfg.PerIP = 0
	cfg.PerEmail = 1
	cfg.Global = 0
	cfg.Strikes = 1
	cfg.Window = "1h"
	now := time.Now()
	h.contactCounts = make(map[string]contactWindow, contactStateLimit)
	h.contactStrikes = make(map[string][]time.Time, contactStateLimit)
	h.contactBans = make(map[string]time.Time, contactStateLimit)
	for i := 0; i < contactStateLimit; i++ {
		key := "entry-" + strconv.Itoa(i)
		h.contactCounts["email:"+key] = contactWindow{start: now, count: 1}
		h.contactStrikes[key] = []time.Time{now}
		h.contactBans[key] = now.Add(time.Hour)
	}
	r := httpRequestForContact()
	r.RemoteAddr = "203.0.113.25:1234"
	r.Form = url.Values{"email": {"new@example.test"}}
	if reason := h.contactRateError(r); reason != "" {
		t.Fatalf("contactRateError() = %q, want no limit", reason)
	}
	if len(h.contactCounts) > contactStateLimit {
		t.Fatalf("contact rate map grew to %d entries", len(h.contactCounts))
	}
	h.contactBlock(r, "honeypot")
	if len(h.contactStrikes) > contactStateLimit || len(h.contactBans) > contactStateLimit {
		t.Fatalf("contact abuse maps exceed limit: strikes=%d bans=%d", len(h.contactStrikes), len(h.contactBans))
	}
}

func httpRequestForContact() *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/server/contact", strings.NewReader(""))
	r.Form = make(url.Values)
	return r
}
