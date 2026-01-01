// SPDX-License-Identifier: MIT
package parser

import (
	"github.com/PuerkitoBio/goquery"
)

// EpornerParser handles Eporner HTML parsing
type EpornerParser struct {
	BaseURL string
}

// NewEpornerParser creates a new Eporner parser
func NewEpornerParser() *EpornerParser {
	return &EpornerParser{BaseURL: "https://www.eporner.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *EpornerParser) ItemSelector() string {
	return "div.mb, div.video-item"
}

// Parse extracts video data from a selection
func (p *EpornerParser) Parse(s *goquery.Selection) *VideoItem {
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
		titleElem := s.Find(".mbtit a, .title")
		item.Title = CleanText(titleElem.Text())
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
	durElem := s.Find(".mbtim, .duration")
	item.Duration, item.DurationSeconds = ParseDuration(CleanText(durElem.Text()))

	// Get views
	viewsElem := s.Find(".mbvie, .views")
	item.Views, item.ViewsCount = ParseViews(CleanText(viewsElem.Text()))

	// Get quality
	item.Quality = ExtractQuality(s)

	// Check for premium
	item.IsPremium = IsPremiumContent(s, item.URL)

	return item
}
