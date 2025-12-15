// SPDX-License-Identifier: MIT
package parsers

import (
	"github.com/PuerkitoBio/goquery"
)

// BeegParser handles Beeg HTML parsing
type BeegParser struct {
	BaseURL string
}

// NewBeegParser creates a new Beeg parser
func NewBeegParser() *BeegParser {
	return &BeegParser{BaseURL: "https://beeg.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *BeegParser) ItemSelector() string {
	return "div.video, article.video, div.video-item"
}

// Parse extracts video data from a selection
func (p *BeegParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Get link
	link := s.Find("a").First()
	href := ExtractAttr(link, "href")
	if href == "" {
		return nil
	}
	item.URL = MakeAbsoluteURL(href, p.BaseURL)

	// Get title
	item.Title = ExtractAttr(link, "title")
	if item.Title == "" {
		item.Title = CleanText(link.Text())
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

	// Get duration
	durElem := s.Find(".duration")
	item.Duration, item.DurationSeconds = ParseDuration(CleanText(durElem.Text()))

	// Get quality
	item.Quality = ExtractQuality(s)

	// Check for premium
	item.IsPremium = IsPremiumContent(s, item.URL)

	return item
}
