// File: cmd/beta.go

package cmd

import (
	"fmt"

	"github.com/bacalhau-project/andaime/cmd/beta"
	"github.com/bacalhau-project/andaime/cmd/beta/azure"
	"github.com/bacalhau-project/andaime/cmd/beta/gcp"
	"github.com/bacalhau-project/andaime/internal"
	"github.com/spf13/cobra"
)

var (
	betaCmd *cobra.Command
)

func GetBetaCmd() *cobra.Command {
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
	betaCmd.AddCommand(beta.GetTestDisplayCmd())
	betaCmd.AddCommand(azure.GetAzureCmd())
	betaCmd.AddCommand(gcp.GetGCPCmd())
	betaCmd.AddCommand(internal.GetGenerateCloudDataCmd())
	// betaCmd.AddCommand(azure.GetAwsCmd())
	return betaCmd
}
