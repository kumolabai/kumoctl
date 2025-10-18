package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/kumolabai/kumoctl/pkg/openapi"
)

// MCPTool represents a tool for testing purposes
type MCPTool struct {
	Name        string
	Description string
	Schema      *jsonschema.Schema
	Handler     func(APIToolInput) (*APIToolOutput, error)
}

// generateMCPToolsFromSpec creates tools for testing (similar to GenerateToolsFromSpec but returns tools)
func generateMCPToolsFromSpec(spec openapi.APISpec) ([]*MCPTool, error) {
	var tools []*MCPTool
	baseURL := spec.GetBaseURL()

	for path, pathItem := range spec.GetPaths() {
		for method, operation := range pathItem.GetOperations() {
			if operation == nil {
				continue
			}

			toolName := generateToolName(method, path, operation.GetOperationID())
			description := operation.GetSummary()
			if description == "" {
				description = fmt.Sprintf("%s %s", strings.ToUpper(method), path)
			}

			// Generate input schema for this tool
			inputSchema, err := openapi.GenerateInputSchema(operation)
			if err != nil {
				return nil, fmt.Errorf("failed to generate input schema for %s %s: %w", method, path, err)
			}

			// Create the tool handler
			handler := createAPIToolHandler(method, baseURL, path, operation)

			tool := &MCPTool{
				Name:        toolName,
				Description: description,
				Schema:      inputSchema,
				Handler:     handler,
			}

			tools = append(tools, tool)
		}
	}

	return tools, nil
}

// createAPIToolHandler creates a handler function for an API tool
func createAPIToolHandler(method, baseURL, path string, operation openapi.Operation) func(APIToolInput) (*APIToolOutput, error) {
	return func(input APIToolInput) (*APIToolOutput, error) {
		// Build the URL
		fullURL, err := buildURL(baseURL, path, input)
		if err != nil {
			return nil, fmt.Errorf("failed to build URL: %w", err)
		}

		// Add query parameters
		if err := addQueryParams(fullURL, operation, input); err != nil {
			return nil, fmt.Errorf("failed to add query parameters: %w", err)
		}

		// Create HTTP request
		var requestBody *bytes.Buffer
		if requestBodyData := operation.GetRequestBody(); requestBodyData != nil {
			bodyMap := make(map[string]interface{})

			// Get the request body schema
			schema, err := requestBodyData.GetJSONSchema()
			if err != nil {
				return nil, fmt.Errorf("failed to get request body schema: %w", err)
			}

			if schema != nil {
				extractFieldsFromSchema(bodyMap, schema, input)

				if len(bodyMap) > 0 {
					bodyBytes, err := json.Marshal(bodyMap)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal request body: %w", err)
					}
					requestBody = bytes.NewBuffer(bodyBytes)
				}
			}
		}

		// Create HTTP request
		var req *http.Request
		if requestBody != nil {
			req, err = http.NewRequest(strings.ToUpper(method), fullURL.String(), requestBody)
			if err != nil {
				return nil, fmt.Errorf("failed to create HTTP request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, err = http.NewRequest(strings.ToUpper(method), fullURL.String(), nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create HTTP request: %w", err)
			}
		}

		// Make the HTTP request
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}
		defer resp.Body.Close()

		// Parse the response
		output, err := parseResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		return &output, nil
	}
}

