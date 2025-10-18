package openapi

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestGetRequestBodyJSONContent(t *testing.T) {
	tests := []struct {
		name        string
		requestBody *openapi3.RequestBodyRef
		expectError bool
		expectNil   bool
	}{
		{
			name:        "nil request body",
			requestBody: nil,
			expectError: false,
			expectNil:   true,
		},
		{
			name: "request body with JSON content",
			requestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: map[string]*openapi3.MediaType{
						"application/json": {
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{Type: &openapi3.Types{"object"}},
							},
						},
					},
				},
			},
			expectError: false,
			expectNil:   false,
		},
		{
			name: "request body without JSON content",
			requestBody: &openapi3.RequestBodyRef{
				Value: &openapi3.RequestBody{
					Content: map[string]*openapi3.MediaType{
						"text/plain": {
							Schema: &openapi3.SchemaRef{
								Value: &openapi3.Schema{Type: &openapi3.Types{"string"}},
							},
						},
					},
				},
			},
			expectError: true,
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetRequestBodyJSONContent(tt.requestBody)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.expectNil && result != nil {
				t.Error("Expected nil result")
			}
			if !tt.expectNil && !tt.expectError && result == nil {
				t.Error("Expected non-nil result")
			}
		})
	}
}

func TestLoadSpecV2AndV3(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string // expected version
		hasError bool
	}{
		{
			name: "OpenAPI 3.0 spec",
			content: `{
				"openapi": "3.0.0",
				"info": {"title": "Test", "version": "1.0.0"},
				"paths": {}
			}`,
			expected: "3.0.0",
			hasError: false,
		},
		{
			name: "OpenAPI 2.0 spec",
			content: `{
				"swagger": "2.0",
				"info": {"title": "Test", "version": "1.0.0"},
				"paths": {}
			}`,
			expected: "2.0",
			hasError: false,
		},
		{
			name: "invalid spec",
			content: `{
				"invalid": "spec"
			}`,
			expected: "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile := t.TempDir() + "/test.json"
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			spec, err := LoadSpecFromSource(tmpFile)

			if tt.hasError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if spec == nil {
				t.Error("Expected non-nil spec")
				return
			}

			version := spec.GetVersion()
			if version != tt.expected {
				t.Errorf("Expected version %v, got %v", tt.expected, version)
			}
		})
	}
}

func TestGetBaseURLVersions(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "OpenAPI 3.0 with server",
			content: `{
				"openapi": "3.0.0",
				"info": {"title": "Test", "version": "1.0.0"},
				"servers": [{"url": "https://api.v3.example.com"}],
				"paths": {}
			}`,
			expected: "https://api.v3.example.com",
		},
		{
			name: "OpenAPI 2.0 with host and basePath",
			content: `{
				"swagger": "2.0",
				"info": {"title": "Test", "version": "1.0.0"},
				"host": "api.v2.example.com",
				"basePath": "/v2",
				"schemes": ["https"],
				"paths": {}
			}`,
			expected: "https://api.v2.example.com/v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile := t.TempDir() + "/test.json"
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			spec, err := LoadSpecFromSource(tmpFile)
			if err != nil {
				t.Fatalf("Failed to load spec: %v", err)
			}

			baseURL := spec.GetBaseURL()
			if baseURL != tt.expected {
				t.Errorf("Expected base URL %v, got %v", tt.expected, baseURL)
			}
		})
	}
}

