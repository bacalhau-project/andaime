package provision

import (
	"context"
	"fmt"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/sshutils"
)

// SystemRequirements defines the minimum system requirements
type SystemRequirements struct {
	MinCPUs    int
	MinRAMGB   int
	MinDiskGB  int
	OSVersions []string
}

// DefaultSystemRequirements returns the default minimum requirements
func DefaultSystemRequirements() *SystemRequirements {
	return &SystemRequirements{
		MinCPUs:    2,
		MinRAMGB:   4,
		MinDiskGB:  20,
		OSVersions: []string{"ubuntu-20.04", "ubuntu-22.04"},
	}
}

// checkSystemRequirements verifies if the system meets minimum requirements
func checkSystemRequirements(ctx context.Context, ssh sshutils.SSHConfiger) error {
	// Check CPU cores
	session, err := ssh.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput("nproc --all")
	if err != nil {
		return fmt.Errorf("failed to get CPU count: %w", err)
	}
	cpuCount := strings.TrimSpace(string(output))

	// Check RAM
	session, err = ssh.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err = session.CombinedOutput("free -g | awk '/^Mem:/{print $2}'")
	if err != nil {
		return fmt.Errorf("failed to get RAM size: %w", err)
	}
	ramGB := strings.TrimSpace(string(output))

	// Check Disk Space
	session, err = ssh.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err = session.CombinedOutput("df -BG / | awk 'NR==2 {print $4}'")
	if err != nil {
		return fmt.Errorf("failed to get disk space: %w", err)
	}
	diskGB := strings.TrimSpace(string(output))

	// Check OS Version
	session, err = ssh.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err = session.CombinedOutput("lsb_release -d")
	if err != nil {
		return fmt.Errorf("failed to get OS version: %w", err)
	}
	osVersion := strings.TrimSpace(string(output))

	// Validate against requirements
	reqs := DefaultSystemRequirements()
	
	// Parse and compare values
	// Add proper error handling and validation here
	
	return nil
}
