// SPDX-License-Identifier: MIT
// Package parsers provides HTML parsing utilities for video site scraping
package parser

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
	Register("xnxx", NewXNXXParser())
	Register("redtube", NewRedTubeParser())
	Register("eporner", NewEpornerParser())
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
	// mm:ss
	case 2:
		m, _ := strconv.Atoi(parts[0])
		s, _ := strconv.Atoi(parts[1])
		secs := m*60 + s
		return duration, secs
	// hh:mm:ss
	case 3:
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

// ParseRating parses rating strings like "93%", "4.5/5", "4.5 stars" and returns (display string, rating as float)
func ParseRating(ratingText string) (string, float64) {
	ratingText = strings.TrimSpace(ratingText)
	if ratingText == "" {
		return "", 0
	}

	// Handle percentage ratings (93% → 93.0)
	if strings.HasSuffix(ratingText, "%") {
		percentStr := strings.TrimSuffix(ratingText, "%")
		if val, err := strconv.ParseFloat(percentStr, 64); err == nil {
			return ratingText, val
		}
	}

	// Handle ratings like "4.5/5" or "4/5"
	if strings.Contains(ratingText, "/") {
		parts := strings.Split(ratingText, "/")
		if len(parts) == 2 {
			if val, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err == nil {
				if max, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil && max > 0 {
					// Convert to 100 scale
					rating := (val / max) * 100
					return ratingText, rating
				}
			}
		}
	}

	// Handle "4.5 stars" or just "4.5"
	ratingText = strings.ToLower(ratingText)
	ratingText = strings.ReplaceAll(ratingText, "stars", "")
	ratingText = strings.ReplaceAll(ratingText, "star", "")
	ratingText = strings.TrimSpace(ratingText)

	if val, err := strconv.ParseFloat(ratingText, 64); err == nil {
		// If value is ≤ 5, assume 5-star scale
		if val <= 5 {
			rating := (val / 5) * 100
			return strings.TrimSpace(ratingText), rating
		}
		// If value is ≤ 10, assume 10-point scale
		if val <= 10 {
			rating := val * 10
			return strings.TrimSpace(ratingText), rating
		}
		// Otherwise assume already on 100 scale
		return strings.TrimSpace(ratingText), val
	}

	return ratingText, 0
}
