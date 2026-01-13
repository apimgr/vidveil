// SPDX-License-Identifier: MIT
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/apimgr/vidveil/src/config"
	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/tor"
)

// extractJSON extracts a JSON object from bytes by counting braces
func extractJSON(data []byte) []byte {
	if len(data) == 0 || data[0] != '{' {
		return nil
	}

	depth := 0
	inString := false
	escaped := false

	for i, b := range data {
		if escaped {
			escaped = false
			continue
		}

		if b == '\\' && inString {
			escaped = true
			continue
		}

		if b == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if b == '{' {
			depth++
		} else if b == '}' {
			depth--
			if depth == 0 {
				return data[:i+1]
			}
		}
	}

	return nil
}

// XHamsterEngine searches xHamster
type XHamsterEngine struct{ *BaseEngine }

// xhamsterInitials represents the window.initials JSON structure
type xhamsterInitials struct {
	SearchResult struct {
		VideoThumbProps []xhamsterVideo `json:"videoThumbProps"`
	} `json:"searchResult"`
}

type xhamsterVideo struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Duration  int    `json:"duration"`
	Views     int    `json:"views"`
	PageURL   string `json:"pageURL"`
	ThumbURL  string `json:"thumbURL"`
	Created   int64  `json:"created"`
}

// NewXHamsterEngine creates a new xHamster engine
func NewXHamsterEngine(cfg *config.Config, torClient *tor.Client) *XHamsterEngine {
	e := &XHamsterEngine{NewBaseEngine("xhamster", "xHamster", "https://xhamster.com", 1, cfg, torClient)}
	// Set capabilities per IDEA.md
	e.SetCapabilities(Capabilities{
		HasPreview:    false, // JSON extraction doesn't include preview URLs
		HasDownload:   false,
		HasDuration:   true,
		HasViews:      true,
		HasRating:     false,
		HasQuality:    false,
		HasUploadDate: true,
		PreviewSource: "",
		APIType:       "json_extraction",
	})
	return e
}

// Search performs a search on xHamster by extracting JSON from the page
func (e *XHamsterEngine) Search(ctx context.Context, query string, page int) ([]model.VideoResult, error) {
	// xHamster uses /search/{query} for page 1, /search/{query}/{page} for others
	var searchURL string
	if page <= 1 {
		searchURL = fmt.Sprintf("%s/search/%s", e.baseURL, strings.ReplaceAll(query, " ", "+"))
	} else {
		searchURL = fmt.Sprintf("%s/search/%s/%d", e.baseURL, strings.ReplaceAll(query, " ", "+"), page)
	}

	resp, err := e.MakeRequest(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	// Extract window.initials JSON from the page
	// Find the start of the JSON
	marker := []byte("window.initials=")
	idx := strings.Index(string(body), string(marker))
	if idx == -1 {
		return nil, fmt.Errorf("failed to find initials marker")
	}

	// Extract JSON by finding matching braces
	jsonStart := idx + len(marker)
	jsonBytes := extractJSON(body[jsonStart:])
	if jsonBytes == nil {
		return nil, fmt.Errorf("failed to extract JSON")
	}

	var initials xhamsterInitials
	if err := json.Unmarshal(jsonBytes, &initials); err != nil {
		return nil, fmt.Errorf("failed to parse initials: %w", err)
	}

	var results []model.VideoResult
	for _, v := range initials.SearchResult.VideoThumbProps {
		if v.Title == "" || v.PageURL == "" {
			continue
		}

		// Format duration
		duration := formatDuration(v.Duration)

		// Format views
		views := formatViewCount(v.Views)

		results = append(results, model.VideoResult{
			ID:              GenerateResultID(v.PageURL, e.Name()),
			URL:             v.PageURL,
			Title:           v.Title,
			Thumbnail:       v.ThumbURL,
			Duration:        duration,
			DurationSeconds: v.Duration,
			Views:           views,
			ViewsCount:      int64(v.Views),
			Source:          e.Name(),
			SourceDisplay:   e.DisplayName(),
		})
	}

	return results, nil
}

// SupportsFeature returns whether the engine supports a feature
func (e *XHamsterEngine) SupportsFeature(feature Feature) bool {
	return feature == FeaturePagination
}

// formatDuration formats seconds into MM:SS or HH:MM:SS
func formatDuration(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}
