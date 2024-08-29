package aws

import (
	"github.com/spf13/cobra"
)

var AwsCmd = &cobra.Command{
	Use:   "aws",
	Short: "AWS-related commands",
	Long:  `Commands for interacting with AWS resources.`,
}

func SetupAWSCommands(rootCmd *cobra.Command) {
	// Add AWS-specific commands to rootCmd
	// This function would be called from the main setup in cmd/root.go
}
