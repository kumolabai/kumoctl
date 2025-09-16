package openapi

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPI2Spec wraps openapi2.T
type OpenAPI2Spec struct {
	spec *openapi2.T
}

type OpenAPI2PathItem struct {
	item *openapi2.PathItem
	spec *openapi2.T
}

type OpenAPI2Parameter struct {
	param *openapi2.Parameter
}

type OpenAPI2RequestBody struct {
	param *openapi2.Parameter
	spec  *openapi2.T
}

type OpenAPI2Schema struct {
	schema *openapi2.Schema
}

func (s *OpenAPI2Spec) GetVersion() string {
	return s.spec.Swagger
}

func (s *OpenAPI2Spec) GetInfo() openapi3.Info {
	return s.spec.Info
}

func (s *OpenAPI2Spec) GetBaseURL() string {
	scheme := "http"
	if len(s.spec.Schemes) > 0 {
		scheme = s.spec.Schemes[0]
	}
	host := s.spec.Host
	if host == "" {
		host = "localhost:8080"
	}
	basePath := s.spec.BasePath
	return fmt.Sprintf("%s://%s%s", scheme, host, basePath)
}

func (s *OpenAPI2Spec) GetPaths() map[string]PathItem {
	paths := make(map[string]PathItem)
	if s.spec.Paths != nil {
		for path, pathItem := range s.spec.Paths {
			paths[path] = &OpenAPI2PathItem{item: pathItem, spec: s.spec}
		}
	}
	return paths
}

func (p *OpenAPI2PathItem) GetOperations() map[string]Operation {
	operations := make(map[string]Operation)

	// Map of method name to operation pointer
	methodOps := map[string]*openapi2.Operation{
		"get":     p.item.Get,
		"post":    p.item.Post,
		"put":     p.item.Put,
		"delete":  p.item.Delete,
		"patch":   p.item.Patch,
		"head":    p.item.Head,
		"options": p.item.Options,
	}

	for method, op := range methodOps {
		if op != nil {
			operations[method] = &OpenAPI2OperationWithPath{op: op, pathItem: p.item, spec: p.spec}
		}
	}

	return operations
}

type OpenAPI2Operation struct {
	op *openapi2.Operation
}

// OpenAPI2OperationWithPath includes both operation and path-level parameters
type OpenAPI2OperationWithPath struct {
	op       *openapi2.Operation
	pathItem *openapi2.PathItem
	spec     *openapi2.T
}

func (o *OpenAPI2Operation) GetOperationID() string {
	return o.op.OperationID
}

func (o *OpenAPI2Operation) GetSummary() string {
	return o.op.Summary
}

func (o *OpenAPI2Operation) GetParameters() []Parameter {
	var params []Parameter
	for _, param := range o.op.Parameters {
		params = append(params, &OpenAPI2Parameter{param: param})
	}
	return params
}

func (o *OpenAPI2Operation) GetRequestBody() RequestBody {
	// In OpenAPI 2.0, request body is defined as a parameter with in: "body"
	for _, param := range o.op.Parameters {
		if param.In == "body" {
			return &OpenAPI2RequestBody{param: param}
		}
	}
	return nil
}

// OpenAPI2OperationWithPath methods
func (o *OpenAPI2OperationWithPath) GetOperationID() string {
	return o.op.OperationID
}

func (o *OpenAPI2OperationWithPath) GetSummary() string {
	return o.op.Summary
}

func (o *OpenAPI2OperationWithPath) GetParameters() []Parameter {
	var params []Parameter

	// Add path-level parameters first
	if o.pathItem.Parameters != nil {
		for _, param := range o.pathItem.Parameters {
			params = append(params, &OpenAPI2Parameter{param: param})
		}
	}

	// Add operation-level parameters
	for _, param := range o.op.Parameters {
		params = append(params, &OpenAPI2Parameter{param: param})
	}

	return params
}

func (o *OpenAPI2OperationWithPath) GetRequestBody() RequestBody {
	// In OpenAPI 2.0, request body is defined as a parameter with in: "body"
	for _, param := range o.op.Parameters {
		if param.In == "body" {
			return &OpenAPI2RequestBody{param: param, spec: o.spec}
		}
	}

	// Also check path-level parameters for body parameters
	if o.pathItem.Parameters != nil {
		for _, param := range o.pathItem.Parameters {
			if param.In == "body" {
				return &OpenAPI2RequestBody{param: param, spec: o.spec}
			}
		}
	}

	return nil
}

func (p *OpenAPI2Parameter) GetName() string {
	return p.param.Name
}

func (p *OpenAPI2Parameter) GetIn() string {
	return p.param.In
}

func (p *OpenAPI2Parameter) GetDescription() string {
	return p.param.Description
}

func (p *OpenAPI2Parameter) IsRequired() bool {
	return p.param.Required
}

func (p *OpenAPI2Parameter) GetType() string {
	if p.param.Type != nil && len(p.param.Type.Slice()) > 0 {
		return p.param.Type.Slice()[0]
	}
	return "string"
}

func (p *OpenAPI2Parameter) GetFormat() string {
	return p.param.Format
}

func (p *OpenAPI2Parameter) GetSchema() Schema {
	if p.param.Schema != nil && p.param.Schema.Value != nil {
		return &OpenAPI2Schema{schema: p.param.Schema.Value}
	}
	return nil
}

func (r *OpenAPI2RequestBody) GetJSONSchema() (Schema, error) {
	if r.param == nil || r.param.Schema == nil {
		return nil, nil
	}

	// Handle schema references in OpenAPI 2.0
	if r.param.Schema.Ref != "" && r.param.Schema.Value == nil {
		// Resolve the reference manually
		refPath := strings.TrimPrefix(r.param.Schema.Ref, "#/definitions/")
		if r.spec != nil && r.spec.Definitions != nil {
			if refSchemaRef, exists := r.spec.Definitions[refPath]; exists && refSchemaRef.Value != nil {
				return &OpenAPI2Schema{schema: refSchemaRef.Value}, nil
			}
		}
		return nil, fmt.Errorf("could not resolve schema reference: %s", r.param.Schema.Ref)
	}

	if r.param.Schema.Value == nil {
		return nil, nil
	}

	return &OpenAPI2Schema{schema: r.param.Schema.Value}, nil
}

func (s *OpenAPI2Schema) GetType() string {
	if s.schema.Type != nil && len(s.schema.Type.Slice()) > 0 {
		return s.schema.Type.Slice()[0]
	}
	return ""
}

func (s *OpenAPI2Schema) GetFormat() string {
	return s.schema.Format
}

func (s *OpenAPI2Schema) GetDescription() string {
	return s.schema.Description
}

func (s *OpenAPI2Schema) GetProperties() map[string]Schema {
	properties := make(map[string]Schema)
	if s.schema.Properties != nil {
		for name, prop := range s.schema.Properties {
			if prop.Value != nil {
				properties[name] = &OpenAPI2Schema{schema: prop.Value}
			}
		}
	}
	return properties
}

func (s *OpenAPI2Schema) GetItems() Schema {
	if s.schema.Items != nil && s.schema.Items.Value != nil {
		return &OpenAPI2Schema{schema: s.schema.Items.Value}
	}
	return nil
}

func (s *OpenAPI2Schema) GetRequired() []string {
	return s.schema.Required
}

func (s *OpenAPI2Schema) GetEnum() []interface{} {
	return s.schema.Enum
}

func (s *OpenAPI2Schema) GetDefault() interface{} {
	return s.schema.Default
}
