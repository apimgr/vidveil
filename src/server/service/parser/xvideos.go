// SPDX-License-Identifier: MIT
package parser

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// extractViewsFromText extracts view count from metadata text like "10 min Gabiconkey - 6.4M Views -"
func extractViewsFromText(text string) (string, int64) {
	// Look for pattern like "6.4M Views" or "500K Views" or "1,234 Views"
	re := regexp.MustCompile(`([\d.,]+)\s*([KkMmBb]?)\s*[Vv]iews`)
	matches := re.FindStringSubmatch(text)
	if len(matches) >= 2 {
		viewStr := matches[1]
		if len(matches) >= 3 && matches[2] != "" {
			viewStr += strings.ToUpper(matches[2])
		}
		return viewStr + " views", ParseViewCount(viewStr)
	}
	return "", 0
}

// XVideosParser handles XVideos HTML parsing
type XVideosParser struct {
	BaseURL string
}

// NewXVideosParser creates a new XVideos parser
func NewXVideosParser() *XVideosParser {
	return &XVideosParser{BaseURL: "https://www.xvideos.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *XVideosParser) ItemSelector() string {
	return "div.thumb-block, div.mozaique div.thumb"
}

// Parse extracts video data from a selection
func (p *XVideosParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Get link
	link := s.Find("a").First()
	href := ExtractAttr(link, "href")
	if href == "" {
		return nil
	}
	item.URL = MakeAbsoluteURL(href, p.BaseURL)

	// Get title
	titleElem := s.Find("p.title a, a[title]")
	item.Title = ExtractAttr(titleElem, "title")
	if item.Title == "" {
		item.Title = CleanText(titleElem.Text())
	}
	if item.Title == "" {
		item.Title = ExtractAttr(link, "title")
	}
	if item.Title == "" {
		return nil
	}

	// Get thumbnail
	img := s.Find("img").First()
	item.Thumbnail = ExtractAttr(img, "data-src", "src")
	if item.Thumbnail != "" {
		item.Thumbnail = MakeAbsoluteURL(item.Thumbnail, "https:")
	}

	// Get preview video URL - XVideos uses data-pvv attribute on img element
	item.PreviewURL = ExtractAttr(img, "data-pvv")
	if item.PreviewURL == "" {
		// Also try on the containing elements
		item.PreviewURL = ExtractAttr(s.Find(".thumb-inside img").First(), "data-pvv")
	}
	if item.PreviewURL != "" {
		item.PreviewURL = MakeAbsoluteURL(item.PreviewURL, "https:")
	}

	// Try to extract download URL from data attributes (if available in search results)
	item.DownloadURL = ExtractAttr(img, "data-video-url", "data-download", "data-src-video")
	if item.DownloadURL == "" {
		item.DownloadURL = ExtractAttr(s, "data-video-url", "data-download")
	}
	if item.DownloadURL != "" {
		item.DownloadURL = MakeAbsoluteURL(item.DownloadURL, "https:")
	}

	// Get duration - use First() to avoid duplication
	durElem := s.Find(".duration").First()
	if durElem.Length() == 0 {
		durElem = s.Find("span.duration").First()
	}
	item.Duration, item.DurationSeconds = ParseDuration(CleanText(durElem.Text()))

	// Get views - XVideos has views in metadata section
	// Format is typically "X.XM Views" or "XXK Views"
	metaText := CleanText(s.Find(".metadata, .video-data").First().Text())
	item.Views, item.ViewsCount = extractViewsFromText(metaText)

	// Get quality
	item.Quality = ExtractQuality(s)

	// Check for premium
	item.IsPremium = IsPremiumContent(s, item.URL)

	return item
}
