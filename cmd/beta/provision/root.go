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
	cmd.Flags().StringVar(&config.IPAddress, "ip", "",
		"IP address of the target node (IPv4 or IPv6 format)")
	cmd.Flags().StringVar(&config.Username, "user", "",
		"SSH username for authentication")
	cmd.Flags().StringVar(&config.PrivateKeyPath, "key", "",
		"Path to SSH private key file (PEM format)")
	cmd.Flags().StringVar(&config.OrchestratorIP, "orchestrator", "",
		"IP address of the orchestrator node (required for compute nodes)")
	cmd.Flags().StringVar(&config.BacalhauSettingsPath, "bacalhau-settings", "",
		"Path to JSON file containing Bacalhau settings")

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
