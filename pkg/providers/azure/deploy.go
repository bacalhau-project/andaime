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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
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

	l := logger.Get()
	for {
		select {
		case <-ctx.Done():
			l.Info("Context cancelled, stopping resource group probe")
			return
		case <-ticker.C:
			l.Debug("Probing resource group")
			resources, err := p.SearchResources(ctx,
				deployment.ResourceGroupName,
				deployment.SubscriptionID,
				deployment.Tags)
			if err != nil {
				l.Errorf("Failed to list resources in group: %v", err)
				continue
			}

			l.Debugf("Found %d resources in the resource group", len(resources))
			p.updateDeploymentStatus(deployment, resources)
			l.Debug("Finished updating deployment status")
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
			// Set the type to "VM"
			deployment.Machines[i].Type = "VM"
			
			// Set the location
			if resource.Location != nil {
				deployment.Machines[i].Location = *resource.Location
			}

			// Update the status
			if resourceProperties, ok := resource.Properties.(map[string]interface{}); ok {
				if provisioningState, ok := resourceProperties["provisioningState"].(string); ok {
					deployment.Machines[i].Status = provisioningState
				}
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

	// Extract VM name from the resource name
	parts := strings.Split(*resource.Name, "/")
	if len(parts) < 2 {
		l.Warn("Invalid resource name format")
		return
	}
	vmName := parts[0]

	extensionName := strings.Join(parts[1:], "/")
	status := models.GetStatusCode(models.StatusString(provisioningState))

	// Initialize VMExtensionsStatus if it's nil
	if deployment.VMExtensionsStatus == nil {
		deployment.VMExtensionsStatus = make(map[string]models.StatusCode)
	}

	// Use the VM name and extension name as the key
	key := fmt.Sprintf("%s/%s", vmName, extensionName)
	deployment.VMExtensionsStatus[key] = status

	l.Debugf("Updated VM extension status: %s = %s", key, status)
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

	if resource.Properties == nil {
		l.Warn("Resource properties are nil, cannot update NIC status")
		return
	}

	properties, ok := resource.Properties.(map[string]interface{})
	if !ok {
		l.Warnf("Failed to cast resource properties to map[string]interface{} for resource: %s", *resource.Name)
		return
	}

	ipConfigurations, ok := properties["ipConfigurations"].([]interface{})
	if !ok || len(ipConfigurations) == 0 {
		l.Warnf("No IP configurations found in NIC properties for resource: %s", *resource.Name)
		return
	}

	for _, ipConfig := range ipConfigurations {
		ipConfigMap, ok := ipConfig.(map[string]interface{})
		if !ok {
			l.Warnf("Failed to cast IP configuration to map[string]interface{} for resource: %s", *resource.Name)
			continue
		}

		privateIPAddress, ok := ipConfigMap["privateIPAddress"].(string)
		if !ok {
			l.Warnf("Failed to get privateIPAddress from IP configuration for resource: %s", *resource.Name)
			continue
		}

		machineUpdated := false
		for i, machine := range deployment.Machines {
			if strings.Contains(*resource.Name, machine.ID) {
				deployment.Machines[i].PrivateIP = privateIPAddress
				l.Infof("Updated private IP for machine %s: %s", machine.ID, privateIPAddress)
				machineUpdated = true
				break
			}
		}

		if !machineUpdated {
			l.Warnf("No matching machine found for NIC resource: %s", *resource.Name)
		}
	}
}

func (p *AzureProvider) updateNSGStatus(
	deployment *models.Deployment,
	resource *armresources.GenericResource,
) {
	l := logger.Get()
	l.Debugf("Updating NSG status for resource: %s (File: deploy.go, Line: 441)", *resource.Name)

	if resource.Properties == nil {
		l.Warnf("Resource properties are nil for NSG: %s (File: deploy.go, Line: 444)", *resource.Name)
		return
	}

	properties, ok := resource.Properties.(map[string]interface{})
	if !ok {
		l.Warnf("Failed to cast resource properties to map[string]interface{} for NSG: %s (File: deploy.go, Line: 449)", *resource.Name)
		l.Debugf("Resource properties: %+v", resource.Properties)
		return
	}

	securityRules, ok := properties["securityRules"].([]interface{})
	if !ok {
		l.Debugf("No security rules found in NSG properties for: %s. This is normal for a new NSG. (File: deploy.go, Line: 455)", *resource.Name)
		l.Debugf("Security rules property: %+v", properties["securityRules"])
		securityRules = []interface{}{}
	}

	// Initialize the NetworkSecurityGroups map if it's nil
	if deployment.NetworkSecurityGroups == nil {
		deployment.NetworkSecurityGroups = make(map[string]*armnetwork.SecurityGroup)
		l.Debugf("Initialized NetworkSecurityGroups map (File: deploy.go, Line: 463)")
	}

	// Create a new SecurityGroup for this NSG
	nsg := &armnetwork.SecurityGroup{
		Name: resource.Name,
		ID:   resource.ID,
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: make([]*armnetwork.SecurityRule, 0, len(securityRules)+len(deployment.AllowedPorts)),
		},
	}
	l.Debugf("Created new SecurityGroup for NSG: %s (File: deploy.go, Line: 473)", *resource.Name)

	// Add existing security rules
	for i, rule := range securityRules {
		ruleMap, ok := rule.(map[string]interface{})
		if !ok {
			l.Warnf("Failed to cast security rule to map[string]interface{} for NSG: %s, rule index: %d (File: deploy.go, Line: 479)", *resource.Name, i)
			l.Debugf("Security rule: %+v", rule)
			continue
		}

		securityRule := &armnetwork.SecurityRule{
			Name: utils.ToPtr(ruleMap["name"].(string)),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 (*armnetwork.SecurityRuleProtocol)(utils.ToPtr(fmt.Sprintf("%v", ruleMap["protocol"]))),
				SourcePortRange:          utils.ToPtr(fmt.Sprintf("%v", ruleMap["sourcePortRange"])),
				DestinationPortRange:     utils.ToPtr(fmt.Sprintf("%v", ruleMap["destinationPortRange"])),
				SourceAddressPrefix:      utils.ToPtr(fmt.Sprintf("%v", ruleMap["sourceAddressPrefix"])),
				DestinationAddressPrefix: utils.ToPtr(fmt.Sprintf("%v", ruleMap["destinationAddressPrefix"])),
				Access:                   (*armnetwork.SecurityRuleAccess)(utils.ToPtr(fmt.Sprintf("%v", ruleMap["access"]))),
				Priority:                 utils.ToPtr(int32(ruleMap["priority"].(float64))),
				Direction:                (*armnetwork.SecurityRuleDirection)(utils.ToPtr(fmt.Sprintf("%v", ruleMap["direction"]))),
			},
		}

		nsg.Properties.SecurityRules = append(nsg.Properties.SecurityRules, securityRule)
		l.Debugf("Added existing security rule: %s to NSG: %s (File: deploy.go, Line: 497)", *securityRule.Name, *resource.Name)
	}

	// Add rules for allowed ports
	for i, port := range deployment.AllowedPorts {
		securityRule := &armnetwork.SecurityRule{
			Name: utils.ToPtr(fmt.Sprintf("AllowPort%d", port)),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 (*armnetwork.SecurityRuleProtocol)(utils.ToPtr("Tcp")),
				SourcePortRange:          utils.ToPtr("*"),
				DestinationPortRange:     utils.ToPtr(fmt.Sprintf("%d", port)),
				SourceAddressPrefix:      utils.ToPtr("*"),
				DestinationAddressPrefix: utils.ToPtr("*"),
				Access:                   (*armnetwork.SecurityRuleAccess)(utils.ToPtr("Allow")),
				Priority:                 utils.ToPtr(int32(1000 + i)),
				Direction:                (*armnetwork.SecurityRuleDirection)(utils.ToPtr("Inbound")),
			},
		}

		nsg.Properties.SecurityRules = append(nsg.Properties.SecurityRules, securityRule)
		l.Debugf("Added allowed port security rule: %s to NSG: %s (File: deploy.go, Line: 517)", *securityRule.Name, *resource.Name)
	}

	// Add the NSG to the deployment's NetworkSecurityGroups map
	deployment.NetworkSecurityGroups[*resource.Name] = nsg

	l.Infof("Updated NSG: %s (File: deploy.go, Line: 522)", *resource.Name)
	l.Debugf("NSG details: %+v", nsg)
}

