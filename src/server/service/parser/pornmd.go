// SPDX-License-Identifier: MIT
package parser

import (
	"github.com/PuerkitoBio/goquery"
)

// PornMDParser handles PornMD HTML parsing
type PornMDParser struct {
	BaseURL string
}

// NewPornMDParser creates a new PornMD parser
func NewPornMDParser() *PornMDParser {
	return &PornMDParser{BaseURL: "https://www.pornmd.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *PornMDParser) ItemSelector() string {
	return "div.card.sub"
}

// Parse extracts video data from a selection
func (p *PornMDParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Get link - PornMD uses <a class="item-link"> with title attribute
	link := s.Find("a.item-link, a.rate-link").First()
	if link.Length() == 0 {
		link = s.Find("a").First()
	}
	href := ExtractAttr(link, "href")
	if href == "" {
		return nil
	}
	// PornMD is a meta-search, URLs are already absolute
	item.URL = href

	// Get title from link's title attribute
	item.Title = ExtractAttr(link, "title")
	if item.Title == "" {
		// Try alt text from image
		img := s.Find("img").First()
		item.Title = ExtractAttr(img, "alt")
	}
	if item.Title == "" {
		return nil
	}

	// Get thumbnail - look for img.item-image or any img
	img := s.Find("img.item-image").First()
	if img.Length() == 0 {
		img = s.Find("img").First()
	}
	item.Thumbnail = ExtractAttr(img, "data-src", "src")
	if item.Thumbnail != "" {
		item.Thumbnail = MakeAbsoluteURL(item.Thumbnail, "https:")
	}

	// Get duration from badge spans (look for time format like "10:30")
	s.Find("span").Each(func(i int, span *goquery.Selection) {
		text := CleanText(span.Text())
		if dur, secs := ParseDuration(text); secs > 0 {
			item.Duration = dur
			item.DurationSeconds = secs
		}
	})

	// Get source info from badge
	sourceElem := s.Find(".source, span.badge")
	sourceText := CleanText(sourceElem.Text())
	if sourceText != "" && item.Duration != sourceText {
		item.Description = "Source: " + sourceText
	}

	// Check for premium
	item.IsPremium = IsPremiumContent(s, item.URL)

	return item
}
