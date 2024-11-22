package models

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
