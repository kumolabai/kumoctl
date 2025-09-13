package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of kumoctl",
	Long:  `All software has versions. This is kumoctl's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kumoctl v0.0.1")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
