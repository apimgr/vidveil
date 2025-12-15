// SPDX-License-Identifier: MIT
package handlers

import (
	"net/http"

	"github.com/apimgr/vidveil/src/config"
)

// OpenAPISpec returns the OpenAPI 3.0 specification in JSON format
func OpenAPISpec(cfg *config.Config) http.HandlerFunc {
	spec := generateOpenAPISpec(cfg)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(spec))
	}
}

// OpenAPISpecYAML returns the OpenAPI 3.0 specification in YAML format
func OpenAPISpecYAML(cfg *config.Config) http.HandlerFunc {
	spec := generateOpenAPISpecYAML(cfg)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		w.Write([]byte(spec))
	}
}

// SwaggerUI returns an HTML page with Swagger UI
func SwaggerUI(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Vidveil API Documentation</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
        body { margin: 0; background: #282a36; }
        .swagger-ui { max-width: 1200px; margin: 0 auto; }
        .swagger-ui .topbar { display: none; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "/openapi.json",
                dom_id: '#swagger-ui',
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIBundle.SwaggerUIStandalonePreset
                ],
                layout: "BaseLayout",
                deepLinking: true
            });
        };
    </script>
</body>
</html>`))
	}
}

func generateOpenAPISpec(cfg *config.Config) string {
	serverURL := "http://localhost:" + cfg.Server.Port
	if cfg.Server.FQDN != "" {
		serverURL = "http://" + cfg.Server.FQDN
	}

	return `{
  "openapi": "3.0.3",
  "info": {
    "title": "Vidveil API",
    "description": "Privacy-respecting adult video meta search engine API",
    "version": "1.0.0",
    "license": {
      "name": "MIT",
      "url": "https://opensource.org/licenses/MIT"
    }
  },
  "servers": [
    {
      "url": "` + serverURL + `/api/v1",
      "description": "API Server"
    }
  ],
  "paths": {
    "/search": {
      "get": {
        "summary": "Search for videos",
        "description": "Search across enabled video search engines",
        "operationId": "search",
        "tags": ["Search"],
        "parameters": [
          {
            "name": "q",
            "in": "query",
            "required": true,
            "description": "Search query",
            "schema": { "type": "string" }
          },
          {
            "name": "page",
            "in": "query",
            "required": false,
            "description": "Page number (default: 1)",
            "schema": { "type": "integer", "default": 1 }
          },
          {
            "name": "engines",
            "in": "query",
            "required": false,
            "description": "Comma-separated list of engine names to use",
            "schema": { "type": "string" }
          }
        ],
        "responses": {
          "200": {
            "description": "Search results",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/SearchResponse" }
              }
            }
          },
          "400": {
            "description": "Bad request (missing query)",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    },
    "/search.txt": {
      "get": {
        "summary": "Search for videos (plain text)",
        "description": "Search and return results in plain text format",
        "operationId": "searchText",
        "tags": ["Search"],
        "parameters": [
          {
            "name": "q",
            "in": "query",
            "required": true,
            "description": "Search query",
            "schema": { "type": "string" }
          }
        ],
        "responses": {
          "200": {
            "description": "Plain text search results",
            "content": {
              "text/plain": {
                "schema": { "type": "string" }
              }
            }
          }
        }
      }
    },
    "/engines": {
      "get": {
        "summary": "List search engines",
        "description": "Get information about all available search engines",
        "operationId": "listEngines",
        "tags": ["Engines"],
        "responses": {
          "200": {
            "description": "List of engines",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/EnginesResponse" }
              }
            }
          }
        }
      }
    },
    "/engines/{name}": {
      "get": {
        "summary": "Get engine details",
        "description": "Get detailed information about a specific search engine",
        "operationId": "getEngine",
        "tags": ["Engines"],
        "parameters": [
          {
            "name": "name",
            "in": "path",
            "required": true,
            "description": "Engine name",
            "schema": { "type": "string" }
          }
        ],
        "responses": {
          "200": {
            "description": "Engine details",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/EngineInfo" }
              }
            }
          },
          "404": {
            "description": "Engine not found",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    },
    "/stats": {
      "get": {
        "summary": "Get server statistics",
        "description": "Returns server statistics and metrics",
        "operationId": "getStats",
        "tags": ["Stats"],
        "responses": {
          "200": {
            "description": "Server statistics",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/StatsResponse" }
              }
            }
          }
        }
      }
    },
    "/healthz": {
      "get": {
        "summary": "Health check",
        "description": "Returns server health status",
        "operationId": "healthCheck",
        "tags": ["Health"],
        "responses": {
          "200": {
            "description": "Server is healthy",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/HealthResponse" }
              }
            }
          }
        }
      }
    },
    "/admin/stats": {
      "get": {
        "summary": "Get admin statistics",
        "description": "Returns detailed server statistics (requires API token)",
        "operationId": "adminStats",
        "tags": ["Admin"],
        "security": [{ "apiToken": [] }],
        "responses": {
          "200": {
            "description": "Detailed statistics",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/AdminStatsResponse" }
              }
            }
          },
          "401": {
            "description": "Unauthorized",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    },
    "/admin/backup": {
      "post": {
        "summary": "Create backup",
        "description": "Creates a backup of configuration and data (requires API token)",
        "operationId": "createBackup",
        "tags": ["Admin"],
        "security": [{ "apiToken": [] }],
        "parameters": [
          {
            "name": "file",
            "in": "query",
            "required": false,
            "description": "Custom backup file path",
            "schema": { "type": "string" }
          }
        ],
        "responses": {
          "200": {
            "description": "Backup created",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/SuccessResponse" }
              }
            }
          },
          "401": {
            "description": "Unauthorized",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    },
    "/admin/maintenance": {
      "post": {
        "summary": "Toggle maintenance mode",
        "description": "Enable or disable maintenance mode (requires API token)",
        "operationId": "setMaintenanceMode",
        "tags": ["Admin"],
        "security": [{ "apiToken": [] }],
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "enabled": { "type": "boolean" }
                }
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Maintenance mode updated",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/SuccessResponse" }
              }
            }
          },
          "401": {
            "description": "Unauthorized",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/Error" }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "SearchResponse": {
        "type": "object",
        "properties": {
          "success": { "type": "boolean" },
          "data": {
            "type": "object",
            "properties": {
              "query": { "type": "string" },
              "results": {
                "type": "array",
                "items": { "$ref": "#/components/schemas/Result" }
              },
              "engines_used": {
                "type": "array",
                "items": { "type": "string" }
              },
              "engines_failed": {
                "type": "array",
                "items": { "type": "string" }
              },
              "search_time_ms": { "type": "integer" }
            }
          },
          "pagination": { "$ref": "#/components/schemas/Pagination" }
        }
      },
      "Result": {
        "type": "object",
        "properties": {
          "id": { "type": "string" },
          "title": { "type": "string" },
          "url": { "type": "string" },
          "thumbnail": { "type": "string" },
          "duration": { "type": "integer", "description": "Duration in seconds" },
          "duration_str": { "type": "string", "description": "Human-readable duration" },
          "views": { "type": "integer" },
          "views_str": { "type": "string" },
          "rating": { "type": "number" },
          "quality": { "type": "string" },
          "source": { "type": "string" }
        }
      },
      "Pagination": {
        "type": "object",
        "properties": {
          "page": { "type": "integer" },
          "limit": { "type": "integer" },
          "total": { "type": "integer" },
          "pages": { "type": "integer" }
        }
      },
      "EnginesResponse": {
        "type": "object",
        "properties": {
          "success": { "type": "boolean" },
          "data": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/EngineInfo" }
          }
        }
      },
      "EngineInfo": {
        "type": "object",
        "properties": {
          "name": { "type": "string" },
          "display_name": { "type": "string" },
          "enabled": { "type": "boolean" },
          "available": { "type": "boolean" },
          "tier": { "type": "integer" },
          "features": {
            "type": "array",
            "items": { "type": "string" }
          }
        }
      },
      "StatsResponse": {
        "type": "object",
        "properties": {
          "success": { "type": "boolean" },
          "data": {
            "type": "object",
            "properties": {
              "engines_count": { "type": "integer" },
              "engines_enabled": { "type": "integer" }
            }
          }
        }
      },
      "AdminStatsResponse": {
        "type": "object",
        "properties": {
          "success": { "type": "boolean" },
          "data": {
            "type": "object",
            "properties": {
              "engines": {
                "type": "object",
                "properties": {
                  "total": { "type": "integer" },
                  "enabled": { "type": "integer" }
                }
              },
              "memory": {
                "type": "object",
                "properties": {
                  "alloc_mb": { "type": "integer" },
                  "total_alloc_mb": { "type": "integer" },
                  "sys_mb": { "type": "integer" },
                  "num_gc": { "type": "integer" }
                }
              },
              "runtime": {
                "type": "object",
                "properties": {
                  "goroutines": { "type": "integer" },
                  "go_version": { "type": "string" },
                  "os": { "type": "string" },
                  "arch": { "type": "string" }
                }
              }
            }
          }
        }
      },
      "HealthResponse": {
        "type": "object",
        "properties": {
          "status": { "type": "string", "enum": ["ok", "degraded", "error"] },
          "engines_enabled": { "type": "integer" }
        }
      },
      "SuccessResponse": {
        "type": "object",
        "properties": {
          "success": { "type": "boolean" },
          "message": { "type": "string" }
        }
      },
      "Error": {
        "type": "object",
        "properties": {
          "success": { "type": "boolean" },
          "error": { "type": "string" }
        }
      }
    },
    "securitySchemes": {
      "apiToken": {
        "type": "apiKey",
        "in": "header",
        "name": "X-API-Token",
        "description": "API token for admin endpoints"
      }
    }
  },
  "tags": [
    { "name": "Search", "description": "Search operations" },
    { "name": "Engines", "description": "Engine management" },
    { "name": "Stats", "description": "Server statistics" },
    { "name": "Health", "description": "Health checks" },
    { "name": "Admin", "description": "Admin operations (requires authentication)" }
  ]
}`
}

func generateOpenAPISpecYAML(cfg *config.Config) string {
	serverURL := "http://localhost:" + cfg.Server.Port
	if cfg.Server.FQDN != "" {
		serverURL = "http://" + cfg.Server.FQDN
	}

	return `openapi: "3.0.3"
