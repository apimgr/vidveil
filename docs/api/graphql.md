# GraphQL API

Endpoint: `/graphql`

Interactive playground: `/graphiql`

## Schema

```graphql
type Query {
  search(query: String!, engines: [String], page: Int): SearchResult!
  engines: [Engine!]!
  engine(name: String!): Engine
  health: Health!
}

type SearchResult {
  query: String!
  page: Int!
  results: [Video!]!
  total: Int!
}

type Video {
  title: String!
  url: String!
  thumbnail: String
  duration: String
  views: String
  engine: String!
}

type Engine {
  name: String!
  enabled: Boolean!
  bang: String
}

type Health {
  status: String!
  uptime: String!
}
```

## Example Query

```graphql
query {
  search(query: "example", page: 1) {
    query
    results {
      title
      url
      duration
      engine
    }
    total
  }
}
```
