// File: cmd/beta.go

package cmd

import (
	"fmt"

	"github.com/bacalhau-project/andaime/cmd/beta/azure"
	"github.com/bacalhau-project/andaime/cmd/beta/destroy"
	"github.com/spf13/cobra"
)

var (
	betaCmd *cobra.Command
)

func getBetaCmd(rootCmd *cobra.Command) *cobra.Command {
	once.Do(func() {
		betaCmd = &cobra.Command{
			Use:   "beta",
			Short: "Beta commands for testing and development",
			Long:  `Beta commands are experimental features that are not yet ready for production use.`,
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Println("Use 'andaime beta [command]' to run a beta command.")
				fmt.Println(
					"Use 'andaime beta --help' for more information about available beta commands.",
				)
			},
		}
		rootCmd.AddCommand(betaCmd)

		betaCmd.AddCommand(getTestDisplayCmd())
		betaCmd.AddCommand(azure.GetAzureCmd())
		betaCmd.AddCommand(destroy.GetDestroyCmd())
	})
	return betaCmd
}