info:
  title: Vidveil API
  description: Privacy-respecting adult video meta search engine API
  version: "1.0.0"
  license:
    name: MIT
    url: https://opensource.org/licenses/MIT
servers:
  - url: ` + serverURL + `/api/v1
    description: API Server
paths:
  /search:
    get:
      summary: Search for videos
      description: Search across enabled video search engines
      operationId: search
      tags:
        - Search
      parameters:
        - name: q
          in: query
          required: true
          description: Search query
          schema:
            type: string
        - name: page
          in: query
          required: false
          description: Page number (default: 1)
          schema:
            type: integer
            default: 1
        - name: engines
          in: query
          required: false
          description: Comma-separated list of engine names to use
          schema:
            type: string
      responses:
        "200":
          description: Search results
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/SearchResponse"
        "400":
          description: Bad request (missing query)
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
  /search.txt:
    get:
      summary: Search for videos (plain text)
      description: Search and return results in plain text format
      operationId: searchText
      tags:
        - Search
      parameters:
        - name: q
          in: query
          required: true
          description: Search query
          schema:
            type: string
      responses:
        "200":
          description: Plain text search results
          content:
            text/plain:
              schema:
                type: string
  /engines:
    get:
      summary: List search engines
      description: Get information about all available search engines
      operationId: listEngines
      tags:
        - Engines
      responses:
        "200":
          description: List of engines
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EnginesResponse"
  /engines/{name}:
    get:
      summary: Get engine details
      description: Get detailed information about a specific search engine
      operationId: getEngine
      tags:
        - Engines
      parameters:
        - name: name
          in: path
          required: true
          description: Engine name
          schema:
            type: string
      responses:
        "200":
          description: Engine details
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/EngineInfo"
        "404":
          description: Engine not found
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
  /stats:
    get:
      summary: Get server statistics
      description: Returns server statistics and metrics
      operationId: getStats
      tags:
        - Stats
      responses:
        "200":
          description: Server statistics
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/StatsResponse"
  /healthz:
    get:
      summary: Health check
      description: Returns server health status
      operationId: healthCheck
      tags:
        - Health
      responses:
        "200":
          description: Server is healthy
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/HealthResponse"
  /admin/stats:
    get:
      summary: Get admin statistics
      description: Returns detailed server statistics (requires API token)
      operationId: adminStats
      tags:
        - Admin
      security:
        - apiToken: []
      responses:
        "200":
          description: Detailed statistics
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/AdminStatsResponse"
        "401":
          description: Unauthorized
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
  /admin/backup:
    post:
      summary: Create backup
      description: Creates a backup of configuration and data (requires API token)
      operationId: createBackup
      tags:
        - Admin
      security:
        - apiToken: []
      parameters:
        - name: file
          in: query
          required: false
          description: Custom backup file path
          schema:
            type: string
      responses:
        "200":
          description: Backup created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/SuccessResponse"
        "401":
          description: Unauthorized
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
  /admin/maintenance:
    post:
      summary: Toggle maintenance mode
      description: Enable or disable maintenance mode (requires API token)
      operationId: setMaintenanceMode
      tags:
        - Admin
      security:
        - apiToken: []
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                enabled:
                  type: boolean
      responses:
        "200":
          description: Maintenance mode updated
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/SuccessResponse"
        "401":
          description: Unauthorized
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
components:
  schemas:
    SearchResponse:
      type: object
      properties:
        success:
          type: boolean
        data:
          type: object
          properties:
            query:
              type: string
            results:
              type: array
              items:
                $ref: "#/components/schemas/Result"
            engines_used:
              type: array
              items:
                type: string
            engines_failed:
              type: array
              items:
                type: string
            search_time_ms:
              type: integer
        pagination:
          $ref: "#/components/schemas/Pagination"
    Result:
      type: object
      properties:
        id:
          type: string
        title:
          type: string
        url:
          type: string
        thumbnail:
          type: string
        duration:
          type: integer
          description: Duration in seconds
        duration_str:
          type: string
          description: Human-readable duration
        views:
          type: integer
        views_str:
          type: string
        rating:
          type: number
        quality:
          type: string
        source:
          type: string
    Pagination:
      type: object
      properties:
        page:
          type: integer
        limit:
          type: integer
        total:
          type: integer
        pages:
          type: integer
    EnginesResponse:
      type: object
      properties:
        success:
          type: boolean
        data:
          type: array
          items:
            $ref: "#/components/schemas/EngineInfo"
    EngineInfo:
      type: object
      properties:
        name:
          type: string
        display_name:
          type: string
        enabled:
          type: boolean
        available:
          type: boolean
        tier:
          type: integer
        features:
          type: array
          items:
            type: string
    StatsResponse:
      type: object
      properties:
        success:
          type: boolean
        data:
          type: object
          properties:
            engines_count:
              type: integer
            engines_enabled:
              type: integer
    AdminStatsResponse:
      type: object
      properties:
        success:
          type: boolean
        data:
          type: object
          properties:
            engines:
              type: object
              properties:
                total:
                  type: integer
                enabled:
                  type: integer
            memory:
              type: object
              properties:
                alloc_mb:
                  type: integer
                total_alloc_mb:
                  type: integer
                sys_mb:
                  type: integer
                num_gc:
                  type: integer
            runtime:
              type: object
              properties:
                goroutines:
                  type: integer
                go_version:
                  type: string
                os:
                  type: string
                arch:
                  type: string
    HealthResponse:
      type: object
      properties:
        status:
          type: string
          enum:
            - ok
            - degraded
            - error
        engines_enabled:
          type: integer
    SuccessResponse:
      type: object
      properties:
        success:
          type: boolean
        message:
          type: string
    Error:
      type: object
      properties:
        success:
          type: boolean
        error:
          type: string
  securitySchemes:
    apiToken:
      type: apiKey
      in: header
      name: X-API-Token
      description: API token for admin endpoints
tags:
  - name: Search
    description: Search operations
  - name: Engines
    description: Engine management
  - name: Stats
    description: Server statistics
  - name: Health
    description: Health checks
  - name: Admin
    description: Admin operations (requires authentication)
`
}