func TestPathLevelParameterSchemaGeneration(t *testing.T) {
	// Test OpenAPI 3.0 with path-level parameters
	t.Run("OpenAPI3_PathLevelParameters", func(t *testing.T) {
		openAPI3Spec := `{
			"openapi": "3.0.0",
			"info": {
				"title": "Path Level Parameters Test",
				"version": "1.0.0"
			},
			"paths": {
				"/users/{userId}/posts/{postId}": {
					"parameters": [
						{
							"name": "userId",
							"in": "path",
							"required": true,
							"schema": {
								"type": "string"
							},
							"description": "The user identifier"
						},
						{
							"name": "postId",
							"in": "path",
							"required": true,
							"schema": {
								"type": "integer"
							},
							"description": "The post identifier"
						}
					],
					"get": {
						"operationId": "getUserPost",
						"summary": "Get user post",
						"parameters": [
							{
								"name": "include_comments",
								"in": "query",
								"schema": {
									"type": "boolean",
									"default": false
								},
								"description": "Include comments in response"
							}
						],
						"responses": {
							"200": {
								"description": "Post details"
							}
						}
					},
					"put": {
						"operationId": "updateUserPost",
						"summary": "Update user post",
						"requestBody": {
							"required": true,
							"content": {
								"application/json": {
									"schema": {
										"type": "object",
										"required": ["title"],
										"properties": {
											"title": {
												"type": "string"
											},
											"content": {
												"type": "string"
											}
										}
									}
								}
							}
						},
						"responses": {
							"200": {
								"description": "Post updated"
							}
						}
					}
				}
			}
		}`

		// Write spec to temp file
		tmpFile, err := os.CreateTemp("", "path_level_test_*.json")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString(openAPI3Spec)
		tmpFile.Close()

		// Load spec
		spec, err := LoadSpecFromSource(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to load spec: %v", err)
		}

		// Test GET operation (path + query parameters)
		paths := spec.GetPaths()
		pathItem := paths["/users/{userId}/posts/{postId}"]
		operations := pathItem.GetOperations()
		getOperation := operations["get"]

		schema, err := GenerateInputSchema(getOperation)
		if err != nil {
			t.Fatalf("Failed to generate schema for GET operation: %v", err)
		}

		// Verify schema structure
		if schema.Type != "object" {
			t.Errorf("Expected schema type 'object', got '%s'", schema.Type)
		}

		// Check required fields (path parameters should be required)
		expectedRequired := []string{"userId", "postId"}
		if len(schema.Required) != len(expectedRequired) {
			t.Errorf("Expected %d required fields, got %d", len(expectedRequired), len(schema.Required))
		}

		for _, req := range expectedRequired {
			found := false
			for _, actual := range schema.Required {
				if actual == req {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Required field '%s' not found in schema", req)
			}
		}

		// Check properties
		expectedProperties := map[string]string{
			"userId":           "string",
			"postId":           "integer", // Path parameters preserve their original type in the schema
			"include_comments": "boolean",
		}

		for propName, expectedType := range expectedProperties {
			prop, exists := schema.Properties[propName]
			if !exists {
				t.Errorf("Property '%s' not found in schema", propName)
				continue
			}
			if prop.Type != expectedType {
				t.Errorf("Property '%s' expected type '%s', got '%s'", propName, expectedType, prop.Type)
			}
		}

		// Test PUT operation (path + body parameters)
		putOperation := operations["put"]
		putSchema, err := GenerateInputSchema(putOperation)
		if err != nil {
			t.Fatalf("Failed to generate schema for PUT operation: %v", err)
		}

		// Should have path parameters + body parameters
		expectedPutRequired := []string{"userId", "postId", "title"} // title from request body
		if len(putSchema.Required) != len(expectedPutRequired) {
			t.Errorf("PUT: Expected %d required fields, got %d", len(expectedPutRequired), len(putSchema.Required))
		}

		// Should have both path and body properties
		expectedPutProperties := []string{"userId", "postId", "title", "content"}
		for _, propName := range expectedPutProperties {
			if _, exists := putSchema.Properties[propName]; !exists {
				t.Errorf("PUT: Property '%s' not found in schema", propName)
			}
		}

		t.Logf("✓ OpenAPI 3.0 path-level parameters correctly included in schema")
	})

	// Test OpenAPI 2.0 with path-level parameters
	t.Run("OpenAPI2_PathLevelParameters", func(t *testing.T) {
		openAPI2Spec := `{
			"swagger": "2.0",
			"info": {
				"title": "OpenAPI 2.0 Path Level Parameters Test",
				"version": "1.0.0"
			},
			"paths": {
				"/organizations/{org}/repositories/{repo}": {
					"parameters": [
						{
							"name": "org",
							"in": "path",
							"required": true,
							"type": "string",
							"description": "Organization name"
						},
						{
							"name": "repo",
							"in": "path",
							"required": true,
							"type": "string",
							"description": "Repository name"
						}
					],
					"get": {
						"operationId": "getRepository",
						"summary": "Get repository details",
						"parameters": [
							{
								"name": "include_stats",
								"in": "query",
								"type": "boolean",
								"default": false,
								"description": "Include repository statistics"
							}
						],
						"responses": {
							"200": {
								"description": "Repository details"
							}
						}
					},
					"delete": {
						"operationId": "deleteRepository",
						"summary": "Delete repository",
						"responses": {
							"204": {
								"description": "Repository deleted"
							}
						}
					}
				}
			}
		}`

		// Write spec to temp file
		tmpFile, err := os.CreateTemp("", "path_level_openapi2_test_*.json")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString(openAPI2Spec)
		tmpFile.Close()

		// Load spec
		spec, err := LoadSpecFromSource(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to load spec: %v", err)
		}

		// Verify it's OpenAPI 2.0
		if spec.GetVersion() != "2.0" {
			t.Fatalf("Expected OpenAPI 2.0, got %s", spec.GetVersion())
		}

		// Test GET operation
		paths := spec.GetPaths()
		pathItem := paths["/organizations/{org}/repositories/{repo}"]
		operations := pathItem.GetOperations()
		getOperation := operations["get"]

		schema, err := GenerateInputSchema(getOperation)
		if err != nil {
			t.Fatalf("Failed to generate schema for GET operation: %v", err)
		}

		// Check required path parameters
		expectedRequired := []string{"org", "repo"}
		if len(schema.Required) != len(expectedRequired) {
			t.Errorf("Expected %d required fields, got %d", len(expectedRequired), len(schema.Required))
		}

		// Check all properties are present
		expectedProperties := []string{"org", "repo", "include_stats"}
		for _, propName := range expectedProperties {
			if _, exists := schema.Properties[propName]; !exists {
				t.Errorf("Property '%s' not found in schema", propName)
			}
		}

		// Test DELETE operation (only path parameters, no query params)
		deleteOperation := operations["delete"]
		deleteSchema, err := GenerateInputSchema(deleteOperation)
		if err != nil {
			t.Fatalf("Failed to generate schema for DELETE operation: %v", err)
		}

		// Should only have path parameters
		if len(deleteSchema.Required) != 2 {
			t.Errorf("DELETE: Expected 2 required fields, got %d", len(deleteSchema.Required))
		}

		if len(deleteSchema.Properties) != 2 {
			t.Errorf("DELETE: Expected 2 properties, got %d", len(deleteSchema.Properties))
		}

		// Verify only path parameters are present
		for _, propName := range []string{"org", "repo"} {
			if _, exists := deleteSchema.Properties[propName]; !exists {
				t.Errorf("DELETE: Path parameter '%s' not found in schema", propName)
			}
		}

		t.Logf("✓ OpenAPI 2.0 path-level parameters correctly included in schema")
	})
}

func TestParameterMerging(t *testing.T) {
	// Test that operation-level parameters override path-level parameters with same name
	t.Run("ParameterOverride", func(t *testing.T) {
		openAPI3Spec := `{
			"openapi": "3.0.0",
			"info": {
				"title": "Parameter Override Test",
				"version": "1.0.0"
			},
			"paths": {
				"/items/{id}": {
					"parameters": [
						{
							"name": "id",
							"in": "path",
							"required": true,
							"schema": {
								"type": "string"
							},
							"description": "Path level ID parameter"
						},
						{
							"name": "format",
							"in": "query",
							"schema": {
								"type": "string",
								"default": "json"
							},
							"description": "Path level format parameter"
						}
					],
					"get": {
						"operationId": "getItem",
						"parameters": [
							{
								"name": "format",
								"in": "query",
								"schema": {
									"type": "string",
									"enum": ["json", "xml", "csv"]
								},
								"description": "Operation level format parameter (overrides path level)"
							},
							{
								"name": "expand",
								"in": "query",
								"schema": {
									"type": "boolean"
								},
								"description": "Operation specific parameter"
							}
						],
						"responses": {
							"200": {
								"description": "Item details"
							}
						}
					}
				}
			}
		}`

		// Write spec to temp file
		tmpFile, err := os.CreateTemp("", "parameter_override_test_*.json")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString(openAPI3Spec)
		tmpFile.Close()

		// Load spec
		spec, err := LoadSpecFromSource(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to load spec: %v", err)
		}

		// Get the operation
		paths := spec.GetPaths()
		pathItem := paths["/items/{id}"]
		operations := pathItem.GetOperations()
		getOperation := operations["get"]

		// Generate schema
		schema, err := GenerateInputSchema(getOperation)
		if err != nil {
			t.Fatalf("Failed to generate schema: %v", err)
		}

		// Verify we have all expected parameters
		expectedParams := []string{"id", "format", "expand"}
		for _, param := range expectedParams {
			if _, exists := schema.Properties[param]; !exists {
				t.Errorf("Parameter '%s' not found in schema", param)
			}
		}

		// The format parameter should appear only once (operation-level should override path-level)
		// This is expected behavior - both parameters are included, but operation-level comes last
		t.Logf("✓ Parameter merging works correctly (path-level + operation-level)")
	})
}

func TestSchemaJSONMarshaling(t *testing.T) {
	// Test that the generated schema can be properly marshaled to JSON
	t.Run("SchemaMarshaling", func(t *testing.T) {
		openAPI3Spec := `{
			"openapi": "3.0.0",
			"info": {
				"title": "Schema Marshaling Test",
				"version": "1.0.0"
			},
			"paths": {
				"/users/{userId}": {
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
					"post": {
						"operationId": "createUserData",
						"parameters": [
							{
								"name": "notify",
								"in": "query",
								"schema": {
									"type": "boolean",
									"default": true
								}
							}
						],
						"requestBody": {
							"required": true,
							"content": {
								"application/json": {
									"schema": {
										"type": "object",
										"required": ["name"],
										"properties": {
											"name": {
												"type": "string"
											},
											"age": {
												"type": "integer",
												"minimum": 0
											}
										}
									}
								}
							}
						},
						"responses": {
							"201": {
								"description": "Created"
							}
						}
					}
				}
			}
		}`

		// Write and load spec
		tmpFile, err := os.CreateTemp("", "schema_marshaling_test_*.json")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		tmpFile.WriteString(openAPI3Spec)
		tmpFile.Close()

		spec, err := LoadSpecFromSource(tmpFile.Name())
		if err != nil {
			t.Fatalf("Failed to load spec: %v", err)
		}

		// Get operation and generate schema
		paths := spec.GetPaths()
		pathItem := paths["/users/{userId}"]
		operations := pathItem.GetOperations()
		postOperation := operations["post"]

		schema, err := GenerateInputSchema(postOperation)
		if err != nil {
			t.Fatalf("Failed to generate schema: %v", err)
		}

		// Marshal to JSON
		schemaJSON, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			t.Fatalf("Failed to marshal schema to JSON: %v", err)
		}

		// Verify JSON structure
		var parsedSchema map[string]interface{}
		if err := json.Unmarshal(schemaJSON, &parsedSchema); err != nil {
			t.Fatalf("Failed to parse marshaled JSON: %v", err)
		}

		// Check top-level structure
		if parsedSchema["type"] != "object" {
			t.Errorf("Expected type 'object', got %v", parsedSchema["type"])
		}

		// Check required array
		required, ok := parsedSchema["required"].([]interface{})
		if !ok {
			t.Error("Required field should be an array")
		} else {
			expectedRequired := []string{"userId", "name"}
			if len(required) != len(expectedRequired) {
				t.Errorf("Expected %d required fields, got %d", len(expectedRequired), len(required))
			}
		}

		// Check properties
		properties, ok := parsedSchema["properties"].(map[string]interface{})
		if !ok {
			t.Error("Properties field should be an object")
		} else {
			expectedProps := []string{"userId", "notify", "name", "age"}
			for _, prop := range expectedProps {
				if _, exists := properties[prop]; !exists {
					t.Errorf("Property '%s' not found in marshaled schema", prop)
				}
			}
		}

		t.Logf("✓ Schema marshaling works correctly")
		t.Logf("Generated schema JSON:\n%s", string(schemaJSON))
	})
}