func (p *AzureProvider) updateVNetStatus(
	deployment *models.Deployment,
	resource *armresources.GenericResource,
) {
	l := logger.Get()
	l.Debugf("Updating VNet status for resource: %s", *resource.Name)

	if resource.Properties == nil {
		l.Warnf("Resource properties are nil for VNet: %s", *resource.Name)
		return
	}

	properties, ok := resource.Properties.(map[string]interface{})
	if !ok {
		l.Warnf("Failed to cast resource properties to map[string]interface{} for VNet: %s", *resource.Name)
		return
	}

	subnets, ok := properties["subnets"].([]interface{})
	if !ok {
		l.Warnf("No subnets found in VNet properties for: %s", *resource.Name)
		return
	}

	for _, subnet := range subnets {
		subnetMap, ok := subnet.(map[string]interface{})
		if !ok {
			l.Warnf("Failed to cast subnet to map[string]interface{} for VNet: %s", *resource.Name)
			continue
		}

		subnetName, ok := subnetMap["name"].(string)
		if !ok {
			l.Warnf("Failed to get subnet name for VNet: %s", *resource.Name)
			continue
		}

		subnetID, ok := subnetMap["id"].(string)
		if !ok {
			l.Warnf("Failed to get subnet ID for VNet: %s", *resource.Name)
			continue
		}

		addressPrefix, ok := subnetMap["addressPrefix"].(string)
		if !ok {
			l.Warnf("Failed to get address prefix for subnet: %s in VNet: %s", subnetName, *resource.Name)
			continue
		}

		if deployment.Subnets == nil {
			deployment.Subnets = make(map[string][]*armnetwork.Subnet)
		}

		deployment.Subnets[*resource.Name] = append(deployment.Subnets[*resource.Name], &armnetwork.Subnet{
			Name: utils.ToPtr(subnetName),
			ID:   utils.ToPtr(subnetID),
			Properties: &armnetwork.SubnetPropertiesFormat{
				AddressPrefix: utils.ToPtr(addressPrefix),
			},
		})
	}

	l.Infof("Updated VNet status for: %s", *resource.Name)
}

