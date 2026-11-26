// SPDX-License-Identifier: MIT
// OpenAPI route annotations for the GraphQL HTTP contract (AI.md PART 14).
package swagger

type parameterAnnotation struct {
	name, description, schemaType string
	required                      bool
}

type operationAnnotation struct {
	path, method, operationID, summary, description string
	parameters                                      []parameterAnnotation
	requestBody                                     bool
}

var graphQLOperations = []operationAnnotation{
	{
		path: "/server/graphql", method: "post", operationID: "executeGraphQL",
		summary: "Execute a GraphQL query", description: "Execute a GraphQL query against the service schema.", requestBody: true,
	},
	{
		path: "/server/graphql", method: "get", operationID: "executeGraphQLQuery",
		summary: "Execute a GraphQL query over GET", description: "Execute a query-only GraphQL operation. Omit query to receive endpoint usage.",
		parameters: []parameterAnnotation{
			{name: "query", description: "GraphQL query document", schemaType: "string"},
			{name: "variables", description: "JSON object of GraphQL variable values", schemaType: "string"},
			{name: "operationName", description: "Name of the operation to execute", schemaType: "string"},
		},
	},
}
