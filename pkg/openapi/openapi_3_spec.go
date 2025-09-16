package openapi

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPI3Spec wraps openapi3.T
type OpenAPI3Spec struct {
	spec *openapi3.T
}

// PathItem implementations
type OpenAPI3PathItem struct {
	item *openapi3.PathItem
}

// Operation implementations
type OpenAPI3Operation struct {
	Op *openapi3.Operation
}

// OpenAPI3OperationWithPath includes both operation and path-level parameters
type OpenAPI3OperationWithPath struct {
	Op       *openapi3.Operation
	pathItem *openapi3.PathItem
}

// Parameter implementations
type OpenAPI3Parameter struct {
	param *openapi3.Parameter
}

type OpenAPI3RequestBody struct {
	body *openapi3.RequestBodyRef
}

type OpenAPI3Schema struct {
	Schema *openapi3.Schema
}

func (p *OpenAPI3Parameter) GetName() string {
	return p.param.Name
}

func (p *OpenAPI3Parameter) GetIn() string {
	return p.param.In
}

func (p *OpenAPI3Parameter) GetDescription() string {
	return p.param.Description
}

func (p *OpenAPI3Parameter) IsRequired() bool {
	return p.param.Required
}

func (p *OpenAPI3Parameter) GetType() string {
	if p.param.Schema != nil && p.param.Schema.Value != nil && len(p.param.Schema.Value.Type.Slice()) > 0 {
		return p.param.Schema.Value.Type.Slice()[0]
	}
	return "string"
}

func (p *OpenAPI3Parameter) GetFormat() string {
	if p.param.Schema != nil && p.param.Schema.Value != nil {
		return p.param.Schema.Value.Format
	}
	return ""
}

func (p *OpenAPI3Parameter) GetSchema() Schema {
	if p.param.Schema != nil && p.param.Schema.Value != nil {
		return &OpenAPI3Schema{Schema: p.param.Schema.Value}
	}
	return nil
}

func (s *OpenAPI3Spec) GetVersion() string {
	return s.spec.OpenAPI
}

func (s *OpenAPI3Spec) GetInfo() openapi3.Info {
	return *s.spec.Info
}

func (s *OpenAPI3Spec) GetBaseURL() string {
	if len(s.spec.Servers) > 0 && s.spec.Servers[0].URL != "" {
		return s.spec.Servers[0].URL
	}
	return "http://localhost:8080"
}

func (s *OpenAPI3Spec) GetPaths() map[string]PathItem {
	paths := make(map[string]PathItem)
	if s.spec.Paths != nil {
		for path, pathItem := range s.spec.Paths.Map() {
			paths[path] = &OpenAPI3PathItem{item: pathItem}
		}
	}
	return paths
}

func (o *OpenAPI3Operation) GetOperationID() string {
	return o.Op.OperationID
}

func (o *OpenAPI3Operation) GetSummary() string {
	return o.Op.Summary
}

func (o *OpenAPI3Operation) GetParameters() []Parameter {
	var params []Parameter
	for _, param := range o.Op.Parameters {
		if param.Value != nil {
			params = append(params, &OpenAPI3Parameter{param: param.Value})
		}
	}
	return params
}

func (o *OpenAPI3Operation) GetRequestBody() RequestBody {
	if o.Op.RequestBody != nil {
		return &OpenAPI3RequestBody{body: o.Op.RequestBody}
	}
	return nil
}

// OpenAPI3OperationWithPath methods
func (o *OpenAPI3OperationWithPath) GetOperationID() string {
	return o.Op.OperationID
}

func (o *OpenAPI3OperationWithPath) GetSummary() string {
	return o.Op.Summary
}

func (o *OpenAPI3OperationWithPath) GetParameters() []Parameter {
	var params []Parameter

	// Add path-level parameters first
	if o.pathItem.Parameters != nil {
		for _, param := range o.pathItem.Parameters {
			if param.Value != nil {
				params = append(params, &OpenAPI3Parameter{param: param.Value})
			}
		}
	}

	// Add operation-level parameters
	for _, param := range o.Op.Parameters {
		if param.Value != nil {
			params = append(params, &OpenAPI3Parameter{param: param.Value})
		}
	}

	return params
}

func (o *OpenAPI3OperationWithPath) GetRequestBody() RequestBody {
	if o.Op.RequestBody != nil {
		return &OpenAPI3RequestBody{body: o.Op.RequestBody}
	}
	return nil
}

func (p *OpenAPI3PathItem) GetOperations() map[string]Operation {
	operations := make(map[string]Operation)

	// Map of method name to operation pointer
	methodOps := map[string]*openapi3.Operation{
		"get":     p.item.Get,
		"post":    p.item.Post,
		"put":     p.item.Put,
		"delete":  p.item.Delete,
		"patch":   p.item.Patch,
		"head":    p.item.Head,
		"options": p.item.Options,
		"trace":   p.item.Trace,
	}

	for method, op := range methodOps {
		if op != nil {
			operations[method] = &OpenAPI3OperationWithPath{Op: op, pathItem: p.item}
		}
	}

	return operations
}

func (r *OpenAPI3RequestBody) GetJSONSchema() (Schema, error) {
	if r.body == nil || r.body.Value == nil {
		return nil, nil
	}

	requestBody := r.body.Value
	if requestBody.Content == nil {
		return nil, nil
	}

	contentType, ok := requestBody.Content["application/json"]
	if !ok {
		return nil, fmt.Errorf("no application/json content-type found for request body")
	}

	if contentType.Schema != nil && contentType.Schema.Value != nil {
		return &OpenAPI3Schema{Schema: contentType.Schema.Value}, nil
	}

	return nil, nil
}

func (s *OpenAPI3Schema) GetType() string {
	if s.Schema.Type != nil && len(s.Schema.Type.Slice()) > 0 {
		return s.Schema.Type.Slice()[0]
	}
	return ""
}

func (s *OpenAPI3Schema) GetFormat() string {
	return s.Schema.Format
}

func (s *OpenAPI3Schema) GetDescription() string {
	return s.Schema.Description
}

func (s *OpenAPI3Schema) GetProperties() map[string]Schema {
	properties := make(map[string]Schema)
	if s.Schema.Properties != nil {
		for name, propRef := range s.Schema.Properties {
			if propRef.Value != nil {
				properties[name] = &OpenAPI3Schema{Schema: propRef.Value}
			}
		}
	}
	return properties
}

func (s *OpenAPI3Schema) GetItems() Schema {
	if s.Schema.Items != nil && s.Schema.Items.Value != nil {
		return &OpenAPI3Schema{Schema: s.Schema.Items.Value}
	}
	return nil
}

func (s *OpenAPI3Schema) GetRequired() []string {
	return s.Schema.Required
}

func (s *OpenAPI3Schema) GetEnum() []interface{} {
	return s.Schema.Enum
}

func (s *OpenAPI3Schema) GetDefault() interface{} {
	return s.Schema.Default
}
