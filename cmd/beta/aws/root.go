package aws

import (
	"sync"

	"github.com/spf13/cobra"
)

var once sync.Once
var AwsCmd = &cobra.Command{
	Use:   "aws",
	Short: "AWS-related commands",
	Long:  `Commands for interacting with AWS resources.`,
}

func GetAwsCmd() *cobra.Command {
	InitializeCommands()
	return AwsCmd
}

func InitializeCommands() {
	once.Do(func() {
		AwsCmd.AddCommand(GetAwsListDeploymentsCmd())
		AwsCmd.AddCommand(GetAwsCreateDeploymentCmd())
		AwsCmd.AddCommand(GetAwsDestroyCmd())
	})
}
