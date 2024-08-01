package azure

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	internal "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/olekukonko/tablewriter"
)

const WaitForIPAddressesTimeout = 20 * time.Second
const WaitForResourcesTimeout = 2 * time.Minute
const WaitForResourcesTicker = 5 * time.Second

const DefaultDiskSize = 30

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
		return fmt.Errorf("failed to prepare resource group: %v", err)
	}

	deployment.ResourceGroupName = resourceGroupName
	deployment.ResourceGroupLocation = resourceGroupLocation

	err = deployment.UpdateViperConfig()
	if err != nil {
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	err = p.DeployARMTemplate(ctx, deployment, disp)
	if err != nil {
		return err
	}

	err = deployment.UpdateViperConfig()
	if err != nil {
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	return p.FinalizeDeployment(ctx, deployment, disp)
}

// DeployARMTemplate deploys the template for the deployment
func (p *AzureProvider) DeployARMTemplate(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	l := logger.Get()
	l.Debugf("Deploying template for deployment: %v", deployment)

	tags := utils.EnsureAzureTags(deployment.Tags, deployment.ProjectID, deployment.UniqueID)

	// Prepare deployment parameters
	params := map[string]interface{}{
		"vmName":                   deployment.Machines[0].ID,
		"adminUsername":            "azureuser",
		"authenticationType":       "sshPublicKey", // Always set to sshPublicKey
		"adminPasswordOrKey": map[string]interface{}{
			"value": deployment.SSHPublicKeyMaterial,
		},
		"dnsLabelPrefix":           fmt.Sprintf("vm-%s", strings.ToLower(deployment.Machines[0].ID)),
		"ubuntuOSVersion":          "Ubuntu-2004",
		"vmSize":                   deployment.Machines[0].VMSize,
		"virtualNetworkName":       fmt.Sprintf("%s-vnet", deployment.ResourceGroupName),
		"subnetName":               fmt.Sprintf("%s-subnet", deployment.ResourceGroupName),
		"networkSecurityGroupName": fmt.Sprintf("%s-nsg", deployment.ResourceGroupName),
		"location":                 deployment.Machines[0].Location,
		"securityType":             "TrustedLaunch",
	}

	vmTemplate, err := internal.GetARMTemplate()
	if err != nil {
		l.Errorf("Failed to get template: %v", err)
		return fmt.Errorf("failed to get template: %w", err)
	}

	paramsMap, err := utils.StructToMap(params)
	if err != nil {
		l.Errorf("Failed to convert struct to map: %v", err)
		return fmt.Errorf("failed to convert struct to map: %w", err)
	}

	// Start the deployment
	future, err := p.Client.DeployTemplate(
		ctx,
		deployment.ResourceGroupName,
		deployment.ResourceGroupName,
		string(vmTemplate),
		paramsMap,
		tags,
	)
	if err != nil {
		l.Errorf("Failed to start template deployment: %v", err)
		return fmt.Errorf("failed to start template deployment: %w", err)
	}

	// Poll the deployment status
	pollInterval := 10 * time.Second
	for {
		select {
		case <-ctx.Done():
			l.Info("Deployment cancelled")
			return ctx.Err()
		default:
			status, err := future.Poll(ctx)
			if err != nil {
				l.Errorf("Error checking deployment status: %v", err)
				return fmt.Errorf("error checking deployment status: %w", err)
			} else if status.Status == "Succeeded" {
				l.Info("Deployment completed successfully")
				return nil
			}

			l.Debugf("Deployment status: %s", status.Status)

			time.Sleep(pollInterval)
		}
	}
}

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
		if machine.PublicIP != "" {
			publicIP = machine.PublicIP
		}
		if machine.PrivateIP != "" {
			privateIP = machine.PrivateIP
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
func printMachineIPTable(deployment *models.Deployment) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Machine Name", "Public IP"})

	for _, machine := range deployment.Machines {
		ipAddress := machine.PublicIP
		if ipAddress == "" {
			ipAddress = "Pending"
		}
		table.Append([]string{machine.Name, ipAddress})
	}

	if table.NumLines() > 0 {
		fmt.Println("Deployed Machines:")
		table.Render()
	} else {
		fmt.Println("No machines have been deployed yet.")
	}
}
