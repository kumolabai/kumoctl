package mcp

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestGenerateToolName(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		operationID string
		expected    string
	}{
		{
			name:        "with operation ID",
			method:      "get",
			path:        "/users/{id}",
			operationID: "getUserById",
			expected:    "getUserById",
		},
		{
			name:        "without operation ID - simple path",
			method:      "get",
			path:        "/users",
			operationID: "",
			expected:    "get_users",
		},
		{
			name:        "without operation ID - path with parameters",
			method:      "post",
			path:        "/users/{id}/posts/{postId}",
			operationID: "",
			expected:    "post_users_id_posts_postId",
		},
		{
			name:        "without operation ID - root path",
			method:      "get",
			path:        "/",
			operationID: "",
			expected:    "get",
		},
		{
			name:        "without operation ID - empty path",
			method:      "delete",
			path:        "",
			operationID: "",
			expected:    "delete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateToolName(tt.method, tt.path, tt.operationID)
			if result != tt.expected {
				t.Errorf("generateToolName() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		path     string
		input    APIToolInput
		expected string
		hasError bool
	}{
		{
			name:     "simple path without parameters",
			baseURL:  "https://api.example.com",
			path:     "/users",
			input:    APIToolInput{},
			expected: "https://api.example.com/users",
			hasError: false,
		},
		{
			name:    "path with single parameter",
			baseURL: "https://api.example.com",
			path:    "/users/{id}",
			input: APIToolInput{
				"id": "123",
			},
			expected: "https://api.example.com/users/123",
			hasError: false,
		},
		{
			name:    "path with multiple parameters",
			baseURL: "https://api.example.com",
			path:    "/users/{userId}/posts/{postId}",
			input: APIToolInput{
				"userId": "456",
				"postId": "789",
			},
			expected: "https://api.example.com/users/456/posts/789",
			hasError: false,
		},
		{
			name:     "path with missing parameter",
			baseURL:  "https://api.example.com",
			path:     "/users/{id}",
			input:    APIToolInput{},
			expected: "",
			hasError: true,
		},
		{
			name:     "path with multiple missing parameters",
			baseURL:  "https://api.example.com",
			path:     "/users/{userId}/posts/{postId}",
			input:    APIToolInput{},
			expected: "",
			hasError: true,
		},
		{
			name:     "baseURL with trailing slash",
			baseURL:  "https://api.example.com/",
			path:     "/users",
			input:    APIToolInput{},
			expected: "https://api.example.com/users",
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := buildURL(tt.baseURL, tt.path, tt.input)
			
			if tt.hasError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tt.hasError && result.String() != tt.expected {
				t.Errorf("buildURL() = %v, expected %v", result.String(), tt.expected)
			}
		})
	}
}

func TestAddQueryParams(t *testing.T) {
	tests := []struct {
		name      string
		operation *openapi3.Operation
		input     APIToolInput
		expected  string
	}{
		{
			name: "no parameters",
			operation: &openapi3.Operation{
				Parameters: nil,
			},
			input:    APIToolInput{},
			expected: "",
		},
		{
			name: "single query parameter",
			operation: &openapi3.Operation{
				Parameters: []*openapi3.ParameterRef{
					{
						Value: &openapi3.Parameter{
							Name: "status",
							In:   "query",
						},
					},
				},
			},
			input: APIToolInput{
				"status": "active",
			},
			expected: "status=active",
		},
		{
			name: "multiple query parameters",
			operation: &openapi3.Operation{
				Parameters: []*openapi3.ParameterRef{
					{
						Value: &openapi3.Parameter{
							Name: "status",
							In:   "query",
						},
					},
					{
						Value: &openapi3.Parameter{
							Name: "limit",
							In:   "query",
						},
					},
				},
			},
			input: APIToolInput{
				"status": "active",
				"limit":  "10",
			},
			expected: "limit=10&status=active",
		},
		{
			name: "mixed parameter types (only query should be added)",
			operation: &openapi3.Operation{
				Parameters: []*openapi3.ParameterRef{
					{
						Value: &openapi3.Parameter{
							Name: "status",
							In:   "query",
						},
					},
					{
						Value: &openapi3.Parameter{
							Name: "Authorization",
							In:   "header",
						},
					},
				},
			},
			input: APIToolInput{
				"status":        "active",
				"Authorization": "Bearer token",
			},
			expected: "status=active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL, _ := url.Parse("https://api.example.com/test")
			err := addQueryParams(baseURL, tt.operation, tt.input)
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if baseURL.RawQuery != tt.expected {
				t.Errorf("addQueryParams() query = %v, expected %v", baseURL.RawQuery, tt.expected)
			}
		})
	}
}

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		headers        map[string][]string
		body           string
		expectedStatus int
		expectedBody   interface{}
	}{
		{
			name:           "JSON object response",
			statusCode:     200,
			headers:        map[string][]string{"Content-Type": {"application/json"}},
			body:           `{"name": "John", "age": 30}`,
			expectedStatus: 200,
			expectedBody:   map[string]interface{}{"name": "John", "age": float64(30)},
		},
		{
			name:           "JSON array response",
			statusCode:     200,
			headers:        map[string][]string{"Content-Type": {"application/json"}},
			body:           `[{"id": 1}, {"id": 2}]`,
			expectedStatus: 200,
			expectedBody:   []interface{}{map[string]interface{}{"id": float64(1)}, map[string]interface{}{"id": float64(2)}},
		},
		{
			name:           "JSON string response",
			statusCode:     200,
			headers:        map[string][]string{"Content-Type": {"application/json"}},
			body:           `"hello world"`,
			expectedStatus: 200,
			expectedBody:   "hello world",
		},
		{
			name:           "JSON number response",
			statusCode:     200,
			headers:        map[string][]string{"Content-Type": {"application/json"}},
			body:           `42`,
			expectedStatus: 200,
			expectedBody:   float64(42),
		},
		{
			name:           "JSON boolean response",
			statusCode:     200,
			headers:        map[string][]string{"Content-Type": {"application/json"}},
			body:           `true`,
			expectedStatus: 200,
			expectedBody:   true,
		},
		{
			name:           "empty response",
			statusCode:     204,
			headers:        map[string][]string{},
			body:           "",
			expectedStatus: 204,
			expectedBody:   nil,
		},
		{
			name:           "invalid JSON response",
			statusCode:     200,
			headers:        map[string][]string{"Content-Type": {"application/json"}},
			body:           `{invalid json}`,
			expectedStatus: 200,
			expectedBody:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP response
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     tt.headers,
				Body:       nil,
			}
			
			if tt.body != "" {
				resp.Body = http.NoBody
				resp.Body = &mockReadCloser{strings.NewReader(tt.body)}
			}

			result, err := parseResponse(resp)
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			if result.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, result.StatusCode)
			}
			
			if !reflect.DeepEqual(result.Body, tt.expectedBody) {
				t.Errorf("Expected body %v (%T), got %v (%T)", tt.expectedBody, tt.expectedBody, result.Body, result.Body)
			}
			
			// Check headers are copied
			if len(tt.headers) > 0 && len(result.Headers) == 0 {
				t.Error("Headers should be copied to result")
			}
		})
	}
}

func TestExtractFieldsFromSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   *openapi3.Schema
		input    APIToolInput
		expected map[string]interface{}
	}{
		{
			name:     "nil schema",
			schema:   nil,
			input:    APIToolInput{},
			expected: map[string]interface{}{},
		},
		{
			name: "object schema with properties",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"object"},
				Properties: map[string]*openapi3.SchemaRef{
					"name": {
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						},
					},
					"age": {
						Value: &openapi3.Schema{
							Type: &openapi3.Types{"integer"},
						},
					},
				},
			},
			input: APIToolInput{
				"name": "John",
				"age":  30,
			},
			expected: map[string]interface{}{
				"name": "John",
				"age":  30,
			},
		},
		{
			name: "schema with default values",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"object"},
				Properties: map[string]*openapi3.SchemaRef{
					"name": {
						Value: &openapi3.Schema{
							Type:    &openapi3.Types{"string"},
							Default: "DefaultName",
						},
					},
					"active": {
						Value: &openapi3.Schema{
							Type:    &openapi3.Types{"boolean"},
							Default: true,
						},
					},
				},
			},
			input: APIToolInput{},
			expected: map[string]interface{}{
				"name":   "DefaultName",
				"active": true,
			},
		},
		{
			name: "input overrides defaults",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"object"},
				Properties: map[string]*openapi3.SchemaRef{
					"name": {
						Value: &openapi3.Schema{
							Type:    &openapi3.Types{"string"},
							Default: "DefaultName",
						},
					},
				},
			},
			input: APIToolInput{
				"name": "ProvidedName",
			},
			expected: map[string]interface{}{
				"name": "ProvidedName",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := make(map[string]interface{})
			extractFieldsFromSchema(target, tt.schema, tt.input)
			
			if !reflect.DeepEqual(target, tt.expected) {
				t.Errorf("extractFieldsFromSchema() = %v, expected %v", target, tt.expected)
			}
		})
	}
}

// mockReadCloser implements io.ReadCloser for testing
type mockReadCloser struct {
	*strings.Reader
}

func (m *mockReadCloser) Close() error {
	return nil
}