package provision

import (
	"fmt"
	"net"
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
	Name                 string `json:"name" yaml:"name"`
	IPAddress            string `json:"ip_address" yaml:"ip_address"`
	Username             string `json:"username" yaml:"username"`
	PrivateKey           string `json:"private_key" yaml:"private_key"`
	OrchestratorIP       string `json:"orchestrator_ip,omitempty" yaml:"orchestrator_ip,omitempty"`
	BacalhauSettingsPath string `json:"bacalhau_settings_path,omitempty" yaml:"bacalhau_settings_path,omitempty"`
	CustomScriptPath     string `json:"custom_script_path,omitempty" yaml:"custom_script_path,omitempty"`
}

// Validate checks if the configuration is valid
func (c *NodeConfig) Validate() error {
	if c.IPAddress == "" {
		return fmt.Errorf("IP address is required")
	}
	// Validate IP address format
	if net.ParseIP(c.IPAddress) == nil {
		return fmt.Errorf("invalid IP address format: %s", c.IPAddress)
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

	// Validate optional file paths if provided
	if c.BacalhauSettingsPath != "" {
		if _, err := os.Stat(c.BacalhauSettingsPath); err != nil {
			return fmt.Errorf("bacalhau settings file error: %w", err)
		}
	}
	if c.CustomScriptPath != "" {
		if _, err := os.Stat(c.CustomScriptPath); err != nil {
			return fmt.Errorf("custom script file error: %w", err)
		}
	}

	// Validate compute node requirements
	if c.OrchestratorIP != "" && net.ParseIP(c.OrchestratorIP) == nil {
		return fmt.Errorf("invalid orchestrator IP address format: %s", c.OrchestratorIP)
	}

	return nil
}
