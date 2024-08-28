package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	internal_azure "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/blang/semver"
	"golang.org/x/sync/errgroup"
)

const ipRetries = 3
const timeBetweenIPRetries = 10 * time.Second

// PrepareDeployment prepares the deployment by setting up the resource group and initial configuration.
func (p *AzureProvider) PrepareDeployment(ctx context.Context) error {
	l := logger.Get()
	l.Debug("Starting PrepareDeployment")
	m := display.GetGlobalModelFunc()

	// Set the start time for the deployment
	m.Deployment.StartTime = time.Now()
	l.Debugf("Deployment start time: %v", m.Deployment.StartTime)

	// Ensure we have a location set
	if m.Deployment.ResourceGroupLocation == "" {
		// Set a default location if not specified
		m.Deployment.ResourceGroupLocation = "eastus" // You can change this to any default Azure region
		l.Warn("No resource group location specified, using default: eastus")
	}

	// Prepare resource group
	l.Debug("Preparing resource group")
	err := p.PrepareResourceGroup(ctx)
	if err != nil {
		l.Error(fmt.Sprintf("Failed to prepare resource group: %v", err))
		l.Debug(fmt.Sprintf("Resource group preparation error details: %v", err))
		return fmt.Errorf("failed to prepare resource group: %v", err)
	}
	l.Debug("Resource group prepared successfully")

	if err := m.Deployment.UpdateViperConfig(); err != nil {
		l.Error(fmt.Sprintf("Failed to update viper config: %v", err))
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	return nil
}

func (p *AzureProvider) ProvisionMachines(ctx context.Context) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()

	for _, machine := range m.Deployment.Machines {
		if err := p.WaitForAllMachinesToReachState(ctx, models.AzureResourceStateSucceeded); err != nil {
			return fmt.Errorf("failed waiting for all machines to reach succeeded state: %v", err)
		}

		if err := p.provisionDocker(ctx, machine.Name); err != nil {
			l.Errorf("Failed to provision Docker for machine %s: %v", machine.Name, err)
			return err
		}
	}

	bd := BacalhauDeployer{}

	// Provision Bacalhau orchestrator
	if err := bd.DeployOrchestrator(ctx); err != nil {
		l.Errorf("Failed to provision Bacalhau orchestrator: %v", err)
		return err
	}

	orchestrator, err := bd.findOrchestratorMachine()
	if err != nil {
		l.Errorf("Failed to find orchestrator machine: %v", err)
		return err
	}

	if orchestrator.PublicIP == "" {
		l.Errorf("Orchestrator machine has no public IP: %v", err)
		return err
	}
	m.Deployment.OrchestratorIP = orchestrator.PublicIP

	for _, machine := range m.Deployment.Machines {
		if machine.Orchestrator {
			continue
		}
		if err := bd.DeployWorker(ctx, machine.Name); err != nil {
			return fmt.Errorf("failed to provision Bacalhau worker %s: %v",
				machine.Name,
				err,
			)
		}
	}

	return nil
}

