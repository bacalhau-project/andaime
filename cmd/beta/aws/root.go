package aws

import (
	"github.com/spf13/cobra"
	"github.com/bacalhau-project/andaime/cmd/beta/destroy"
)

var AwsCmd = &cobra.Command{
	Use:   "aws",
	Short: "AWS-related commands",
	Long:  `Commands for interacting with AWS resources.`,
}

func init() {
	// Add all AWS-related subcommands
	AwsCmd.AddCommand(createDeploymentCmd)
	AwsCmd.AddCommand(destroy.GetDestroyCmd())
}
