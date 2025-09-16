package cmd

import (
	"fmt"
	"log"
	"os"

	kumo_mcp "github.com/kumolabai/kumoctl/pkg/mcp"
	"github.com/kumolabai/kumoctl/pkg/openapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP Server from OpenAPI Spec",
	Args: func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(1)(cmd, args); err != nil {
			return err
		}

		filePath := args[0]
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", filePath)
		}

		if _, err := openapi.LoadSpec(filePath); err != nil {
			return err
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {

		filePath := args[0]
		openapiSpec, err := openapi.LoadSpec(filePath)
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
		if err := kumo_mcp.GenerateToolsFromSpec(server, openapiSpec); err != nil {
			return fmt.Errorf("failed to generate tools from OpenAPI spec: %w", err)
		}

		// Run the server over stdin/stdout, until the client disconnects
		if err := server.Run(cmd.Context(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