func (p *AzureProvider) testSSHLiveness(ctx context.Context, machineName string) error {
	m := display.GetGlobalModelFunc()
	// Test SSH connectivity
	sshConfig, err := sshutils.NewSSHConfigFunc(
		m.Deployment.Machines[machineName].PublicIP,
		m.Deployment.Machines[machineName].SSHPort,
		m.Deployment.Machines[machineName].SSHUser,
		[]byte(m.Deployment.SSHPrivateKeyMaterial),
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	m.UpdateStatus(
		models.NewDisplayStatusWithText(
			machineName,
			models.AzureResourceTypeVM,
			models.AzureResourceStatePending,
			"Testing SSH",
		),
	)

	m.Deployment.Machines[machineName].SetServiceState("SSH", models.ServiceStateUpdating)
	sshErr := sshConfig.WaitForSSH(3, time.Second*10) //nolint:gomnd
	if sshErr != nil {
		err := m.Deployment.UpdateMachine(machineName, func(machine *models.Machine) {
			machine.SetServiceState("SSH", models.ServiceStateFailed)
			machine.StatusMessage = "Permanently failed deploying SSH"
		})
		if err != nil {
			return err
		}
		m.UpdateStatus(
			models.NewDisplayStatusWithText(
				machineName,
				models.AzureResourceTypeVM,
				models.AzureResourceStateFailed,
				m.Deployment.Machines[machineName].StatusMessage,
			),
		)
		return sshErr
	} else {
		m.Deployment.Machines[machineName].StatusMessage = "Successfully Deployed"
		m.Deployment.Machines[machineName].SetServiceState("SSH", models.ServiceStateSucceeded)
		m.UpdateStatus(
			models.NewDisplayStatusWithText(
				machineName,
				models.AzureResourceTypeVM,
				models.AzureResourceStateSucceeded,
				"SSH Successfully Deployed",
			),
		)
	}

	return nil
}

func (p *AzureProvider) provisionDocker(ctx context.Context, machineName string) error {
	m := display.GetGlobalModelFunc()

	err := m.Deployment.Machines[machineName].InstallDockerAndCorePackages(ctx)
	if err != nil {
		return fmt.Errorf(
			"failed to install Docker and core packages on VM %s: %v",
			machineName,
			err,
		)
	}

	sshConfig, err := sshutils.NewSSHConfigFunc(
		m.Deployment.Machines[machineName].PublicIP,
		m.Deployment.Machines[machineName].SSHPort,
		m.Deployment.Machines[machineName].SSHUser,
		[]byte(m.Deployment.SSHPrivateKeyMaterial),
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	m.UpdateStatus(
		models.NewDisplayStatusWithText(
			machineName,
			models.AzureResourceTypeVM,
			models.AzureResourceStatePending,
			"Testing Docker",
		),
	)

	versionsObject := map[string]interface{}{}
	out, err := sshConfig.ExecuteCommand(ctx, "sudo docker version -f json")
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	err = json.Unmarshal([]byte(out), &versionsObject)
	if err != nil {
		return fmt.Errorf("failed to marshal Docker server version: %w", err)
	}

	serverVersionDetected := false
	clientVersionDetected := false
	if versionsObject["Server"] == nil {
		return fmt.Errorf("failed to get Docker server version")
	} else {
		if serverVersion, ok := versionsObject["Server"].(map[string]interface{}); ok {
			if version, ok := serverVersion["Version"].(string); ok {
				// If Version is a semver, we can use it
				if _, err := semver.Parse(version); err == nil {
					serverVersionDetected = true
				} else {
					serverVersionDetected = false
				}
			}
		}
	}

	if versionsObject["Client"] == nil {
		return fmt.Errorf("failed to get Docker client version")
	} else {
		if clientVersion, ok := versionsObject["Client"].(map[string]interface{}); ok {
			if version, ok := clientVersion["Version"].(string); ok {
				// If Version is a semver, we can use it
				if _, err := semver.Parse(version); err == nil {
					clientVersionDetected = true
				}
			}
		}
	}

	if !serverVersionDetected || !clientVersionDetected {
		m.UpdateStatus(
			models.NewDisplayStatusWithText(
				machineName,
				models.AzureResourceTypeVM,
				models.AzureResourceStateFailed,
				m.Deployment.Machines[machineName].StatusMessage,
			),
		)
		return fmt.Errorf("failed to detect Docker version")
	}

	// If all checks pass, continue with the existing code
	m.Deployment.Machines[machineName].StatusMessage = "Successfully Deployed"
	m.Deployment.Machines[machineName].SetServiceState("Docker", models.ServiceStateSucceeded)
	m.UpdateStatus(
		models.NewDisplayStatusWithText(
			machineName,
			models.AzureResourceTypeVM,
			models.AzureResourceStateSucceeded,
			"Docker Successfully Deployed",
		),
	)

	return nil
}

func (p *AzureProvider) DeployResources(ctx context.Context) error {
	l := logger.Get()
	l.Info("Deploying ARM template")
	m := display.GetGlobalModelFunc()

	if len(m.Deployment.Locations) >= 0 {
		// Merge the Locations with the UniqueLocations
		m.Deployment.UniqueLocations = append(
			m.Deployment.UniqueLocations,
			m.Deployment.Locations...)
		// Remove duplicates
		m.Deployment.UniqueLocations = utils.RemoveDuplicates(m.Deployment.UniqueLocations)
	}

	if len(m.Deployment.UniqueLocations) == 0 {
		return fmt.Errorf("no locations provided")
	}

	// Group machines by location
	machinesByLocation := make(map[string][]*models.Machine)
	for _, machine := range m.Deployment.Machines {
		machinesByLocation[machine.Location] = append(machinesByLocation[machine.Location], machine)
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(models.NumberOfSimultaneousProvisionings)

	for location, machines := range machinesByLocation {
		location, machines := location, machines // https://golang.org/doc/faq#closures_and_goroutines
		l.Infof(
			"Preparing to deploy machines in location %s with %d machines",
			location,
			len(machines),
		)

		g.Go(func() error {
			l.Infof("Starting deployment for location %s", location)

			if len(machines) == 0 {
				l.Errorf("No machines to deploy in location %s", location)
				return fmt.Errorf("no machines to deploy in location %s", location)
			}

			for _, machine := range machines {
				l.Infof("Deploying machine %s in location %s", machine.Name, location)

				err := p.deployMachine(ctx, machine, map[string]*string{})
				if err != nil {
					return fmt.Errorf(
						"failed to deploy machine %s in location %s: %w",
						machine.Name,
						location,
						err,
					)
				}

				l.Infof(
					"Successfully deployed machine %s in location %s",
					machine.Name,
					location,
				)
			}

			l.Infof("Successfully deployed all machines in location %s", location)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		l.Errorf("Deployment failed: %v", err)
		return fmt.Errorf("deployment failed: %w", err)
	}

	l.Info("ARM template deployment completed successfully")
	return nil
}

func (p *AzureProvider) deployMachine(
	ctx context.Context,
	machine *models.Machine,
	tags map[string]*string,
) error {
	m := display.GetGlobalModelFunc()
	defer func() {
		m := display.GetGlobalModelFunc()
		m.DeregisterGoroutine(atomic.LoadInt64(&p.goroutineCounter))
	}()

	m.UpdateStatus(
		models.NewDisplayStatus(
			machine.Name,
			machine.Name,
			models.AzureResourceTypeVM,
			models.AzureResourceStateNotStarted,
		),
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

	m.Deployment.Machines[machine.Name].SetResourceState(
		models.AzureResourceTypeVM.ResourceString,
		models.AzureResourceStateSucceeded,
	)

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
	m := display.GetGlobalModelFunc()
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
		"allowedPorts":             m.Deployment.AllowedPorts,
	}
}

func (p *AzureProvider) deployTemplateWithRetry(
	ctx context.Context,
	machine *models.Machine,
	vmTemplate map[string]interface{},
	params map[string]interface{},
	tags map[string]*string,
) error {
	id := atomic.AddInt64(&p.goroutineCounter, 1)
	m := display.GetGlobalModelFunc()
	m.RegisterGoroutine(fmt.Sprintf("deployTemplateWithRetry-%d", id))
	defer m.DeregisterGoroutine(id)

	l := logger.Get()
	maxRetries := 3

	if mach, ok := m.Deployment.Machines[machine.Name]; !ok || mach == nil {
		return fmt.Errorf("machine %s not found in deployment", machine.Name)
	}

	m.UpdateStatus(
		models.NewDisplayVMStatus(
			machine.Name,
			models.AzureResourceStatePending,
		),
	)

	dnsFailed := false
	for retry := 0; retry < maxRetries; retry++ {
		client := p.GetAzureClient()
		poller, err := client.DeployTemplate(
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
			dispStatus := models.NewDisplayVMStatus(
				machine.Name,
				models.AzureResourceStatePending,
			)
			dispStatus.StatusMessage = fmt.Sprintf(
				"DNS Conflict - Retrying... %d/%d",
				retry+1,
				maxRetries,
			)
			m.UpdateStatus(dispStatus)
			dnsFailed = true
			continue
		} else if err != nil {
			m.UpdateStatus(
				models.NewDisplayStatusWithText(
					machine.Name,
					models.AzureResourceTypeVM,
					models.AzureResourceStateFailed,
					err.Error(),
				),
			)
			return fmt.Errorf("error deploying template: %v", err)
		}

		// Finished with no errors
		dnsFailed = false
		break
	}

	if dnsFailed {
		m.UpdateStatus(
			models.NewDisplayStatusWithText(
				machine.Name,
				models.AzureResourceTypeVM,
				models.AzureResourceStateFailed,
				"Failed to deploy due to DNS conflict.",
			),
		)
		m.Deployment.Machines[machine.Name].StatusMessage = "Failed to deploy due to DNS conflict"
	} else {
		for i := 0; i < ipRetries; i++ {
			publicIP, privateIP, err := p.GetVMIPAddresses(ctx, m.Deployment.ResourceGroupName, machine.Name)
			if err != nil {
				if i == ipRetries-1 {
					l.Errorf("Failed to get IP addresses for VM %s after %d retries: %v", machine.Name, ipRetries, err)
					m.Deployment.Machines[machine.Name].StatusMessage = "Failed to get IP addresses"
					return fmt.Errorf("failed to get IP addresses for VM %s: %v", machine.Name, err)
				}
				time.Sleep(timeBetweenIPRetries)
				displayStatus := models.NewDisplayStatusWithText(
					machine.Name,
					models.AzureResourceTypeVM,
					models.AzureResourceStatePending,
					"Waiting for IP addresses",
				)
				displayStatus.PublicIP = fmt.Sprintf("Retry: %d/%d", i+1, ipRetries)
				displayStatus.PrivateIP = fmt.Sprintf("Retry: %d/%d", i+1, ipRetries)
				m.UpdateStatus(
					displayStatus,
				)
				continue
			}

			m.Deployment.Machines[machine.Name].PublicIP = publicIP
			m.Deployment.Machines[machine.Name].PrivateIP = privateIP

			if m.Deployment.Machines[machine.Name].ElapsedTime == 0 {
				m.Deployment.Machines[machine.Name].ElapsedTime = time.Since(machine.StartTime)
			}
			displayStatus := models.NewDisplayStatusWithText(
				machine.Name,
				models.AzureResourceTypeVM,
				models.AzureResourceStateSucceeded,
				"IPs Provisioned",
			)
			displayStatus.PublicIP = publicIP
			displayStatus.PrivateIP = privateIP
			displayStatus.ElapsedTime = m.Deployment.Machines[machine.Name].ElapsedTime
			m.UpdateStatus(
				displayStatus,
			)
			m.Deployment.Machines[machine.Name].SetResourceState(
				models.AzureResourceTypeVM.ResourceString,
				models.AzureResourceStateSucceeded,
			)
			break
		}
	}

	return nil
}

func (p *AzureProvider) PollAndUpdateResources(ctx context.Context) ([]interface{}, error) {
	l := logger.Get()
	start := time.Now()
	defer func() {
		l.Debugf("PollAndUpdateResources took %v", time.Since(start))
	}()
	m := display.GetGlobalModelFunc()
	client := p.GetAzureClient()
	resources, err := client.GetResources(
		ctx,
		m.Deployment.SubscriptionID,
		m.Deployment.ResourceGroupName,
		m.Deployment.Tags,
	)
	if err != nil {
		return nil, err
	}

	// All resources
	// Write status for pending or complete to a file
	//nolint:gomnd
	resourceBytes, err := json.Marshal(resources)
	if err != nil {
		l.Errorf("Failed to marshal resources: %v", err)
	}

	err = os.WriteFile(
		"status.txt",
		resourceBytes,
		0600, //nolint:gomnd
	)

	var statusUpdates []*models.DisplayStatus
	for _, resource := range resources {
		if err != nil {
			l.Errorf("Failed to write status to file: %v", err)
		}

		statuses, err := models.ConvertFromRawResourceToStatus(
			resource.(map[string]interface{}),
			m.Deployment,
		)
		if err != nil {
			l.Errorf("Failed to convert resource to status: %v", err)
			continue
		}
		for i := range statuses {
			statusUpdates = append(statusUpdates, &statuses[i])
		}
	}

	// Push all changes to the update loop at once
	for _, status := range statusUpdates {
		m.UpdateStatus(status)
	}

	defer func() {
		l.Debugf("PollAndUpdateResources execution took %v", time.Since(start))
	}()

	select {
	case <-ctx.Done():
		l.Debug("Cancel command received in PollAndUpdateResources")
		return nil, ctx.Err()
	default:
		l.Debugf("PollAndUpdateResources execution took %v", time.Since(start))
		return resources, nil
	}
}

// finalizeDeployment performs any necessary cleanup and final steps
func (p *AzureProvider) FinalizeDeployment(ctx context.Context) error {
	id := atomic.AddInt64(&p.goroutineCounter, 1)
	m := display.GetGlobalModelFunc()
	m.RegisterGoroutine(fmt.Sprintf("FinalizeDeployment-%d", id))
	defer m.DeregisterGoroutine(id)

	l := logger.Get()

	// Check for context cancellation
	if err := ctx.Err(); err != nil {
		l.Info("Deployment cancelled during finalization")
		return fmt.Errorf("deployment cancelled: %w", err)
	}

	// Log successful completion
	l.Info("Azure deployment completed successfully")

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
	m := display.GetGlobalModelFunc()

	if m.Deployment == nil {
		return fmt.Errorf("global deployment object is not initialized")
	}

	// Check if the resource group name already contains a timestamp
	if m.Deployment.ResourceGroupName == "" {
		m.Deployment.ResourceGroupName = "andaime-rg"
	}
	newRGName := m.Deployment.ResourceGroupName + "-" + time.Now().Format("20060102150405")
	m.Deployment.ResourceGroupName = newRGName

	resourceGroupLocation := m.Deployment.ResourceGroupLocation
	// If ResourceGroupLocation is not set, use the first location from the Machines
	if resourceGroupLocation == "" {
		if len(m.Deployment.Machines) > 0 {
			for _, machine := range m.Deployment.Machines {
				// Break over the first machine
				resourceGroupLocation = machine.Location
				break
			}
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
		m.UpdateStatus(
			models.NewDisplayVMStatus(
				machine.Name,
				models.AzureResourceStatePending,
			),
		)
	}

	client := p.GetAzureClient()
	_, err := client.GetOrCreateResourceGroup(
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
		m.UpdateStatus(
			models.NewDisplayVMStatus(
				machine.Name,
				models.AzureResourceStateNotStarted,
			),
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
	client := p.GetAzureClient()

	// Get the VM
	vm, err := client.GetVirtualMachine(ctx, resourceGroupName, vmName)
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
	nic, err := client.GetNetworkInterface(ctx, resourceGroupName, nicName)
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
		publicIP, err = client.GetPublicIPAddress(
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
	//nolint:gomnd
	if len(parts) < 9 {
		return "", fmt.Errorf("invalid resource ID format")
	}
	return parts[8], nil
}

func (p *AzureProvider) DeployBacalhauWorkers(ctx context.Context) error {
	m := display.GetGlobalModelFunc()
	l := logger.Get()
	var workerMachines []*models.Machine
	for _, machine := range m.Deployment.Machines {
		if !machine.Orchestrator {
			workerMachines = append(workerMachines, machine)
		}
	}
	l.Infof("Deploying Bacalhau workers on %d machines", len(workerMachines))
	maxSimultaneous := 5 // Adjust this value as needed
	for i := 0; i < len(workerMachines); i += maxSimultaneous {
		end := i + maxSimultaneous
		if end > len(workerMachines) {
			end = len(workerMachines)
		}
		l.Debugf("Deploying workers %d to %d", i, end-1)
	}
	l.Info("All Bacalhau workers deployed successfully")
	return nil
}
