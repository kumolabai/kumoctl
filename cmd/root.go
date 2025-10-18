package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "kumoctl",
	Short:   "A CLI tool for kumolab.ai",
	Long:    `kumoctl is a command-line interface for interacting with kumolab.ai`,
	Version: version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
