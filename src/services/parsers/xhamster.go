// SPDX-License-Identifier: MIT
package parsers

import (
	"github.com/PuerkitoBio/goquery"
)

// XHamsterParser handles XHamster HTML parsing
type XHamsterParser struct {
	BaseURL string
}

// NewXHamsterParser creates a new XHamster parser
func NewXHamsterParser() *XHamsterParser {
	return &XHamsterParser{BaseURL: "https://xhamster.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *XHamsterParser) ItemSelector() string {
	return "div.thumb-list__item, div.video-thumb, article.thumb-list__item"
}

// Parse extracts video data from a selection
func (p *XHamsterParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Get link
	link := s.Find("a.video-thumb-info__name, a.video-thumb__image-container, a").First()
	href := ExtractAttr(link, "href")
	if href == "" {
		return nil
	}
	item.URL = MakeAbsoluteURL(href, p.BaseURL)

	// Get title
	titleElem := s.Find("a.video-thumb-info__name, .video-thumb__title")
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

	// Get duration
	durElem := s.Find(".thumb-image-container__duration, .duration")
	item.Duration, item.DurationSeconds = ParseDuration(CleanText(durElem.Text()))

	// Get views
	viewsElem := s.Find(".video-thumb-views, .views")
	item.Views, item.ViewsCount = ParseViews(CleanText(viewsElem.Text()))

	// Get quality
	item.Quality = ExtractQuality(s)

	// Check for premium
	item.IsPremium = IsPremiumContent(s, item.URL)

	return item
}
