// SPDX-License-Identifier: MIT
// AI.md PART 32: CLI Client - API Client
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// API client defaults
// Per AI.md PART 1: No magic strings/numbers - use named constants
const (
	APIClientDefaultServerURL      = ""
	APIClientDefaultAPIVersion     = "v1"
	APIClientDefaultTimeoutSeconds = 30
)

// APIClient is the API client for VidVeil
type APIClient struct {
	baseURL    string
	apiVersion string
	token      string
	httpClient *http.Client
	userAgent  string
}

// SearchResult represents a single search result
// Per AI.md PART 32: field tags match the server's actual /api/v1/search
// result schema (src/server/model/result.go) - "source" is the engine slug.
type SearchResult struct {
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Thumbnail   string   `json:"thumbnail"`
	Duration    string   `json:"duration"`
	Views       string   `json:"views"`
	Engine      string   `json:"source"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// SearchResponse is the API response for search.
// Per AI.md PART 14, the server wraps search results in an "ok"/"data"/
// "pagination" envelope, not a flat top-level object - see UnmarshalJSON.
type SearchResponse struct {
	Ok           bool           `json:"ok"`
	Query        string         `json:"query"`
	Results      []SearchResult `json:"results"`
	Count        int            `json:"count"`
	SearchTimeMS int64          `json:"search_time"`
	Error        string         `json:"error,omitempty"`
}

// searchResponseWire mirrors the actual wire shape returned by
// GET /api/{version}/search: {"ok":bool,"data":{...},"pagination":{...}}.
type searchResponseWire struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  struct {
		Query        string         `json:"query"`
		Results      []SearchResult `json:"results"`
		SearchTimeMS int64          `json:"search_time_ms"`
	} `json:"data"`
	Pagination struct {
		Total int `json:"total"`
	} `json:"pagination"`
}

// UnmarshalJSON adapts the server's nested ok/data/pagination envelope onto
// the flat SearchResponse fields consumed throughout src/client/cmd.
func (s *SearchResponse) UnmarshalJSON(raw []byte) error {
	var wire searchResponseWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return err
	}

	s.Ok = wire.Ok
	s.Error = wire.Error
	s.Query = wire.Data.Query
	s.Results = wire.Data.Results
	s.SearchTimeMS = wire.Data.SearchTimeMS
	if wire.Pagination.Total > 0 {
		s.Count = wire.Pagination.Total
	} else {
		s.Count = len(wire.Data.Results)
	}

	return nil
}

// VersionResponse is the API response for version
type VersionResponse struct {
	Ok      bool   `json:"ok"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Built   string `json:"build_date"`
}

// AutodiscoverResponse is the non-versioned client bootstrap response.
type AutodiscoverResponse struct {
	Primary    string `json:"primary"`
	APIVersion string `json:"api_version"`
	Timeout    int    `json:"timeout"`
	Retry      int    `json:"retry"`
	RetryDelay int    `json:"retry_delay"`
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL, token string, timeout int, apiVersion string) *APIClient {
	if baseURL == "" {
		baseURL = APIClientDefaultServerURL
	}
	apiVersion = strings.Trim(strings.TrimSpace(apiVersion), "/")
	if apiVersion == "" {
		apiVersion = APIClientDefaultAPIVersion
	}
	if timeout <= 0 {
		timeout = APIClientDefaultTimeoutSeconds
	}

	return &APIClient{
		baseURL:    baseURL,
		apiVersion: apiVersion,
		token:      token,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		// AI.md PART 32: User-Agent always uses original project name
		userAgent: "vidveil-cli/dev",
	}
}

// SetUserAgent sets the user agent (called from main with version)
func (c *APIClient) SetUserAgent(version string) {
	c.userAgent = fmt.Sprintf("vidveil-cli/%s", version)
}

// Search performs a video search
func (c *APIClient) Search(query string, page, limit int, engines []string) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("q", query)
	if page > 0 {
		params.Set("page", fmt.Sprintf("%d", page))
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if len(engines) > 0 {
		for _, e := range engines {
			params.Add("engines", e)
		}
	}

	url := fmt.Sprintf("%s/search?%s", c.GetAPIBaseURL(), params.Encode())

	var resp SearchResponse
	if err := c.get(url, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// GetVersion gets server version info
func (c *APIClient) GetVersion() (*VersionResponse, error) {
	url := fmt.Sprintf("%s/version", c.GetAPIBaseURL())

	var resp VersionResponse
	if err := c.get(url, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Autodiscover gets server connection defaults from the non-versioned autodiscover endpoint.
func (c *APIClient) Autodiscover() (*AutodiscoverResponse, error) {
	url := fmt.Sprintf("%s/api/autodiscover", c.baseURL)

	var resp AutodiscoverResponse
	if err := c.get(url, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Health checks if the server is reachable
func (c *APIClient) Health() (bool, error) {
	url := fmt.Sprintf("%s/server/healthz", c.GetAPIBaseURL())

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

// GetBaseURL returns the base URL of the server
func (c *APIClient) GetBaseURL() string {
	return c.baseURL
}

// GetAPIBaseURL returns the versioned API base URL.
func (c *APIClient) GetAPIBaseURL() string {
	return fmt.Sprintf("%s/api/%s", c.baseURL, c.apiVersion)
}

// FetchURLResponseBytes performs a GET request and returns response body as bytes
func (c *APIClient) FetchURLResponseBytes(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to server at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// get performs a GET request and decodes JSON response
func (c *APIClient) get(url string, result interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot connect to server at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}
