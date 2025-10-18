package cmd

import (
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	kumo_mcp "github.com/kumolabai/kumoctl/pkg/mcp"
	"github.com/kumolabai/kumoctl/pkg/openapi"
	"github.com/spf13/cobra"
)

var listToolsCmd = &cobra.Command{
	Use:   "tools [spec-path-or-url]",
	Short: "List generated tools from spec",
	Args:  verifySpecSource,
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]
		openapiSpec, err := openapi.LoadSpecFromSource(source)
		if err != nil {
			return err
		}

		// Dynamically generate tools from OpenAPI paths
		tools, err := kumo_mcp.GetToolsFromSpec(openapiSpec)
		if err != nil {
			return fmt.Errorf("failed to generate tools from OpenAPI spec: %w", err)
		}

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendHeader(table.Row{"#", "Name", "Description"})
		for i, tool := range tools {
			t.AppendRow(table.Row{
				i + 1, tool.Name, tool.Description,
			})
			t.AppendSeparator()
		}
		t.Render()

		return nil
	},
}

func init() {
	listCmd.AddCommand(listToolsCmd)
}
