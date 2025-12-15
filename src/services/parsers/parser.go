// SPDX-License-Identifier: MIT
// Package parsers provides HTML parsing utilities for video site scraping
package parsers

import (
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

// Parser interface for site-specific parsing
type Parser interface {
	ItemSelector() string
	Parse(s *goquery.Selection) *VideoItem
}

// ParserRegistry manages parser instances by site name
type ParserRegistry struct {
	mu      sync.RWMutex
	parsers map[string]Parser
}

// Global registry instance
var registry = &ParserRegistry{
	parsers: make(map[string]Parser),
}

// Register adds a parser to the registry
func Register(name string, p Parser) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.parsers[name] = p
}

// GetParser retrieves a parser by name
func GetParser(name string) Parser {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.parsers[name]
}

// init registers all built-in parsers
func init() {
	Register("pornhub", NewPornHubParser())
	Register("xvideos", NewXVideosParser())
	Register("xhamster", NewXHamsterParser())
	Register("xnxx", NewXNXXParser())
	Register("redtube", NewRedTubeParser())
	Register("youporn", NewYouPornParser())
	Register("spankbang", NewSpankBangParser())
	Register("eporner", NewEpornerParser())
	Register("beeg", NewBeegParser())
	Register("pornmd", NewPornMDParser())
}

// VideoItem represents parsed video data
type VideoItem struct {
	URL             string
	Title           string
	Thumbnail       string
	PreviewURL      string
	Duration        string
	DurationSeconds int
	Views           string
	ViewsCount      int64
	Quality         string
	Description     string
	Uploader        string
	Rating          string
	IsPremium       bool
}

// CleanText removes extra whitespace and trims text
func CleanText(s string) string {
	s = strings.TrimSpace(s)
	// Collapse multiple whitespace
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(s, " ")
}

// ExtractAttr extracts the first non-empty attribute value from a list of attribute names
func ExtractAttr(s *goquery.Selection, attrs ...string) string {
	for _, attr := range attrs {
		if val, exists := s.Attr(attr); exists && val != "" {
			return val
		}
	}
	return ""
}

// ParseDuration parses various duration formats to (display string, seconds)
func ParseDuration(duration string) (string, int) {
	duration = strings.TrimSpace(duration)
	if duration == "" {
		return "", 0
	}

	// Clean up the duration string
	duration = strings.ReplaceAll(duration, " ", "")

	// Handle formats like "12:34" or "1:23:45"
	parts := strings.Split(duration, ":")
	switch len(parts) {
	case 2: // mm:ss
		m, _ := strconv.Atoi(parts[0])
		s, _ := strconv.Atoi(parts[1])
		secs := m*60 + s
		return duration, secs
	case 3: // hh:mm:ss
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		s, _ := strconv.Atoi(parts[2])
		secs := h*3600 + m*60 + s
		return duration, secs
	}

	// Try to parse "12 min" or "12min" format
	re := regexp.MustCompile(`(\d+)\s*min`)
	if matches := re.FindStringSubmatch(strings.ToLower(duration)); len(matches) > 1 {
		m, _ := strconv.Atoi(matches[1])
		secs := m * 60
		return duration, secs
	}

	return duration, 0
}

// ParseViews parses view counts like "1.2M" or "500K" to (display string, count)
func ParseViews(views string) (string, int64) {
	views = strings.TrimSpace(views)
	if views == "" {
		return "", 0
	}

	original := views
	views = strings.ToUpper(views)

	// Remove "views" suffix
	views = strings.TrimSuffix(views, " VIEWS")
	views = strings.TrimSuffix(views, "VIEWS")
	views = strings.TrimSpace(views)

	multiplier := int64(1)
	if strings.HasSuffix(views, "K") {
		multiplier = 1000
		views = strings.TrimSuffix(views, "K")
	} else if strings.HasSuffix(views, "M") {
		multiplier = 1000000
		views = strings.TrimSuffix(views, "M")
	} else if strings.HasSuffix(views, "B") {
		multiplier = 1000000000
		views = strings.TrimSuffix(views, "B")
	}

	// Parse the number
	views = strings.ReplaceAll(views, ",", "")
	views = strings.ReplaceAll(views, " ", "")

	if f, err := strconv.ParseFloat(views, 64); err == nil {
		return original, int64(f * float64(multiplier))
	}

	return original, 0
}

// ExtractQuality looks for quality indicators in a selection
func ExtractQuality(s *goquery.Selection) string {
	// Look for common quality badges
	qualitySelectors := []string{
		".quality", ".hd-badge", ".quality-badge",
		"[class*='quality']", "[class*='hd']", "[class*='4k']",
	}

	for _, sel := range qualitySelectors {
		if q := s.Find(sel).First(); q.Length() > 0 {
			text := CleanText(q.Text())
			if text != "" {
				return text
			}
			// Check for class-based quality
			if class, _ := q.Attr("class"); class != "" {
				if strings.Contains(strings.ToLower(class), "4k") {
					return "4K"
				}
				if strings.Contains(strings.ToLower(class), "hd") {
					return "HD"
				}
			}
		}
	}
	return ""
}

// MakeAbsoluteURL makes a URL absolute given a base URL
func MakeAbsoluteURL(href, baseURL string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		return baseURL + href
	}
	return baseURL + "/" + href
}

// ParseViewCount parses view count string and returns count as int
func ParseViewCount(views string) int64 {
	_, count := ParseViews(views)
	return count
}

// IsPremiumContent checks if the content appears to be premium/paid
func IsPremiumContent(s *goquery.Selection, url string) bool {
	// Check URL for premium indicators
	lowerURL := strings.ToLower(url)
	if strings.Contains(lowerURL, "gold") ||
		strings.Contains(lowerURL, "premium") ||
		strings.Contains(lowerURL, "vip") ||
		strings.Contains(lowerURL, "paid") {
		return true
	}

	// Check for premium badges/classes
	html, _ := s.Html()
	lowerHTML := strings.ToLower(html)
	premiumIndicators := []string{"premium", "gold", "vip", "paid", "exclusive", "members-only"}
	for _, indicator := range premiumIndicators {
		if strings.Contains(lowerHTML, indicator) {
			return true
		}
	}

	return false
}
