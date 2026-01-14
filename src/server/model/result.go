// SPDX-License-Identifier: MIT
package model

import (
	"time"
)

// VideoResult represents a single video search result
// Per AI.md PART 1: "Result" alone is ambiguous - result of what?
type VideoResult struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	URL             string    `json:"url"`
	Thumbnail       string    `json:"thumbnail"`
	PreviewURL      string    `json:"preview_url,omitempty"`
	DownloadURL     string    `json:"download_url,omitempty"`
	Duration        string    `json:"duration"`
	DurationSeconds int       `json:"duration_seconds"`
	Views           string    `json:"views"`
	ViewsCount      int64     `json:"views_count"`
	Rating          float64   `json:"rating,omitempty"`
	Quality         string    `json:"quality,omitempty"`
	Source          string    `json:"source"`
	SourceDisplay   string    `json:"source_display"`
	Published       time.Time `json:"published,omitempty"`
	Description     string    `json:"description,omitempty"`
}

// SearchResponse represents the API response for a search
// Per AI.md PART 14: Error response format uses error (code) + message (human-readable)
type SearchResponse struct {
	Ok         bool             `json:"ok"`
	Data       SearchData       `json:"data"`
	Pagination PaginationData   `json:"pagination"`
	Error      string           `json:"error,omitempty"`   // ERROR_CODE (machine-readable)
	Message    string           `json:"message,omitempty"` // Human-readable message
}

// SearchData holds the search results and metadata
// SearchQuery is the query after bang parsing
// HasBang indicates whether bangs were used
// BangEngines contains engines from bang parsing
// Cached indicates whether results came from cache
type SearchData struct {
	Query         string   `json:"query"`
	SearchQuery   string   `json:"search_query,omitempty"`
	Results       []VideoResult `json:"results"`
	EnginesUsed   []string `json:"engines_used"`
	EnginesFailed []string `json:"engines_failed"`
	SearchTimeMS  int64    `json:"search_time_ms"`
	HasBang       bool     `json:"has_bang,omitempty"`
	BangEngines   []string `json:"bang_engines,omitempty"`
	Cached        bool     `json:"cached,omitempty"`
}

// PaginationData holds pagination information
type PaginationData struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
	Pages int `json:"pages"`
}

// EngineInfo represents information about a search engine
type EngineInfo struct {
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Enabled     bool              `json:"enabled"`
	Available   bool              `json:"available"`
	Features    []string          `json:"features"`
	Tier        int               `json:"tier"`
	Capabilities *EngineCapabilities `json:"capabilities,omitempty"`
}

// EngineCapabilities represents engine feature support
type EngineCapabilities struct {
	HasPreview  bool `json:"has_preview"`
	HasDownload bool `json:"has_download"`
}

// EnginesResponse represents the API response for engines list
type EnginesResponse struct {
	Ok      bool         `json:"ok"`
	Data    []EngineInfo `json:"data"`
}
