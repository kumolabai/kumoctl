package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of kumoctl",
	Long:  `All software has versions. This is kumoctl's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("kumoctl %s\n", version)
		fmt.Printf("  Commit: %s\n", commit)
		fmt.Printf("  Built:  %s\n", date)
		fmt.Printf("  OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("  Go:     %s\n", runtime.Version())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
