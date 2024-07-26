package azure

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var createAzureDeploymentCmd = &cobra.Command{
	Use:   "create-deployment",
	Short: "Create a deployment in Azure",
	Long:  `Create a deployment in Azure using the configuration specified in the config file.`,
	RunE:  executeCreateDeployment,
}

func GetAzureCreateDeploymentCmd() *cobra.Command {
	return createAzureDeploymentCmd
}

func executeCreateDeployment(cmd *cobra.Command, args []string) error {
	logger.InitProduction(false, true)
	l := logger.Get()

	l.Debug("Starting executeCreateDeployment")

	l.Debug("Initializing Azure provider")
	azureProvider, err := azure.AzureProviderFunc(viper.GetViper())
	if err != nil {
		errString := fmt.Sprintf("Failed to initialize Azure provider: %s", err.Error())
		l.Error(errString)
		return fmt.Errorf(errString)
	}
	l.Debug("Azure provider initialized successfully")

	l.Debug("Setting up signal channel")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	l.Debug("Creating display")
	disp := display.NewDisplay(1)
	l.Debug("Starting display")
	go func() {
		l.Debug("Display Start() called")
		disp.Start(sigChan)
		l.Debug("Display Start() returned")
	}()

	defer func() {
		l.Debug("Stopping display")
		disp.Stop()
		l.Debug("Display stopped")
	}()

	l.Debug("Updating initial status")
	disp.UpdateStatus(&models.Status{
		ID:     "azure-deployment",
		Type:   "Azure",
		Status: "Initializing",
	})

	l.Debug("Starting resource deployment")
	err = azureProvider.DeployResources(cmd.Context(), disp)
	if err != nil {
		errString := fmt.Sprintf("Failed to deploy resources: %s", err.Error())
		l.Error(errString)
		disp.UpdateStatus(&models.Status{
			ID:     "azure-deployment",
			Type:   "Azure",
			Status: "Failed",
		})
		return fmt.Errorf(errString)
	}

	l.Debug("Resource deployment completed")

	disp.UpdateStatus(&models.Status{
		ID:     "azure-deployment",
		Type:   "Azure",
		Status: "Completed",
	})

	l.Info("Azure deployment created successfully")
	cmd.Println("Azure deployment created successfully")
	cmd.Println("Press 'q' and Enter to quit")

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			l.Debug("Waiting for input")
			char, _, err := reader.ReadRune()
			if err != nil {
				l.Error(fmt.Sprintf("Error reading input: %s", err.Error()))
				continue
			}
			if char == 'q' || char == 'Q' {
				l.Debug("Quit signal received")
				sigChan <- os.Interrupt
				return
			}
		}
	}()

	l.Debug("Waiting for signal")
	<-sigChan
	l.Debug("Signal received, exiting")
	return nil
}
