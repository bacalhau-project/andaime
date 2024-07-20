package cmd

import (
	"github.com/spf13/cobra"
)

var azureCmd = &cobra.Command{
	Use:   "azure",
	Short: "Azure-related commands",
	Long:  `Commands for interacting with Azure resources.`,
}
