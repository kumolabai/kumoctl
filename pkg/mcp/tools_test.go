package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/kumolabai/kumoctl/pkg/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
		operation openapi.Operation
		input     APIToolInput
		expected  string
	}{
		{
			name: "no parameters",
			operation: &openapi.OpenAPI3Operation{
				Op: &openapi3.Operation{
					Parameters: nil,
				},
			},
			input:    APIToolInput{},
			expected: "",
		},
		{
			name: "single query parameter",
			operation: &openapi.OpenAPI3Operation{
				Op: &openapi3.Operation{
					Parameters: []*openapi3.ParameterRef{
						{
							Value: &openapi3.Parameter{
								Name: "status",
								In:   "query",
							},
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
			operation: &openapi.OpenAPI3Operation{
				Op: &openapi3.Operation{
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
			},
			input: APIToolInput{
				"status": "active",
				"limit":  "10",
			},
			expected: "limit=10&status=active",
		},
		{
			name: "mixed parameter types (only query should be added)",
			operation: &openapi.OpenAPI3Operation{
				Op: &openapi3.Operation{
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
		schema   openapi.Schema
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
			schema: &openapi.OpenAPI3Schema{
				Schema: &openapi3.Schema{
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
			schema: &openapi.OpenAPI3Schema{
				Schema: &openapi3.Schema{
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
			},
			input: APIToolInput{},
			expected: map[string]interface{}{
				"name":   "DefaultName",
				"active": true,
			},
		},
		{
			name: "input overrides defaults",
			schema: &openapi.OpenAPI3Schema{
				Schema: &openapi3.Schema{
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

func TestPathParametersSpecific(t *testing.T) {
	// Create a mock server that logs exactly what it receives
	requestLog := make([]string, 0)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestLog = append(requestLog, fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		t.Logf("Received request: %s %s", r.Method, r.URL.Path)

		switch r.URL.Path {
		case "/users/123":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   "123",
				"name": "User 123",
			})
		case "/users/456/posts/789":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"userId": "456",
				"postId": "789",
				"title":  "Post 789 by User 456",
			})
		case "/categories/electronics/products/laptop":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"category": "electronics",
				"product":  "laptop",
				"price":    999.99,
			})
		default:
			t.Logf("Unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Not found"})
		}
	}))
	defer mockServer.Close()

	// Create OpenAPI spec with various path parameter patterns
	openAPISpec := fmt.Sprintf(`{
		"openapi": "3.0.0",
		"info": {
			"title": "Path Parameter Test API",
			"version": "1.0.0"
		},
		"servers": [
			{
				"url": "%s"
			}
		],
		"paths": {
			"/users/{userId}": {
				"get": {
					"operationId": "getUserById",
					"summary": "Get user by ID",
					"parameters": [
						{
							"name": "userId",
							"in": "path",
							"required": true,
							"schema": {
								"type": "string"
							}
						}
					],
					"responses": {
						"200": {
							"description": "User details"
						}
					}
				}
			},
			"/users/{userId}/posts/{postId}": {
				"get": {
					"operationId": "getUserPost",
					"summary": "Get specific post by user",
					"parameters": [
						{
							"name": "userId",
							"in": "path",
							"required": true,
							"schema": {
								"type": "string"
							}
						},
						{
							"name": "postId",
							"in": "path",
							"required": true,
							"schema": {
								"type": "string"
							}
						}
					],
					"responses": {
						"200": {
							"description": "Post details"
						}
					}
				}
			},
			"/categories/{categoryName}/products/{productName}": {
				"get": {
					"operationId": "getProduct",
					"summary": "Get product from category",
					"parameters": [
						{
							"name": "categoryName",
							"in": "path",
							"required": true,
							"schema": {
								"type": "string"
							}
						},
						{
							"name": "productName",
							"in": "path",
							"required": true,
							"schema": {
								"type": "string"
							}
						}
					],
					"responses": {
						"200": {
							"description": "Product details"
						}
					}
				}
			}
		}
	}`, mockServer.URL)

	// Write spec to temp file
	tmpFile, err := os.CreateTemp("", "path_params_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(openAPISpec)
	tmpFile.Close()

	// Load spec and generate tools
	spec, err := openapi.LoadSpecFromSource(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load spec: %v", err)
	}

	tools, err := generateMCPToolsFromSpec(spec)
	if err != nil {
		t.Fatalf("Failed to generate tools: %v", err)
	}

	// Test cases for path parameters
	testCases := []struct {
		name         string
		toolName     string
		input        APIToolInput
		expectedPath string
		shouldPass   bool
	}{
		{
			name:         "Single path parameter",
			toolName:     "getUserById",
			input:        APIToolInput{"userId": "123"},
			expectedPath: "/users/123",
			shouldPass:   true,
		},
		{
			name:         "Multiple path parameters",
			toolName:     "getUserPost",
			input:        APIToolInput{"userId": "456", "postId": "789"},
			expectedPath: "/users/456/posts/789",
			shouldPass:   true,
		},
		{
			name:         "String path parameters",
			toolName:     "getProduct",
			input:        APIToolInput{"categoryName": "electronics", "productName": "laptop"},
			expectedPath: "/categories/electronics/products/laptop",
			shouldPass:   true,
		},
		{
			name:         "Missing path parameter should fail",
			toolName:     "getUserById",
			input:        APIToolInput{}, // Missing userId
			expectedPath: "",
			shouldPass:   false,
		},
		{
			name:         "Partial missing parameters should fail",
			toolName:     "getUserPost",
			input:        APIToolInput{"userId": "456"}, // Missing postId
			expectedPath: "",
			shouldPass:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear request log
			requestLog = requestLog[:0]

			// Find the tool
			var targetTool *MCPTool
			for _, tool := range tools {
				if tool.Name == tc.toolName {
					targetTool = tool
					break
				}
			}

			if targetTool == nil {
				t.Fatalf("Tool %s not found", tc.toolName)
			}

			// Execute the tool
			output, err := targetTool.Handler(tc.input)

			if tc.shouldPass {
				if err != nil {
					t.Errorf("Tool execution should have succeeded but failed: %v", err)
					return
				}

				if output.StatusCode != 200 {
					t.Errorf("Expected status code 200, got %d", output.StatusCode)
				}

				// Check if the correct path was requested
				if len(requestLog) == 0 {
					t.Error("No HTTP requests were made")
					return
				}

				lastRequest := requestLog[len(requestLog)-1]
				expectedRequest := fmt.Sprintf("GET %s", tc.expectedPath)

				if lastRequest != expectedRequest {
					t.Errorf("Expected request '%s', got '%s'", expectedRequest, lastRequest)
				}

				t.Logf("✓ Path parameter test passed: %s → %s", tc.toolName, tc.expectedPath)
			} else {
				if err == nil {
					t.Error("Tool execution should have failed but succeeded")
				}
				t.Logf("✓ Expected failure test passed: %v", err)
			}
		})
	}
}

func TestBuildURLDirectly(t *testing.T) {
	// Test the buildURL function directly
	testCases := []struct {
		name        string
		baseURL     string
		path        string
		input       APIToolInput
		expected    string
		expectError bool
	}{
		{
			name:        "Simple path parameter",
			baseURL:     "https://api.example.com",
			path:        "/users/{id}",
			input:       APIToolInput{"id": "123"},
			expected:    "https://api.example.com/users/123",
			expectError: false,
		},
		{
			name:        "Multiple path parameters",
			baseURL:     "https://api.example.com",
			path:        "/users/{userId}/posts/{postId}",
			input:       APIToolInput{"userId": "456", "postId": "789"},
			expected:    "https://api.example.com/users/456/posts/789",
			expectError: false,
		},
		{
			name:        "No path parameters",
			baseURL:     "https://api.example.com",
			path:        "/users",
			input:       APIToolInput{},
			expected:    "https://api.example.com/users",
			expectError: false,
		},
		{
			name:        "Missing required parameter",
			baseURL:     "https://api.example.com",
			path:        "/users/{id}",
			input:       APIToolInput{},
			expected:    "",
			expectError: true,
		},
		{
			name:        "BaseURL with trailing slash",
			baseURL:     "https://api.example.com/",
			path:        "/users/{id}",
			input:       APIToolInput{"id": "123"},
			expected:    "https://api.example.com/users/123",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := buildURL(tc.baseURL, tc.path, tc.input)

			if tc.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result.String() != tc.expected {
				t.Errorf("Expected URL '%s', got '%s'", tc.expected, result.String())
			}
		})
	}
}

func TestCreateAPIHandlerForTool_AdditionalHeaders(t *testing.T) {
	// Create a mock server that logs the headers it receives
	var receivedHeaders http.Header

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Store the headers for verification
		receivedHeaders = r.Header.Clone()
		t.Logf("Received headers: %+v", r.Header)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
	}))
	defer mockServer.Close()

	// Create a simple operation for testing
	operation := &openapi.OpenAPI3Operation{
		Op: &openapi3.Operation{
			OperationID: "testOperation",
			Summary:     "Test operation",
			Parameters:  nil,
		},
	}

	// Create an enriched tool
	tool := &EnrichedTool{
		Tool: &mcp.Tool{
			Name:        "testTool",
			Description: "Test tool",
			InputSchema: nil,
		},
		BaseUrl:   mockServer.URL,
		Method:    "get",
		Path:      "/test",
		Operation: operation,
	}

	testCases := []struct {
		name              string
		additionalHeaders http.Header
		input             APIToolInput
		expectedHeaders   map[string]string
	}{
		{
			name: "Single additional header",
			additionalHeaders: http.Header{
				"Authorization": []string{"Bearer token123"},
			},
			input: APIToolInput{},
			expectedHeaders: map[string]string{
				"Authorization": "Bearer token123",
			},
		},
		{
			name: "Multiple additional headers",
			additionalHeaders: http.Header{
				"Authorization": []string{"Bearer token123"},
				"X-Api-Key":     []string{"api-key-456"},
				"X-Custom":      []string{"custom-value"},
			},
			input: APIToolInput{},
			expectedHeaders: map[string]string{
				"Authorization": "Bearer token123",
				"X-Api-Key":     "api-key-456",
				"X-Custom":      "custom-value",
			},
		},
		{
			name:              "No additional headers",
			additionalHeaders: http.Header{},
			input:             APIToolInput{},
			expectedHeaders:   map[string]string{},
		},
		{
			name: "Header with multiple values",
			additionalHeaders: http.Header{
				"X-Multi": []string{"value1"},
			},
			input: APIToolInput{},
			expectedHeaders: map[string]string{
				"X-Multi": "value1",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset received headers
			receivedHeaders = nil

			// Create handler with additional headers
			handler := createAPIHandlerForTool(tool, tc.additionalHeaders)

			// Execute the handler
			_, output, err := handler(context.Background(), nil, tc.input)

			if err != nil {
				t.Fatalf("Handler execution failed: %v", err)
			}

			if output.Error != "" {
				t.Fatalf("Handler returned error: %s", output.Error)
			}

			if output.StatusCode != http.StatusOK {
				t.Fatalf("Expected status code 200, got %d", output.StatusCode)
			}

			// Verify that all expected headers are present
			for key, expectedValue := range tc.expectedHeaders {
				actualValue := receivedHeaders.Get(key)
				if actualValue != expectedValue {
					t.Errorf("Header %s: expected '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Verify no unexpected additional headers if we specified some
			if len(tc.additionalHeaders) > 0 {
				for key := range tc.additionalHeaders {
					if receivedHeaders.Get(key) == "" {
						t.Errorf("Expected header %s was not found in request", key)
					}
				}
			}

			t.Logf("✓ Additional headers test passed: %s", tc.name)
		})
	}
}

func TestCreateAPIHandlerForTool_AdditionalHeadersWithRequestBody(t *testing.T) {
	// Create a mock server that logs the headers it receives
	var receivedHeaders http.Header

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Store the headers for verification
		receivedHeaders = r.Header.Clone()
		t.Logf("Received headers: %+v", r.Header)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "created",
		})
	}))
	defer mockServer.Close()

	// Create an operation with request body
	operation := &openapi.OpenAPI3Operation{
		Op: &openapi3.Operation{
			OperationID: "createResource",
			Summary:     "Create a resource",
			RequestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: map[string]*openapi3.MediaType{
						"application/json": {
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{
									Type: &openapi3.Types{"object"},
									Properties: map[string]*openapi3.SchemaRef{
										"name": {
											Value: &openapi3.Schema{
												Type: &openapi3.Types{"string"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Create an enriched tool
	tool := &EnrichedTool{
		Tool: &mcp.Tool{
			Name:        "createResource",
			Description: "Create a resource",
			InputSchema: nil,
		},
		BaseUrl:   mockServer.URL,
		Method:    "post",
		Path:      "/resources",
		Operation: operation,
	}

	// Test with additional headers and request body
	additionalHeaders := http.Header{
		"Authorization": []string{"Bearer secret-token"},
		"X-Request-Id":  []string{"req-12345"},
	}

	handler := createAPIHandlerForTool(tool, additionalHeaders)

	input := APIToolInput{
		"name": "Test Resource",
	}

	_, output, err := handler(context.Background(), nil, input)

	if err != nil {
		t.Fatalf("Handler execution failed: %v", err)
	}

	if output.Error != "" {
		t.Fatalf("Handler returned error: %s", output.Error)
	}

	if output.StatusCode != http.StatusOK {
		t.Fatalf("Expected status code 200, got %d", output.StatusCode)
	}

	// Verify additional headers are present
	if receivedHeaders.Get("Authorization") != "Bearer secret-token" {
		t.Errorf("Authorization header: expected 'Bearer secret-token', got '%s'", receivedHeaders.Get("Authorization"))
	}

	if receivedHeaders.Get("X-Request-Id") != "req-12345" {
		t.Errorf("X-Request-Id header: expected 'req-12345', got '%s'", receivedHeaders.Get("X-Request-Id"))
	}

	// Verify Content-Type is set for request body
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type header: expected 'application/json', got '%s'", receivedHeaders.Get("Content-Type"))
	}

	t.Logf("✓ Additional headers with request body test passed")
}

func TestPathParametersOpenAPI2(t *testing.T) {
	// Create a mock server for OpenAPI 2.0 path parameter testing
	requestLog := make([]string, 0)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestLog = append(requestLog, fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		t.Logf("Received request: %s %s", r.Method, r.URL.Path)

		switch r.URL.Path {
		case "/v1/users/42":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":   "42",
				"name": "User 42",
			})
		case "/v1/users/42/orders/100":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"userId":  "42",
				"orderId": "100",
				"total":   99.99,
			})
		case "/v1/products/electronics":
			// Test query parameters along with path parameters
			limit := r.URL.Query().Get("limit")
			if limit == "" {
				limit = "10" // default
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"category": "electronics",
				"limit":    limit,
				"products": []string{"laptop", "phone"},
			})
		default:
			t.Logf("Unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Not found"})
		}
	}))
	defer mockServer.Close()

	// Create OpenAPI 2.0 spec with path parameters
	openAPI2Spec := fmt.Sprintf(`{
		"swagger": "2.0",
		"info": {
			"title": "OpenAPI 2.0 Path Parameter Test",
			"version": "1.0.0"
		},
		"host": "%s",
		"basePath": "/v1",
		"schemes": ["http"],
		"paths": {
			"/users/{userId}": {
				"get": {
					"operationId": "getUserById",
					"summary": "Get user by ID",
					"parameters": [
						{
							"name": "userId",
							"in": "path",
							"required": true,
							"type": "string"
						}
					],
					"responses": {
						"200": {
							"description": "User details"
						}
					}
				}
			},
			"/users/{userId}/orders/{orderId}": {
				"get": {
					"operationId": "getUserOrder",
					"summary": "Get user order",
					"parameters": [
						{
							"name": "userId",
							"in": "path",
							"required": true,
							"type": "string"
						},
						{
							"name": "orderId",
							"in": "path",
							"required": true,
							"type": "integer"
						}
					],
					"responses": {
						"200": {
							"description": "Order details"
						}
					}
				}
			},
			"/products/{category}": {
				"get": {
					"operationId": "getProductsByCategory",
					"summary": "Get products by category",
					"parameters": [
						{
							"name": "category",
							"in": "path",
							"required": true,
							"type": "string"
						},
						{
							"name": "limit",
							"in": "query",
							"type": "integer",
							"default": 10
						}
					],
					"responses": {
						"200": {
							"description": "Products in category"
						}
					}
				}
			}
		}
	}`, mockServer.Listener.Addr().String())

	// Write spec to temp file
	tmpFile, err := os.CreateTemp("", "openapi2_path_params_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString(openAPI2Spec)
	tmpFile.Close()

	// Load spec and verify it's OpenAPI 2.0
	spec, err := openapi.LoadSpecFromSource(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load spec: %v", err)
	}

	if spec.GetVersion() != "2.0" {
		t.Fatalf("Expected OpenAPI 2.0, got %s", spec.GetVersion())
	}

	// Generate tools
	tools, err := generateMCPToolsFromSpec(spec)
	if err != nil {
		t.Fatalf("Failed to generate tools: %v", err)
	}

	// Test cases for OpenAPI 2.0 path parameters
	testCases := []struct {
		name         string
		toolName     string
		input        APIToolInput
		expectedPath string
		shouldPass   bool
	}{
		{
			name:         "OpenAPI2 - Single path parameter",
			toolName:     "getUserById",
			input:        APIToolInput{"userId": "42"},
			expectedPath: "/v1/users/42",
			shouldPass:   true,
		},
		{
			name:         "OpenAPI2 - Multiple path parameters",
			toolName:     "getUserOrder",
			input:        APIToolInput{"userId": "42", "orderId": 100},
			expectedPath: "/v1/users/42/orders/100",
			shouldPass:   true,
		},
		{
			name:         "OpenAPI2 - Path + query parameters",
			toolName:     "getProductsByCategory",
			input:        APIToolInput{"category": "electronics", "limit": 5},
			expectedPath: "/v1/products/electronics",
			shouldPass:   true,
		},
		{
			name:         "OpenAPI2 - Path param only (query param uses default)",
			toolName:     "getProductsByCategory",
			input:        APIToolInput{"category": "electronics"},
			expectedPath: "/v1/products/electronics",
			shouldPass:   true,
		},
		{
			name:         "OpenAPI2 - Missing required path parameter",
			toolName:     "getUserById",
			input:        APIToolInput{},
			expectedPath: "",
			shouldPass:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear request log
			requestLog = requestLog[:0]

			// Find the tool
			var targetTool *MCPTool
			for _, tool := range tools {
				if tool.Name == tc.toolName {
					targetTool = tool
					break
				}
			}

			if targetTool == nil {
				t.Fatalf("Tool %s not found", tc.toolName)
			}

			// Execute the tool
			output, err := targetTool.Handler(tc.input)

			if tc.shouldPass {
				if err != nil {
					t.Errorf("Tool execution should have succeeded but failed: %v", err)
					return
				}

				if output.StatusCode != 200 {
					t.Errorf("Expected status code 200, got %d", output.StatusCode)
				}

				// Check if the correct path was requested
				if len(requestLog) == 0 {
					t.Error("No HTTP requests were made")
					return
				}

				lastRequest := requestLog[len(requestLog)-1]
				expectedRequest := fmt.Sprintf("GET %s", tc.expectedPath)

				if lastRequest != expectedRequest {
					t.Errorf("Expected request '%s', got '%s'", expectedRequest, lastRequest)
				}

				// For the query parameter test, also check query params
				if tc.toolName == "getProductsByCategory" {
					// The mock server response should include the limit
					bodyObj, ok := output.Body.(map[string]interface{})
					if !ok {
						t.Error("Expected object response")
					} else {
						expectedLimit := "10" // default
						if limitVal, exists := tc.input["limit"]; exists {
							expectedLimit = fmt.Sprintf("%v", limitVal)
						}
						if bodyObj["limit"] != expectedLimit {
							t.Errorf("Expected limit %s, got %v", expectedLimit, bodyObj["limit"])
						}
					}
				}

				t.Logf("✓ OpenAPI 2.0 path parameter test passed: %s → %s", tc.toolName, tc.expectedPath)
			} else {
				if err == nil {
					t.Error("Tool execution should have failed but succeeded")
				}
				t.Logf("✓ Expected failure test passed: %v", err)
			}
		})
	}
}
