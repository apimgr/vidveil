// Package markdown renders the small Markdown subset used by operator-supplied
// configuration prose (server.privacy.content.*) into sanitized HTML.
// Only paragraphs, "-" bullet lists and **bold** are supported — the output is
// always piped through sanitize.HTML, so no untrusted markup can escape.
package markdown

import (
	"html"
	"html/template"
	"strings"

	"github.com/apimgr/vidveil/src/common/sanitize"
)

// ToHTML converts the supported Markdown subset to sanitized HTML.
// An empty input yields empty output so callers can branch on it in templates.
func ToHTML(src string) template.HTML {
	if strings.TrimSpace(src) == "" {
		return template.HTML("")
	}

	var b strings.Builder
	var list []string

	// flushList closes an open bullet list before switching block types
	flushList := func() {
		if len(list) == 0 {
			return
		}
		b.WriteString("<ul>")
		for _, item := range list {
			b.WriteString("<li>")
			b.WriteString(inline(item))
			b.WriteString("</li>")
		}
		b.WriteString("</ul>")
		list = nil
	}

	var para []string

	// flushPara closes an open paragraph before switching block types
	flushPara := func() {
		if len(para) == 0 {
			return
		}
		b.WriteString("<p>")
		b.WriteString(inline(strings.Join(para, " ")))
		b.WriteString("</p>")
		para = nil
	}

	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			flushPara()
			flushList()
		case strings.HasPrefix(trimmed, "- "):
			flushPara()
			list = append(list, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		default:
			flushList()
			para = append(para, trimmed)
		}
	}
	flushPara()
	flushList()

	return sanitize.HTML(b.String())
}

// inline escapes a text run and applies **bold** emphasis.
func inline(s string) string {
	escaped := html.EscapeString(s)
	parts := strings.Split(escaped, "**")
	if len(parts) < 3 {
		return escaped
	}

	var b strings.Builder
	for i, part := range parts {
		// Odd indexes sit between a matched pair of delimiters
		if i%2 == 1 && i < len(parts)-1 {
			b.WriteString("<strong>")
			b.WriteString(part)
			b.WriteString("</strong>")
			continue
		}
		if i%2 == 1 {
			b.WriteString("**")
		}
		b.WriteString(part)
	}
	return b.String()
}
