// SPDX-License-Identifier: MIT
package parser

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// XNXXParser handles XNXX HTML parsing
type XNXXParser struct {
	BaseURL string
}

// NewXNXXParser creates a new XNXX parser
func NewXNXXParser() *XNXXParser {
	return &XNXXParser{BaseURL: "https://www.xnxx.com"}
}

// ItemSelector returns the CSS selector for video items
func (p *XNXXParser) ItemSelector() string {
	return "div.thumb-block"
}

// Parse extracts video data from a selection
func (p *XNXXParser) Parse(s *goquery.Selection) *VideoItem {
	item := &VideoItem{}

	// Get link - the main link is in div.thumb-under p a with title attribute
	link := s.Find("div.thumb-under p a[href]").First()
	href := ExtractAttr(link, "href")
	if href == "" {
		// Fallback to any anchor
		link = s.Find("a[href]").First()
		href = ExtractAttr(link, "href")
	}
	if href == "" {
		return nil
	}
	item.URL = MakeAbsoluteURL(href, p.BaseURL)

	// Skip gold/premium content
	if strings.Contains(item.URL, "xnxx.gold") || strings.Contains(item.URL, "/gold/") {
		return nil
	}

	// Get title from title attribute (most reliable on XNXX)
	item.Title = ExtractAttr(link, "title")
	if item.Title == "" {
		item.Title = CleanText(link.Text())
	}
	if item.Title == "" {
		return nil
	}

	// Get thumbnail - in div.thumb-inside img with data-src
	img := s.Find("div.thumb-inside img").First()
	item.Thumbnail = ExtractAttr(img, "data-src", "data-webp", "src")
	if item.Thumbnail != "" && !strings.HasPrefix(item.Thumbnail, "http") {
		item.Thumbnail = "https:" + item.Thumbnail
	}

	// Get duration - XNXX format: "36min" in p.metadata text content
	metaElem := s.Find("p.metadata")
	if metaElem.Length() > 0 {
		// Get direct text nodes (duration is before span elements)
		metaText := metaElem.Contents().FilterFunction(func(i int, sel *goquery.Selection) bool {
			return goquery.NodeName(sel) == "#text"
		}).Text()
		item.Duration, item.DurationSeconds = ParseDuration(metaText)
	}

	// Get quality from span.video-hd
	hdElem := s.Find("span.video-hd")
	if hdElem.Length() > 0 {
		item.Quality = CleanText(hdElem.Text())
	}

	// Get views from span containing view count
	viewSpan := s.Find("p.metadata span.right")
	if viewSpan.Length() > 0 {
		viewText := CleanText(viewSpan.Contents().First().Text())
		item.Views = viewText
		item.ViewsCount = ParseViewCount(viewText)
	}

	// Check for premium/gold
	item.IsPremium = IsPremiumContent(s, item.URL)
	if item.IsPremium {
		return nil
	}

	return item
}
