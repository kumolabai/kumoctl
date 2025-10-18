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

	"github.com/kumolabai/kumoctl/pkg/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// APIToolInput represents the input for dynamically generated API tools
type APIToolInput map[string]interface{}

// APIToolOutput represents the output from API calls
// TODO: Look into changing this to the actual response schema from the OpenAPI Spec
type APIToolOutput struct {
	StatusCode int               `json:"status_code"`
	Body       interface{}       `json:"body,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Error      string            `json:"error,omitempty"`
}

func GenerateToolsFromSpec(server *mcp.Server, spec openapi.APISpec, additionalHeaders http.Header) error {
	tools, err := GetToolsFromSpec(spec)
	if err != nil {
		return err
	}

	for _, tool := range tools {
		// Create the handler function for this specific operation
		handler := createAPIHandlerForTool(tool, additionalHeaders)
		mcp.AddTool(server, tool.Tool, handler)
	}

	return nil
}

func GetToolsFromSpec(spec openapi.APISpec) ([]*EnrichedTool, error) {
	tools := []*EnrichedTool{}
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

			tools = append(tools, &EnrichedTool{
				Tool: &mcp.Tool{
					Name:        toolName,
					Description: description,
					InputSchema: inputSchema,
				},
				BaseUrl:   baseURL,
				Method:    method,
				Path:      path,
				Operation: operation,
			})

		}
	}

	return tools, nil
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
func addQueryParams(fullURL *url.URL, operation openapi.Operation, input APIToolInput) error {
	query := fullURL.Query()
	for _, param := range operation.GetParameters() {
		if param.GetIn() == "query" {
			if value, exists := input[param.GetName()]; exists {
				query.Set(param.GetName(), fmt.Sprintf("%v", value))
			}
		}
	}
	fullURL.RawQuery = query.Encode()
	return nil
}

// hasRequestBody checks if the operation expects a request body
func hasRequestBody(operation openapi.Operation) bool {
	return operation.GetRequestBody() != nil
}

// buildRequestBody constructs the JSON request body
func buildRequestBody(operation openapi.Operation, input APIToolInput) ([]byte, error) {
	requestBody := operation.GetRequestBody()
	if requestBody == nil {
		return nil, nil
	}

	schema, err := requestBody.GetJSONSchema()
	if err != nil {
		return nil, err
	}

	if schema == nil {
		return nil, nil
	}

	// Build request body from input based on schema
	body := make(map[string]interface{})
	extractFieldsFromSchema(body, schema, input)

	return json.Marshal(body)
}

// setHeaders sets HTTP headers based on operation parameters and defaults
func setHeaders(req *http.Request, operation openapi.Operation, input APIToolInput, additionalHeaders http.Header) error {
	// Set default content type for requests with body
	if hasRequestBody(operation) {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add header parameters
	for _, param := range operation.GetParameters() {
		if param.GetIn() == "header" {
			if value, exists := input[param.GetName()]; exists {
				req.Header.Set(param.GetName(), fmt.Sprintf("%v", value))
			}
		}
	}

	for headerKey := range additionalHeaders {
		req.Header.Add(headerKey, additionalHeaders.Get(headerKey))
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
func extractFieldsFromSchema(target map[string]interface{}, schema openapi.Schema, input APIToolInput) {
	if schema == nil {
		return
	}

	// Handle object properties
	if schema.GetType() == "object" {
		for propName, propSchema := range schema.GetProperties() {
			if value, exists := input[propName]; exists {
				target[propName] = value
			} else if defaultVal := propSchema.GetDefault(); defaultVal != nil {
				target[propName] = defaultVal
			}
		}
	}
}

// createAPIHandler creates a handler function for a specific API operation
func createAPIHandlerForTool(tool *EnrichedTool, additionalHeaders http.Header) func(context.Context, *mcp.CallToolRequest, APIToolInput) (*mcp.CallToolResult, APIToolOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input APIToolInput) (*mcp.CallToolResult, APIToolOutput, error) {
		// Build the full URL with path parameters
		fullURL, err := buildURL(tool.BaseUrl, tool.Path, input)
		if err != nil {
			return nil, APIToolOutput{Error: fmt.Sprintf("Failed to build URL: %v", err)}, nil
		}

		// Add query parameters
		if err := addQueryParams(fullURL, tool.Operation, input); err != nil {
			return nil, APIToolOutput{Error: fmt.Sprintf("Failed to add query params: %v", err)}, nil
		}

		// Create HTTP request
		bodyReader := &bytes.Reader{}
		if hasRequestBody(tool.Operation) {
			body, err := buildRequestBody(tool.Operation, input)
			if err != nil {
				return nil, APIToolOutput{Error: fmt.Sprintf("Failed to build request body: %v", err)}, nil
			}
			bodyReader = bytes.NewReader(body)
		}

		httpReq, err := http.NewRequestWithContext(ctx, strings.ToUpper(tool.Method), fullURL.String(), bodyReader)
		if err != nil {
			return nil, APIToolOutput{Error: fmt.Sprintf("Failed to create request: %v", err)}, nil
		}

		// Set headers
		if err := setHeaders(httpReq, tool.Operation, input, additionalHeaders); err != nil {
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
