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
	cmd.Flags().StringVar(&config.OrchestratorIP, "orchestrator", "", "Orchestrator IP (required for compute nodes)")
	cmd.Flags().Var(newNodeTypeValue(&config.NodeType), "type", "Node type (orchestrator or compute)")

	// Mark required flags
	cmd.MarkFlagRequired("ip")
	cmd.MarkFlagRequired("user")
	cmd.MarkFlagRequired("key")

	return cmd
}

// nodeTypeValue implements pflag.Value interface for NodeType
type nodeTypeValue NodeType

func newNodeTypeValue(p *NodeType) *nodeTypeValue {
	*p = OrchestratorNode // default value
	return (*nodeTypeValue)(p)
}

func (n *nodeTypeValue) String() string {
	switch NodeType(*n) {
	case OrchestratorNode:
		return "requester"
	case ComputeNode:
		return "compute"
	}
	return "unknown"
}

func (n NodeType) String() string {
	switch n {
	case OrchestratorNode:
		return "requester"
	case ComputeNode:
		return "compute"
	}
	return "unknown"
}

func (n *nodeTypeValue) Set(val string) error {
	switch val {
	case "orchestrator":
		*n = nodeTypeValue(OrchestratorNode)
	case "compute":
		*n = nodeTypeValue(ComputeNode)
	default:
		return fmt.Errorf("must be either 'orchestrator' or 'compute'")
	}
	return nil
}

func (n *nodeTypeValue) Type() string {
	return "nodeType"
}
