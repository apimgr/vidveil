// SPDX-License-Identifier: MIT
package handler

import (
	"net"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type recentSearch struct {
	Query string
	At    time.Time
}

var (
	recentEmailPattern  = regexp.MustCompile(`(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\b`)
	recentStreetPattern = regexp.MustCompile(`(?i)\b\d+\s+\w+(?:\s+\w+){0,3}\s+(?:street|st|avenue|ave|road|rd|lane|ln|drive|dr|blvd|court|way|suite|apt)\b|\bP\.?\s*O\.?\s*Box\s+\d+\b`)
	recentSSNPattern    = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	recentDigitRun      = regexp.MustCompile(`\d{9,}`)
	recentPhonePattern  = regexp.MustCompile(`\+?[0-9 .()\-]{7,}`)
	recentTokenPattern  = regexp.MustCompile(`[A-Za-z0-9+/_=-]{20,}`)
	recentSensitiveURL  = regexp.MustCompile(`(?i)[?&](?:token|(?:api_)?key|password|secret)=`)
)

func (h *SearchHandler) recordRecentSearch(query string) {
	query = strings.TrimSpace(query)
	if unsafeRecentSearch(query) {
		return
	}
	h.recentSearchMu.Lock()
	defer h.recentSearchMu.Unlock()
	for _, entry := range h.recentSearches {
		if entry.Query == query {
			entry.At = time.Now().UTC()
			h.recentSearches = append([]recentSearch{entry}, h.recentSearchesWithout(query)...)
			return
		}
	}
	h.recentSearches = append([]recentSearch{{Query: query, At: time.Now().UTC()}}, h.recentSearches...)
	maxItems := 50
	if h.appConfig != nil && h.appConfig.Server.SEO.Feed.MaxItems >= 1 && h.appConfig.Server.SEO.Feed.MaxItems <= 200 {
		maxItems = h.appConfig.Server.SEO.Feed.MaxItems
	}
	if len(h.recentSearches) > maxItems {
		h.recentSearches = h.recentSearches[:maxItems]
	}
}

// RecordRecentSearch records a query after applying the feed's privacy filters.
func (h *SearchHandler) RecordRecentSearch(query string) {
	h.recordRecentSearch(query)
}

func (h *SearchHandler) recentSearchSnapshot() []recentSearch {
	h.recentSearchMu.Lock()
	defer h.recentSearchMu.Unlock()
	items := make([]recentSearch, 0, len(h.recentSearches))
	for _, item := range h.recentSearches {
		if !unsafeRecentSearch(item.Query) {
			items = append(items, item)
		}
	}
	return items
}

func (h *SearchHandler) recentSearchesWithout(query string) []recentSearch {
	items := make([]recentSearch, 0, len(h.recentSearches)-1)
	for _, item := range h.recentSearches {
		if item.Query != query {
			items = append(items, item)
		}
	}
	return items
}

func unsafeRecentSearch(query string) bool {
	if strings.TrimSpace(query) == "" || utf8.RuneCountInString(query) > 100 || recentEmailPattern.MatchString(query) || recentStreetPattern.MatchString(query) || recentSSNPattern.MatchString(query) || recentSensitiveURL.MatchString(query) {
		return true
	}
	for _, phone := range recentPhonePattern.FindAllString(query, -1) {
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, phone)
		if len(digits) >= 7 {
			return true
		}
	}
	for _, run := range recentDigitRun.FindAllString(query, -1) {
		if len(run) >= 9 || (len(run) >= 13 && len(run) <= 19 && passesLuhn(run)) {
			return true
		}
	}
	for _, token := range recentTokenPattern.FindAllString(query, -1) {
		hasLetter, hasDigit := false, false
		for _, r := range token {
			hasLetter = hasLetter || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
			hasDigit = hasDigit || (r >= '0' && r <= '9')
		}
		if hasLetter && hasDigit {
			return true
		}
	}
	for _, candidate := range strings.FieldsFunc(query, func(r rune) bool {
		return !(r == ':' || r == '.' || r == '[' || r == ']' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
	}) {
		if net.ParseIP(strings.Trim(candidate, "[]")) != nil {
			return true
		}
	}
	return false
}

func passesLuhn(value string) bool {
	sum, double := 0, false
	for i := len(value) - 1; i >= 0; i-- {
		digit := int(value[i] - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}
	return sum%10 == 0
}
