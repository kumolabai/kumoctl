package mcp

import (
	"github.com/kumolabai/kumoctl/pkg/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EnrichedTool struct {
	*mcp.Tool
	BaseUrl   string
	Method    string
	Path      string
	Operation openapi.Operation
}
