// File: cmd/beta.go

package cmd

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"
)

var (
	betaCmd *cobra.Command
	once    sync.Once // Ensures the lazy initialization happens only once
)

func getBetaCmd(rootCmd *cobra.Command) *cobra.Command {
	once.Do(func() {
		betaCmd = &cobra.Command{
			Use:   "beta",
			Short: "Beta commands for testing and development",
			Long:  `Beta commands are experimental features that are not yet ready for production use.`,
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("Use 'andaime beta [command]' to run a beta command.")
				fmt.Println("Use 'andaime beta --help' for more information about available beta commands.")
			},
		}
		rootCmd.AddCommand(betaCmd)

		betaCmd.AddCommand(getTestDisplayCmd())
		betaCmd.AddCommand(createCmd, destroyCmd, listCmd)
	})
	return betaCmd
}
