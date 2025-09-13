package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/kumolabai/kumoctl/pkg/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// APIToolInput represents the input for dynamically generated API tools
type APIToolInput map[string]interface{}

// APIToolOutput represents the output from API calls
type APIToolOutput struct {
	StatusCode int                 `json:"status_code"`
	Body       interface{}         `json:"body,omitempty"`
	Headers    map[string]string   `json:"headers,omitempty"`
	Error      string              `json:"error,omitempty"`
}

func GenerateToolsFromSpec(server *mcp.Server, spec *openapi3.T) error {
	baseURL := openapi.GetBaseURL(spec)

	for path, pathItem := range spec.Paths.Map() {
		for method, operation := range openapi.GetPathOperations(pathItem) {
			if operation == nil {
				continue
			}

			toolName := generateToolName(method, path, operation.OperationID)
			description := operation.Summary
			if description == "" {
				description = fmt.Sprintf("%s %s", strings.ToUpper(method), path)
			}

			// Generate input schema for this tool
			inputSchema, err := openapi.GenerateInputSchema(operation)
			if err != nil {
				return fmt.Errorf("failed to generate input schema for %s %s: %w", method, path, err)
			}

			tool := &mcp.Tool{
				Name:        toolName,
				Description: description,
				InputSchema: inputSchema,
			}

			// Create the handler function for this specific operation
			handler := createAPIHandler(baseURL, method, path, operation)

			mcp.AddTool(server, tool, handler)
		}
	}

	return nil
}

func generateToolName(method, path string, operationID string) string {
	if operationID != "" {
		return operationID
	}

	// Clean path for tool name
	cleanPath := strings.ReplaceAll(path, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "{", "")
	cleanPath = strings.ReplaceAll(cleanPath, "}", "")
	cleanPath = strings.Trim(cleanPath, "_")

	if cleanPath == "" {
		return method
	}

	return fmt.Sprintf("%s_%s", method, cleanPath)
}

// buildURL constructs the full URL with path parameters replaced
func buildURL(baseURL, path string, input APIToolInput) (*url.URL, error) {
	// Replace path parameters
	pathParamRegex := regexp.MustCompile(`\{([^}]+)\}`)
	var missingParams []string
	
	finalPath := pathParamRegex.ReplaceAllStringFunc(path, func(match string) string {
		paramName := match[1 : len(match)-1] // Remove { and }
		if value, exists := input[paramName]; exists {
			return fmt.Sprintf("%v", value)
		}
		missingParams = append(missingParams, paramName)
		return match // Keep original for error reporting
	})

	// Return error if any path parameters are missing
	if len(missingParams) > 0 {
		return nil, fmt.Errorf("missing required path parameters: %v", missingParams)
	}

	fullURLStr := strings.TrimSuffix(baseURL, "/") + finalPath
	return url.Parse(fullURLStr)
}

// addQueryParams adds query parameters to the URL
func addQueryParams(fullURL *url.URL, operation *openapi3.Operation, input APIToolInput) error {
	if operation.Parameters == nil {
		return nil
	}

	query := fullURL.Query()
	for _, paramRef := range operation.Parameters {
		param := paramRef.Value
		if param.In == "query" {
			if value, exists := input[param.Name]; exists {
				query.Set(param.Name, fmt.Sprintf("%v", value))
			}
		}
	}
	fullURL.RawQuery = query.Encode()
	return nil
}

// hasRequestBody checks if the operation expects a request body
func hasRequestBody(operation *openapi3.Operation) bool {
	return operation.RequestBody != nil
}

// buildRequestBody constructs the JSON request body
func buildRequestBody(operation *openapi3.Operation, input APIToolInput) ([]byte, error) {
	contentType, err := openapi.GetRequestBodyJSONContent(operation.RequestBody)
	if err != nil {
		return nil, err
	}

	schema := contentType.Schema.Value

	// Build request body from input based on schema
	body := make(map[string]interface{})
	extractFieldsFromSchema(body, schema, input)

	return json.Marshal(body)
}

