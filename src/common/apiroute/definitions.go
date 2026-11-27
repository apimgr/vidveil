// SPDX-License-Identifier: MIT
package apiroute

import (
	"encoding/json"
	"net/http"
)

// Envelope is the common success response used by JSON API operations.
type Envelope struct {
	OK      bool   `json:"ok"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

type HelpPageLink struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

type ServerHelpData struct {
	Title   string         `json:"title"`
	Content string         `json:"content"`
	Format  string         `json:"format"`
	Pages   []HelpPageLink `json:"pages"`
}

// ErrorEnvelope is the common error response used by JSON API operations.
type ErrorEnvelope struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// SearchQueryParameters describes the query parameters accepted by search APIs.
type SearchQueryParameters struct {
	Q       string `json:"q"`
	Page    int    `json:"page,omitempty"`
	Engines string `json:"engines,omitempty"`
}

// SearchResultData mirrors the stable search response shape exposed by the handlers.
type SearchResultData struct {
	Query           string         `json:"query"`
	SearchQuery     string         `json:"search_query,omitempty"`
	Results         []VideoResult  `json:"results"`
	EnginesUsed     []string       `json:"engines_used"`
	EnginesFailed   []string       `json:"engines_failed"`
	SearchTimeMS    int64          `json:"search_time_ms"`
	HasBang         bool           `json:"has_bang,omitempty"`
	BangEngines     []string       `json:"bang_engines,omitempty"`
	Cached          bool           `json:"cached,omitempty"`
	EngineStats     map[string]any `json:"engine_stats,omitempty"`
	RelatedSearches []string       `json:"related_searches,omitempty"`
	SpellSuggestion string         `json:"spell_suggestion,omitempty"`
	InvalidBang     string         `json:"invalid_bang,omitempty"`
	Display         map[string]any `json:"display,omitempty"`
}

// VideoResult is one video result returned by a search operation.
type VideoResult struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	URL             string   `json:"url"`
	Thumbnail       string   `json:"thumbnail"`
	PreviewURL      string   `json:"preview_url,omitempty"`
	DownloadURL     string   `json:"download_url,omitempty"`
	Duration        string   `json:"duration"`
	DurationSeconds int      `json:"duration_seconds"`
	Views           string   `json:"views"`
	ViewsCount      int64    `json:"views_count"`
	Rating          float64  `json:"rating,omitempty"`
	Quality         string   `json:"quality,omitempty"`
	Source          string   `json:"source"`
	SourceDisplay   string   `json:"source_display"`
	Published       string   `json:"published,omitempty"`
	Description     string   `json:"description,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Performer       string   `json:"performer,omitempty"`
}

// SearchResultResponse is the JSON search response.
type SearchResultResponse struct {
	OK         bool             `json:"ok"`
	Data       SearchResultData `json:"data"`
	Pagination map[string]int   `json:"pagination"`
	Error      string           `json:"error,omitempty"`
	Message    string           `json:"message,omitempty"`
}

// BatchSearchRequest describes the bounded batch-search request body.
type BatchSearchRequest struct {
	Queries []BatchSearchQuery `json:"queries"`
}
type BatchSearchQuery struct {
	Q       string `json:"q"`
	Page    int    `json:"page"`
	Engines string `json:"engines,omitempty"`
}
type BatchSearchResponse struct {
	OK   bool                   `json:"ok"`
	Data []SearchResultResponse `json:"data"`
}

