package openapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
)

// APISpec represents either OpenAPI 2.0 or 3.0 specification
type APISpec interface {
	GetVersion() string
	GetBaseURL() string
	GetPaths() map[string]PathItem
	GetInfo() openapi3.Info
}

// PathItem represents a path item that can contain operations
type PathItem interface {
	GetOperations() map[string]Operation
}

// Operation represents an API operation
type Operation interface {
	GetOperationID() string
	GetSummary() string
	GetParameters() []Parameter
	GetRequestBody() RequestBody
}

// Parameter represents an API parameter
type Parameter interface {
	GetName() string
	GetIn() string
	GetDescription() string
	IsRequired() bool
	GetType() string
	GetFormat() string
	GetSchema() Schema
}

// RequestBody represents a request body
type RequestBody interface {
	GetJSONSchema() (Schema, error)
}

// Schema represents a schema definition
type Schema interface {
	GetType() string
	GetFormat() string
	GetDescription() string
	GetProperties() map[string]Schema
	GetItems() Schema
	GetRequired() []string
	GetEnum() []interface{}
	GetDefault() interface{}
}

// LoadSpecFromSource loads an OpenAPI spec from either a file path or URL
func LoadSpecFromSource(source string) (APISpec, error) {
	var data []byte
	var err error

	// Check if source is a URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		data, err = fetchFromURL(source)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch from URL: %w", err)
		}
	} else {
		data, err = os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
	}

	return LoadSpec(data)
}

func fetchFromURL(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func LoadSpec(data []byte) (APISpec, error) {

	// Try OpenAPI 3.0 first
	loader := openapi3.NewLoader()
	if spec, err := loader.LoadFromData(data); err == nil {
		if err := spec.Validate(loader.Context); err == nil {
			return &OpenAPI3Spec{spec: spec}, nil
		}
	}

	// Fallback to OpenAPI 2
	var spec2 openapi2.T
	if err := json.Unmarshal(data, &spec2); err == nil {
		if spec2.Swagger != "" {
			return &OpenAPI2Spec{spec: &spec2}, nil
		}
	}

	// Try OpenAPI 2.0 with YAML - convert to JSON first to avoid unmarshaling issues
	var yamlData interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return nil, fmt.Errorf("unsupported or invalid OpenAPI specification")
	}

	// Convert YAML data to JSON
	jsonData, err := json.Marshal(yamlData)
	if err != nil {
		return nil, fmt.Errorf("unsupported or invalid OpenAPI specification")
	}

	spec2 = openapi2.T{}
	if err := json.Unmarshal(jsonData, &spec2); err != nil {
		return nil, fmt.Errorf("unsupported or invalid OpenAPI specification")
	}

	if spec2.Swagger == "" {
		return nil, fmt.Errorf("unsupported or invalid OpenAPI specification")
	}

	return &OpenAPI2Spec{spec: &spec2}, nil
}

// generateInputSchema creates a JSON schema for the tool input based on OpenAPI operation
func GenerateInputSchema(operation interface{}) (*jsonschema.Schema, error) {
	// Handle both old and new interfaces
	switch op := operation.(type) {
	case *openapi3.Operation:
		return generateInputSchemaV3(op)
	case Operation:
		return generateInputSchemaFromInterface(op)
	default:
		return nil, fmt.Errorf("unsupported operation type")
	}
}

