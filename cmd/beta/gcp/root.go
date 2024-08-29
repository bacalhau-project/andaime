package gcp

import (
	"github.com/spf13/cobra"
)

var gcpCmd *cobra.Command

func GetGCPCmd() *cobra.Command {
	if gcpCmd == nil {
		gcpCmd = &cobra.Command{
			Use:   "gcp",
			Short: "GCP-related commands",
			Long:  `Commands for interacting with Google Cloud Platform (GCP).`,
		}
		gcpCmd.AddCommand(getCreateDeploymentCmd())
	}
	return gcpCmd
}