type AutocompleteResponse struct {
	OK          bool   `json:"ok"`
	Suggestions any    `json:"suggestions"`
	Type        string `json:"type"`
	Replace     string `json:"replace,omitempty"`
}
type EngineCapabilities struct {
	HasPreview  bool `json:"has_preview"`
	HasDownload bool `json:"has_download"`
	HasDuration bool `json:"has_duration"`
	HasViews    bool `json:"has_views"`
}
type EnginePrivacyData struct {
	RequiresJS  bool `json:"requires_js"`
	SetsCookies bool `json:"sets_cookies"`
	HasTracking bool `json:"has_tracking"`
}
type EngineModel struct {
	Name         string              `json:"name"`
	DisplayName  string              `json:"display_name"`
	Enabled      bool                `json:"enabled"`
	Available    bool                `json:"available"`
	Features     []string            `json:"features"`
	Tier         int                 `json:"tier"`
	Capabilities *EngineCapabilities `json:"capabilities,omitempty"`
	Privacy      EnginePrivacyData   `json:"privacy"`
}
type EngineHealthData struct {
	CircuitState     string  `json:"circuit_state"`
	CircuitFailures  int     `json:"circuit_failures"`
	LastFailureAt    string  `json:"last_failure_at"`
	TotalSuccesses   uint64  `json:"total_successes"`
	TotalFailures    uint64  `json:"total_failures"`
	LastSuccessAt    string  `json:"last_success_at"`
	AvgLatencyMS     int64   `json:"avg_latency_ms"`
	UptimePct        float64 `json:"uptime_pct"`
	RateLimitedUntil string  `json:"rate_limited_until"`
	IsRateLimited    bool    `json:"is_rate_limited"`
}
type EngineHealthModel struct {
	EngineModel
	Health EngineHealthData `json:"health"`
}
type EngineListResponse struct {
	OK   bool          `json:"ok"`
	Data []EngineModel `json:"data"`
}
type EngineHealthResponse struct {
	OK   bool                `json:"ok"`
	Data []EngineHealthModel `json:"data"`
}
type EngineDetailsResponse struct {
	OK   bool        `json:"ok"`
	Data EngineModel `json:"data"`
}
type BangModel struct {
	Bang        string `json:"bang"`
	EngineName  string `json:"engine_name"`
	DisplayName string `json:"display_name"`
	ShortCode   string `json:"short_code"`
}
type BangListResponse struct {
	OK    bool        `json:"ok"`
	Data  []BangModel `json:"data"`
	Count int         `json:"count"`
}
type StatsData struct {
	EnginesEnabled int `json:"engines_enabled"`
	EnginesTotal   int `json:"engines_total"`
}
type StatsResponse struct {
	OK   bool      `json:"ok"`
	Data StatsData `json:"data"`
}
type VersionResponse struct {
	OK           bool   `json:"ok"`
	Version      string `json:"version"`
	Commit       string `json:"commit"`
	BuildDate    string `json:"build_date"`
	OfficialSite string `json:"official_site"`
}
type AboutData struct {
	Name        string            `json:"name"`
	Tagline     string            `json:"tagline"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Features    []string          `json:"features"`
	Links       map[string]string `json:"links"`
}
type PrivacySummary struct {
	DataStoredOnServer bool `json:"data_stored_on_server"`
	DataSold           bool `json:"data_sold"`
	UserControl        bool `json:"user_control"`
}
type PrivacyCollection struct {
	UserAccounts       bool `json:"user_accounts"`
	BrowserFingerprint bool `json:"browser_fingerprint"`
	UserProfiles       bool `json:"user_profiles"`
	ThirdPartySharing  bool `json:"third_party_sharing"`
	DataSold           bool `json:"data_sold"`
}
type PrivacyData struct {
	PolicyVersion     string            `json:"policy_version"`
	LastUpdated       string            `json:"last_updated"`
	Summary           PrivacySummary    `json:"summary"`
	DataCollection    PrivacyCollection `json:"data_collection"`
	ClientSideStorage []string          `json:"client_side_storage"`
	ServerSideStorage []string          `json:"server_side_storage"`
	CookiesUsed       []string          `json:"cookies_used"`
	ThumbnailProxy    bool              `json:"thumbnail_proxy"`
	TorSupported      bool              `json:"tor_supported"`
	ThirdPartySent    string            `json:"third_party_sent"`
}
type TermsData struct {
	TermsVersion string   `json:"terms_version"`
	LastUpdated  string   `json:"last_updated"`
	Summary      string   `json:"summary"`
	Sections     []string `json:"sections"`
}

// Favorite and related models match the owner-scoped REST API.
type FavoriteModel struct {
	ID        int64  `json:"id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Source    string `json:"source,omitempty"`
	CreatedAt string `json:"created_at"`
}
type FavoriteListData struct {
	Favorites []FavoriteModel `json:"favorites"`
}
type FavoriteMutationRequest struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Source    string `json:"source,omitempty"`
}
type FavoriteMutationData struct {
	Message    string `json:"message"`
	OwnerToken string `json:"owner_token,omitempty"`
}

