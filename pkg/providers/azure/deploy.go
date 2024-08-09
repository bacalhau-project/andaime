package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"

	internal_azure "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"

	"github.com/olekukonko/tablewriter"
)

var (
	globalDeployment *models.Deployment
	deploymentMutex  sync.RWMutex
)

func GetGlobalDeployment() *models.Deployment {
	deploymentMutex.RLock()
	defer deploymentMutex.RUnlock()
	return globalDeployment
}

func SetGlobalDeployment(deployment *models.Deployment) {
	deploymentMutex.Lock()
	globalDeployment = deployment
	deploymentMutex.Unlock()
}

func UpdateGlobalDeployment(updateFunc func(*models.Deployment)) {
	deploymentMutex.Lock()
	defer deploymentMutex.Unlock()
	if globalDeployment == nil {
		globalDeployment = &models.Deployment{}
	}
	updateFunc(globalDeployment)
}

func UpdateGlobalDeploymentKeyValue(key string, value interface{}) {
	deploymentMutex.Lock()
	defer deploymentMutex.Unlock()
	if globalDeployment == nil {
		globalDeployment = &models.Deployment{}
	}
	reflect.ValueOf(globalDeployment).Elem().FieldByName(key).Set(reflect.ValueOf(value))
}

const WaitForIPAddressesTimeout = 20 * time.Second
const WaitForResourcesTimeout = 2 * time.Minute
const WaitForResourcesTicker = 5 * time.Second
const maximumSimultaneousDeployments = 5

const DefaultDiskSize = 30