func (p *AzureProvider) updateDiskStatus(
	deployment *models.Deployment,
	resource *armresources.GenericResource,
) {
	l := logger.Get()
	l.Debugf("Updating Disk status for resource: %s", *resource.Name)

	if resource.Properties == nil {
		l.Warnf("Resource properties are nil for Disk: %s", *resource.Name)
		return
	}

	properties, ok := resource.Properties.(map[string]interface{})
	if !ok {
		l.Warnf("Failed to cast resource properties to map[string]interface{} for Disk: %s", *resource.Name)
		return
	}

	diskSizeGB, ok := properties["diskSizeGB"].(float64)
	if !ok {
		l.Warnf("Failed to get diskSizeGB from properties for Disk: %s", *resource.Name)
		return
	}

	diskState, ok := properties["diskState"].(string)
	if !ok {
		l.Warnf("Failed to get diskState from properties for Disk: %s", *resource.Name)
		return
	}

	// Initialize Disks map if it's nil
	if deployment.Disks == nil {
		deployment.Disks = make(map[string]*models.Disk)
	}

	deployment.Disks[*resource.Name] = &models.Disk{
		Name:  *resource.Name,
		ID:    *resource.ID,
		SizeGB: int(diskSizeGB),
		State: diskState,
	}

	l.Infof("Updated Disk status for: %s", *resource.Name)
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
		[]string{"ID", "Type", "Location", "Status", "Public IP", "Private IP", "Instance ID", "Elapsed Time (s)"},
	)

	startTime := deployment.StartTime
	if startTime.IsZero() {
		startTime = time.Now() // Fallback if start time wasn't set
	}

	for _, machine := range deployment.Machines {
		publicIP := machine.PublicIP
		privateIP := machine.PrivateIP
		if publicIP == "" {
			publicIP = "Pending"
		}
		if privateIP == "" {
			privateIP = "Pending"
		}
		elapsedTime := time.Since(startTime).Seconds()
		table.Append([]string{
			machine.ID,
			machine.Type,
			machine.Location,
			machine.Status,
			publicIP,
			privateIP,
			machine.InstanceID,
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
