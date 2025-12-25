// SPDX-License-Identifier: MIT
// Package parser provides HTML parsing utilities for video metadata extraction
package parser

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// VideoMetadata holds extracted video information
type VideoMetadata struct {
	Title    string
	URL      string
	Thumbnail string
	// Animated/hover thumbnail if available
	ThumbnailPreview string
	Duration         string
	DurationSeconds  int
	Views            string
	ViewsCount       int64
	Rating           string
	RatingPercent    float64
	Author           string
	AuthorURL        string
	Description      string
	Categories       []string
	Tags             []string
	UploadDate       time.Time
	// HD, 4K, etc.
	Quality string
}

// ParseDuration converts various duration formats to seconds
// Supports: "12:34", "1:23:45", "12 min", "12min 34sec"
func ParseDuration(duration string) (string, int) {
	duration = strings.TrimSpace(duration)
	if duration == "" {
		return "", 0
	}

	// Handle HH:MM:SS or MM:SS format
	parts := strings.Split(duration, ":")
	switch len(parts) {
	// MM:SS format
	case 2:
		m, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		s, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		secs := m*60 + s
		return duration, secs
	// HH:MM:SS format
	case 3:
		h, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
		m, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		s, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
		secs := h*3600 + m*60 + s
		return duration, secs
	}

	// Handle "12 min" or "12min" format
	re := regexp.MustCompile(`(\d+)\s*min`)
	if matches := re.FindStringSubmatch(strings.ToLower(duration)); len(matches) > 1 {
		m, _ := strconv.Atoi(matches[1])
		secs := m * 60
		// Check for seconds
		reSec := regexp.MustCompile(`(\d+)\s*sec`)
		if secMatches := reSec.FindStringSubmatch(strings.ToLower(duration)); len(secMatches) > 1 {
			s, _ := strconv.Atoi(secMatches[1])
			secs += s
		}
		return duration, secs
	}

	// Handle just seconds
	reSeconds := regexp.MustCompile(`^(\d+)$`)
	if matches := reSeconds.FindStringSubmatch(duration); len(matches) > 1 {
		s, _ := strconv.Atoi(matches[1])
		mins := s / 60
		secs := s % 60
		formatted := strconv.Itoa(mins) + ":" + strconv.Itoa(secs)
		return formatted, s
	}

	return duration, 0
}

// ParseViews converts view counts like "1.2M", "500K", "1,234,567" to integer
func ParseViews(views string) (string, int64) {
	views = strings.TrimSpace(views)
	if views == "" {
		return "", 0
	}

	original := views
	views = strings.ToUpper(views)

	// Remove common suffixes
	views = strings.TrimSuffix(views, " VIEWS")
	views = strings.TrimSuffix(views, "VIEWS")
	views = strings.TrimSpace(views)

	// Extract just the number part
	re := regexp.MustCompile(`([\d.,]+)\s*([KMBT]?)`)
	matches := re.FindStringSubmatch(views)
	if len(matches) < 2 {
		return original, 0
	}

	numStr := strings.ReplaceAll(matches[1], ",", "")
	numStr = strings.ReplaceAll(numStr, " ", "")

	multiplier := int64(1)
	if len(matches) > 2 {
		switch matches[2] {
		case "K":
			multiplier = 1000
		case "M":
			multiplier = 1000000
		case "B":
			multiplier = 1000000000
		case "T":
			multiplier = 1000000000000
		}
	}

	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return original, 0
	}

	return original, int64(f * float64(multiplier))
}

// ParseRating extracts rating percentage from various formats
// Supports: "85%", "4.5/5", "8.5/10"
func ParseRating(rating string) (string, float64) {
	rating = strings.TrimSpace(rating)
	if rating == "" {
		return "", 0
	}

	// Check for percentage
	if strings.HasSuffix(rating, "%") {
		numStr := strings.TrimSuffix(rating, "%")
		if f, err := strconv.ParseFloat(numStr, 64); err == nil {
			return rating, f
		}
	}

	// Check for X/Y format
	re := regexp.MustCompile(`([\d.]+)\s*/\s*([\d.]+)`)
	if matches := re.FindStringSubmatch(rating); len(matches) == 3 {
		num, _ := strconv.ParseFloat(matches[1], 64)
		denom, _ := strconv.ParseFloat(matches[2], 64)
		if denom > 0 {
			percent := (num / denom) * 100
			return rating, percent
		}
	}

	return rating, 0
}