type PreferencesData struct {
	Theme string            `json:"theme"`
	Lang  string            `json:"lang"`
	Prefs map[string]string `json:"prefs"`
}
type PreferencesUpdateData struct {
	Theme   string            `json:"theme"`
	Lang    string            `json:"lang"`
	Prefs   map[string]string `json:"prefs"`
	Applied []string          `json:"applied,omitempty"`
	Ignored []string          `json:"ignored,omitempty"`
}
type PreferencesExportData struct {
	PreferencesData
	URL  string `json:"url"`
	Code string `json:"code"`
}
type ContactRequest struct {
	Name                string `json:"name,omitempty"`
	Email               string `json:"email,omitempty"`
	Subject             string `json:"subject,omitempty"`
	Message             string `json:"message,omitempty"`
	Type                string `json:"type,omitempty"`
	Captcha             string `json:"captcha,omitempty"`
	FormToken           string `json:"form_token,omitempty"`
	ReviewToken         string `json:"review_token,omitempty"`
	Step                string `json:"step,omitempty"`
	Confirmed           string `json:"confirmed,omitempty"`
	SecurityID          string `json:"security_id,omitempty"`
	Component           string `json:"component,omitempty"`
	Severity            string `json:"severity,omitempty"`
	Summary             string `json:"summary,omitempty"`
	Steps               string `json:"steps,omitempty"`
	Impact              string `json:"impact,omitempty"`
	Endpoint            string `json:"endpoint,omitempty"`
	SuggestedFix        string `json:"suggested_fix,omitempty"`
	CreditPreference    string `json:"credit_preference,omitempty"`
	CreditHandle        string `json:"credit_handle,omitempty"`
	DisclosureAgreement string `json:"disclosure_agreement,omitempty"`
	DisclosureDays      int    `json:"disclosure_days,omitempty"`
	ResearcherGPG       string `json:"researcher_gpg,omitempty"`
}
type ContactSubmissionData struct {
	Message    string `json:"message,omitempty"`
	TrackingID string `json:"tracking_id,omitempty"`
}
type TokenCreateRequest struct {
	Name    string `json:"name"`
	Expires string `json:"expires"`
}
type TokenMetadata struct {
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
	Prefix     string `json:"prefix"`
	Suffix     string `json:"suffix"`
	CreatedAt  any    `json:"created_at"`
	LastUsedAt any    `json:"last_used_at"`
	ExpiresAt  any    `json:"expires_at,omitempty"`
	Status     string `json:"status,omitempty"`
}
type TokenListData struct {
	Primary TokenMetadata   `json:"primary"`
	Linked  []TokenMetadata `json:"linked"`
}
type TokenData struct {
	Token      string `json:"token,omitempty"`
	OwnerToken string `json:"owner_token,omitempty"`
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
	Updated    *bool  `json:"updated,omitempty"`
	Revoked    *bool  `json:"revoked,omitempty"`
}
type ProxyQuery struct {
	URL string `json:"url"`
}
type GraphQLRequest struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables,omitempty"`
	OperationName string         `json:"operationName,omitempty"`
}
type GraphQLResponse struct {
	Data   map[string]any   `json:"data,omitempty"`
	Errors []map[string]any `json:"errors,omitempty"`
}
type BrowserReportRequest = json.RawMessage
type EmptyResponse struct{}
type AutodiscoverRequest struct {
	Channel string `json:"channel,omitempty"`
}
type AutodiscoverResponse struct {
	OK   bool             `json:"ok"`
	Data AutodiscoverData `json:"data"`
}
type AutodiscoverData struct {
	Name          string         `json:"name"`
	Version       string         `json:"version"`
	API           map[string]any `json:"api"`
	URLs          []string       `json:"urls"`
	Onion         string         `json:"onion,omitempty"`
	I2P           string         `json:"i2p,omitempty"`
	CLIMinVersion string         `json:"cli_min_version"`
	CLIVersions   map[string]any `json:"cli_versions"`
}
type APIAuthData struct {
	Kind string `json:"kind"`
}

