package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	internal "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/sanity-io/litter"
)

const WaitForIPAddressesTimeout = 20 * time.Second
const WaitForResourcesTimeout = 2 * time.Minute
const WaitForResourcesTicker = 5 * time.Second
const maximumSimultaneousDeployments = 5

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

func (p *AzureProvider) DeployARMTemplate(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	l := logger.Get()
	l.Debugf("Deploying template for deployment: %v", deployment)

	tags := utils.EnsureAzureTags(deployment.Tags, deployment.ProjectID, deployment.UniqueID)

	// Create wait group
	wg := sync.WaitGroup{}

	// Run maximum 5 deployments at a time
	sem := make(chan struct{}, maximumSimultaneousDeployments)

	// Start a goroutine to continuously probe the resource group
	go p.probeResourceGroup(ctx, deployment)

	for _, machine := range deployment.Machines {
		internalMachine := machine
		sem <- struct{}{}
		wg.Add(1)

		go func(goRoutineMachine *models.Machine) {
			defer func() {
				<-sem
				wg.Done()
			}()

			// Prepare deployment parameters
			params := map[string]interface{}{
				"vmName":             goRoutineMachine.ID,
				"adminUsername":      "azureuser",
				"authenticationType": "sshPublicKey", // Always set to sshPublicKey
				"adminPasswordOrKey": deployment.SSHPublicKeyMaterial,
				"dnsLabelPrefix": fmt.Sprintf(
					"vm-%s",
					strings.ToLower(deployment.Machines[0].ID),
				),
				"ubuntuOSVersion":          "Ubuntu-2004",
				"vmSize":                   deployment.Machines[0].VMSize,
				"virtualNetworkName":       fmt.Sprintf("%s-vnet", goRoutineMachine.Location),
				"subnetName":               fmt.Sprintf("%s-subnet", goRoutineMachine.Location),
				"networkSecurityGroupName": fmt.Sprintf("%s-nsg", goRoutineMachine.Location),
				"location":                 goRoutineMachine.Location,
				"securityType":             "TrustedLaunch",
			}

			// Prepare the virtual machine template
			wrappedParameters := map[string]interface{}{}
			for k, v := range params {
				wrappedParameters[k] = map[string]interface{}{
					"Value": v,
				}
			}

			vmTemplate, err := internal.GetARMTemplate()
			if err != nil {
				l.Errorf("Failed to get template: %v", err)
				return
			}

			paramsMap, err := utils.StructToMap(wrappedParameters)
			if err != nil {
				l.Errorf("Failed to convert struct to map: %v", err)
				return
			}

			// Convert the template to a map
			var vmTemplateMap map[string]interface{}
			err = json.Unmarshal(vmTemplate, &vmTemplateMap)
			if err != nil {
				l.Errorf("Failed to convert struct to map: %v", err)
				return
			}

			// Start the deployment
			future, err := p.Client.DeployTemplate(
				ctx,
				deployment.ResourceGroupName,
				fmt.Sprintf("deployment-vm-%s", goRoutineMachine.ID),
				vmTemplateMap,
				paramsMap,
				tags,
			)
			if err != nil {
				l.Errorf("Failed to start template deployment: %v", err)
				return
			}

			// Poll the deployment status
			pollInterval := 1 * time.Second
			for {
				select {
				case <-ctx.Done():
					l.Info("Deployment cancelled")
					return
				default:
					status, err := future.Poll(ctx)
					if err != nil {
						if isQuotaExceededError(err) {
							l.Errorf(`Azure quota exceeded: %v. Please contact Azure support
 to increase your quota for PublicIpAddress resources`, err)
							break
						}
						l.Errorf("Error polling deployment status: %v", err)
						continue
					}
					statusBytes, err := io.ReadAll(status.Body)
					if err != nil {
						l.Errorf("Error reading deployment status: %v", err)
						break
					}

					var statusObject map[string]interface{}
					err = json.Unmarshal(statusBytes, &statusObject)
					if err != nil {
						l.Errorf("Error unmarshalling deployment status: %v", err)
					}
					l.Debugf("Deployment status: %s", status.Status)

					if status.Status == "Succeeded" {
						l.Info("Deployment completed successfully")
						break
					}

					str := litter.Sdump(statusObject)
					l.Debugf("Deployment status: %s", str)

					time.Sleep(pollInterval)
				}
			}
		}(&internalMachine)
	}

	// Wait for all deployments to complete
	wg.Wait()

	return nil
}

func (p *AzureProvider) probeResourceGroup(ctx context.Context, deployment *models.Deployment) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			resources, err := p.SearchResources(ctx,
				deployment.ResourceGroupName,
				deployment.SubscriptionID,
				deployment.Tags)
			if err != nil {
				logger.Get().Errorf("Failed to list resources in group: %v", err)
				continue
			}

			p.updateDeploymentStatus(deployment, resources)
		}
	}
}