func TestMCPToolIntegrationWithMockServer(t *testing.T) {
	// Create a mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request for debugging
		t.Logf("Mock server received: %s %s", r.Method, r.URL.Path)
		t.Logf("Query params: %s", r.URL.RawQuery)
		t.Logf("Headers: %v", r.Header)

		// Handle different endpoints
		switch {
		case r.Method == "GET" && r.URL.Path == "/users":
			// Test query parameters
			status := r.URL.Query().Get("status")
			limit := r.URL.Query().Get("limit")

			users := []map[string]interface{}{
				{
					"id":     "1",
					"name":   "John Doe",
					"email":  "john@example.com",
					"active": status != "inactive",
				},
			}

			if limit == "0" {
				users = []map[string]interface{}{}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(users)

		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/users/"):
			// Test path parameters
			userID := strings.TrimPrefix(r.URL.Path, "/users/")

			if userID == "999" {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
				return
			}

			user := map[string]interface{}{
				"id":     userID,
				"name":   fmt.Sprintf("User %s", userID),
				"email":  fmt.Sprintf("user%s@example.com", userID),
				"active": true,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(user)

		case r.Method == "POST" && r.URL.Path == "/users":
			// Test request body
			var requestBody map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
				return
			}

			// Create response with generated ID
			user := requestBody
			user["id"] = "123"

			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(user)

		default:
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Endpoint not found"})
		}
	}))
	defer mockServer.Close()

	// Create OpenAPI spec that points to the mock server
	openAPISpec := fmt.Sprintf(`{
		"openapi": "3.0.0",
		"info": {
			"title": "Test API",
			"version": "1.0.0"
		},
		"servers": [
			{
				"url": "%s"
			}
		],
		"paths": {
			"/users": {
				"get": {
					"operationId": "getUsers",
					"summary": "Get all users",
					"parameters": [
						{
							"name": "status",
							"in": "query",
							"schema": {
								"type": "string",
								"enum": ["active", "inactive"]
							}
						},
						{
							"name": "limit",
							"in": "query",
							"schema": {
								"type": "integer",
								"format": "int32",
								"default": 10
							}
						}
					],
					"responses": {
						"200": {
							"description": "List of users"
						}
					}
				},
				"post": {
					"operationId": "createUser",
					"summary": "Create a new user",
					"requestBody": {
						"required": true,
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"required": ["name", "email"],
									"properties": {
										"name": {
											"type": "string"
										},
										"email": {
											"type": "string",
											"format": "email"
										},
										"active": {
											"type": "boolean",
											"default": true
										}
									}
								}
							}
						}
					},
					"responses": {
						"201": {
							"description": "User created"
						}
					}
				}
			},
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
			}
		}
	}`, mockServer.URL)

	// Write the spec to a temporary file
	tmpFile, err := os.CreateTemp("", "openapi_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(openAPISpec); err != nil {
		t.Fatalf("Failed to write spec to temp file: %v", err)
	}
	tmpFile.Close()

	// Load the OpenAPI spec
	spec, err := openapi.LoadSpecFromSource(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load OpenAPI spec: %v", err)
	}

	// Generate MCP tools from the spec
	tools, err := generateMCPToolsFromSpec(spec)
	if err != nil {
		t.Fatalf("Failed to generate MCP tools: %v", err)
	}

	// Verify we have the expected tools
	expectedTools := []string{"getUsers", "createUser", "getUserById"}
	if len(tools) != len(expectedTools) {
		t.Fatalf("Expected %d tools, got %d", len(expectedTools), len(tools))
	}

	for _, expectedTool := range expectedTools {
		found := false
		for _, tool := range tools {
			if tool.Name == expectedTool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected tool %s not found", expectedTool)
		}
	}

	// Test scenarios
	testCases := []struct {
		name         string
		toolName     string
		input        APIToolInput
		expectedCode int
		validate     func(*testing.T, *APIToolOutput)
	}{
		{
			name:         "GET /users with query parameters",
			toolName:     "getUsers",
			input:        APIToolInput{"status": "active", "limit": 1},
			expectedCode: 200,
			validate: func(t *testing.T, output *APIToolOutput) {
				if output.StatusCode != 200 {
					t.Errorf("Expected status code 200, got %d", output.StatusCode)
				}

				// Body should be an array
				bodyArray, ok := output.Body.([]interface{})
				if !ok {
					t.Errorf("Expected array response, got %T", output.Body)
					return
				}

				if len(bodyArray) != 1 {
					t.Errorf("Expected 1 user, got %d", len(bodyArray))
				}
			},
		},
		{
			name:         "GET /users/{userId} with path parameter",
			toolName:     "getUserById",
			input:        APIToolInput{"userId": "42"},
			expectedCode: 200,
			validate: func(t *testing.T, output *APIToolOutput) {
				if output.StatusCode != 200 {
					t.Errorf("Expected status code 200, got %d", output.StatusCode)
				}

				// Body should be an object
				bodyObj, ok := output.Body.(map[string]interface{})
				if !ok {
					t.Errorf("Expected object response, got %T", output.Body)
					return
				}

				if bodyObj["id"] != "42" {
					t.Errorf("Expected user ID 42, got %v", bodyObj["id"])
				}
			},
		},
		{
			name:         "POST /users with request body",
			toolName:     "createUser",
			input:        APIToolInput{"name": "Alice Smith", "email": "alice@example.com", "active": false},
			expectedCode: 201,
			validate: func(t *testing.T, output *APIToolOutput) {
				if output.StatusCode != 201 {
					t.Errorf("Expected status code 201, got %d", output.StatusCode)
				}

				bodyObj, ok := output.Body.(map[string]interface{})
				if !ok {
					t.Errorf("Expected object response, got %T", output.Body)
					return
				}

				if bodyObj["name"] != "Alice Smith" {
					t.Errorf("Expected name 'Alice Smith', got %v", bodyObj["name"])
				}
				if bodyObj["id"] == nil {
					t.Error("Expected generated ID, got nil")
				}
			},
		},
		{
			name:         "GET /users with limit=0 (empty array response)",
			toolName:     "getUsers",
			input:        APIToolInput{"limit": 0},
			expectedCode: 200,
			validate: func(t *testing.T, output *APIToolOutput) {
				bodyArray, ok := output.Body.([]interface{})
				if !ok {
					t.Errorf("Expected array response, got %T", output.Body)
					return
				}

				if len(bodyArray) != 0 {
					t.Errorf("Expected empty array, got %d items", len(bodyArray))
				}
			},
		},
		{
			name:         "GET /users/{userId} with non-existent user (404)",
			toolName:     "getUserById",
			input:        APIToolInput{"userId": "999"},
			expectedCode: 404,
			validate: func(t *testing.T, output *APIToolOutput) {
				if output.StatusCode != 404 {
					t.Errorf("Expected status code 404, got %d", output.StatusCode)
				}

				bodyObj, ok := output.Body.(map[string]interface{})
				if !ok {
					t.Errorf("Expected object response, got %T", output.Body)
					return
				}

				if bodyObj["error"] != "User not found" {
					t.Errorf("Expected error message, got %v", bodyObj["error"])
				}
			},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
			if err != nil {
				t.Fatalf("Tool execution failed: %v", err)
			}

			// Validate the output
			if output.StatusCode != tc.expectedCode {
				t.Errorf("Expected status code %d, got %d", tc.expectedCode, output.StatusCode)
			}

			// Run custom validation
			if tc.validate != nil {
				tc.validate(t, output)
			}

			// Verify response headers
			if output.Headers["Content-Type"] == "" {
				t.Error("Expected Content-Type header")
			}

			t.Logf("Tool %s executed successfully: %d %v", tc.toolName, output.StatusCode, output.Body)
		})
	}
}

