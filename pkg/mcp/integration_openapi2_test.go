package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/kumolabai/kumoctl/pkg/openapi"
)

func TestMCPToolIntegrationWithOpenAPI2(t *testing.T) {
	// Create a mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Mock server received: %s %s", r.Method, r.URL.Path)
		t.Logf("Query params: %s", r.URL.RawQuery)

		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/users":
			status := r.URL.Query().Get("status")
			users := []map[string]interface{}{
				{
					"id":     "1",
					"name":   "John Doe",
					"email":  "john@example.com",
					"active": status == "active",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(users)

		case r.Method == "POST" && r.URL.Path == "/v1/users":
			var requestBody map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			user := requestBody
			user["id"] = "456"

			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(user)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	// Create OpenAPI 2.0 spec that points to the mock server
	// Note: We need to extract the host and set schemes appropriately
	openAPI2Spec := fmt.Sprintf(`{
		"swagger": "2.0",
		"info": {
			"title": "Test API v2",
			"version": "1.0.0",
			"description": "A test API using OpenAPI 2.0"
		},
		"host": "%s",
		"basePath": "/v1",
		"schemes": ["http"],
		"consumes": ["application/json"],
		"produces": ["application/json"],
		"paths": {
			"/users": {
				"get": {
					"operationId": "getUsers",
					"summary": "Get all users",
					"parameters": [
						{
							"name": "status",
							"in": "query",
							"type": "string",
							"enum": ["active", "inactive"]
						},
						{
							"name": "limit", 
							"in": "query",
							"type": "integer",
							"format": "int32",
							"default": 10
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
					"parameters": [
						{
							"name": "user",
							"in": "body",
							"required": true,
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
					],
					"responses": {
						"201": {
							"description": "User created"
						}
					}
				}
			}
		}
	}`, mockServer.Listener.Addr().String())

	// Write the spec to a temporary file
	tmpFile, err := os.CreateTemp("", "openapi2_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(openAPI2Spec); err != nil {
		t.Fatalf("Failed to write spec to temp file: %v", err)
	}
	tmpFile.Close()

	// Load the OpenAPI 2.0 spec
	spec, err := openapi.LoadSpecFromSource(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load OpenAPI 2.0 spec: %v", err)
	}

	// Verify it's recognized as OpenAPI 2.0
	version := spec.GetVersion()
	if version != "2.0" {
		t.Fatalf("Expected OpenAPI version 2.0, got %s", version)
	}

	// Verify the base URL construction
	expectedBaseURL := fmt.Sprintf("http://%s/v1", mockServer.Listener.Addr().String())
	if spec.GetBaseURL() != expectedBaseURL {
		t.Fatalf("Expected base URL %s, got %s", expectedBaseURL, spec.GetBaseURL())
	}

	// Generate MCP tools from the spec
	tools, err := generateMCPToolsFromSpec(spec)
	if err != nil {
		t.Fatalf("Failed to generate MCP tools: %v", err)
	}

	// Verify we have the expected tools
	expectedTools := []string{"getUsers", "createUser"}
	if len(tools) != len(expectedTools) {
		t.Fatalf("Expected %d tools, got %d", len(expectedTools), len(tools))
	}

	// Test GET /users
	t.Run("OpenAPI2_GET_users", func(t *testing.T) {
		var getUsersTool *MCPTool
		for _, tool := range tools {
			if tool.Name == "getUsers" {
				getUsersTool = tool
				break
			}
		}

		if getUsersTool == nil {
			t.Fatal("getUsers tool not found")
		}

		// Execute the tool
		output, err := getUsersTool.Handler(APIToolInput{"status": "active", "limit": 5})
		if err != nil {
			t.Fatalf("Tool execution failed: %v", err)
		}

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

		t.Logf("OpenAPI 2.0 GET test successful: %v", output.Body)
	})

	// Test POST /users
	t.Run("OpenAPI2_POST_users", func(t *testing.T) {
		var createUserTool *MCPTool
		for _, tool := range tools {
			if tool.Name == "createUser" {
				createUserTool = tool
				break
			}
		}

		if createUserTool == nil {
			t.Fatal("createUser tool not found")
		}

		// Execute the tool
		output, err := createUserTool.Handler(APIToolInput{
			"name":   "Bob Wilson",
			"email":  "bob@example.com",
			"active": true,
		})
		if err != nil {
			t.Fatalf("Tool execution failed: %v", err)
		}

		if output.StatusCode != 201 {
			t.Errorf("Expected status code 201, got %d", output.StatusCode)
		}

		bodyObj, ok := output.Body.(map[string]interface{})
		if !ok {
			t.Errorf("Expected object response, got %T", output.Body)
			return
		}

		if bodyObj["name"] != "Bob Wilson" {
			t.Errorf("Expected name 'Bob Wilson', got %v", bodyObj["name"])
		}
		if bodyObj["id"] != "456" {
			t.Errorf("Expected ID '456', got %v", bodyObj["id"])
		}

		t.Logf("OpenAPI 2.0 POST test successful: %v", output.Body)
	})
}
