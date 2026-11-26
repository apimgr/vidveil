// SPDX-License-Identifier: MIT
// Per AI.md PART 14: GraphQL handler MUST be in src/graphql/graphql.go
package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/apimgr/vidveil/src/common/i18n"
)

type requestContextKey struct{}

const maxRequestBody = 1 << 20

// Request is the GraphQL-over-HTTP request shape.
type Request struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables,omitempty"`
}

type apiError struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Handle accepts standard GraphQL GET and POST requests. All parsing, validation,
// operation selection, and execution are delegated to the executable schema.
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet, http.MethodPost:
	default:
		w.Header().Set("Allow", "GET, POST")
		writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	req, err := readRequest(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if strings.TrimSpace(req.Query) == "" && r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "GraphQL endpoint. Supply a query parameter, for example ?query={%20health%20{%20status%20}%20}.\n")
		return
	}

	ctx := context.WithValue(r.Context(), requestContextKey{}, r)
	result := h.schemaForLocale(i18n.DetectLocale(r)).Exec(ctx, req.Query, req.OperationName, req.Variables)
	for _, graphErr := range result.Errors {
		if graphErr.Extensions == nil {
			graphErr.Extensions = make(map[string]any)
		}
		if _, ok := graphErr.Extensions["code"]; !ok {
			code := "VALIDATION_FAILED"
			if graphErr.ResolverError != nil {
				code = "INTERNAL_ERROR"
			}
			graphErr.Extensions["code"] = code
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	body, err := json.MarshalIndent(result, "", "  ")
	if err == nil {
		_, _ = w.Write(append(body, '\n'))
	}
}

func readRequest(r *http.Request) (Request, error) {
	var req Request
	if r.Method == http.MethodGet {
		values := r.URL.Query()
		req.Query = values.Get("query")
		req.OperationName = values.Get("operationName")
		if variables := values.Get("variables"); variables != "" {
			if err := json.Unmarshal([]byte(variables), &req.Variables); err != nil || req.Variables == nil {
				return req, errInvalidVariables
			}
		}
		return req, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody+1))
	if err != nil || len(body) > maxRequestBody {
		return req, errInvalidJSON
	}
	if trimmed := strings.TrimSpace(string(body)); trimmed == "" || trimmed[0] != '{' {
		return req, errInvalidJSON
	}
	var payload struct {
		Query         string          `json:"query"`
		OperationName string          `json:"operationName"`
		Variables     json.RawMessage `json:"variables"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return req, errInvalidJSON
	}
	req.Query = payload.Query
	req.OperationName = payload.OperationName
	if len(payload.Variables) > 0 && !bytes.Equal(bytes.TrimSpace(payload.Variables), []byte("null")) {
		var err error
		req.Variables, err = decodeObject(payload.Variables)
		if err != nil {
			return req, err
		}
	}
	return req, nil
}

var (
	errInvalidJSON      = requestError("Invalid JSON request body")
	errInvalidVariables = requestError("variables must be a JSON object")
)

type requestError string

func (e requestError) Error() string { return string(e) }

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	body, err := json.MarshalIndent(apiError{OK: false, Error: code, Message: message}, "", "  ")
	if err != nil {
		return
	}
	_, _ = w.Write(append(body, '\n'))
}

// decodeObject rejects JSON null, arrays and scalars for GraphQL variables.
func decodeObject(raw json.RawMessage) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var values map[string]any
	if err := decoder.Decode(&values); err != nil || values == nil {
		return nil, errInvalidVariables
	}
	return values, nil
}

// CaptureResponse invokes an existing JSON API handler with the original
// request and returns its response body for the GraphQL JSONResult scalar.
func CaptureResponse(handler http.HandlerFunc, request *http.Request) (string, error) {
	if handler == nil || request == nil {
		return "", resolverError{code: "SERVICE_UNAVAILABLE", message: "Read operation is unavailable"}
	}
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	clone.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	handler(response, clone)
	if response.Code >= http.StatusBadRequest {
		var payload struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(response.Body.Bytes(), &payload)
		code := payload.Error
		if code == "" {
			code = http.StatusText(response.Code)
		}
		message := payload.Message
		if message == "" {
			message = "Read operation failed"
		}
		return "", resolverError{code: code, message: message}
	}
	return strings.TrimSpace(response.Body.String()), nil
}

// CaptureJSONPost adapts an existing read-only POST endpoint to a GraphQL
// query while preserving its original request context and authorization.
func CaptureJSONPost(handler http.HandlerFunc, request *http.Request, payload any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", resolverError{code: "BAD_REQUEST", message: "Invalid query payload"}
	}
	clone := request.Clone(request.Context())
	clone.Method = http.MethodPost
	clone.Body = io.NopCloser(bytes.NewReader(body))
	clone.ContentLength = int64(len(body))
	clone.Header = request.Header.Clone()
	clone.Header.Set("Content-Type", "application/json")
	clone.Header.Set("Accept", "application/json")
	return CaptureResponse(handler, clone)
}
