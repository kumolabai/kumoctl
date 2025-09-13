package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/jsonschema-go/jsonschema"
)

func LoadSpec(filePath string) (*openapi3.T, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	loader := openapi3.NewLoader()

	var spec *openapi3.T

	spec, err = loader.LoadFromData(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Validate the spec
	err = spec.Validate(loader.Context)
	if err != nil {
		return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
	}

	return spec, nil
}

func GetBaseURL(spec *openapi3.T) string {
	if len(spec.Servers) > 0 && spec.Servers[0].URL != "" {
		return spec.Servers[0].URL
	}
	return "http://localhost:8080"
}

func GetPathOperations(pathItem *openapi3.PathItem) map[string]*openapi3.Operation {
	return map[string]*openapi3.Operation{
		"get":     pathItem.Get,
		"post":    pathItem.Post,
		"put":     pathItem.Put,
		"delete":  pathItem.Delete,
		"patch":   pathItem.Patch,
		"head":    pathItem.Head,
		"options": pathItem.Options,
		"trace":   pathItem.Trace,
	}
}

// generateInputSchema creates a JSON schema for the tool input based on OpenAPI operation
func GenerateInputSchema(operation *openapi3.Operation) (*jsonschema.Schema, error) {
	schema := &jsonschema.Schema{
		Type:       "object",
		Properties: make(map[string]*jsonschema.Schema),
		Required:   []string{},
	}

	// Extract path parameters
	for _, param := range operation.Parameters {
		if param.Value == nil {
			continue
		}
		paramValue := param.Value

		schema.Properties[paramValue.Name] = convertParameterToJSONSchema(paramValue)
		if paramValue.Required {
			schema.Required = append(schema.Required, paramValue.Name)
		}
		if paramValue.Schema.Value.Format != "" {
			schema.Format = paramValue.Schema.Value.Format
		}
	}

	// Add request body properties if present
	if operation.RequestBody != nil {
		bodySchema, err := convertRequestBodyToJSONSchema(operation.RequestBody)
		if err != nil {
			return nil, fmt.Errorf("failed to convert request body to schema: %w", err)
		}

		// Merge body schema properties into main schema
		if bodySchema != nil && bodySchema.Properties != nil {
			for propName, propSchema := range bodySchema.Properties {
				schema.Properties[propName] = propSchema
			}

			// Add required properties from body schema
			if bodySchema.Required != nil {
				schema.Required = append(schema.Required, bodySchema.Required...)
			}
		}
	}

	return schema, nil
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

// convertParameterToJSONSchema converts an OpenAPI parameter to JSON schema
func convertParameterToJSONSchema(param *openapi3.Parameter) *jsonschema.Schema {
	if param == nil {
		return nil
	}

	schema := &jsonschema.Schema{}

	// Set description
	desc := param.Description
	if desc == "" {
		desc = fmt.Sprintf("%s parameter: %s", strings.ToUpper(param.In[:1])+param.In[1:], param.Name)
	}
	schema.Description = desc

	// Handle parameter schema
	if param.Schema != nil && param.Schema.Value != nil {
		return convertOpenAPISchemaToJSONSchema(param.Schema.Value)
	}

	// Fallback to string type
	schema.Type = "string"
	return schema
}

// convertRequestBodyToJSONSchema converts OpenAPI request body to JSON schema
func convertRequestBodyToJSONSchema(requestBodyRef *openapi3.RequestBodyRef) (*jsonschema.Schema, error) {
	contentType, err := GetRequestBodyJSONContent(requestBodyRef)
	if err != nil {
		return nil, err
	}

	schema := contentType.Schema.Value
	return convertOpenAPISchemaToJSONSchema(schema), nil
}

// convertOpenAPISchemaToJSONSchema converts OpenAPI schema to JSON schema
func convertOpenAPISchemaToJSONSchema(schema *openapi3.Schema) *jsonschema.Schema {
	if schema == nil {
		return nil
	}

	jsonSchema := &jsonschema.Schema{
		Description: schema.Description,
	}

	// Handle default value with proper type casting
	if schema.Default != nil {
		if defaultBytes, err := json.Marshal(schema.Default); err == nil {
			jsonSchema.Default = json.RawMessage(defaultBytes)
		}
	}

	jsonSchema.Type = schema.Type.Slice()[0]
	jsonSchema.Properties = make(map[string]*jsonschema.Schema)
	if schema.Format != "" {
		jsonSchema.Format = schema.Format
	}

	// Convert properties
	if schema.Properties != nil {
		for propName, propSchemaRef := range schema.Properties {
			if propSchemaRef.Value != nil {
				jsonSchema.Properties[propName] = convertOpenAPISchemaToJSONSchema(propSchemaRef.Value)
			}
		}
	}

	if schema.Items != nil && schema.Items.Value != nil {
		jsonSchema.Items = convertOpenAPISchemaToJSONSchema(schema.Items.Value)
	}

	// Set required fields
	if len(schema.Required) > 0 {
		jsonSchema.Required = schema.Required
	}

	// Handle enum
	if len(schema.Enum) > 0 {
		jsonSchema.Enum = schema.Enum
	}

	return jsonSchema
}
