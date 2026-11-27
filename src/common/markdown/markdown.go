// Package markdown renders the small Markdown subset used by project and
// operator-authored prose into sanitized HTML.
// Paragraphs, "-" bullet lists, shifted headings, and **bold** are supported —
// output is always piped through the appropriate sanitizer.
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
	return render(src, false, false)
}

// ToDocumentHTML renders Markdown with the broader safe document allowlist.
func ToDocumentHTML(src string) template.HTML {
	return render(src, true, true)
}

func render(src string, document, shiftHeadings bool) template.HTML {
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
		case shiftHeadings && markdownHeading(trimmed):
			flushPara()
			flushList()
			level, text := markdownHeadingContent(trimmed)
			if level > 4 {
				level = 6
			} else {
				level += 2
			}
			b.WriteString("<h")
			b.WriteByte(byte('0' + level))
			b.WriteByte('>')
			b.WriteString(inline(text))
			b.WriteString("</h")
			b.WriteByte(byte('0' + level))
			b.WriteByte('>')
		default:
			flushList()
			para = append(para, trimmed)
		}
	}
	flushPara()
	flushList()

	if document {
		return sanitize.Document(b.String())
	}
	return sanitize.HTML(b.String())
}

func markdownHeading(line string) bool {
	level, _ := markdownHeadingContent(line)
	return level > 0
}

func markdownHeadingContent(line string) (int, string) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level == len(line) || line[level] != ' ' {
		return 0, ""
	}
	return level, strings.TrimSpace(line[level:])
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