func TestMCPToolErrorHandling(t *testing.T) {
	// Create a mock server that returns various error conditions
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/timeout":
			// Simulate timeout
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		case "/invalid-json":
			// Return invalid JSON
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"invalid": json}`))
		case "/server-error":
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Internal server error"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	// Create a simple OpenAPI spec
	openAPISpec := fmt.Sprintf(`{
		"openapi": "3.0.0",
		"info": {"title": "Error Test API", "version": "1.0.0"},
		"servers": [{"url": "%s"}],
		"paths": {
			"/timeout": {
				"get": {
					"operationId": "timeoutEndpoint",
					"responses": {"200": {"description": "OK"}}
				}
			},
			"/invalid-json": {
				"get": {
					"operationId": "invalidJsonEndpoint", 
					"responses": {"200": {"description": "OK"}}
				}
			},
			"/server-error": {
				"get": {
					"operationId": "serverErrorEndpoint",
					"responses": {"500": {"description": "Error"}}
				}
			}
		}
	}`, mockServer.URL)

	// Write spec to temp file
	tmpFile, err := os.CreateTemp("", "error_test_*.json")
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

	// Test error conditions
	errorTests := []struct {
		name     string
		toolName string
		timeout  time.Duration
	}{
		{
			name:     "Server error response",
			toolName: "serverErrorEndpoint",
			timeout:  5 * time.Second,
		},
		{
			name:     "Invalid JSON response",
			toolName: "invalidJsonEndpoint",
			timeout:  5 * time.Second,
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			var targetTool *MCPTool
			for _, tool := range tools {
				if tool.Name == tt.toolName {
					targetTool = tool
					break
				}
			}

			if targetTool == nil {
				t.Fatalf("Tool %s not found", tt.toolName)
			}

			// Execute tool with timeout
			done := make(chan bool, 1)
			var output *APIToolOutput
			var err error

			go func() {
				output, err = targetTool.Handler(APIToolInput{})
				done <- true
			}()

			select {
			case <-done:
				// Tool completed
				if err != nil {
					t.Logf("Tool returned error as expected: %v", err)
				}
				if output != nil {
					t.Logf("Tool output: %d %v", output.StatusCode, output.Body)
				}
			case <-time.After(tt.timeout):
				t.Errorf("Tool execution timed out after %v", tt.timeout)
			}
		})
	}
}
