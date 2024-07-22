package aws

import (
	"github.com/spf13/cobra"
)

var AwsCmd = &cobra.Command{
	Use:   "aws",
	Short: "AWS-related commands",
	Long:  `Commands for interacting with AWS resources.`,
}

func init() {
	// Add all AWS-related subcommands
	AwsCmd.AddCommand(createDeploymentCmd)
}
