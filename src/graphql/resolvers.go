// SPDX-License-Identifier: MIT
// GraphQL field resolvers are the only bridge to VidVeil read operations.
package graphql

import (
	"context"
	"net/http"
	"strings"

	"github.com/apimgr/vidveil/src/server/model"
	"github.com/apimgr/vidveil/src/server/service/engine"
)

type queryResolver struct{ handler *Handler }

type searchArgs struct {
	Query   string
	Page    *int32
	Engines *string
}

type batchQueryInput struct {
	Q       string
	Page    *int32
	Engines *string
}

type batchSearchArgs struct{ Queries []batchQueryInput }

func (r *queryResolver) Search(ctx context.Context, args searchArgs) (*searchResolver, error) {
	page := 1
	if args.Page != nil {
		page = int(*args.Page)
	}
	if page < 1 {
		return nil, resolverError{code: "BAD_REQUEST", message: "page must be greater than zero"}
	}
	if r.handler.engineMgr == nil {
		return nil, resolverError{code: "SERVICE_UNAVAILABLE", message: "Search is unavailable"}
	}
	query := strings.TrimSpace(args.Query)
	if query == "" {
		return nil, resolverError{code: "BAD_REQUEST", message: "query must not be empty"}
	}
	if r.handler.searchRecorder != nil {
		r.handler.searchRecorder(query)
	}
	var names []string
	if args.Engines != nil && strings.TrimSpace(*args.Engines) != "" {
		for _, name := range strings.Split(*args.Engines, ",") {
			if value := strings.TrimSpace(name); value != "" {
				names = append(names, value)
			}
		}
	}
	response := r.handler.engineMgr.Search(ctx, query, page, names, "")
	return newSearchResolver(response), nil
}

func (r *queryResolver) SearchBatch(ctx context.Context, args batchSearchArgs) (*jsonResolver, error) {
	if len(args.Queries) == 0 || len(args.Queries) > 5 {
		return nil, resolverError{code: "BAD_REQUEST", message: "batch queries must contain between one and five items"}
	}
	queries := make([]map[string]any, 0, len(args.Queries))
	for _, item := range args.Queries {
		page := 1
		if item.Page != nil {
			page = int(*item.Page)
		}
		query := map[string]any{"q": item.Q, "page": page}
		if item.Engines != nil {
			query["engines"] = *item.Engines
		}
		queries = append(queries, query)
	}
	return r.read(ctx, "searchBatch", map[string]any{"queries": queries})
}

func (r *queryResolver) Bangs() []*bangResolver {
	if r.handler.engineMgr == nil {
		return []*bangResolver{}
	}
	items := r.handler.engineMgr.ListBangs()
	result := make([]*bangResolver, 0, len(items))
	for _, item := range items {
		result = append(result, newBangResolver(item))
	}
	return result
}

type autocompleteArgs struct{ Prefix string }

func (r *queryResolver) Autocomplete(args autocompleteArgs) []*bangResolver {
	if r.handler.engineMgr == nil {
		return []*bangResolver{}
	}
	items := r.handler.engineMgr.AutocompleteBangs(args.Prefix)
	result := make([]*bangResolver, 0, len(items))
	for _, item := range items {
		result = append(result, &bangResolver{
			BangValue: item.Bang, EngineNameValue: item.EngineName,
			DisplayNameValue: item.DisplayName, ShortCodeValue: item.ShortCode,
		})
	}
	return result
}

func (r *queryResolver) Engines() []*engineResolver {
	if r.handler.engineMgr == nil {
		return []*engineResolver{}
	}
	items := r.handler.engineMgr.ListEngines()
	result := make([]*engineResolver, 0, len(items))
	for _, item := range items {
		result = append(result, &engineResolver{info: item})
	}
	return result
}

func (r *queryResolver) Health() *healthResolver {
	count := 0
	if r.handler.engineMgr != nil {
		count = r.handler.engineMgr.EnabledCount()
	}
	return &healthResolver{StatusValue: "ok", EnginesEnabledValue: int32(count)}
}

func (r *queryResolver) EngineHealth(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "engineHealth", nil)
}

type engineDetailsArgs struct{ Name string }

func (r *queryResolver) EngineDetails(ctx context.Context, args engineDetailsArgs) (*jsonResolver, error) {
	return r.read(ctx, "engineDetails", map[string]any{"name": args.Name})
}

