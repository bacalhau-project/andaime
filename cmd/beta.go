// File: cmd/beta.go

package cmd

import (
	"github.com/bacalhau-project/andaime/cmd/beta"
	"github.com/bacalhau-project/andaime/cmd/beta/aws"
	"github.com/bacalhau-project/andaime/cmd/beta/azure"
	"github.com/bacalhau-project/andaime/cmd/beta/gcp"
	"github.com/bacalhau-project/andaime/cmd/beta/provision"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add commands in a deterministic order
	betaCmd.AddCommand(
		aws.GetAwsCmd(),
		azure.GetAzureCmd(),
		gcp.GetGCPCmd(),
		beta.GetTestDisplayCmd(),
		internal.GetGenerateCloudDataCmd(),
		provision.GetProvisionNodeCmd(),
	)

	return betaCmd
}
