// SPDX-License-Identifier: MIT
package handler

import (
	"strings"
	"testing"
)

func TestUnsafeRecentSearch(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{name: "empty", query: "  ", want: true},
		{name: "ordinary query", query: "privacy respecting search", want: false},
		{name: "phone number", query: "call +1 (212) 555-0199", want: true},
		{name: "email address", query: "person@example.com", want: true},
		{name: "email without dotted domain", query: "name@host", want: true},
		{name: "street address", query: "123 Main Street", want: true},
		{name: "po box", query: "PO Box 1234", want: true},
		{name: "government number", query: "123456789", want: true},
		{name: "luhn card number", query: "4111111111111111", want: true},
		{name: "ssn", query: "123-45-6789", want: true},
		{name: "ipv4", query: "192.0.2.10", want: true},
		{name: "ipv6", query: "2001:db8::1", want: true},
		{name: "token-like", query: "abcDEF0123456789abcdef", want: true},
		{name: "credential URL", query: "https://example.com/?key=secret", want: true},
		{name: "too long", query: strings.Repeat("x", 101), want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := unsafeRecentSearch(test.query); got != test.want {
				t.Errorf("unsafeRecentSearch(%q) = %t, want %t", test.query, got, test.want)
			}
		})
	}
}

func TestRecordRecentSearchFiltersAndBounds(t *testing.T) {
	h := &SearchHandler{}
	h.recordRecentSearch("person@example.com")
	h.recordRecentSearch("privacy search")
	h.recordRecentSearch("privacy search")
	if got := h.recentSearchSnapshot(); len(got) != 1 || got[0].Query != "privacy search" {
		t.Fatalf("recent searches = %#v, want one safe unique query", got)
	}

	h.appConfig = createTestConfig()
	h.appConfig.Server.SEO.Feed.MaxItems = 2
	h.recentSearches = nil
	for _, query := range []string{"first search", "second search", "third search"} {
		h.recordRecentSearch(query)
	}
	got := h.recentSearchSnapshot()
	if len(got) != 2 || got[0].Query != "third search" || got[1].Query != "second search" {
		t.Fatalf("bounded recent searches = %#v, want newest two", got)
	}
}