func (r *queryResolver) Stats(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "stats", nil)
}
func (r *queryResolver) Version(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "version", nil)
}
func (r *queryResolver) Healthz(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "healthz", nil)
}
func (r *queryResolver) ServerAbout(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "serverAbout", nil)
}
func (r *queryResolver) ServerPrivacy(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "serverPrivacy", nil)
}
func (r *queryResolver) ServerDpo(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "serverDpo", nil)
}
func (r *queryResolver) ServerContact(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "serverContact", nil)
}
func (r *queryResolver) ServerHelp(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "serverHelp", nil)
}
func (r *queryResolver) ServerHelpMcp(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "serverHelpMcp", nil)
}
func (r *queryResolver) ServerTerms(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "serverTerms", nil)
}
func (r *queryResolver) Preferences(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "preferences", nil)
}
func (r *queryResolver) PreferenceSync(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "preferenceSync", nil)
}
func (r *queryResolver) PreferenceTokens(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "preferenceTokens", nil)
}
func (r *queryResolver) Favorites(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "favorites", nil)
}
func (r *queryResolver) OpenAPI(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "openAPI", nil)
}
func (r *queryResolver) ServerSecurity(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "serverSecurity", nil)
}
func (r *queryResolver) ServerSecurityPolicy(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "serverSecurityPolicy", nil)
}
func (r *queryResolver) ServerSecurityThanks(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "serverSecurityThanks", nil)
}
func (r *queryResolver) OperatorStatus(ctx context.Context) (*jsonResolver, error) {
	return r.read(ctx, "operatorStatus", nil)
}

func (r *queryResolver) read(ctx context.Context, name string, args map[string]any) (*jsonResolver, error) {
	request, _ := ctx.Value(requestContextKey{}).(*http.Request)
	operation := r.handler.readers[name]
	if request == nil || operation == nil {
		return nil, resolverError{code: "SERVICE_UNAVAILABLE", message: "Read operation is unavailable"}
	}
	value, err := operation(request, args)
	if err != nil {
		return nil, err
	}
	return &jsonResolver{value: value}, nil
}

type jsonResolver struct{ value string }

func (r *jsonResolver) JSON() string { return r.value }

type searchResolver struct {
	data       model.SearchData
	pagination model.PaginationData
}

func newSearchResolver(response *model.SearchResponse) *searchResolver {
	if response == nil {
		return &searchResolver{}
	}
	return &searchResolver{data: response.Data, pagination: response.Pagination}
}
func (r *searchResolver) Query() string { return r.data.Query }
func (r *searchResolver) Results() []*videoResolver {
	results := make([]*videoResolver, 0, len(r.data.Results))
	for _, item := range r.data.Results {
		results = append(results, &videoResolver{result: item})
	}
	return results
}
func (r *searchResolver) EnginesUsed() []string   { return r.data.EnginesUsed }
func (r *searchResolver) EnginesFailed() []string { return r.data.EnginesFailed }
func (r *searchResolver) SearchTimeMs() int32     { return int32(r.data.SearchTimeMS) }
func (r *searchResolver) Pagination() *paginationResolver {
	return &paginationResolver{data: r.pagination}
}

type videoResolver struct{ result model.VideoResult }

func (r *videoResolver) ID() string            { return r.result.ID }
func (r *videoResolver) Title() string         { return r.result.Title }
func (r *videoResolver) URL() string           { return r.result.URL }
func (r *videoResolver) Thumbnail() string     { return r.result.Thumbnail }
func (r *videoResolver) Duration() int32       { return int32(r.result.DurationSeconds) }
func (r *videoResolver) DurationStr() string   { return r.result.Duration }
func (r *videoResolver) Views() int32          { return int32(r.result.ViewsCount) }
func (r *videoResolver) ViewsStr() string      { return r.result.Views }
func (r *videoResolver) Source() string        { return r.result.Source }
func (r *videoResolver) SourceDisplay() string { return r.result.SourceDisplay }
func (r *videoResolver) Description() string   { return r.result.Description }

type bangResolver struct {
	BangValue, EngineNameValue, DisplayNameValue, ShortCodeValue string
}

func newBangResolver(item engine.BangInfo) *bangResolver {
	return &bangResolver{item.Bang, item.EngineName, item.DisplayName, item.ShortCode}
}
func (r *bangResolver) Bang() string        { return r.BangValue }
func (r *bangResolver) EngineName() string  { return r.EngineNameValue }
func (r *bangResolver) DisplayName() string { return r.DisplayNameValue }
func (r *bangResolver) ShortCode() string   { return r.ShortCodeValue }

type engineResolver struct{ info model.EngineInfo }

func (r *engineResolver) Name() string        { return r.info.Name }
func (r *engineResolver) DisplayName() string { return r.info.DisplayName }
func (r *engineResolver) Enabled() bool       { return r.info.Enabled }
func (r *engineResolver) Available() bool     { return r.info.Available }
func (r *engineResolver) Tier() int32         { return int32(r.info.Tier) }
func (r *engineResolver) Features() []string  { return r.info.Features }

type paginationResolver struct{ data model.PaginationData }

func (r *paginationResolver) Page() int32  { return int32(r.data.Page) }
func (r *paginationResolver) Limit() int32 { return int32(r.data.Limit) }
func (r *paginationResolver) Total() int32 { return int32(r.data.Total) }
func (r *paginationResolver) Pages() int32 { return int32(r.data.Pages) }

type healthResolver struct {
	StatusValue         string
	EnginesEnabledValue int32
}

func (r *healthResolver) Status() string        { return r.StatusValue }
func (r *healthResolver) EnginesEnabled() int32 { return r.EnginesEnabledValue }

type resolverError struct{ code, message string }

func (e resolverError) Error() string              { return e.message }
func (e resolverError) Extensions() map[string]any { return map[string]any{"code": e.code} }