func (p *AzureProvider) updateDeploymentStatus(
	deployment *models.Deployment,
	resources []*armresources.GenericResource,
) {
	l := logger.Get()
	for _, resource := range resources {
		switch strings.ToLower(*resource.Type) {
		case "microsoft.compute/virtualmachines":
			p.updateVMStatus(deployment, resource)
		case "microsoft.compute/virtualmachines/extensions":
			p.updateVMExtensionsStatus(deployment, resource)
		case "microsoft.network/publicipaddresses":
			p.updatePublicIPStatus(deployment, resource)
		case "microsoft.network/networkinterfaces":
			p.updateNICStatus(deployment, resource)
		case "microsoft.network/networksecuritygroups":
			p.updateNSGStatus(deployment, resource)
		case "microsoft.network/virtualnetworks":
			p.updateVNetStatus(deployment, resource)
		case "microsoft.compute/disks":
			p.updateDiskStatus(deployment, resource)
		default:
			l.Debugf("Unhandled resource type: %v (reason: resource type not recognized)", *resource.Type)
		}

	}
	// Update the Viper config after processing all resources
	if err := deployment.UpdateViperConfig(); err != nil {
		logger.Get().Errorf("Failed to update Viper config: %v", err)
	}
}

func (p *AzureProvider) updateVMStatus(
	deployment *models.Deployment,
	resource *armresources.GenericResource,
) {
	for i, machine := range deployment.Machines {
		if machine.ID == *resource.Name {
			// Test to see if resource.Properties is a slice
			if resourceProperties, ok := resource.Properties.(map[string]interface{}); ok {
				deployment.Machines[i].Status = resourceProperties["provisioningState"].(string)
			}
			break
		}
	}
}

func (p *AzureProvider) updateVMExtensionsStatus(
	deployment *models.Deployment,
	resource *armresources.GenericResource,
) {
	l := logger.Get()
	l.Debugf("Updating VM extensions status for resource: %s", *resource.Name)

	if resource.Properties == nil {
		l.Warn("Resource properties are nil, cannot update VM extension status")
		return
	}

	properties, ok := resource.Properties.(map[string]interface{})
	if !ok {
		l.Warn("Failed to cast resource properties to map[string]interface{}")
		return
	}

	provisioningState, ok := properties["provisioningState"].(string)
	if !ok {
		l.Warn("Failed to get provisioningState from properties")
		return
	}

	extensionName := strings.TrimPrefix(*resource.Name, deployment.VMName+"/")
	status := models.GetStatusCode(models.StatusString(provisioningState))
	deployment.VMExtensionsStatus[extensionName] = status

	l.Debugf("Updated VM extension status: %s = %s", extensionName, status)
}

func (p *AzureProvider) updatePublicIPStatus(
	deployment *models.Deployment,
	resource *armresources.GenericResource,
) {
	// Assuming the public IP resource name contains the VM name
	for i, machine := range deployment.Machines {
		if strings.Contains(*resource.Name, machine.ID) {
			if resourceProperties, ok := resource.Properties.(map[string]interface{}); ok {
				deployment.Machines[i].PublicIP = resourceProperties["ipAddress"].(string)
			}
			break
		}
	}
}

func (p *AzureProvider) updateNICStatus(
	deployment *models.Deployment,
	resource *armresources.GenericResource,
) {
	l := logger.Get()
	l.Debugf("Updating NIC status for resource: %s", *resource.Name)
	l.Debugf("NOT IMPLEMENTED: %s", litter.Sdump(resource))
}

func (p *AzureProvider) updateNSGStatus(
	deployment *models.Deployment,
	resource *armresources.GenericResource,
) {
	l := logger.Get()
	l.Debugf("Updating NSG status for resource: %s", *resource.Name)
	l.Debugf("NOT IMPLEMENTED: %s", litter.Sdump(resource))
}

func (p *AzureProvider) updateVNetStatus(
	deployment *models.Deployment,
	resource *armresources.GenericResource,
) {
	l := logger.Get()
	l.Debugf("Updating VNet status for resource: %s", *resource.Name)
	l.Debugf("NOT IMPLEMENTED: %s", litter.Sdump(resource))
}

func (p *AzureProvider) updateDiskStatus(
	deployment *models.Deployment,
	resource *armresources.GenericResource,
) {
	l := logger.Get()
	l.Debugf("Updating Disk status for resource: %s", *resource.Name)
	l.Debugf("NOT IMPLEMENTED: %s", litter.Sdump(resource))
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
