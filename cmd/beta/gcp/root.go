package gcp

import (
	"sync"

	"github.com/spf13/cobra"
)

var once sync.Once

var GCPCmd = &cobra.Command{
	Use:   "gcp",
	Short: "GCP-related commands",
	Long:  `Commands for interacting with Google Cloud Platform (GCP).`,
}

func InitializeCommands() {
	once.Do(func() {
		GCPCmd.AddCommand(createDeploymentCmd())
	})
}

func GetGCPCmd() *cobra.Command {
	InitializeCommands()
	return GCPCmd
}