func generateInputSchemaFromInterface(operation Operation) (*jsonschema.Schema, error) {
	schema := &jsonschema.Schema{
		Type:       "object",
		Properties: make(map[string]*jsonschema.Schema),
		Required:   []string{},
	}

	// Extract parameters (skip body parameters as they are handled separately)
	for _, param := range operation.GetParameters() {
		// Skip body parameters - they will be handled in the request body section
		if param.GetIn() == "body" {
			continue
		}

		paramSchema := convertParameterToJSONSchemaFromInterface(param)
		if paramSchema != nil {
			schema.Properties[param.GetName()] = paramSchema
			if param.IsRequired() {
				schema.Required = append(schema.Required, param.GetName())
			}
		}
	}

	// Add request body properties if present
	if requestBody := operation.GetRequestBody(); requestBody != nil {
		bodySchema, err := requestBody.GetJSONSchema()
		if err != nil {
			return nil, fmt.Errorf("failed to convert request body to schema: %w", err)
		}

		// Merge body schema properties into main schema
		if bodySchema != nil {
			bodyJSONSchema := convertSchemaToJSONSchema(bodySchema)
			if bodyJSONSchema != nil && bodyJSONSchema.Properties != nil {
				for propName, propSchema := range bodyJSONSchema.Properties {
					schema.Properties[propName] = propSchema
				}

				// Add required properties from body schema
				if bodyJSONSchema.Required != nil {
					schema.Required = append(schema.Required, bodyJSONSchema.Required...)
				}
			}
		}
	}

	return schema, nil
}

func generateInputSchemaV3(operation *openapi3.Operation) (*jsonschema.Schema, error) {
	// Convert to interface and use the new implementation
	return generateInputSchemaFromInterface(&OpenAPI3Operation{Op: operation})
}

func GetRequestBodyJSONContent(requestBodyRef *openapi3.RequestBodyRef) (*openapi3.MediaType, error) {
	if requestBodyRef == nil || requestBodyRef.Value == nil {
		return nil, nil
	}

	requestBody := requestBodyRef.Value
	if requestBody.Content == nil {
		return nil, nil
	}

	contentType, ok := requestBody.Content["application/json"]
	if !ok {
		return nil, fmt.Errorf("no application/json content-type found for request body")
	}

	return contentType, nil
}

// Helper functions for converting to jsonschema
func convertParameterToJSONSchemaFromInterface(param Parameter) *jsonschema.Schema {
	if param == nil {
		return nil
	}

	schema := &jsonschema.Schema{
		Description: param.GetDescription(),
	}

	if schema.Description == "" {
		schema.Description = fmt.Sprintf("%s parameter: %s",
			strings.ToUpper(param.GetIn()[:1])+param.GetIn()[1:],
			param.GetName())
	}

	// Handle parameter schema if available
	if paramSchema := param.GetSchema(); paramSchema != nil {
		return convertSchemaToJSONSchema(paramSchema)
	}

	// Use type and format directly
	schema.Type = param.GetType()
	if schema.Type == "" {
		schema.Type = "string"
	}
	if param.GetFormat() != "" {
		schema.Format = param.GetFormat()
	}

	return schema
}

func convertSchemaToJSONSchema(schema Schema) *jsonschema.Schema {
	if schema == nil {
		return nil
	}

	jsonSchema := &jsonschema.Schema{
		Type:        schema.GetType(),
		Format:      schema.GetFormat(),
		Description: schema.GetDescription(),
		Properties:  make(map[string]*jsonschema.Schema),
	}

	// Handle default value
	if defaultVal := schema.GetDefault(); defaultVal != nil {
		if defaultBytes, err := json.Marshal(defaultVal); err == nil {
			jsonSchema.Default = json.RawMessage(defaultBytes)
		}
	}

	// Convert properties
	if properties := schema.GetProperties(); properties != nil {
		for propName, propSchema := range properties {
			jsonSchema.Properties[propName] = convertSchemaToJSONSchema(propSchema)
		}
	}

	// Handle items for arrays
	if items := schema.GetItems(); items != nil {
		jsonSchema.Items = convertSchemaToJSONSchema(items)
	}

	// Set required fields
	if required := schema.GetRequired(); len(required) > 0 {
		jsonSchema.Required = required
	}

	// Handle enum
	if enum := schema.GetEnum(); len(enum) > 0 {
		jsonSchema.Enum = enum
	}

	return jsonSchema
}
