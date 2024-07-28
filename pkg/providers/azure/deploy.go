package azure

import (
	"context"
	"fmt"
	"os"
	"strings"
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

	deployment.ResourceGroupName = resourceGroupName
	deployment.ResourceGroupLocation = resourceGroupLocation

	err = deployment.UpdateViperConfig()
	if err != nil {
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	// Deploy Bicep template
	err = p.DeployBicepTemplate(ctx, deployment, disp)
	if err != nil {
		return err
	}

	err = deployment.UpdateViperConfig()
	if err != nil {
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	return p.FinalizeDeployment(ctx, deployment, disp)
}

// DeployBicepTemplate deploys the Bicep template for the deployment
func (p *AzureProvider) DeployBicepTemplate(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	l := logger.Get()

	l.Debugf("Deploying Bicep template for deployment: %v", deployment)

	// Prepare deployment parameters
	params := struct {
		VMName                   string `json:"vmName"`
		AdminUsername            string `json:"adminUsername"`
		AuthenticationType       string `json:"authenticationType"`
		AdminPasswordOrKey       string `json:"adminPasswordOrKey"`
		DNSLabelPrefix           string `json:"dnsLabelPrefix"`
		UbuntuOSVersion          string `json:"ubuntuOSVersion"`
		VMSize                   string `json:"vmSize"`
		VirtualNetworkName       string `json:"virtualNetworkName"`
		SubnetName               string `json:"subnetName"`
		NetworkSecurityGroupName string `json:"networkSecurityGroupName"`
		Location                 string `json:"location"`
		SecurityType             string `json:"securityType"`
	}{
		VMName:                   deployment.Machines[0].ID,
		AdminUsername:            "azureuser",
		AuthenticationType:       "password",
		AdminPasswordOrKey:       "MyP@ssw0rd123!",
		DNSLabelPrefix:           fmt.Sprintf("vm-%s", strings.ToLower(deployment.Machines[0].ID)),
		UbuntuOSVersion:          "Ubuntu-2004",
		VMSize:                   deployment.Machines[0].Parameters[0].Type,
		VirtualNetworkName:       fmt.Sprintf("%s-vnet", deployment.ResourceGroupName),
		SubnetName:               fmt.Sprintf("%s-subnet", deployment.ResourceGroupName),
		NetworkSecurityGroupName: fmt.Sprintf("%s-nsg", deployment.ResourceGroupName),
		Location:                 deployment.Machines[0].Location,
		SecurityType:             "TrustedLaunch",
	}

	// Start the deployment
	future, err := p.Client.DeployTemplate(ctx, deployment.ResourceGroupName, "vm-deployment", params, nil, nil)
	if err != nil {
		l.Errorf("Failed to start Bicep template deployment: %v", err)
		return fmt.Errorf("failed to start Bicep template deployment: %w", err)
	}

	// Poll the deployment status
	pollInterval := 10 * time.Second
	for {
		select {
		case <-ctx.Done():
			l.Info("Deployment cancelled")
			return ctx.Err()
		default:
			status, err := future.PollUntilDone(ctx, nil)
			if err != nil {
				l.Errorf("Error checking deployment status: %v", err)
				return fmt.Errorf("error checking deployment status: %w", err)
			}

			if status.Properties != nil && status.Properties.ProvisioningState != nil {
				state := *status.Properties.ProvisioningState
				if err != nil {
					l.Errorf("Failed to get deployment result: %v", err)
					return fmt.Errorf("failed to get deployment result: %w", err)
				}

				if deploymentResult.Properties != nil && deploymentResult.Properties.ProvisioningState != nil {
					state := *deploymentResult.Properties.ProvisioningState
					l.Infof("Deployment completed with state: %s", state)
					if state == "Succeeded" {
						l.Debugf("Bicep template deployed successfully for deployment: %v", deployment)
						return nil
					} else {
						return fmt.Errorf("deployment failed with state: %s", state)
					}
				}
			}

			l.Debugf("Deployment in progress...")
			disp.UpdateStatus(&models.Status{
				ID:     "bicep-deployment",
				Type:   "Azure",
				Status: "In Progress",
			})

			time.Sleep(pollInterval)
		}
	}
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
			Type:   "VM",
			Status: "Processing",
		})

		// TODO: Implement any necessary post-deployment processing for each machine
		// This might include:
		// 1. Retrieving VM information from Azure
		// 2. Updating the machine struct with actual deployment details
		// 3. Performing any required post-deployment configuration

		// Update status to indicate completion
		disp.UpdateStatus(&models.Status{
			ID:     machine.ID,
			Type:   "VM",
			Status: "Processed",
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