// DeployResources deploys Azure resources based on the provided configuration.
// Config should be the Azure subsection of the viper config.
func (p *AzureProvider) DeployResources(ctx context.Context) error {
	l := logger.Get()

	// Set the start time for the deployment
	UpdateGlobalDeploymentKeyValue("StartTime", time.Now())

	// Ensure we have a deployment object
	deployment := GetGlobalDeployment()
	if deployment == nil {
		return fmt.Errorf("global deployment object is not initialized")
	}

	// Ensure we have a location set
	if deployment.ResourceGroupLocation == "" {
		if len(deployment.Machines) > 0 {
			deployment.ResourceGroupLocation = deployment.Machines[0].Location
		} else {
			return fmt.Errorf("no resource group location specified and no machines to infer from")
		}
	}

	// Prepare resource group
	resourceGroupName, resourceGroupLocation, err := p.PrepareResourceGroup(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to prepare resource group: %v", err))
		return fmt.Errorf("failed to prepare resource group: %v", err)
	}

	UpdateGlobalDeploymentKeyValue("ResourceGroupName", resourceGroupName)
	UpdateGlobalDeploymentKeyValue("ResourceGroupLocation", resourceGroupLocation)

	if err := deployment.UpdateViperConfig(); err != nil {
		l.Error(fmt.Sprintf("Failed to update viper config: %v", err))
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	if err := p.DeployARMTemplate(ctx); err != nil {
		l.Error(fmt.Sprintf("Failed to deploy ARM template: %v", err))
		return err
	}

	if err := GetGlobalDeployment().UpdateViperConfig(); err != nil {
		l.Error(fmt.Sprintf("Failed to update viper config: %v", err))
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	if err := p.FinalizeDeployment(ctx); err != nil {
		l.Error(fmt.Sprintf("Failed to finalize deployment: %v", err))
		return err
	}

	return nil
}
func (p *AzureProvider) DeployARMTemplate(ctx context.Context) error {
	l := logger.Get()
	disp := display.GetCurrentDisplay()
	sm := GetGlobalStateMachine()
	l.Debugf("Deploying template for deployment: %v", GetGlobalDeployment())

	tags := utils.EnsureAzureTags(
		GetGlobalDeployment().Tags,
		GetGlobalDeployment().ProjectID,
		GetGlobalDeployment().UniqueID,
	)

	// Create wait group
	wg := sync.WaitGroup{}

	// Run maximum 5 deployments at a time
	sem := make(chan struct{}, maximumSimultaneousDeployments)

	for _, machine := range GetGlobalDeployment().Machines {
		internalMachine := machine
		sem <- struct{}{}
		wg.Add(1)

		go func(goRoutineMachine *models.Machine) {
			defer func() {
				<-sem
				wg.Done()
			}()

			err := p.deployMachine(ctx, goRoutineMachine, tags, disp, sm)
			if err != nil {
				l.Errorf("Failed to deploy machine %s: %v", goRoutineMachine.ID, err)
				sm.UpdateStatus("VM", goRoutineMachine.ID, StateFailed)
			}
		}(&internalMachine)
	}

	// Wait for all deployments to complete
	wg.Wait()

	return nil
}

func (p *AzureProvider) deployMachine(
	ctx context.Context,
	machine *models.Machine,
	tags map[string]*string,
	disp *display.Display,
	stateMachine *AzureStateMachine,
) error {
	stateMachine.UpdateStatus("VM", machine.ID, StateProvisioning)

	params := p.prepareDeploymentParams(machine)
	vmTemplate, err := p.getAndPrepareTemplate()
	if err != nil {
		return err
	}

	err = p.deployTemplateWithRetry(
		ctx,
		machine,
		vmTemplate,
		params,
		tags,
	)
	if err != nil {
		return err
	}

	return nil
}

func (p *AzureProvider) getAndPrepareTemplate() (map[string]interface{}, error) {
	vmTemplate, err := internal_azure.GetARMTemplate()
	if err != nil {
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	var vmTemplateMap map[string]interface{}
	err = json.Unmarshal(vmTemplate, &vmTemplateMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert struct to map: %w", err)
	}

	return vmTemplateMap, nil
}

func (p *AzureProvider) prepareDeploymentParams(
	machine *models.Machine,
) map[string]interface{} {
	deployment := GetGlobalDeployment()
	return map[string]interface{}{
		"vmName":             fmt.Sprintf("%s-vm", machine.ID),
		"adminUsername":      "azureuser",
		"authenticationType": "sshPublicKey",
		"adminPasswordOrKey": deployment.SSHPublicKeyMaterial,
		"dnsLabelPrefix": fmt.Sprintf(
			"vm-%s-%s",
			strings.ToLower(machine.ID),
			utils.GenerateUniqueID()[:6],
		),
		"ubuntuOSVersion":          "Ubuntu-2004",
		"vmSize":                   machine.VMSize,
		"virtualNetworkName":       fmt.Sprintf("%s-vnet", machine.Location),
		"subnetName":               fmt.Sprintf("%s-subnet", machine.Location),
		"networkSecurityGroupName": fmt.Sprintf("%s-nsg", machine.Location),
		"location":                 machine.Location,
		"securityType":             "TrustedLaunch",
	}
}

func (p *AzureProvider) deployTemplateWithRetry(
	ctx context.Context,
	machine *models.Machine,
	vmTemplate map[string]interface{},
	params map[string]interface{},
	tags map[string]*string,
) error {
	l := logger.Get()
	sm := GetGlobalStateMachine()
	maxRetries := 3
	deployment := GetGlobalDeployment()

	for retry := 0; retry < maxRetries; retry++ {
		_, err := p.Client.DeployTemplate(
			ctx,
			deployment.ResourceGroupName,
			fmt.Sprintf("deployment-vm-%s", machine.ID),
			vmTemplate,
			params,
			tags,
		)

		if strings.Contains(err.Error(), "DnsRecordCreateConflict") {
			l.Warnf(
				"DNS conflict occurred, retrying with a new DNS label prefix (attempt %d of %d)",
				retry+1,
				maxRetries,
			)
			params["dnsLabelPrefix"] = map[string]interface{}{
				"Value": fmt.Sprintf(
					"vm-%s-%s",
					strings.ToLower(machine.ID),
					utils.GenerateUniqueID()[:6],
				),
			}
		} else {
			sm.UpdateStatus("VM", machine.ID, StateFailed)
			return fmt.Errorf("failed to start template deployment: %w", err)
		}
	}

	sm.UpdateStatus("VM", machine.ID, StateFailed)
	return fmt.Errorf("failed to start template deployment after %d retries", maxRetries)
}

func (p *AzureProvider) PollAndUpdateResources(
	ctx context.Context,
	deployment *models.Deployment,
	stateMachine *AzureStateMachine,
) error {
	resources, err := p.ListAllResourcesInSubscription(
		ctx,
		deployment.SubscriptionID,
		deployment.Tags,
	)
	if err != nil {
		return fmt.Errorf("failed to list resources: %w", err)
	}

	for resource := range resources.GetAllResources() {
		// Get unique resource key

		switch r := resource.Resource.(type) {
		case armcompute.VirtualMachine:
			stateMachine.UpdateStatus("VM", *r.Name, ResourceState(*r.Properties.ProvisioningState))
		case armcompute.VirtualMachineExtension:
			stateMachine.UpdateStatus("VMExtension", *r.Name, ResourceState(*r.Properties.ProvisioningState))
		case armnetwork.PublicIPAddress:
			stateMachine.UpdateStatus("PublicIP", *r.Name, ResourceState(*r.Properties.ProvisioningState))
		case armnetwork.Interface:
			stateMachine.UpdateStatus("NIC", *r.Name, ResourceState(*r.Properties.ProvisioningState))
		case armnetwork.SecurityGroup:
			stateMachine.UpdateStatus("NSG", *r.Name, ResourceState(*r.Properties.ProvisioningState))
		case armnetwork.VirtualNetwork:
			stateMachine.UpdateStatus("VNet", *r.Name, ResourceState(*r.Properties.ProvisioningState))
		}
	}

	return nil
}

func (p *AzureProvider) updateDeploymentStatus(
	deployment *models.Deployment,
	resources AzureResources,
) {
	l := logger.Get()
	for res := range resources.GetAllResources() {
		switch strings.ToLower(res.Type) {
		case "microsoft.compute/virtualmachines":
			if vm, ok := res.Resource.(armcompute.VirtualMachine); ok {
				p.updateVMStatus(deployment, &vm)
			}
		case "microsoft.compute/virtualmachines/extensions":
			if vmExt, ok := res.Resource.(armcompute.VirtualMachineExtension); ok {
				p.updateVMExtensionsStatus(
					deployment,
					&vmExt,
				)
			}
		case "microsoft.network/publicipaddresses":
			if publicIP, ok := res.Resource.(armnetwork.PublicIPAddress); ok {
				p.updatePublicIPStatus(deployment, &publicIP)
			}
		case "microsoft.network/networkinterfaces":
			if nic, ok := res.Resource.(armnetwork.Interface); ok {
				p.updateNICStatus(deployment, &nic)
			}
		case "microsoft.network/networksecuritygroups":
			if nsg, ok := res.Resource.(armnetwork.SecurityGroup); ok {
				p.updateNSGStatus(deployment, &nsg)
			}
		case "microsoft.network/virtualnetworks":
			if vnet, ok := res.Resource.(armnetwork.VirtualNetwork); ok {
				p.updateVNetStatus(deployment, &vnet)
			}
		case "microsoft.compute/disks":
			if disk, ok := res.Resource.(armcompute.Disk); ok {
				p.updateDiskStatus(deployment, &disk)
			}
		default:
			l.Debugf(
				"Unhandled resource type: %v (reason: resource type not recognized)",
				res.Type,
			)
		}

	}
	// Update the Viper config after processing all resources
	if err := deployment.UpdateViperConfig(); err != nil {
		logger.Get().Errorf("Failed to update Viper config: %v", err)
	}
}

func (p *AzureProvider) updateVMStatus(
	deployment *models.Deployment,
	resource *armcompute.VirtualMachine,
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
			if resource.Properties != nil {
				deployment.Machines[i].Status = *resource.Properties.ProvisioningState
			}
			break
		}
	}
}

func (p *AzureProvider) updateVMExtensionsStatus(
	deployment *models.Deployment,
	resource *armcompute.VirtualMachineExtension,
) {
	l := logger.Get()
	l.Debugf("Updating VM extensions status for resource: %s", *resource.Name)

	if resource.Properties == nil {
		l.Warn("Resource properties are nil, cannot update VM extension status")
		return
	}

	provisioningState := resource.Properties.ProvisioningState

	// Extract VM name from the resource name
	parts := strings.Split(*resource.Name, "/")
	if len(parts) < 2 {
		l.Warn("Invalid resource name format")
		return
	}
	vmName := parts[0]

	extensionName := strings.Join(parts[1:], "/")
	status := models.GetStatusCode(models.StatusString(*provisioningState))

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
	resource *armnetwork.PublicIPAddress,
) {
	// Assuming the public IP resource name contains the VM name
	for i, machine := range deployment.Machines {
		if strings.Contains(*resource.Name, machine.ID) && resource.Properties != nil {
			if resource.Properties.IPAddress != nil {
				deployment.Machines[i].PublicIP = *resource.Properties.IPAddress
			} else {
				deployment.Machines[i].PublicIP = "SUCCEEDED - Getting..."
			}
			break
		}
	}
}

func (p *AzureProvider) updateNICStatus(
	deployment *models.Deployment,
	resource *armnetwork.Interface,
) {
	l := logger.Get()
	l.Debugf("Updating NIC status for resource: %s", *resource.Name)

	for _, ipConfig := range resource.Properties.IPConfigurations {
		if ipConfig.Properties.PrivateIPAddress == nil {
			l.Warnf(
				"Failed to get privateIPAddress from IP configuration for resource: %s",
				*resource.Name,
			)
			continue
		}

		privateIPAddress := ipConfig.Properties.PrivateIPAddress
		if privateIPAddress == nil {
			l.Warnf(
				"Failed to get privateIPAddress from IP configuration for resource: %s",
				*resource.Name,
			)
			continue
		}

		machineUpdated := false
		for i, machine := range deployment.Machines {
			if strings.Contains(*resource.Name, machine.ID) {
				deployment.Machines[i].PrivateIP = *privateIPAddress
				l.Infof("Updated private IP for machine %s: %s", machine.ID, *privateIPAddress)
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
	resource *armnetwork.SecurityGroup,
) {
	l := logger.Get()
	disp := display.GetGlobalDisplay()
	if resource == nil || resource.Name == nil || resource.Location == nil {
		l.Warn("Resource, resource name, or resource location is nil, cannot update NSG status")
		return
	}
	l.Debugf("Updating NSG status for resource: %s", *resource.Name)

	// Initialize the NetworkSecurityGroups map if it's nil
	if deployment.NetworkSecurityGroups == nil {
		deployment.NetworkSecurityGroups = make(map[string]*armnetwork.SecurityGroup)
		l.Debugf("Initialized NetworkSecurityGroups map")
	}

	// Create a deep copy of the resource
	copiedResource := *resource
	deployment.NetworkSecurityGroups[*resource.Name] = &copiedResource

	for _, machine := range deployment.Machines {
		if machine.Location == *resource.Location {
			disp.UpdateStatus(
				&models.Status{
					ID:     *resource.Name,
					Type:   "NSG",
					Status: "SUCCEEDED",
				},
			)
		}
	}

	l.Debugf(
		"Updated NetworkSecurityGroups map. Current size: %d",
		len(deployment.NetworkSecurityGroups),
	)
}

func (p *AzureProvider) updateVNetStatus(
	deployment *models.Deployment,
	resource *armnetwork.VirtualNetwork,
) {
	l := logger.Get()
	l.Debugf("Updating VNet status for resource: %s", *resource.Name)

	if resource.Properties == nil {
		l.Warnf("Resource properties are nil for VNet: %s", *resource.Name)
		return
	}

	for _, subnet := range resource.Properties.Subnets {
		if deployment.Subnets == nil {
			deployment.Subnets = make(map[string][]*armnetwork.Subnet)
		}

		deployment.Subnets[*resource.Name] = append(
			deployment.Subnets[*resource.Name],
			&armnetwork.Subnet{
				Name: subnet.Name,
				ID:   subnet.ID,
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix: subnet.Properties.AddressPrefix,
				},
			},
		)
	}

	l.Infof("Updated VNet status for: %s", *resource.Name)
}

func (p *AzureProvider) updateDiskStatus(
	deployment *models.Deployment,
	resource *armcompute.Disk,
) {
	l := logger.Get()

	if resource == nil {
		l.Warn("Resource is nil, cannot update Disk status")
		return
	}

	if resource.Name == nil {
		l.Warn("Resource name is nil, cannot update Disk status")
		return
	}

	l.Debugf("Updating Disk status for resource: %s", *resource.Name)

	if resource.Properties == nil {
		l.Warnf("Resource properties are nil for Disk: %s", *resource.Name)
		return
	}

	// Initialize Disks map if it's nil
	if deployment.Disks == nil {
		deployment.Disks = make(map[string]*models.Disk)
	}

	disk := &models.Disk{
		Name: *resource.Name,
	}

	if resource.ID != nil {
		disk.ID = *resource.ID
	}

	if resource.Properties.DiskSizeGB != nil {
		disk.SizeGB = *resource.Properties.DiskSizeGB
	}

	if resource.Properties.DiskState != nil {
		disk.State = *resource.Properties.DiskState
	}

	// Only add the disk to the map if it has valid properties
	if disk.ID != "" || disk.SizeGB != 0 || disk.State != "" {
		deployment.Disks[*resource.Name] = disk
	}

	l.Infof("Updated Disk status for: %s", *resource.Name)
}

// finalizeDeployment performs any necessary cleanup and final steps
func (p *AzureProvider) FinalizeDeployment(
	ctx context.Context,
) error {
	l := logger.Get()
	deployment := GetGlobalDeployment()
	disp := display.GetCurrentDisplay()

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
		[]string{
			"ID",
			"Type",
			"Location",
			"Status",
			"Public IP",
			"Private IP",
			"Instance ID",
			"Elapsed Time (s)",
		},
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
func (p *AzureProvider) PrepareResourceGroup(ctx context.Context) (string, string, error) {
	l := logger.Get()
	deployment := GetGlobalDeployment()

	if deployment == nil {
		return "", "", fmt.Errorf("global deployment object is not initialized")
	}

	// Check if the resource group name already contains a timestamp
	if deployment.ResourceGroupName == "" {
		deployment.ResourceGroupName = "andaime-rg"
	}
	resourceGroupName := deployment.ResourceGroupName + "-" + time.Now().Format("20060102150405")
	resourceGroupLocation := deployment.ResourceGroupLocation

	// If ResourceGroupLocation is not set, use the first location from the Machines
	if resourceGroupLocation == "" {
		if len(deployment.Machines) > 0 {
			resourceGroupLocation = deployment.Machines[0].Location
		}
		if resourceGroupLocation == "" {
			return "", "", fmt.Errorf("resource group location is not set and couldn't be inferred from machines")
		}
	}

	l.Debugf("Creating Resource Group - %s in location %s", resourceGroupName, resourceGroupLocation)

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

	l.Debugf("Created Resource Group - %s in location %s", resourceGroupName, resourceGroupLocation)

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
