package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	internal_azure "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/globals"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

const ipRetries = 3
const timeBetweenIPRetries = 10 * time.Second

// DeployResources deploys Azure resources based on the provided configuration.
// Config should be the Azure subsection of the viper config.
func (p *AzureProvider) DeployResources(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModel()

	// Set the start time for the deployment
	m.Deployment.StartTime = time.Now()

	// Ensure we have a location set
	if m.Deployment.ResourceGroupLocation == "" {
		return fmt.Errorf("no resource group location specified")
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		l.Info("Deployment cancelled before starting")
		return ctx.Err()
	default:
	}

	// Prepare resource group
	err := p.PrepareResourceGroup(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to prepare resource group: %v", err))
		return fmt.Errorf("failed to prepare resource group: %v", err)
	}

	if err := m.Deployment.UpdateViperConfig(); err != nil {
		l.Error(fmt.Sprintf("Failed to update viper config: %v", err))
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	if err := p.DeployARMTemplate(ctx); err != nil {
		l.Error(fmt.Sprintf("Failed to deploy ARM template: %v", err))
		return err
	}

	if err := m.Deployment.UpdateViperConfig(); err != nil {
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
	prog := display.GetGlobalProgram()
	m := display.GetGlobalModel()
	// Remove the state machine reference

	l.Debugf("Deploying template for deployment: %v", m.Deployment)

	tags := utils.EnsureAzureTags(
		m.Deployment.Tags,
		m.Deployment.ProjectID,
		m.Deployment.UniqueID,
	)

	// Create wait group
	wg := sync.WaitGroup{}

	// Run maximum 5 deployments at a time
	sem := make(chan struct{}, globals.MaximumSimultaneousDeployments)

	for _, machine := range m.Deployment.Machines {
		internalMachine := machine

		sem <- struct{}{}
		wg.Add(1)

		go func(goRoutineMachine *models.Machine) {
			defer func() {
				<-sem
				wg.Done()
			}()

			err := p.deployMachine(ctx, goRoutineMachine, tags)
			if err != nil {
				l.Errorf("Failed to deploy machine %s: %v", goRoutineMachine.ID, err)
				prog.UpdateStatus(
					&models.Status{
						Name:   goRoutineMachine.Name,
						Type:   models.UpdateStatusResourceTypeVM,
						Status: "Failed",
					},
				)
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
) error {
	prog := display.GetGlobalProgram()
	prog.UpdateStatus(
		&models.Status{
			Name:   machine.Name,
			Type:   models.UpdateStatusResourceTypeVM,
			Status: "Provisioning",
		},
	)

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
	m := display.GetGlobalModel()
	return map[string]interface{}{
		"vmName":             fmt.Sprintf("%s-vm", machine.ID),
		"adminUsername":      "azureuser",
		"authenticationType": "sshPublicKey",
		"adminPasswordOrKey": m.Deployment.SSHPublicKeyMaterial,
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
	maxRetries := 3
	prog := display.GetGlobalProgram()
	m := display.GetGlobalModel()

	machineIndex := -1
	for i, depMachine := range m.Deployment.Machines {
		if depMachine.ID == machine.ID {
			machineIndex = i
			break
		}
	}

	if machineIndex == -1 {
		return fmt.Errorf("machine %s not found in deployment", machine.ID)
	}

	prog.UpdateStatus(
		&models.Status{
			Name:   machine.Name,
			Status: models.CreateStateMessage("VM", models.StatusCreating, machine.Name),
		},
	)

	dnsFailed := false
	for retry := 0; retry < maxRetries; retry++ {
		poller, err := p.Client.DeployTemplate(
			ctx,
			m.Deployment.ResourceGroupName,
			fmt.Sprintf("deployment-%s", machine.Name),
			vmTemplate,
			params,
			tags,
		)
		if err != nil {
			return err
		}

		resp, err := poller.PollUntilDone(ctx, nil)
		l.Debugf("Deployment response: %v", resp)
		if resp.Properties != nil && resp.Properties.ProvisioningState != nil {
			if *resp.Properties.ProvisioningState == "Failed" {
				if resp.Properties.Error != nil && resp.Properties.Error.Message != nil {
					return fmt.Errorf("deployment failed: %s", *resp.Properties.Error.Message)
				}
				return fmt.Errorf("deployment failed with unknown error")
			}
		} else {
			return fmt.Errorf("deployment response or provisioning state is nil")
		}

		if err != nil && strings.Contains(err.Error(), "DnsRecordCreateConflict") {
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
			prog.UpdateStatus(
				&models.Status{
					Name:   machine.Name,
					Type:   models.UpdateStatusResourceTypeVM,
					Status: fmt.Sprintf("DNS Conflict - Retrying... %d/%d", retry+1, maxRetries),
				},
			)
			dnsFailed = true
			continue
		} else if err != nil {
			prog.UpdateStatus(
				&models.Status{
					Name:   machine.Name,
					Type:   models.UpdateStatusResourceTypeVM,
					Status: fmt.Sprintf("Failed: %v", err),
				},
			)
			return fmt.Errorf("error deploying template: %v", err)
		}

		// Finished with no errors
		dnsFailed = false
		break
	}

	if dnsFailed {
		prog.UpdateStatus(
			&models.Status{
				Name:   machine.Name,
				Type:   models.UpdateStatusResourceTypeVM,
				Status: "Failed to deploy due to DNS conflict.",
			},
		)
		m.Deployment.Machines[machineIndex].Status = "Failed to deploy due to DNS conflict"
	} else {
		for i := 0; i < ipRetries; i++ {
			publicIP, privateIP, err := p.GetVMIPAddresses(ctx, m.Deployment.ResourceGroupName, machine.Name)
			if err != nil {
				if i == ipRetries-1 {
					l.Errorf("Failed to get IP addresses for VM %s after %d retries: %v", machine.Name, ipRetries, err)
					m.Deployment.Machines[machineIndex].Status = "Failed to get IP addresses"
					return fmt.Errorf("failed to get IP addresses for VM %s: %v", machine.Name, err)
				}
				time.Sleep(timeBetweenIPRetries)
				prog.UpdateStatus(
					&models.Status{
						Name:      machine.Name,
						Type:      models.UpdateStatusResourceTypeVM,
						Status:    "Waiting for IP addresses",
						PublicIP:  fmt.Sprintf("Retry: %d/%d", i+1, ipRetries),
						PrivateIP: fmt.Sprintf("Retry: %d/%d", i+1, ipRetries),
					},
				)
				continue
			}

			m.Deployment.Machines[machineIndex].PublicIP = publicIP
			m.Deployment.Machines[machineIndex].PrivateIP = privateIP
			m.Deployment.Machines[machineIndex].Status = "Testing SSH"
			if m.Deployment.Machines[machineIndex].ElapsedTime == 0 {
				m.Deployment.Machines[machineIndex].ElapsedTime = time.Since(machine.StartTime)
			}
			prog.UpdateStatus(
				&models.Status{
					ID:          machine.Name,
					Type:        models.UpdateStatusResourceTypeVM,
					Status:      "Testing SSH",
					PublicIP:    publicIP,
					PrivateIP:   privateIP,
					ElapsedTime: m.Deployment.Machines[machineIndex].ElapsedTime,
				},
			)

			// Test SSH connectivity
			sshConfig := &sshutils.SSHConfig{
				Host:               publicIP,
				User:               "azureuser",
				PrivateKeyMaterial: m.Deployment.SSHPrivateKeyMaterial,
				Port:               22,
			}

			sshWaiter := sshutils.NewSSHWaiter(nil)
			sshErr := sshWaiter.WaitForSSH(sshConfig)

			if sshErr != nil {
				m.Deployment.Machines[machineIndex].Status = "Failed"
				m.Deployment.Machines[machineIndex].SSH = "❌"
				prog.UpdateStatus(
					&models.Status{
						ID:     machine.Name,
						Type:   models.UpdateStatusResourceTypeVM,
						Status: "Failed",
					},
				)
			} else {
				m.Deployment.Machines[machineIndex].Status = "Successfully Deployed"
				m.Deployment.Machines[machineIndex].SSH = "✅"
				prog.UpdateStatus(
					&models.Status{
						ID:     machine.Name,
						Type:   models.UpdateStatusResourceTypeVM,
						Status: "Successfully Deployed",
					},
				)
			}
			break
		}
	}

	return nil
}

func (p *AzureProvider) PollAndUpdateResources(ctx context.Context) ([]interface{}, error) {
	m := display.GetGlobalModel()
	resources, err := p.Client.GetResources(
		ctx,
		m.Deployment.ResourceGroupName,
	)
	if err != nil {
		return nil, err
	}

	return resources, nil
}

// This function is already defined in azure_state_machine.go, so we can remove it from here

// finalizeDeployment performs any necessary cleanup and final steps
func (p *AzureProvider) FinalizeDeployment(
	ctx context.Context,
) error {
	l := logger.Get()
	m := display.GetGlobalModel()

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
		m.Deployment.ResourceGroupName,
	)
	summaryMsg += fmt.Sprintf("Location: %s\n", m.Deployment.ResourceGroupLocation)
	l.Info(summaryMsg)

	startTime := m.Deployment.StartTime
	if startTime.IsZero() {
		startTime = time.Now() // Fallback if start time wasn't set
	}

	// Use the existing View method to generate the table
	tableOutput := m.View()

	fmt.Println("\nDeployment completed. Full list of deployed machines:")
	fmt.Print(tableOutput)

	// Ensure all configurations are saved
	if err := m.Deployment.UpdateViperConfig(); err != nil {
		l.Errorf("Failed to save final configuration: %v", err)
		return fmt.Errorf("failed to save final configuration: %w", err)
	}

	l.Info("Deployment finalized successfully")

	return nil
}

// PrepareResourceGroup prepares or creates a resource group for the Azure deployment.
// It ensures that a valid resource group name and location are set, creating them if necessary.
//
// Parameters:
//   - ctx: The context.Context for the operation, used for cancellation and timeout.
//
// Returns:
//   - error: An error if the resource group preparation fails, nil otherwise.
//
// The function performs the following steps:
// 1. Retrieves the global deployment object.
// 2. Ensures a resource group name is set, appending a timestamp if necessary.
// 3. Determines the resource group location, using the first machine's location if not explicitly set.
// 4. Creates or retrieves the resource group using the Azure client.
// 5. Updates the global deployment object with the finalized resource group information.
func (p *AzureProvider) PrepareResourceGroup(ctx context.Context) error {
	l := logger.Get()
	prog := display.GetGlobalProgram()
	m := display.GetGlobalModel()

	if m.Deployment == nil {
		return fmt.Errorf("global deployment object is not initialized")
	}

	// Check if the resource group name already contains a timestamp
	if m.Deployment.ResourceGroupName == "" {
		m.Deployment.ResourceGroupName = "andaime-rg"
	}
	newRGName := m.Deployment.ResourceGroupName + "-" + time.Now().Format("20060102150405")
	m.Deployment.ResourceGroupName = newRGName

	var resourceGroupLocation string
	// If ResourceGroupLocation is not set, use the first location from the Machines
	if resourceGroupLocation == "" {
		if len(m.Deployment.Machines) > 0 {
			resourceGroupLocation = m.Deployment.Machines[0].Location
		}
		if resourceGroupLocation == "" {
			return fmt.Errorf(
				"resource group location is not set and couldn't be inferred from machines",
			)
		}
	}
	m.Deployment.ResourceGroupLocation = resourceGroupLocation

	l.Debugf(
		"Creating Resource Group - %s in location %s",
		m.Deployment.ResourceGroupName,
		m.Deployment.ResourceGroupLocation,
	)

	for _, machine := range m.Deployment.Machines {
		prog.UpdateStatus(
			&models.Status{
				Name:   machine.Name,
				Type:   models.UpdateStatusResourceTypeVM,
				Status: "Provisioning",
			},
		)
	}

	_, err := p.Client.GetOrCreateResourceGroup(
		ctx,
		m.Deployment.ResourceGroupName,
		m.Deployment.ResourceGroupLocation,
		m.Deployment.Tags,
	)
	if err != nil {
		l.Errorf("Failed to create Resource Group - %s: %v", m.Deployment.ResourceGroupName, err)
		return fmt.Errorf("failed to create resource group: %w", err)
	}

	for _, machine := range m.Deployment.Machines {
		prog.UpdateStatus(
			&models.Status{
				Name: machine.Name,
				Type: models.UpdateStatusResourceTypeVM,
				Status: models.CreateStateMessage(
					models.UpdateStatusResourceTypeVM,
					models.StatusCreating,
					machine.Name,
				),
			},
		)
	}

	l.Debugf(
		"Created Resource Group - %s in location %s",
		m.Deployment.ResourceGroupName,
		m.Deployment.ResourceGroupLocation,
	)

	return nil
}
func (p *AzureProvider) GetVMIPAddresses(
	ctx context.Context,
	resourceGroupName, vmName string,
) (string, string, error) {
	l := logger.Get()
	l.Debugf("Getting IP addresses for VM %s in resource group %s", vmName, resourceGroupName)

	// Get the VM
	vm, err := p.Client.GetVirtualMachine(ctx, resourceGroupName, vmName)
	if err != nil {
		return "", "", fmt.Errorf("failed to get virtual machine: %w", err)
	}

	// Check if the VM has a network profile
	if vm.Properties.NetworkProfile == nil ||
		len(vm.Properties.NetworkProfile.NetworkInterfaces) == 0 {
		return "", "", fmt.Errorf("VM has no network interfaces")
	}

	// Get the network interface ID
	nicID := vm.Properties.NetworkProfile.NetworkInterfaces[0].ID
	if nicID == nil {
		return "", "", fmt.Errorf("network interface ID is nil")
	}

	// Parse the network interface ID to get its name
	nicName, err := parseResourceID(*nicID)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse network interface ID: %w", err)
	}

	// Get the network interface
	nic, err := p.Client.GetNetworkInterface(ctx, resourceGroupName, nicName)
	if err != nil {
		return "", "", fmt.Errorf("failed to get network interface: %w", err)
	}

	// Check if the network interface has IP configurations
	if nic.Properties.IPConfigurations == nil || len(nic.Properties.IPConfigurations) == 0 {
		return "", "", fmt.Errorf("network interface has no IP configurations")
	}

	if len(nic.Properties.IPConfigurations) > 1 {
		l.Warnf("Network interface %s has multiple IP configurations, using the first one", nicName)
	}

	ipConfig := nic.Properties.IPConfigurations[0]

	// Get public IP address
	publicIP := ""
	if ipConfig.Properties.PublicIPAddress != nil {
		publicIP, err = p.Client.GetPublicIPAddress(
			ctx,
			resourceGroupName,
			ipConfig.Properties.PublicIPAddress,
		)
		if err != nil {
			return "", "", fmt.Errorf("failed to get public IP address: %w", err)
		}
	}

	privateIP := ""
	if ipConfig.Properties.PrivateIPAddress != nil {
		privateIP = *ipConfig.Properties.PrivateIPAddress
	}

	return publicIP, privateIP, nil
}

func parseResourceID(resourceID string) (string, error) {
	parts := strings.Split(resourceID, "/")
	if len(parts) < 9 {
		return "", fmt.Errorf("invalid resource ID format")
	}
	return parts[8], nil
}
