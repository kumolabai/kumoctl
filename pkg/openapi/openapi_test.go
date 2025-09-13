package openapi

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestGetBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		spec     *openapi3.T
		expected string
	}{
		{
			name: "spec with server URL",
			spec: &openapi3.T{
				Servers: []*openapi3.Server{
					{URL: "https://api.example.com"},
				},
			},
			expected: "https://api.example.com",
		},
		{
			name: "spec with empty server URL",
			spec: &openapi3.T{
				Servers: []*openapi3.Server{
					{URL: ""},
				},
			},
			expected: "http://localhost:8080",
		},
		{
			name:     "spec with no servers",
			spec:     &openapi3.T{},
			expected: "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBaseURL(tt.spec)
			if result != tt.expected {
				t.Errorf("GetBaseURL() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetPathOperations(t *testing.T) {
	pathItem := &openapi3.PathItem{
		Get:     &openapi3.Operation{OperationID: "getOp"},
		Post:    &openapi3.Operation{OperationID: "postOp"},
		Put:     nil,
		Delete:  &openapi3.Operation{OperationID: "deleteOp"},
		Patch:   nil,
		Head:    nil,
		Options: nil,
		Trace:   nil,
	}

	operations := GetPathOperations(pathItem)

	// Test that all methods are represented
	expectedMethods := []string{"get", "post", "put", "delete", "patch", "head", "options", "trace"}
	if len(operations) != len(expectedMethods) {
		t.Errorf("Expected %d operations, got %d", len(expectedMethods), len(operations))
	}

	// Test specific operations
	if operations["get"] == nil || operations["get"].OperationID != "getOp" {
		t.Error("GET operation not correctly mapped")
	}
	if operations["post"] == nil || operations["post"].OperationID != "postOp" {
		t.Error("POST operation not correctly mapped")
	}
	if operations["put"] != nil {
		t.Error("PUT operation should be nil")
	}
	if operations["delete"] == nil || operations["delete"].OperationID != "deleteOp" {
		t.Error("DELETE operation not correctly mapped")
	}
}

func TestConvertParameterToJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		param    *openapi3.Parameter
		expected string // expected type
	}{
		{
			name:     "nil parameter",
			param:    nil,
			expected: "",
		},
		{
			name: "parameter with string schema",
			param: &openapi3.Parameter{
				Name:        "testParam",
				In:          "query",
				Description: "Test parameter",
				Schema: &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type: &openapi3.Types{"string"},
					},
				},
			},
			expected: "string",
		},
		{
			name: "parameter without schema",
			param: &openapi3.Parameter{
				Name: "testParam",
				In:   "header",
			},
			expected: "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertParameterToJSONSchema(tt.param)

			if tt.expected == "" {
				if result != nil {
					t.Errorf("Expected nil result for nil parameter")
				}
				return
			}

			if result == nil {
				t.Errorf("Expected non-nil result")
				return
			}

			if result.Type != tt.expected {
				t.Errorf("Expected type %v, got %v", tt.expected, result.Type)
			}
		})
	}
}

func TestConvertOpenAPISchemaToJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		schema   *openapi3.Schema
		expected string // expected type
	}{
		{
			name:     "nil schema",
			schema:   nil,
			expected: "",
		},
		{
			name: "string schema",
			schema: &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "A string field",
			},
			expected: "string",
		},
		{
			name: "integer schema",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"integer"},
			},
			expected: "integer",
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
				Required: []string{"name"},
			},
			expected: "object",
		},
		{
			name: "array schema",
			schema: &openapi3.Schema{
				Type: &openapi3.Types{"array"},
				Items: &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type: &openapi3.Types{"string"},
					},
				},
			},
			expected: "array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertOpenAPISchemaToJSONSchema(tt.schema)

			if tt.expected == "" {
				if result != nil {
					t.Errorf("Expected nil result for nil schema")
				}
				return
			}

			if result == nil {
				t.Errorf("Expected non-nil result")
				return
			}

			if result.Type != tt.expected {
				t.Errorf("Expected type %v, got %v", tt.expected, result.Type)
			}

			// Additional checks for specific types
			if tt.expected == "object" && result.Properties == nil {
				t.Error("Object schema should have properties map initialized")
			}

			if tt.expected == "object" && tt.schema.Properties != nil {
				expectedProps := len(tt.schema.Properties)
				actualProps := len(result.Properties)
				if actualProps != expectedProps {
					t.Errorf("Expected %d properties, got %d", expectedProps, actualProps)
				}
			}

			if tt.expected == "array" && tt.schema.Items != nil && result.Items == nil {
				t.Error("Array schema should have items defined")
			}
		})
	}
}

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
