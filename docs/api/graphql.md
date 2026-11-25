# GraphQL API

The versioned endpoint is `https://x.scour.li/api/v1/server/graphql` and
accepts POST and query-only GET requests. The unversioned `/api/graphql` alias
accepts POST. GET parameters are `query`, optional JSON `variables`, and
optional `operationName`; omitting `query` returns endpoint usage.

GraphQL is query-only. Project read routes and applicable template API reads
have matching query fields. The read-only `POST /api/v1/search/batch` route is
available as a GraphQL query. Binary media streams, machine-only checks, and
PART 20's internal metrics endpoints remain outside the GraphQL surface.
Mutating routes, including contact submissions and preference import, remain
REST-only.

GraphiQL interface: `https://x.scour.li/server/docs/graphql`

The `operatorStatus` field requires the configured operator Bearer token and
returns the same status data as `GET /api/v1/status`.

## Query fields

The search, discovery, and engine fields return typed GraphQL objects:

```graphql
type Query {
  search(query: String!, page: Int, engines: String): SearchResult!
  searchBatch(queries: [BatchQueryInput!]!): JSONResult!
  bangs: [Bang!]!
  autocomplete(prefix: String!): [Bang!]!
  engines: [Engine!]!
  health: Health!
  engineHealth: JSONResult!
  engineDetails(name: String!): JSONResult!
  stats: JSONResult!
  version: JSONResult!
  healthz: JSONResult!
  serverAbout: JSONResult!
  serverPrivacy: JSONResult!
  serverDpo: JSONResult!
  serverContact: JSONResult!
  serverHelp: JSONResult!
  serverHelpMcp: JSONResult!
  serverTerms: JSONResult!
  serverSecurity: JSONResult!
  serverSecurityPolicy: JSONResult!
  serverSecurityThanks: JSONResult!
  preferences: JSONResult!
  preferenceSync: JSONResult!
  preferenceTokens: JSONResult!
  favorites: JSONResult!
  operatorStatus: JSONResult!
  openAPI: JSONResult!
}

input BatchQueryInput { q: String!, page: Int, engines: String }
type JSONResult { json: String! }

type SearchResult {
  query: String!
  results: [VideoResult!]!
  enginesUsed: [String!]!
  enginesFailed: [String!]!
  searchTimeMs: Int!
  pagination: Pagination!
}
type VideoResult {
  id: String!
  title: String!
  url: String!
  thumbnail: String!
  duration: Int!
  durationStr: String!
  views: Int!
  viewsStr: String!
  source: String!
  sourceDisplay: String!
  description: String!
}
type Bang { bang: String!, engineName: String!, displayName: String!, shortCode: String! }
type Engine { name: String!, displayName: String!, enabled: Boolean!, available: Boolean!, tier: Int!, features: [String!]! }
type Pagination { page: Int!, limit: Int!, total: Int!, pages: Int! }
type Health { status: String!, enginesEnabled: Int! }
```

The JSON result fields mirror their REST response envelopes. `json` contains
the exact response body returned by the corresponding REST handler. This
preserves handler-specific fields and does not transform authentication,
cookies, or ownership checks. `searchBatch` accepts the same `q`, `page`, and
optional `engines` values as `POST /api/v1/search/batch`. `openAPI` returns the
runtime-filtered specification JSON as a string.

`favorites` retains the REST API's bearer-token requirement and owner scoping.
`preferenceTokens` retains the primary-token requirement. `preferences` and
`preferenceSync` use the incoming request's cookies. Binary thumbnail/video
proxy streams and PART 20's internal metrics/authentication routes are not
represented as GraphQL values.

`operatorStatus` retains the configured operator Bearer token requirement.

## Search example

```graphql
query Search($query: String!, $page: Int) {
  search(query: $query, page: $page) {
    query
    results { id title url thumbnail durationStr viewsStr source sourceDisplay }
    enginesUsed
    searchTimeMs
    pagination { page pages total }
  }
}
```

Variables:

```json
{"query":"example","page":1}
```

## Engine detail example

```graphql
query Engine($name: String!) {
  engineDetails(name: $name) { json }
}
```

## Favorites example

Send the same bearer token used for `GET /api/v1/favorites`:

```graphql
query Favorites {
  favorites { json }
}
```

## Theme and localization

GraphQL schema type and field descriptions use the selected request language.
The generated SDL is embedded from Go schema annotations and is regenerated
with the API artifacts during builds.
