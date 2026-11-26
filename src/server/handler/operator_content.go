// SPDX-License-Identifier: MIT
package handler

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/apimgr/vidveil/src/common/i18n"
	"github.com/apimgr/vidveil/src/common/markdown"
	"github.com/apimgr/vidveil/src/common/sanitize"
	"github.com/apimgr/vidveil/src/common/version"
	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

type operatorConfigContextKey struct{}
type operatorVariablesContextKey struct{}

var operatorWarnings sync.Map
var contactHrefPattern = regexp.MustCompile(`(?i)href="/server/contact(?:[?#][^"]*)?"`)
var externalAnchorPattern = regexp.MustCompile(`(?is)<a\b[^>]*href=["']https?://[^"']*["'][^>]*>(.*?)</a>`)
var protocolRelativeAnchorPattern = regexp.MustCompile(`(?is)<a\b[^>]*href=["']//[^"']*["'][^>]*>(.*?)</a>`)

func withOperatorConfig(r *http.Request, cfg *config.AppConfig) *http.Request {
	if r == nil {
		return nil
	}
	return r.WithContext(contextWithOperatorConfig(r, cfg))
}

func withOperatorVariables(r *http.Request, data map[string]interface{}) *http.Request {
	if r == nil {
		return nil
	}
	values := make(map[string]string)
	for _, key := range []string{"OnionAddress", "I2PAddress"} {
		if value, ok := data[key].(string); ok {
			values[key] = value
		}
	}
	return r.WithContext(context.WithValue(r.Context(), operatorVariablesContextKey{}, values))
}

func contextWithOperatorConfig(r *http.Request, cfg *config.AppConfig) context.Context {
	return context.WithValue(r.Context(), operatorConfigContextKey{}, cfg)
}

func operatorHTML(r *http.Request, name string) template.HTML {
	if r == nil {
		return template.HTML("")
	}
	cfg, _ := r.Context().Value(operatorConfigContextKey{}).(*config.AppConfig)
	if cfg == nil {
		return template.HTML("")
	}
	entry, key := operatorContentEntry(cfg.Server.Branding.Content, name)
	if entry == nil {
		return template.HTML("")
	}
	format := strings.ToLower(strings.TrimSpace(entry.Format))
	if format == "" {
		format = "md"
		if name == "header" || name == "footer" {
			format = "html"
		}
	}
	if (name == "header" || name == "footer") && format != "html" {
		return invalidOperatorContent(key, "format")
	}
	if format != "md" && format != "html" && format != "txt" {
		return invalidOperatorContent(key, "format")
	}
	body := entry.Body.Resolve(i18n.DetectLocale(r))
	if len([]byte(body)) > 256*1024 {
		return invalidOperatorContent(key, "body")
	}
	if strings.TrimSpace(body) == "" {
		return template.HTML("")
	}
	vars, _ := r.Context().Value(operatorVariablesContextKey{}).(map[string]string)
	body = substituteOperatorVariables(body, vars)
	var rendered template.HTML
	switch format {
	case "html":
		if name == "header" || name == "footer" {
			rendered = sanitize.HTML(body)
		} else {
			rendered = sanitize.Document(body)
		}
	case "md":
		rendered = markdown.ToDocumentHTML(body)
	case "txt":
		var paragraphs []string
		for _, paragraph := range strings.Split(strings.TrimSpace(body), "\n\n") {
			paragraph = strings.TrimSpace(paragraph)
			if paragraph != "" {
				paragraphs = append(paragraphs, "<p>"+html.EscapeString(paragraph)+"</p>")
			}
		}
		rendered = sanitize.Document(strings.Join(paragraphs, ""))
	}
	clean := contactHrefPattern.ReplaceAllString(string(rendered), "")
	if urlvar.IsTorRequest(r) {
		clean = stripExternalAnchorLinks(clean)
	}
	return sanitize.Document(clean)
}

func stripExternalAnchorLinks(markup string) string {
	markup = externalAnchorPattern.ReplaceAllString(markup, "$1")
	return protocolRelativeAnchorPattern.ReplaceAllString(markup, "$1")
}

func operatorContentConfigured(r *http.Request, name string) bool {
	if r == nil {
		return false
	}
	cfg, _ := r.Context().Value(operatorConfigContextKey{}).(*config.AppConfig)
	if cfg == nil {
		return false
	}
	entry, key := operatorContentEntry(cfg.Server.Branding.Content, name)
	if entry == nil {
		return false
	}
	body := entry.Body.Resolve(i18n.DetectLocale(r))
	if len([]byte(body)) > 256*1024 {
		invalidOperatorContent(key, "body")
		return false
	}
	format := strings.ToLower(strings.TrimSpace(entry.Format))
	if format == "" {
		format = "md"
		if name == "header" || name == "footer" {
			format = "html"
		}
	}
	if name == "header" || name == "footer" {
		if format != "html" {
			invalidOperatorContent(key, "format")
			return false
		}
		return true
	}
	if format != "md" && format != "html" && format != "txt" {
		invalidOperatorContent(key, "format")
		return false
	}
	return strings.TrimSpace(body) != ""
}

func operatorContentEntry(content config.BrandingContentConfig, name string) (*config.BrandingPageContent, string) {
	switch name {
	case "header":
		return &content.Header, "server.branding.content.header"
	case "about":
		return &content.About, "server.branding.content.about"
	case "privacy":
		return &content.Privacy, "server.branding.content.privacy"
	case "terms":
		return &content.Terms, "server.branding.content.terms"
	case "footer":
		return &content.Footer, "server.branding.content.footer"
	default:
		return nil, ""
	}
}

func invalidOperatorContent(key, field string) template.HTML {
	warningKey := key + "." + field
	if _, loaded := operatorWarnings.LoadOrStore(warningKey, struct{}{}); !loaded {
		log.Printf("config: invalid %s ignored; using built-in content", warningKey)
	}
	return template.HTML("")
}

func substituteOperatorVariables(body string, runtime map[string]string) string {
	values := map[string]string{
		"current_year":    fmt.Sprint(time.Now().Year()),
		"project_name":    "vidveil",
		"project_org":     "apimgr",
		"project_version": version.GetVersion(),
		"build_datetime":  BuildDateTime(),
		"onion_address":   runtime["OnionAddress"],
		"i2p_address":     runtime["I2PAddress"],
	}
	for key, value := range values {
		body = strings.ReplaceAll(body, "{"+key+"}", html.EscapeString(value))
	}
	return body
}
