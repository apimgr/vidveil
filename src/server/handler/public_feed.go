// SPDX-License-Identifier: MIT
package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/service/urlvar"
)

func (h *SearchHandler) PublicFeed(w http.ResponseWriter, r *http.Request, atom bool) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	cfg := h.appConfig
	if cfg == nil {
		cfg = config.DefaultAppConfig()
	}
	title := strings.TrimSpace(cfg.Server.SEO.Feed.Title)
	if title == "" {
		title = cfg.Server.Branding.Title
	}
	if title == "" {
		title = "vidveil"
	}
	items := h.recentSearchSnapshot()
	if cfg.Server.SEO.Feed.MaxItems >= 1 && cfg.Server.SEO.Feed.MaxItems <= 200 && len(items) > cfg.Server.SEO.Feed.MaxItems {
		items = items[:cfg.Server.SEO.Feed.MaxItems]
	}
	updated := time.Now().UTC().Truncate(24 * time.Hour)
	if len(items) > 0 {
		updated = items[0].At
	}
	updated = updated.Truncate(time.Second)
	var data []byte
	if atom {
		feed := struct {
			XMLName xml.Name `xml:"feed"`
			XMLNS   string   `xml:"xmlns,attr"`
			Title   string   `xml:"title"`
			ID      string   `xml:"id"`
			Updated string   `xml:"updated"`
			Link    struct {
				Href string `xml:"href,attr"`
				Rel  string `xml:"rel,attr"`
				Type string `xml:"type,attr"`
			} `xml:"link"`
			Entries []struct {
				Title   string `xml:"title"`
				Summary string `xml:"summary"`
				ID      string `xml:"id"`
				Updated string `xml:"updated"`
				Link    struct {
					Href string `xml:"href,attr"`
				} `xml:"link"`
			} `xml:"entry"`
		}{XMLNS: "http://www.w3.org/2005/Atom", Title: title, ID: urlvar.BuildURL(r, "/feed.xml"), Updated: updated.Format(time.RFC3339)}
		feed.Link.Href, feed.Link.Rel, feed.Link.Type = urlvar.BuildURL(r, "/feed.xml"), "self", "application/atom+xml"
		for _, item := range items {
			permalink := urlvar.BuildURL(r, "/search?q="+url.QueryEscape(item.Query))
			title := truncateRunes(cleanXMLText(item.Query), 140)
			summary := truncateRunes(cleanXMLText(item.Query), 280)
			entry := struct {
				Title   string `xml:"title"`
				Summary string `xml:"summary"`
				ID      string `xml:"id"`
				Updated string `xml:"updated"`
				Link    struct {
					Href string `xml:"href,attr"`
				} `xml:"link"`
			}{Title: title, Summary: summary, ID: permalink, Updated: item.At.Format(time.RFC3339)}
			entry.Link.Href = permalink
			feed.Entries = append(feed.Entries, entry)
		}
		data, _ = xml.Marshal(feed)
		w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	} else {
		feed := struct {
			XMLName xml.Name `xml:"rss"`
			Version string   `xml:"version,attr"`
			Channel struct {
				Title string `xml:"title"`
				Link  string `xml:"link"`
				Desc  string `xml:"description"`
				Date  string `xml:"lastBuildDate"`
				Items []struct {
					Title       string `xml:"title"`
					Description string `xml:"description"`
					Link        string `xml:"link"`
					GUID        string `xml:"guid"`
					Date        string `xml:"pubDate"`
				} `xml:"item"`
			} `xml:"channel"`
		}{Version: "2.0"}
		feed.Channel.Title, feed.Channel.Link = title, urlvar.BuildURL(r, "/rss.xml")
		feed.Channel.Desc, feed.Channel.Date = title, updated.Format(time.RFC822Z)
		for _, item := range items {
			permalink := urlvar.BuildURL(r, "/search?q="+url.QueryEscape(item.Query))
			feed.Channel.Items = append(feed.Channel.Items, struct {
				Title       string `xml:"title"`
				Description string `xml:"description"`
				Link        string `xml:"link"`
				GUID        string `xml:"guid"`
				Date        string `xml:"pubDate"`
			}{Title: truncateRunes(cleanXMLText(item.Query), 140), Description: truncateRunes(cleanXMLText(item.Query), 280), Link: permalink, GUID: permalink, Date: item.At.Format(time.RFC822Z)})
		}
		data, _ = xml.Marshal(feed)
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	}
	body := append([]byte(xml.Header), data...)
	tag := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(tag[:]) + `"`
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", updated.Format(http.TimeFormat))
	if etagListMatches(r.Header.Get("If-None-Match"), etag) || (r.Header.Get("If-None-Match") == "" && r.Header.Get("If-Modified-Since") != "" && !updated.After(parseHTTPDate(r.Header.Get("If-Modified-Since")))) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

func etagListMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}

func cleanXMLText(value string) string {
	return strings.Map(func(r rune) rune {
		if r == 0x9 || r == 0xA || r == 0xD || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF {
			return r
		}
		return -1
	}, value)
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func parseHTTPDate(value string) time.Time {
	parsed, err := time.Parse(http.TimeFormat, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
