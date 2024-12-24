package models

import (
	"fmt"
	"time"
)

// ProvisioningStage represents different stages of machine provisioning
type ProvisioningStage string

const (
	// VM provisioning stages
	StageVMRequested     ProvisioningStage = "Requesting VM"
	StageVMProvisioning  ProvisioningStage = "Provisioning VM"
	StageVMProvisioned   ProvisioningStage = "VM Provisioned"
	StageVMFailed        ProvisioningStage = "VM Provisioning Failed"

	// SSH provisioning stages
	StageSSHConfiguring  ProvisioningStage = "Configuring SSH"
	StageSSHConfigured   ProvisioningStage = "SSH Configured"
	StageSSHFailed       ProvisioningStage = "SSH Configuration Failed"

	// Docker installation stages
	StageDockerInstalling ProvisioningStage = "Installing Docker"
	StageDockerInstalled  ProvisioningStage = "Docker Installed"
	StageDockerFailed     ProvisioningStage = "Docker Installation Failed"

	// Bacalhau installation stages
	StageBacalhauInstalling ProvisioningStage = "Installing Bacalhau"
	StageBacalhauInstalled  ProvisioningStage = "Bacalhau Installed"
	StageBacalhauFailed     ProvisioningStage = "Bacalhau Installation Failed"

	// Script execution stages
	StageScriptExecuting    ProvisioningStage = "Executing Script"
	StageScriptCompleted    ProvisioningStage = "Script Completed"
	StageScriptFailed       ProvisioningStage = "Script Execution Failed"
)

// DisplayStatus represents the current status of a machine
type DisplayStatus struct {
	Name           string
	Location       string
	PublicIP       string
	PrivateIP      string
	StatusMessage  string
	ElapsedTime    time.Duration
	Orchestrator   bool
	Stage          ProvisioningStage
	SpotInstance   bool
	ServiceStates  map[string]ServiceState
}

// NewDisplayStatus creates a new DisplayStatus instance
func NewDisplayStatus(name string) *DisplayStatus {
	return &DisplayStatus{
		Name:          name,
		ServiceStates: make(map[string]ServiceState),
		Stage:         StageVMRequested,
	}
}

// SetStage updates the stage and status message
func (s *DisplayStatus) SetStage(stage ProvisioningStage) {
	s.Stage = stage
	prefix := ""
	if s.SpotInstance {
		prefix = "[Spot] "
	}
	s.StatusMessage = fmt.Sprintf("%s%s", prefix, stage)
}

// UpdateFromMachine updates the display status from a machine's state
func (s *DisplayStatus) UpdateFromMachine(m *Machine) {
	s.Location = m.Location
	s.PublicIP = m.PublicIP
	s.PrivateIP = m.PrivateIP
	s.ElapsedTime = m.ElapsedTime
	s.Orchestrator = m.Orchestrator
	s.SpotInstance = m.CloudSpecific.SpotMarketOptions != nil

	// Determine stage based on machine's timing fields
	switch {
	case m.Failed:
		s.SetStage(StageVMFailed)
	case !m.CreationEndTime.IsZero() && m.SSHStartTime.IsZero():
		s.SetStage(StageVMProvisioned)
	case !m.SSHStartTime.IsZero() && m.SSHEndTime.IsZero():
		s.SetStage(StageSSHConfiguring)
	case !m.SSHEndTime.IsZero() && m.DockerStartTime.IsZero():
		s.SetStage(StageSSHConfigured)
	case !m.DockerStartTime.IsZero() && m.DockerEndTime.IsZero():
		s.SetStage(StageDockerInstalling)
	case !m.DockerEndTime.IsZero() && m.BacalhauStartTime.IsZero():
		s.SetStage(StageDockerInstalled)
	case !m.BacalhauStartTime.IsZero() && m.BacalhauEndTime.IsZero():
		s.SetStage(StageBacalhauInstalling)
	case !m.BacalhauEndTime.IsZero():
		s.SetStage(StageBacalhauInstalled)
	default:
		s.SetStage(StageVMProvisioning)
	}
}
