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
	diskSizeGB int,
	vmSize string,
	diskImageFamily string,
	diskImageURL string,
	privateKeyPath string,
	privateKeyBytes []byte,
	publicKeyPath string,
	publicKeyBytes []byte,
) (models.Machiner, error) {
	newMachine, err := models.NewMachine(deploymentType,
		location,
		vmSize,
		diskSizeGB,
		models.CloudSpecificInfo{},
	)
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
	sshPort := viper.GetInt("general.ssh_port")

	newMachine.SetSSHPort(sshPort)
	newMachine.SetSSHUser(sshUser)
	newMachine.SetSSHPrivateKeyPath(privateKeyPath)
	newMachine.SetSSHPrivateKeyMaterial(privateKeyBytes)
	newMachine.SetSSHPublicKeyPath(publicKeyPath)
	newMachine.SetSSHPublicKeyMaterial(publicKeyBytes)
	newMachine.SetDiskImageFamily(diskImageFamily)
	newMachine.SetDiskImageURL(diskImageURL)

	return newMachine, nil
}
