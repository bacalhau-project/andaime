package azure

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/olekukonko/tablewriter"
)

// DeployResources deploys Azure resources based on the provided configuration.
// Config should be the Azure subsection of the viper config.
func (p *AzureProvider) DeployResources(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	// Set the start time for the deployment
	deployment.StartTime = time.Now()

	// Prepare resource group
	resourceGroupName, resourceGroupLocation, err := p.PrepareResourceGroup(
		ctx,
		deployment,
		disp,
	)
	if err != nil {
		return fmt.Errorf("failed to prepare resource group: %w", err)
	}

	deployment.ResourceGroupName = resourceGroupName
	deployment.ResourceGroupLocation = resourceGroupLocation

	err = deployment.UpdateViperConfig()
	if err != nil {
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	// Deploy Bicep template
	err = p.ProcessMachines(ctx, deployment, disp)
	if err != nil {
		return err
	}

	err = deployment.UpdateViperConfig()
	if err != nil {
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	return p.FinalizeDeployment(ctx, deployment, disp)
}

// ProcessMachines processes the machines defined in the deployment
func (p *AzureProvider) ProcessMachines(ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display) error {
	l := logger.Get()

	l.Debugf("Processing machines for deployment: %v", deployment)

	for _, machine := range deployment.Machines {
		// Update status for each machine
		disp.UpdateStatus(&models.Status{
			ID:     machine.ID,
			Status: "Processing",
		})

		createdMachine, err := p.CreateVirtualMachine(
			ctx,
			deployment,
			machine,
			disp,
		)
		if err != nil {
			return fmt.Errorf("failed to create virtual machine: %w", err)
		}

		// TODO: Implement any necessary post-deployment processing for each machine
		// This might include:
		// 1. Retrieving VM information from Azure
		// 2. Updating the machine struct with actual deployment details
		// 3. Performing any required post-deployment configuration

		// Update status to indicate completion
		disp.UpdateStatus(&models.Status{
			ID:     machine.ID,
			Status: fmt.Sprintf("Complete - %s", *createdMachine.ID),
		})
	}

	// After processing all machines, update the global deployment struct
	if err := deployment.UpdateViperConfig(); err != nil {
		l.Errorf("Failed to update viper config: %v", err)
		return fmt.Errorf("failed to update viper config: %w", err)
	}
	l.Debug("Successfully updated viper config after processing machines")
	return nil
}

// This function is no longer needed with Bicep templates

// finalizeDeployment performs any necessary cleanup and final steps
func (p *AzureProvider) FinalizeDeployment(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	l := logger.Get()

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		l.Info("Deployment cancelled during finalization")
		return fmt.Errorf("deployment cancelled: %w", err)
	}

	// Log successful completion
	l.Info("Azure deployment completed successfully")

	// Print summary of deployed resources
	summaryMsg := fmt.Sprintf(
		"\nDeployment Summary for Resource Group: %s\n",
		deployment.ResourceGroupName,
	)
	summaryMsg += fmt.Sprintf("Location: %s\n", deployment.ResourceGroupLocation)
	l.Info(summaryMsg)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(
		[]string{"ID", "Type", "Location", "Status", "Public IP", "Private IP", "Elapsed Time (s)"},
	)

	startTime := deployment.StartTime
	if startTime.IsZero() {
		startTime = time.Now() // Fallback if start time wasn't set
	}

	for _, machine := range deployment.Machines {
		publicIP := ""
		privateIP := ""
		if machine.PublicIPAddress != nil {
			publicIP = *machine.PublicIPAddress.Properties.IPAddress
		}
		if machine.PrivateIPAddress != nil {
			privateIP = *machine.PrivateIPAddress.Properties.PrivateIPAddress
		}
		elapsedTime := time.Since(startTime).Seconds()
		table.Append([]string{
			machine.ID,
			"VM",
			machine.Location,
			machine.Status,
			publicIP,
			privateIP,
			fmt.Sprintf("%.2f", elapsedTime),
		})
	}

	table.Render()

	// Ensure all configurations are saved
	if err := deployment.UpdateViperConfig(); err != nil {
		l.Errorf("Failed to save final configuration: %v", err)
		return fmt.Errorf("failed to save final configuration: %w", err)
	}

	// Update final status in the display
	disp.UpdateStatus(&models.Status{
		ID:     "azure-deployment",
		Type:   "Azure",
		Status: "Completed",
	})

	l.Info("Deployment finalized successfully")

	return nil
}

// prepareResourceGroup prepares or creates a resource group for the deployment
func (p *AzureProvider) PrepareResourceGroup(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) (string, string, error) {
	l := logger.Get()

	// Check if the resource group name already contains a timestamp
	resourceGroupName := deployment.ResourceGroupName + "-" + time.Now().Format("20060102150405")
	resourceGroupLocation := deployment.ResourceGroupLocation

	l.Debugf("Creating Resource Group - %s", resourceGroupName)

	_, err := p.Client.GetOrCreateResourceGroup(
		ctx,
		resourceGroupName,
		resourceGroupLocation,
		deployment.Tags,
	)
	if err != nil {
		l.Errorf("Failed to create Resource Group - %s: %v", resourceGroupName, err)
		return "", "", fmt.Errorf("failed to create resource group: %w", err)
	}

	l.Debugf("Created Resource Group - %s", resourceGroupName)

	return resourceGroupName, resourceGroupLocation, nil
}
