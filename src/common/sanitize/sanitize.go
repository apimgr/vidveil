// Package sanitize provides the single strict HTML allowlist policy used to
// render any operator- or user-supplied HTML (footer custom branding,
// markdown-rendered content, translation strings containing markup) per
// AI.md PART 16 "Footer Customization" / "Untrusted Content Handling".
package sanitize

import (
	"html/template"

	"github.com/microcosm-cc/bluemonday"
)

// policy is the single strict allowlist used everywhere untrusted HTML is
// rendered. It allows only basic text-formatting tags plus safe-attribute
// links and images, forces rel="noopener noreferrer" on links, restricts
// image/link URL schemes to https/data, and strips
// script/iframe/frame/object/embed/form/input/button/style/link/meta/base,
// all event-handler attributes, javascript: URLs, and the style attribute.
var policy = newPolicy()

func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()

	// Allowed tags (text formatting only).
	p.AllowElements("p", "br", "span", "div")
	p.AllowElements("strong", "b", "em", "i", "u", "s", "small")
	p.AllowElements("h1", "h2", "h3", "h4", "h5", "h6")
	p.AllowElements("ul", "ol", "li")

	// Allowed: links with safe attributes; force rel="noopener noreferrer".
	p.AllowAttrs("href", "title", "target", "rel").OnElements("a")
	p.RequireNoReferrerOnLinks(true)

	// Allowed: images with safe attributes.
	p.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
	// Only https and data: URLs are permitted for links/images.
	p.AllowURLSchemes("https", "data")

	// Allowed: class and id for styling (no style attribute — use classes).
	p.AllowAttrs("class", "id").Globally()

	// NEVER allowed (stripped automatically):
	// - <script>, <noscript>, <iframe>, <frame>, <object>, <embed>,
	//   <form>, <input>, <button>, <style>, <link>, <meta>, <base>
	// - onclick, onload, onerror, onmouseover, etc. (all event handlers)
	// - javascript: URLs
	// - style attribute

	return p
}

// HTML sanitizes s through the strict PART 16 allowlist policy and returns
// it marked safe for html/template rendering. Every byte of the returned
// value has passed through the allowlist — this is the only sanitizer
// permitted to feed a template.HTML value in this codebase.
func HTML(s string) template.HTML {
	return template.HTML(policy.Sanitize(s)) // #nosec G203 -- sanitized by bluemonday strict allowlist above
}

// String sanitizes s through the same strict allowlist and returns the
// plain sanitized string (for callers that validate/log before rendering,
// e.g. server.web.footer.custom_html).
func String(s string) string {
	return policy.Sanitize(s)
}