// setHeaders sets HTTP headers based on operation parameters and defaults
func setHeaders(req *http.Request, operation *openapi3.Operation, input APIToolInput) error {
	// Set default content type for requests with body
	if hasRequestBody(operation) {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add header parameters
	if operation.Parameters != nil {
		for _, paramRef := range operation.Parameters {
			param := paramRef.Value
			if param.In == "header" {
				if value, exists := input[param.Name]; exists {
					req.Header.Set(param.Name, fmt.Sprintf("%v", value))
				}
			}
		}
	}

	return nil
}

// parseResponse parses the HTTP response into APIToolOutput
func parseResponse(resp *http.Response) (APIToolOutput, error) {
	output := APIToolOutput{
		StatusCode: resp.StatusCode,
		Headers:    make(map[string]string),
	}

	// Copy response headers
	for key, values := range resp.Header {
		if len(values) > 0 {
			output.Headers[key] = values[0]
		}
	}

	// Parse response body if present
	if resp.Body != nil {
		var body interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
			// Accept any valid JSON: objects, arrays, or primitives
			output.Body = body
		}
	}

	return output, nil
}

// extractFieldsFromSchema recursively extracts fields from schema and input
func extractFieldsFromSchema(target map[string]interface{}, schema *openapi3.Schema, input APIToolInput) {
	if schema == nil {
		return
	}

	// Handle object properties
	if schema.Type.Is("object") && schema.Properties != nil {
		for propName, propSchemaRef := range schema.Properties {
			if propSchemaRef.Value == nil {
				continue
			}
			propSchema := propSchemaRef.Value

			if value, exists := input[propName]; exists {
				target[propName] = value
			} else if propSchema.Default != nil {
				target[propName] = propSchema.Default
			}
		}
	}
}

// createAPIHandler creates a handler function for a specific API operation
func createAPIHandler(baseURL, method, path string, operation *openapi3.Operation) func(context.Context, *mcp.CallToolRequest, APIToolInput) (*mcp.CallToolResult, APIToolOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input APIToolInput) (*mcp.CallToolResult, APIToolOutput, error) {
		// Build the full URL with path parameters
		fullURL, err := buildURL(baseURL, path, input)
		if err != nil {
			return nil, APIToolOutput{Error: fmt.Sprintf("Failed to build URL: %v", err)}, nil
		}

		// Add query parameters
		if err := addQueryParams(fullURL, operation, input); err != nil {
			return nil, APIToolOutput{Error: fmt.Sprintf("Failed to add query params: %v", err)}, nil
		}

		// Create HTTP request
		var bodyReader *bytes.Reader = nil
		if hasRequestBody(operation) {
			body, err := buildRequestBody(operation, input)
			if err != nil {
				return nil, APIToolOutput{Error: fmt.Sprintf("Failed to build request body: %v", err)}, nil
			}
			bodyReader = bytes.NewReader(body)
		}

		httpReq, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), fullURL.String(), bodyReader)
		if err != nil {
			return nil, APIToolOutput{Error: fmt.Sprintf("Failed to create request: %v", err)}, nil
		}

		// Set headers
		if err := setHeaders(httpReq, operation, input); err != nil {
			return nil, APIToolOutput{Error: fmt.Sprintf("Failed to set headers: %v", err)}, nil
		}

		// Make the HTTP request
		client := &http.Client{}
		resp, err := client.Do(httpReq)
		if err != nil {
			return nil, APIToolOutput{Error: fmt.Sprintf("HTTP request failed: %v", err)}, nil
		}
		defer resp.Body.Close()

		// Parse response
		output, err := parseResponse(resp)
		if err != nil {
			return nil, APIToolOutput{Error: fmt.Sprintf("Failed to parse response: %v", err)}, nil
		}

		return nil, output, nil
	}
}
