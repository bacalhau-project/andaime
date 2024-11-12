package provision

import (
	"fmt"
	"os"
)

// NodeType represents the type of node being provisioned
type NodeType int

const (
	OrchestratorNode NodeType = iota
	ComputeNode
)

// NodeConfig holds the configuration for a node to be provisioned
type NodeConfig struct {
	// Required fields
	IPAddress  string
	Username   string
	PrivateKey string

	// Optional fields
	OrchestratorIP    string   // Required only for compute nodes
	NodeType          NodeType
	BacalhauSettings  []string // Key=value pairs for Bacalhau configuration
	CustomScriptPath  string   // Path to custom script to run after installation
}

// Validate checks if the configuration is valid
func (c *NodeConfig) Validate() error {
	if c.IPAddress == "" {
		return fmt.Errorf("IP address is required")
	}
	if c.Username == "" {
		return fmt.Errorf("username is required")
	}
	if c.PrivateKey == "" {
		return fmt.Errorf("private key is required")
	}

	// Verify private key exists and is readable
	if _, err := os.Stat(c.PrivateKey); err != nil {
		return fmt.Errorf("private key file error: %w", err)
	}

	// If this is a compute node, orchestrator IP is required
	if c.NodeType == ComputeNode && c.OrchestratorIP == "" {
		return fmt.Errorf("orchestrator IP is required for compute nodes")
	}

	return nil
}
