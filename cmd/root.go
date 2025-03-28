package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/johnsaigle/go-unmaintained/pkg/scan"
)

var (
	target string
	token  string

	rootCmd = &cobra.Command{
		Use: "go-unmaintained",
		Run: run,
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&target, "target", "", "go project to scan")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "GitHub token")
}

func run(cmd *cobra.Command, args []string) {

	if len(target) == 0 {
		fmt.Println("Must provide target")
		os.Exit(1)
	}
	if len(token) == 0 {
		token = os.Getenv("GITHUB_TOKEN")
		if len(token) == 0 {
			fmt.Println("Must provide token")
			os.Exit(1)
		}

	}
	scan.ScanRepo(scan.Config{Token: token, Target: target})
}
