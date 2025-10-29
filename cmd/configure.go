package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// MCPServerConfig represents a single MCP server configuration
type MCPServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// ClaudeDesktopConfig represents the full Claude Desktop configuration
type MCPClientConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

var configureCmd = &cobra.Command{
	Use:   "configure [spec-path-or-url] [server-name]",
	Short: "Generate MCP server configuration for LLM clients",
	Long: `Generate and optionally install MCP server configuration for various LLM clients.
This command helps you automatically configure kumoctl as an MCP server in your LLM client.

Supported clients:
- Claude Desktop (default)
- Cursor

Examples:
  # Generate configuration for Claude Desktop
  kumoctl configure examples/openapi2-example.json my-api

  # Generate configuration from URL
  kumoctl configure https://api.example.com/openapi.json my-api

  # Add custom headers
  kumoctl configure examples/openapi.json my-api --headers "Authorization=Bearer token" --headers "X-Api-Key=key123"

  # Generate configuration without installing
  kumoctl configure --dry-run examples/openapi3-example.yaml weather-api

  # Specify custom client
  kumoctl configure --client=cursor examples/openapi2-example.json my-tools`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigure,
}

var (
	dryRun bool
	client string
)

func init() {
	rootCmd.AddCommand(configureCmd)

	configureCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print configuration without installing")
	configureCmd.Flags().StringVar(&client, "client", "claude-desktop", "Target LLM client (claude-desktop, cursor)")
	configureCmd.Flags().StringArray("headers", []string{}, "Headers to inject on requests in the form of key=value")
}

func runConfigure(cmd *cobra.Command, args []string) error {
	specSource := args[0]
	serverName := args[1]

	// Get headers flag
	headers, err := cmd.Flags().GetStringArray("headers")
	if err != nil {
		return err
	}

	// Check if source is a URL or file path
	isURL := strings.HasPrefix(specSource, "http://") || strings.HasPrefix(specSource, "https://")

	var specPath string
	if isURL {
		// For URLs, we'll pass the URL directly
		specPath = specSource
	} else {
		// Validate spec file exists
		if _, err := os.Stat(specSource); os.IsNotExist(err) {
			return fmt.Errorf("spec file does not exist: %s", specSource)
		}

		// Get absolute path to spec file
		absSpecFile, err := filepath.Abs(specSource)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for spec file: %w", err)
		}
		specPath = absSpecFile
	}

	// Get kumoctl executable path
	executable, err := getKumoctlPath()
	if err != nil {
		return fmt.Errorf("failed to locate kumoctl executable: %w", err)
	}

	// Generate configuration based on client
	switch strings.ToLower(client) {
	case "claude-desktop":
		return configureClaudeDesktop(executable, specPath, serverName, headers)
	case "cursor":
		return configureCursor(executable, specPath, serverName, headers)
	default:
		return fmt.Errorf("unsupported client: %s", client)
	}
}

func getKumoctlPath() (string, error) {
	// First, check if we're running with 'go run'
	if len(os.Args) > 0 && strings.Contains(os.Args[0], "go") {
		// We're running with 'go run', use that
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("go run %s", wd), nil
	}

	// Otherwise, get the current executable path
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}

	return executable, nil
}

func configureClaudeDesktop(executable, specFile, serverName string, headers []string) error {
	configDir := getClaudeDesktopConfigDir()
	configFile := filepath.Join(configDir, "claude_desktop_config.json")

	if err := configureMCPClient(configDir, configFile, executable, specFile, serverName, headers); err != nil {
		return err
	}

	fmt.Printf("Successfully configured MCP server '%s' for Claude Desktop\n", serverName)
	fmt.Printf("Please restart Claude Desktop for changes to take effect.\n")

	return nil
}

func configureCursor(executable, specFile, serverName string, headers []string) error {
	// Cursor uses a similar configuration format to Claude Desktop
	// but in a different location
	configDir := getCursorConfigDir()
	configFile := filepath.Join(configDir, "mcp_config.json")

	if err := configureMCPClient(configDir, configFile, executable, specFile, serverName, headers); err != nil {
		return nil
	}

	fmt.Printf("Successfully configured MCP server '%s' for Cursor\n", serverName)
	fmt.Printf("Note: Cursor MCP integration is experimental. Please refer to Cursor documentation for the latest setup instructions.\n")

	return nil
}

func getClaudeDesktopConfigDir() string {
	switch runtime.GOOS {
	case "darwin": // macOS
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "Claude")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, _ := os.UserHomeDir()
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Claude")
	default: // Linux and others
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "claude")
	}
}

func getCursorConfigDir() string {
	switch runtime.GOOS {
	case "darwin": // macOS
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "Cursor")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			home, _ := os.UserHomeDir()
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Cursor")
	default: // Linux and others
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "cursor")
	}
}

func getMCPClientConfig(configFile string, executable string, specFile string, serverName string, headers []string) (*MCPClientConfig, error) {
	// Read existing configuration
	var config MCPClientConfig
	if data, err := os.ReadFile(configFile); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse existing config: %w", err)
		}
	}

	// Initialize mcpServers if it doesn't exist
	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	// Create the server configuration
	args := []string{"serve", specFile}

	// Add headers if provided
	for _, header := range headers {
		args = append(args, "--headers", header)
	}

	serverConfig := MCPServerConfig{
		Command: executable,
		Args:    args,
	}

	// Add or update the server
	config.MCPServers[serverName] = serverConfig

	return &config, nil
}

func configureMCPClient(configDir string, configFile string, executable string, specFile string, serverName string, headers []string) error {
	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	config, err := getMCPClientConfig(configFile, executable, specFile, serverName, headers)
	if err != nil {
		return err
	}

	if dryRun {
		// Print the configuration
		configJSON, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal configuration: %w", err)
		}

		fmt.Printf("%s\n", configJSON)
		return nil
	}

	// Write the configuration
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := os.WriteFile(configFile, configJSON, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}
