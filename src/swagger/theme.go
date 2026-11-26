// SPDX-License-Identifier: MIT
package swagger

import (
	"net/http"

	sharedtheme "github.com/apimgr/vidveil/src/common/theme"
)

// DetectTheme resolves a documentation theme preference from the request.
// The shared API-documentation templates use the same root theme classes and
// CSS custom properties as the rest of the server pages.
func DetectTheme(r *http.Request) string {
	if r != nil {
		if cookie, err := r.Cookie("theme"); err == nil {
			switch cookie.Value {
			case "dark", "light", "auto":
				return cookie.Value
			}
		}
		if value := r.URL.Query().Get("theme"); value == "dark" || value == "light" || value == "auto" {
			return value
		}
	}
	return "auto"
}

// ThemeClass returns the root class consumed by the shared documentation CSS.
func ThemeClass(r *http.Request) string {
	return "theme-" + DetectTheme(r)
}

// Palette returns the shared source-of-truth palette used by the CSS theme.
// Auto starts from dark; common.css applies the light palette through the
// prefers-color-scheme media query when the visitor's system requests it.
func Palette(name string) sharedtheme.ColorPalette {
	if name == "light" {
		return sharedtheme.Light
	}
	return sharedtheme.Dark
}
