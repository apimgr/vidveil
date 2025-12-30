// SPDX-License-Identifier: MIT
// See AI.md PART 1 lines 481-484, PART 20 for API/Swagger rules
package swagger

import (
	"net/http"

	"github.com/apimgr/vidveil/src/config"
)

// Handler returns the Swagger UI handler
func Handler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Theme detection (light/dark/auto) - see theme.go
		theme := DetectTheme(r)
		
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(generateSwaggerUI(cfg, theme)))
	}
}

// SpecHandler returns the OpenAPI 3.0 specification in JSON format
func SpecHandler(cfg *config.Config) http.HandlerFunc {
	spec := GenerateSpec(cfg)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(spec))
	}
}

func generateSwaggerUI(cfg *config.Config, theme string) string {
	bgColor := "#282a36" // dark theme default
	if theme == "light" {
		bgColor = "#ffffff"
	}

	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Vidveil API Documentation</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
        body { margin: 0; background: ` + bgColor + `; }
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
</html>`
}
