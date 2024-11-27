package models

import "fmt"

// NodeInfo represents the deployment information for a single node
type NodeInfo struct {
	Name              string `yaml:"name"`
	NodeIP            string `yaml:"node_ip"`
	OrchestratorIP    string `yaml:"orchestrator_ip,omitempty"`
	DockerInstalled   bool   `yaml:"docker_installed"`
	BacalhauInstalled bool   `yaml:"bacalhau_installed"`
	CustomScript      bool   `yaml:"custom_script_installed"`
	SSHUsername       string `yaml:"ssh_username"`
	SSHKeyPath        string `yaml:"ssh_key_path"`
}

// NodeInfoList represents a collection of node information
type NodeInfoList struct {
	Nodes []NodeInfo `yaml:"nodes"`
}

// Validate checks if the node information is valid
func (n *NodeInfo) Validate() error {
	if n.Name == "" {
		return fmt.Errorf("node name is required")
	}
	if n.NodeIP == "" {
		return fmt.Errorf("node IP address is required")
	}
	if n.SSHUsername == "" {
		return fmt.Errorf("SSH username is required")
	}
	if n.SSHKeyPath == "" {
		return fmt.Errorf("SSH key path is required")
	}
	return nil
}