// ExtractText gets clean text from a goquery selection
func ExtractText(s *goquery.Selection) string {
	return strings.TrimSpace(s.Text())
}

// ExtractAttr gets an attribute value, trying multiple attribute names
func ExtractAttr(s *goquery.Selection, attrs ...string) string {
	for _, attr := range attrs {
		if val, exists := s.Attr(attr); exists && val != "" {
			return val
		}
	}
	return ""
}

// ExtractThumbnail tries to get the best thumbnail URL
func ExtractThumbnail(s *goquery.Selection) (thumbnail, preview string) {
	// Common thumbnail attributes in order of preference
	thumbnailAttrs := []string{
		"data-thumb_url",
		"data-src",
		"data-original",
		"data-lazy-src",
		"src",
	}

	previewAttrs := []string{
		"data-preview",
		"data-gif",
		"data-animated",
		"data-mediabook",
	}

	img := s.Find("img").First()
	thumbnail = ExtractAttr(img, thumbnailAttrs...)
	preview = ExtractAttr(img, previewAttrs...)

	// Also check for source element
	if thumbnail == "" {
		source := s.Find("source").First()
		thumbnail = ExtractAttr(source, "src", "srcset")
	}

	return thumbnail, preview
}

// ExtractLink extracts URL and title from a link element
func ExtractLink(s *goquery.Selection, baseURL string) (url, title string) {
	link := s.Find("a").First()
	url = ExtractAttr(link, "href")
	title = ExtractAttr(link, "title")
	if title == "" {
		title = ExtractText(link)
	}

	// Make URL absolute if needed
	if url != "" && !strings.HasPrefix(url, "http") {
		url = baseURL + url
	}

	return url, title
}

// CleanText removes extra whitespace and normalizes text
func CleanText(text string) string {
	// Remove extra whitespace
	re := regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// ExtractQuality extracts video quality indicators
func ExtractQuality(s *goquery.Selection) string {
	// Look for HD/4K/etc badges
	qualitySelectors := []string{
		".hd-badge",
		".quality",
		".hd",
		"[data-quality]",
		".video-quality",
	}

	for _, selector := range qualitySelectors {
		if q := s.Find(selector).First(); q.Length() > 0 {
			text := CleanText(q.Text())
			if text != "" {
				return text
			}
			// Check data attribute
			if quality := ExtractAttr(q, "data-quality"); quality != "" {
				return quality
			}
		}
	}

	// Check for quality in class names
	class := ExtractAttr(s, "class")
	if strings.Contains(strings.ToLower(class), "4k") {
		return "4K"
	}
	if strings.Contains(strings.ToLower(class), "hd") {
		return "HD"
	}

	return ""
}

// ParseDate attempts to parse various date formats
func ParseDate(dateStr string) time.Time {
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return time.Time{}
	}

	// Common date formats
	formats := []string{
		"2006-01-02",
		"Jan 2, 2006",
		"January 2, 2006",
		"02 Jan 2006",
		"2 Jan 2006",
		"2006/01/02",
		"01/02/2006",
		"02/01/2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t
		}
	}

	// Handle relative dates like "2 days ago", "1 week ago"
	lowerDate := strings.ToLower(dateStr)
	now := time.Now()

	reNum := regexp.MustCompile(`(\d+)\s*(day|week|month|year)s?\s*ago`)
	if matches := reNum.FindStringSubmatch(lowerDate); len(matches) == 3 {
		num, _ := strconv.Atoi(matches[1])
		switch matches[2] {
		case "day":
			return now.AddDate(0, 0, -num)
		case "week":
			return now.AddDate(0, 0, -num*7)
		case "month":
			return now.AddDate(0, -num, 0)
		case "year":
			return now.AddDate(-num, 0, 0)
		}
	}

	return time.Time{}
}
