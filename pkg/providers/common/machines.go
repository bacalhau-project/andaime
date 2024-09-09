package common

import (
	"fmt"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
)

func CreateNewMachine(
	deploymentType models.DeploymentType,
	location string,
	diskSizeGB int32,
	vmSize string,
	diskImageFamily string,
	diskImageURL string,
	privateKeyPath string,
	privateKeyBytes []byte,
	sshPort int,
) (*models.Machine, error) {
	newMachine, err := models.NewMachine(deploymentType, location, vmSize, diskSizeGB)
	if err != nil {
		return nil, fmt.Errorf("failed to create new machine: %w", err)
	}

	if err := newMachine.EnsureMachineServices(); err != nil {
		logger.Get().Errorf("Failed to ensure machine services: %v", err)
	}

	for _, service := range models.RequiredServices {
		newMachine.SetServiceState(service.Name, models.ServiceStateNotStarted)
	}

	sshUser := viper.GetString("general.ssh_user")

	newMachine.SSHPort = sshPort
	newMachine.SSHUser = sshUser
	newMachine.SSHPrivateKeyPath = privateKeyPath
	newMachine.SSHPrivateKeyMaterial = privateKeyBytes

	newMachine.DiskImageFamily = diskImageFamily
	newMachine.DiskImageURL = diskImageURL

	return newMachine, nil
}
