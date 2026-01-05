# GraphQL API

Endpoint: `/graphql`

Interactive playground: `/graphiql`

Schema definition: `/graphql/schema`

## Schema

```graphql
type Query {
  search(query: String!, page: Int, engines: String): SearchResult
  bangs: [Bang]
  autocomplete(prefix: String!): [Bang]
  engines: [Engine]
  health: Health
}

type SearchResult {
  query: String!
  results: [Result]
  enginesUsed: [String]
  enginesFailed: [String]
  searchTimeMs: Int
  pagination: Pagination
}

type Result {
  id: String!
  title: String!
  url: String!
  thumbnail: String
  duration: Int
  durationStr: String
  views: Int
  viewsStr: String
  source: String!
  sourceDisplay: String
  description: String
}

type Bang {
  bang: String!
  engineName: String!
  displayName: String!
  shortCode: String!
}

type Engine {
  name: String!
  displayName: String!
  enabled: Boolean!
  available: Boolean!
  tier: Int!
  features: [String]
}

type Pagination {
  page: Int!
  limit: Int!
  total: Int!
  pages: Int!
}

type Health {
  status: String!
  enginesEnabled: Int!
}
```

## Example Queries

### Search

```graphql
query Search($query: String!, $page: Int) {
  search(query: $query, page: $page) {
    query
    results {
      id
      title
      url
      thumbnail
      durationStr
      viewsStr
      source
      sourceDisplay
    }
    enginesUsed
    searchTimeMs
    pagination {
      page
      pages
      total
    }
  }
}
```

Variables:
```json
{
  "query": "example",
  "page": 1
}
```

### List Engines

```graphql
query {
  engines {
    name
    displayName
    enabled
    available
    tier
    features
  }
}
```

### List Bangs

```graphql
query {
  bangs {
    bang
    engineName
    displayName
    shortCode
  }
}
```

### Autocomplete

```graphql
query Autocomplete($prefix: String!) {
  autocomplete(prefix: $prefix) {
    bang
    engineName
    displayName
    shortCode
  }
}
```

Variables:
```json
{
  "prefix": "!po"
}
```

### Health Check

```graphql
query {
  health {
    status
    enginesEnabled
  }
}
```

## Using GraphiQL

Navigate to `/graphiql` to access the interactive GraphQL playground with:

- Query editor with syntax highlighting
- Automatic schema documentation
- Query history
- Variable editor
