package aws

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create AWS resources",
	Long:  `Create various AWS resources including deployments.`,
}

func init() {
	createCmd.AddCommand(createDeploymentCmd)
}

var createDeploymentCmd = &cobra.Command{
	Use:   "deployment",
	Short: "Create a deployment in AWS",
	Long:  `Create a deployment in AWS using the configuration specified in the config file.`,
	RunE:  executeCreateDeployment,
}

func executeCreateDeployment(cmd *cobra.Command, args []string) error {
	logger.InitProduction()
	log := logger.Get()

	awsProvider, err := awsprovider.NewAWSProvider(viper.GetViper())
	if err != nil {
		errString := fmt.Sprintf("Failed to initialize AWS provider: %s", err.Error())
		log.Error(errString)
		return fmt.Errorf(errString)
	}

	//nolint:gomnd
	sigChan := utils.CreateSignalChannel("sigChan", 5)

	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM) //nolint:sigchanyzer

	disp := display.NewDisplay()

	defer func() {
		disp.Stop()
		utils.CloseChannel(sigChan)
	}()

	err = awsProvider.CreateDeployment(cmd.Context())
	if err != nil {
		errString := fmt.Sprintf("Failed to create deployment: %s", err.Error())
		log.Error(errString)
		return fmt.Errorf(errString)
	}

	// TODO: Implement resource status updates when AWS provider supports it
	log.Info("AWS deployment created successfully")
	cmd.Println("AWS deployment created successfully")
	return nil
}
