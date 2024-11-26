package provision

import (
	"fmt"
	"net"
	"os"

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
	cmd.Flags().StringVar(&config.PrivateKey, "key", "",
		"Path to SSH private key file (PEM format)")
	cmd.Flags().StringVar(&config.OrchestratorIP, "orchestrator", "",
		"IP address of the orchestrator node (required for compute nodes)")
	cmd.Flags().StringVar(&config.BacalhauSettingsPath, "bacalhau-settings", "",
		"Path to JSON file containing Bacalhau settings")

	// Validate IP address
	if net.ParseIP(config.IPAddress) == nil {
		fmt.Printf("Invalid IP address: %s", config.IPAddress)
		return cmd
	}

	// Validate orchestrator IP if provided
	if config.OrchestratorIP != "" && net.ParseIP(config.OrchestratorIP) == nil {
		fmt.Printf("Invalid orchestrator IP address: %s", config.OrchestratorIP)
		return cmd
	}

	// Validate private key file path
	if _, err := os.Stat(config.PrivateKey); os.IsNotExist(err) {
		fmt.Printf("Private key file does not exist: %s", config.PrivateKey)
		return cmd
	}
	cmd.Flags().BoolVar(&testMode, "test", false,
		"Run in test mode (simulation only)")

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
