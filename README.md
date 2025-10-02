# kumoctl

A CLI tool to dynamically serve MCP (Model Context Protocol) servers from OpenAPI specifications. Each operation/path in your OpenAPI spec becomes an MCP tool that can be called by LLM clients like Claude.

## Installation

### Linux, macOS

```bash
curl -fsSL https://get.kumolab.ai | sh
```

### Windows

```powershell
irm https://get.kumolab.ai/install.ps1 | iex
```

## Usage

### Quick Start with Auto-Configuration

The fastest way to get started is using the `configure` command to automatically set up kumoctl with your LLM client:

```bash
# Auto-configure for Claude Desktop
kumoctl configure examples/openapi2-example.json my-api-server

# Preview configuration without installing
kumoctl configure --dry-run examples/openapi3-example.yaml weather-api

# Configure for cursor
kumoctl configure --client=cursor examples/openapi2-example.json my-tools
```

### Manual Usage

```bash
# Serve MCP from an OpenAPI spec file
kumoctl serve <path-to-openapi-spec>

# Examples
kumoctl serve ./examples/openapi2-example.json
kumoctl serve ./examples/openapi3-example.yaml
```

### Command Line Options

```bash
# Get help for all commands
kumoctl --help

# Get help for specific commands
kumoctl serve --help
kumoctl configure --help
```

## Commands

### `kumoctl serve`

Starts an MCP server that exposes tools based on your OpenAPI specification.

```bash
kumoctl serve <path-to-openapi-spec>
```

### `kumoctl configure`

Automatically configures kumoctl as an MCP server in your LLM client. This eliminates the need for manual JSON configuration.

```bash
kumoctl configure [OPTIONS] <spec-file> <server-name>
```

**Options:**
- `--dry-run`: Preview the configuration without installing it
- `--client <client>`: Target LLM client (claude-desktop, cursor, custom)
- `--config-path <path>`: Custom path to configuration file

**Supported Clients:**
- **Claude Desktop** (default): Automatically adds kumoctl to your Claude Desktop MCP configuration
- **Cursor**: Automatically adds kumoctl to Cursor. MCP support in Cursor IDE is Experimental

**Examples:**
```bash
# Configure for Claude Desktop (easiest method)
kumoctl configure examples/openapi2-example.json my-api

# Preview what would be configured
kumoctl configure --dry-run examples/openapi3-example.yaml weather-service

# Get JSON for manual configuration
kumoctl configure --client=custom examples/openapi2-example.json my-tools
```

**What it does:**
1. Locates your LLM client's configuration file
2. Adds kumoctl with your OpenAPI spec to the MCP servers list
3. Uses absolute paths to ensure reliability
4. Preserves existing MCP server configurations
5. Provides clear next steps (like restarting Claude Desktop)

## How It Works

1. **Load OpenAPI Spec**: kumoctl reads your OpenAPI 2.0 or 3.0 specification
2. **Generate MCP Tools**: Each operation (GET, POST, etc.) becomes an MCP tool
3. **Create Input Schemas**: Tool schemas include all parameters (path, query, body)
4. **Handle API Calls**: When a tool is called, kumoctl makes HTTP requests to your API
5. **Return Responses**: API responses are returned to the MCP client

### Tool Naming

Tools are named using the following priority:
1. `operationId` (if specified in the spec)
2. `{method}_{path}` (cleaned and normalized)

### Input Schema Generation

For each operation, kumoctl creates a JSON schema that includes:

- **Path Parameters**: Required parameters in the URL path
- **Query Parameters**: Optional/required query string parameters
- **Header Parameters**: HTTP headers to be sent
- **Body Parameters**: Individual fields from request body schemas (properly expanded from `$ref`)

#### Example: OpenAPI 2.0 Body Parameter Expansion

**OpenAPI Spec:**
```json
{
  "paths": {
    "/users": {
      "post": {
        "operationId": "users_create",
        "parameters": [
          {
            "name": "data",
            "in": "body",
            "schema": {"$ref": "#/definitions/UserRequest"}
          }
        ]
      }
    }
  },
  "definitions": {
    "UserRequest": {
      "type": "object",
      "required": ["name", "email"],
      "properties": {
        "name": {"type": "string"},
        "email": {"type": "string", "format": "email"},
        "age": {"type": "integer"}
      }
    }
  }
}
```

**Generated MCP Tool Schema:**
```json
{
  "type": "object",
  "required": ["name", "email"],
  "properties": {
    "name": {"type": "string"},
    "email": {"type": "string", "format": "email"},
    "age": {"type": "integer"}
  }
}
```

## Limitations and Considerations

### LLM Context Limits

⚠️ **Large OpenAPI Specifications**: If your OpenAPI spec is very large (hundreds of operations), the generated MCP tools may exceed LLM context limits.

### Current Limitations

1. **Authentication**: Currently no built-in authentication support
2. **Response Schemas**: Tool outputs are raw HTTP responses
3. **Error Handling**: HTTP errors are returned as-is
4. **Base URL Resolution**: Uses the first server URL found in the spec
   - OpenAPI 2.0: Constructs from `host`, `basePath`, and `schemes`
   - OpenAPI 3.0: Uses first entry in `servers` array
5. **STDIO Transport ONLY**: kumoctl only support STDIO transport for now

# Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request