// HealthStatusResponse mirrors the public health endpoint's fixed top-level structure.
type HealthStatusResponse struct {
	Project        map[string]string `json:"project"`
	Status         string            `json:"status"`
	PendingRestart bool              `json:"pending_restart,omitempty"`
	RestartReason  []string          `json:"restart_reason,omitempty"`
	Version        string            `json:"version"`
	GoVersion      string            `json:"go_version"`
	Build          map[string]string `json:"build"`
	Uptime         string            `json:"uptime"`
	Mode           string            `json:"mode"`
	Timestamp      string            `json:"timestamp"`
	Maintenance    any               `json:"maintenance,omitempty"`
	Features       map[string]any    `json:"features"`
	Checks         map[string]string `json:"checks"`
	Stats          map[string]any    `json:"stats"`
}

// SecurityContact is one configured RFC 9116 contact channel.
type SecurityContact struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// SecurityPGP describes the published public-key availability.
type SecurityPGP struct {
	Available   bool    `json:"available"`
	URL         *string `json:"url"`
	Fingerprint *string `json:"fingerprint"`
}

// SecurityOverviewData is the public security overview response model.
type SecurityOverviewData struct {
	Contacts  []SecurityContact `json:"contacts"`
	Expires   string            `json:"expires"`
	PGP       SecurityPGP       `json:"pgp"`
	PolicyURL string            `json:"policy_url"`
	ThanksURL string            `json:"thanks_url"`
	ReportURL *string           `json:"report_url,omitempty"`
}

// SecurityOverviewResponse is the wrapped security overview response.
type SecurityOverviewResponse struct {
	OK   bool                 `json:"ok"`
	Data SecurityOverviewData `json:"data"`
}

// SecurityPolicyData is the public security policy response model.
type SecurityPolicyData struct {
	Policy string `json:"policy"`
}

// SecurityPolicyResponse is the wrapped security policy response.
type SecurityPolicyResponse struct {
	OK   bool               `json:"ok"`
	Data SecurityPolicyData `json:"data"`
}

// SecurityThanksEntry is a contact-free disclosed researcher credit.
type SecurityThanksEntry struct {
	Credit   string `json:"credit"`
	Date     string `json:"date"`
	Severity string `json:"severity"`
}

// SecurityThanksData is the public hall-of-fame response model.
type SecurityThanksData struct {
	Entries []SecurityThanksEntry `json:"entries"`
}

// SecurityThanksResponse is the wrapped security thanks response.
type SecurityThanksResponse struct {
	OK   bool               `json:"ok"`
	Data SecurityThanksData `json:"data"`
}

