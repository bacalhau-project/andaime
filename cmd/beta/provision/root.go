package provision

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	config = &NodeConfig{}
)

func GetProvisionNodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision a new node",
		Long: `Provision either an orchestrator or compute node using SSH.
For compute nodes, an orchestrator IP must be provided.`,
		RunE: runProvision,
	}

	// Add flags
	cmd.Flags().StringVar(&config.IPAddress, "ip", "", "IP address of the target node")
	cmd.Flags().StringVar(&config.Username, "user", "", "SSH username")
	cmd.Flags().StringVar(&config.PrivateKey, "key", "", "Path to private key file (PEM format)")
	cmd.Flags().
		StringVar(&config.OrchestratorIP, "orchestrator", "", "Orchestrator IP (required for compute nodes)")
	cmd.Flags().
		StringSliceVar(&config.BacalhauSettings, "bacalhau-setting", []string{},
			"Bacalhau settings in key=value format (can be specified multiple times)")
	cmd.Flags().
		StringVar(&config.CustomScriptPath, "custom-script", "",
			"Path to custom script to run after installation")

	// Mark required flags
	err := cmd.MarkFlagRequired("ip")
	if err != nil {
		fmt.Println(err)
		return cmd
	}
	err = cmd.MarkFlagRequired("user")
	if err != nil {
		fmt.Println(err)
		return cmd
	}
	err = cmd.MarkFlagRequired("key")
	if err != nil {
		fmt.Println(err)
		return cmd
	}

	return cmd
}
