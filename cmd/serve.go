package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	kumo_mcp "github.com/kumolabai/kumoctl/pkg/mcp"
	"github.com/kumolabai/kumoctl/pkg/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:     "serve [spec-path-or-url]",
	Short:   "Start MCP Server from OpenAPI Spec",
	Example: "  kumoctl serve ./spec.json --headers \"Authorization=Basic <creds>\"\n  kumoctl serve https://api.example.com/openapi.json --headers \"Authorization=Bearer token\"",
	Args:    verifySpecSource,
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		openapiSpec, err := openapi.LoadSpecFromSource(source)
		if err != nil {
			return err
		}

		headers, err := cmd.Flags().GetStringArray("headers")
		if err != nil {
			return err
		}

		parsedHeaders, err := parseHeaders(headers)
		if err != nil {
			return err
		}

		serverName := "kumolab-mcp-server"
		serverTitle := "KumoLab.ai MCP Server"
		version := "v0.0.1"

		if openapiSpec.GetInfo().Title != "" {
			serverTitle = openapiSpec.GetInfo().Title
		}

		if openapiSpec.GetVersion() != "" {
			version = openapiSpec.GetVersion()
		}

		server := mcp.NewServer(&mcp.Implementation{Name: serverName, Title: serverTitle, Version: version}, nil)

		// Dynamically generate tools from OpenAPI paths
		if err := kumo_mcp.GenerateToolsFromSpec(server, openapiSpec, parsedHeaders); err != nil {
			return fmt.Errorf("failed to generate tools from OpenAPI spec: %w", err)
		}

		// Run the server over stdin/stdout, until the client disconnects
		if err := server.Run(cmd.Context(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}

		return nil
	},
}

func parseHeaders(headerStrings []string) (http.Header, error) {
	headers := make(http.Header)
	for _, h := range headerStrings {
		parts := strings.SplitN(h, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header format: %s (expected 'key=value')", h)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		headers.Add(key, value)
	}
	return headers, nil
}

func verifySpecSource(cmd *cobra.Command, args []string) error {
	if err := cobra.ExactArgs(1)(cmd, args); err != nil {
		return err
	}

	source := args[0]

	// Only validate file existence if it's not a URL
	if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		if _, err := os.Stat(source); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", source)
		}
	}

	if _, err := openapi.LoadSpecFromSource(source); err != nil {
		return err
	}

	return nil
}

func init() {
	serveCmd.Flags().StringArray("headers", []string{}, "headers to inject on requests in the form of key=value")
	rootCmd.AddCommand(serveCmd)
}