// MCPRequest is the JSON-RPC envelope accepted by the MCP transport.
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPError describes a JSON-RPC protocol error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPResponse is the JSON-RPC response envelope returned by the MCP transport.
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// routeDefinitions is the canonical static API inventory. Runtime handlers
// bind to these operation IDs; generators consume this same metadata.
var routeDefinitions = []Definition{
	{Method: http.MethodGet, Path: "/api/{api_version}/search", OperationID: "searchVideos", Summary: "Search videos", Auth: "none", Listed: true, Request: SearchQueryParameters{}, Response: SearchResultResponse{}, Primary: "results", Message: "Retrieved successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/search/batch", OperationID: "batchSearch", Summary: "Batch search", Auth: "none", Listed: true, Request: BatchSearchRequest{}, Response: BatchSearchResponse{}, Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/bangs", OperationID: "listBangs", Summary: "List search bangs", Auth: "none", Listed: true, Request: map[string]any{}, Response: BangListResponse{}, Primary: "bangs", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/bangs/autocomplete", OperationID: "autocompleteBangs", Summary: "Autocomplete search bangs", Auth: "none", Listed: true, Request: map[string]any{"q": "string"}, Response: AutocompleteResponse{}, Primary: "suggestions", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/engines", OperationID: "listEngines", Summary: "List search engines", Auth: "none", Listed: true, Request: map[string]any{}, Response: EngineListResponse{}, Primary: "engines", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/engines/health", OperationID: "getEngineHealth", Summary: "Engine health", Auth: "none", Listed: true, Request: map[string]any{}, Response: EngineHealthResponse{}, Primary: "engines", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/engines/{name}", OperationID: "getEngineDetails", Summary: "Get search engine details", Auth: "none", Listed: true, Request: map[string]any{"name": "string"}, Response: EngineDetailsResponse{}, Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/stats", OperationID: "getServerStats", Summary: "Get search statistics", Auth: "none", Listed: true, Request: map[string]any{}, Response: StatsResponse{}, Primary: "stats", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/version", OperationID: "getServerVersion", Summary: "Get server version", Auth: "none", Listed: true, Request: map[string]any{}, Response: VersionResponse{}, Primary: "version", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/healthz", OperationID: "getServerHealth", Summary: "Health check", Auth: "none", Listed: true, Request: map[string]any{}, Response: HealthStatusResponse{}, Message: "Retrieved successfully", Aliases: []Alias{{Method: http.MethodGet, Path: "/api/{api_version}/server/healthz.txt"}, {Method: http.MethodGet, Path: "/api/healthz"}}},
	{Method: http.MethodGet, Path: "/api/{api_version}/status", OperationID: "getOperatorStatus", Summary: "Get operator status", Auth: "operator", Listed: false, Response: Envelope{}, Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/about", OperationID: "getServerAbout", Summary: "Get server about information", Auth: "none", Listed: true, Request: map[string]any{}, Response: Envelope{Data: AboutData{}}, Primary: "about", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/privacy", OperationID: "getServerPrivacy", Summary: "Get server privacy information", Auth: "none", Listed: true, Request: map[string]any{}, Response: Envelope{Data: PrivacyData{}}, Primary: "privacy", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/dpo", OperationID: "getServerDPO", Summary: "Get data protection officer information", Auth: "none", Listed: true, Request: map[string]any{}, Response: Envelope{Data: map[string]string{"name": "", "email": "", "privacy_url": ""}}, Primary: "dpo", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/auth/check", OperationID: "checkServerToken", Summary: "Check server authentication status", Auth: "bearer", Listed: true, Request: map[string]any{}, Response: Envelope{Data: APIAuthData{}}, Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/contact", OperationID: "getServerContactForm", Summary: "Get contact form description", Auth: "none", Listed: true, Request: map[string]any{"security_id": "string"}, Response: Envelope{Data: map[string]any{"fields": []any{}, "types": []string{}, "captcha": map[string]any{}, "form_token": "string", "security_url": "string", "abuse_email": "string"}}, Primary: "contact", Message: "Retrieved successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/contact", OperationID: "submitServerContact", Summary: "Submit a contact or security report", Auth: "none", Listed: true, Request: ContactRequest{}, Response: Envelope{Data: ContactSubmissionData{}}, Message: "Created successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/help", OperationID: "getServerHelp", Summary: "Get server help", Auth: "none", Listed: true, Request: map[string]any{}, Response: Envelope{Data: ServerHelpData{}}, Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/help/mcp", OperationID: "getServerHelpMCP", Summary: "Get MCP integration help", Auth: "none", Listed: true, Request: map[string]any{}, Response: Envelope{Data: map[string]any{}}, Primary: "help", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/terms", OperationID: "getServerTerms", Summary: "Get server terms", Auth: "none", Listed: true, Request: map[string]any{}, Response: Envelope{Data: TermsData{}}, Primary: "terms", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/security", OperationID: "getServerSecurity", Summary: "Get server security contact information", Auth: "none", Listed: true, Request: map[string]any{}, Response: SecurityOverviewResponse{}, Primary: "contacts", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/security/policy", OperationID: "getServerSecurityPolicy", Summary: "Get the security disclosure policy", Auth: "none", Listed: true, Request: map[string]any{}, Response: SecurityPolicyResponse{}, Primary: "policy", Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/security/thanks", OperationID: "getServerSecurityThanks", Summary: "List credited security researchers", Auth: "none", Listed: true, Request: map[string]any{}, Response: SecurityThanksResponse{}, Primary: "entries", Message: "Retrieved successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/reports/default", OperationID: "submitDefaultReport", Summary: "Submit a browser default error report", Auth: "none", Listed: true, Request: BrowserReportRequest{}, Response: EmptyResponse{}, Message: "Created successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/reports/nel", OperationID: "submitNetworkErrorReport", Summary: "Submit a network error report", Auth: "none", Listed: true, Request: BrowserReportRequest{}, Response: EmptyResponse{}, Message: "Created successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/reports/csp", OperationID: "submitCSPReport", Summary: "Submit a content security policy report", Auth: "none", Listed: true, Request: BrowserReportRequest{}, Response: EmptyResponse{}, Message: "Created successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/proxy/thumbnails", OperationID: "proxyThumbnail", Summary: "Fetch a remote thumbnail", Auth: "none", Listed: true, Request: ProxyQuery{}, Response: nil, Binary: "image/*"},
	{Method: http.MethodGet, Path: "/api/{api_version}/proxy/videos", OperationID: "proxyVideo", Summary: "Fetch a remote video stream", Auth: "none", Listed: true, Request: ProxyQuery{}, Response: nil, Binary: "video/*"},
	{Method: http.MethodGet, Path: "/api/{api_version}/favorites", OperationID: "listFavorites", Summary: "List saved favorites", Auth: "owner", Listed: true, Request: map[string]any{}, Response: Envelope{Data: FavoriteListData{}}, Primary: "favorites", Message: "Retrieved successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/favorites", OperationID: "addFavorite", Summary: "Save a favorite", Auth: "owner", Listed: true, Request: FavoriteMutationRequest{}, Response: Envelope{Data: FavoriteMutationData{}}, Message: "Created successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/favorites/import", OperationID: "importFavorites", Summary: "Import saved favorites", Auth: "owner", Listed: true, Request: json.RawMessage{}, Response: Envelope{Data: map[string]any{"imported": 0, "owner_token": "string"}}, Message: "Updated successfully"},
	{Method: http.MethodDelete, Path: "/api/{api_version}/favorites/{id}", OperationID: "removeFavorite", Summary: "Remove a saved favorite", Auth: "owner", Listed: true, Request: map[string]any{"id": "integer"}, Response: Envelope{Data: map[string]string{"message": "removed"}}, Message: "Deleted successfully"},
	{Method: http.MethodDelete, Path: "/api/{api_version}/favorites", OperationID: "clearFavorites", Summary: "Clear saved favorites", Auth: "owner", Listed: true, Request: map[string]any{}, Response: Envelope{Data: map[string]string{"message": "cleared"}}, Message: "Deleted successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/preferences", OperationID: "getPreferences", Summary: "Get preferences", Auth: "none", Listed: true, Request: map[string]any{}, Response: Envelope{Data: PreferencesData{}}, Message: "Retrieved successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/preferences", OperationID: "savePreferences", Summary: "Set preferences", Auth: "none", Listed: true, Request: map[string]string{}, Response: Envelope{Data: PreferencesUpdateData{}}, Message: "Updated successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/preferences/sync", OperationID: "exportPreferences", Summary: "Export preferences", Auth: "none", Listed: true, Request: map[string]any{}, Response: Envelope{Data: PreferencesExportData{}}, Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/preferences/sync/import", OperationID: "importPreferences", Summary: "Get preference import descriptor", Auth: "none", Listed: true, Request: map[string]any{"code": "string"}, Response: Envelope{Data: PreferencesUpdateData{}}, Message: "Retrieved successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/preferences/tokens", OperationID: "listOwnerTokens", Summary: "List owner tokens", Auth: "owner", Listed: true, Request: map[string]any{}, Response: Envelope{Data: TokenListData{}}, Message: "Retrieved successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/preferences/tokens", OperationID: "createLinkedToken", Summary: "Create a linked owner token", Auth: "owner", Listed: true, Request: TokenCreateRequest{}, Response: Envelope{Data: TokenData{}}, Message: "Created successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/preferences/tokens/primary", OperationID: "issuePrimaryOwnerToken", Summary: "Issue the primary owner token", Auth: "none", Listed: true, Request: map[string]any{}, Response: Envelope{Data: TokenData{}}, Message: "Created successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/preferences/tokens/primary/reveal", OperationID: "revealPrimaryToken", Summary: "Reveal the primary owner token once", Auth: "owner", Listed: true, Request: map[string]any{}, Response: Envelope{Data: TokenData{}}, Message: "Retrieved successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/preferences/tokens/primary/rotate", OperationID: "rotatePrimaryToken", Summary: "Rotate the primary owner token", Auth: "owner", Listed: true, Request: map[string]any{}, Response: Envelope{Data: TokenData{}}, Message: "Updated successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/preferences/tokens/{id}/reveal", OperationID: "revealLinkedToken", Summary: "Reveal a linked owner token once", Auth: "owner", Listed: true, Request: map[string]any{"id": "string"}, Response: Envelope{Data: TokenData{}}, Message: "Retrieved successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/preferences/tokens/{id}/rotate", OperationID: "rotateLinkedToken", Summary: "Rotate a linked owner token", Auth: "owner", Listed: true, Request: map[string]any{"id": "string"}, Response: Envelope{Data: TokenData{}}, Message: "Updated successfully"},
	{Method: http.MethodPatch, Path: "/api/{api_version}/server/preferences/tokens/{id}", OperationID: "updateLinkedToken", Summary: "Update a linked owner token", Auth: "owner", Listed: true, Request: TokenCreateRequest{}, Response: Envelope{Data: map[string]bool{"updated": true}}, Message: "Updated successfully"},
	{Method: http.MethodDelete, Path: "/api/{api_version}/server/preferences/tokens/{id}", OperationID: "revokeLinkedToken", Summary: "Revoke a linked owner token", Auth: "owner", Listed: true, Request: map[string]any{"id": "string"}, Response: Envelope{Data: map[string]bool{"revoked": true}}, Message: "Deleted successfully"},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/swagger", OperationID: "getOpenAPISpec", Summary: "Get the OpenAPI specification", Auth: "none", Listed: true, Request: map[string]any{}, Response: map[string]any{}, Message: "Retrieved successfully", SkipText: true, Aliases: []Alias{{Method: http.MethodGet, Path: "/api/swagger"}}},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/graphql", OperationID: "executeGraphQL", Summary: "Execute a read-only GraphQL operation", Auth: "none", Listed: true, Request: GraphQLRequest{}, Response: GraphQLResponse{}, Aliases: []Alias{{Method: http.MethodPost, Path: "/api/graphql"}}},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/graphql", OperationID: "executeGraphQLQuery", Summary: "Execute a query-only GraphQL operation", Auth: "none", Listed: true, Request: GraphQLRequest{}, Response: GraphQLResponse{}},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/metrics", OperationID: "getMetrics", Summary: "Get internal server metrics", Auth: "service", Listed: false, Internal: true, Response: nil, Condition: "server.metrics.enabled", Aliases: []Alias{{Method: http.MethodGet, Path: "/api/metrics", Condition: "server.metrics.enabled"}, {Method: http.MethodGet, Path: "/server/metrics", Condition: "server.metrics.enabled"}, {Method: http.MethodGet, Path: "/metrics", Condition: "server.metrics.root.enabled"}}},
	{Method: http.MethodGet, Path: "/api/{api_version}/server/metrics/{service}", OperationID: "getServiceMetrics", Summary: "Get internal service metrics", Auth: "service", Listed: false, Internal: true, Response: nil, Condition: "server.metrics.enabled", Aliases: []Alias{{Method: http.MethodGet, Path: "/api/metrics/{service}", Condition: "server.metrics.enabled"}, {Method: http.MethodGet, Path: "/server/metrics/{service}", Condition: "server.metrics.enabled"}, {Method: http.MethodGet, Path: "/metrics/{service}", Condition: "server.metrics.root.enabled"}}},
	{Method: http.MethodGet, Path: "/api/{api_version}/debug/engines", OperationID: "debugEngines", Summary: "Get debug engine diagnostics", Auth: "operator", Listed: false, Internal: true, DebugOnly: true, Response: Envelope{}, Aliases: []Alias{{Method: http.MethodGet, Path: "/api/debug/engines"}}},
	{Method: http.MethodGet, Path: "/api/{api_version}/debug/engines/{name}", OperationID: "debugEngine", Summary: "Get debug engine details", Auth: "operator", Listed: false, Internal: true, DebugOnly: true, Response: Envelope{}, Aliases: []Alias{{Method: http.MethodGet, Path: "/api/debug/engines/{name}"}}},
	{Method: http.MethodGet, Path: "/api/autodiscover", OperationID: "autodiscover", Summary: "Discover API version and endpoints", Auth: "none", Listed: true, Request: AutodiscoverRequest{}, Response: AutodiscoverResponse{}, Message: "Retrieved successfully"},
	{Method: http.MethodPost, Path: "/api/{api_version}/server/mcp", OperationID: "executeMCP", Summary: "Use the Model Context Protocol endpoint", Auth: "bearer", Listed: true, Request: MCPRequest{}, Response: MCPResponse{}, SkipText: true, Aliases: []Alias{{Method: http.MethodPost, Path: "/api/mcp"}, {Method: http.MethodPost, Path: "/mcp"}}},
}
