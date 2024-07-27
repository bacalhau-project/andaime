package azure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
)

// DeployResources deploys Azure resources based on the provided configuration.
// Config should be the Azure subsection of the viper config.
func (p *AzureProvider) DeployResources(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	// Prepare resource group
	resourceGroupName, resourceGroupLocation, err := p.PrepareResourceGroup(ctx, deployment, disp)
	if err != nil {
		return err
	}

	deployment.ResourceGroupName = resourceGroupName
	deployment.ResourceGroupLocation = resourceGroupLocation

	err = deployment.UpdateViperConfig()
	if err != nil {
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	err = p.CreateNetworkInfrastructure(ctx, deployment, disp)
	if err != nil {
		return err
	}

	err = deployment.UpdateViperConfig()
	if err != nil {
		return fmt.Errorf("failed to update viper config: %v", err)
	}

	err = p.ProcessMachines(ctx, deployment, disp)
	if err != nil {
		return err
	}

	return p.FinalizeDeployment(ctx, deployment, disp)
}

// createNetworkInfrastructure sets up the network infrastructure for the deployment
func (p *AzureProvider) CreateNetworkInfrastructure(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	l := logger.Get()

	l.Debugf("Creating network infrastructure for deployment: %v", deployment)
	locations := make([]string, 0)
	for _, machine := range deployment.Machines {
		locations = append(locations, machine.Location)
	}
	if deployment.OrchestratorNode != nil {
		locations = append(locations, deployment.OrchestratorNode.Location)
	}

	err := p.CreateNetworkResources(ctx, deployment, locations, disp)
	if err != nil {
		return err
	}

	l.Debugf("Created network infrastructure for deployment: %v", deployment)
	return deployment.UpdateViperConfig()
}

// createNetworkResources creates the network resources for each location
func (p *AzureProvider) CreateNetworkResources(ctx context.Context,
	deployment *models.Deployment,
	locations []string,
	disp *display.Display) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(locations))

	for _, location := range locations {
		wg.Add(1)
		go func(loc string) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				errChan <- fmt.Errorf("operation cancelled for location %s: %w", loc, ctx.Err())
				return
			default:
				subnets, err := p.CreateNetworkResourcesForLocation(
					ctx,
					deployment,
					loc,
					disp,
				)
				if err != nil {
					errChan <- err
					return
				}
				deployment.SetSubnet(loc, subnets...)
			}
		}(location)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// createNetworkResourcesForLocation creates network resources for a specific location
func (p *AzureProvider) CreateNetworkResourcesForLocation(
	ctx context.Context,
	deployment *models.Deployment,
	location string,
	disp *display.Display,
) ([]*armnetwork.Subnet, error) {
	for _, machine := range deployment.Machines {
		if machine.Location != location {
			continue
		}
		disp.UpdateStatus(&models.Status{
			ID:     machine.ID,
			Type:   "VM",
			Status: fmt.Sprintf("Creating VNET for %s", location),
		})
	}
	vnet, err := p.Client.CreateVirtualNetwork(
		ctx,
		deployment.ResourceGroupName,
		location,
		deployment.Tags,
	)
	if err != nil {
		disp.UpdateStatus(&models.Status{
			ID:     fmt.Sprintf("vnet-%s", location),
			Type:   "VNET",
			Status: "Failed",
		})
		return nil, fmt.Errorf("failed to create virtual network in %s: %v", location, err)
	}
	disp.UpdateStatus(&models.Status{
		ID:     fmt.Sprintf("vnet-%s", location),
		Type:   "VNET",
		Status: "Created",
	})

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf(
			"operation cancelled after creating VNET for location %s: %w",
			location,
			err,
		)
	}

	createdNSG, err := p.Client.CreateNetworkSecurityGroup(
		ctx,
		deployment.ResourceGroupName,
		location,
		deployment.AllowedPorts,
		deployment.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network security group in %s: %v", location, err)
	}

	if deployment.NetworkSecurityGroups == nil {
		deployment.NetworkSecurityGroups = make(map[string]*armnetwork.SecurityGroup)
	}
	deployment.NetworkSecurityGroups[location] = &createdNSG
	if deployment.Subnets == nil {
		deployment.Subnets = make(map[string][]*armnetwork.Subnet)
	}
	deployment.Subnets[location] = []*armnetwork.Subnet{vnet.Properties.Subnets[0]}
	return []*armnetwork.Subnet{vnet.Properties.Subnets[0]}, nil
}

// processMachines processes a list of machines
func (p *AzureProvider) ProcessMachines(ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display) error {
	l := logger.Get()
	defaultCount := viper.GetInt("azure.default_count_per_zone")
	defaultType := viper.GetString("azure.default_machine_type")

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("processMachines cancelled before starting: %w", err)
	}

	for _, machine := range deployment.Machines {
		internalMachine := machine
		count := defaultCount
		machineType := defaultType
		isOrchestrator := false

		if len(internalMachine.Parameters) > 0 {
			if internalMachine.Parameters[0].Count > 0 {
				count = internalMachine.Parameters[0].Count
			}
			if internalMachine.Parameters[0].Type != "" {
				machineType = internalMachine.Parameters[0].Type
			}
			isOrchestrator = internalMachine.Parameters[0].Orchestrator
		}

		internalMachine.Parameters = []models.Parameters{
			{
				Count:        count,
				Type:         machineType,
				Orchestrator: isOrchestrator,
			},
		}

		var wg sync.WaitGroup
		errChan := make(chan error, internalMachine.Parameters[0].Count)

		for i := 0; i < internalMachine.Parameters[0].Count; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				select {
				case <-ctx.Done():
					errChan <- fmt.Errorf("VM deployment cancelled: %w", ctx.Err())
					return
				default:
					vmName := fmt.Sprintf("%s-%d", internalMachine.Location, index)
					disp.UpdateStatus(&models.Status{
						ID:   vmName,
						Type: "VM",
						Status: fmt.Sprintf(
							"Creating network resources and VM %s in %s",
							vmName,
							internalMachine.Location,
						),
					})
					
					// Create network resources for the machine
					publicIP, nic, nsg, err := p.createNetworkResourcesForMachine(ctx, deployment, &internalMachine, disp)
					if err != nil {
						errChan <- fmt.Errorf("failed to create network resources for VM %s in %s: %v", vmName, internalMachine.Location, err)
						disp.UpdateStatus(&models.Status{
							ID:     vmName,
							Type:   "VM",
							Status: "Failed to create network resources",
						})
						return
					}

					// Update machine with network information
					internalMachine.PublicIP = publicIP
					internalMachine.Interface = nic
					internalMachine.NetworkSecurityGroup = nsg

					// Create the virtual machine
					_, err = p.CreateVirtualMachine(ctx, deployment, internalMachine, disp)
					if err != nil {
						errChan <- fmt.Errorf("error deploying VM %s in %s: %v", vmName, internalMachine.Location, err)
						disp.UpdateStatus(&models.Status{
							ID:     vmName,
							Type:   "VM",
							Status: "Failed to create VM",
						})
						return
					}

					publicIPAddress := ""
					if publicIP != nil && publicIP.Properties != nil && publicIP.Properties.IPAddress != nil {
						publicIPAddress = *publicIP.Properties.IPAddress
					}
					disp.UpdateStatus(&models.Status{
						ID:        vmName,
						Type:      "VM",
						Status:    "Created",
						PublicIP:  publicIPAddress,
						PrivateIP: "",
					})
				}
			}(i)
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-ctx.Done():
			l.Info("Waiting for ongoing VM deployments to finish after cancellation")
			<-done
			return fmt.Errorf("VM deployment process cancelled: %w", ctx.Err())
		case <-done:
			close(errChan)
			for err := range errChan {
				if err != nil {
					return err
				}
			}
		}
	}

	// After processing all machines, update the global deployment struct
	if err := deployment.UpdateViperConfig(); err != nil {
		l.Errorf("Failed to update viper config: %v", err)
		return fmt.Errorf("failed to update viper config: %w", err)
	}
	l.Debug("Successfully updated viper config after processing machines")
	return nil
}

// createNetworkResourcesForMachine creates network resources for a single machine
func (p *AzureProvider) createNetworkResourcesForMachine(
	ctx context.Context,
	deployment *models.Deployment,
	machine *models.Machine,
	disp *display.Display,
) (*armnetwork.PublicIPAddress, *armnetwork.Interface, *armnetwork.SecurityGroup, error) {
	l := logger.Get()
	l.Debugf("Creating network resources for machine %s in location %s", machine.ID, machine.Location)

	// Create Public IP
	publicIP, err := p.Client.CreatePublicIP(
		ctx,
		deployment.ResourceGroupName,
		machine.Location,
		machine.ID,
		deployment.Tags,
	)
	if err != nil {
		l.Errorf("Failed to create public IP for machine %s: %v", machine.ID, err)
		return nil, nil, nil, fmt.Errorf("failed to create public IP for machine %s: %w", machine.ID, err)
	}
	l.Debugf("Created public IP for machine %s", machine.ID)

	// Get subnet for the machine's location
	subnet, ok := deployment.Subnets[machine.Location]
	if !ok || len(subnet) == 0 {
		l.Errorf("No subnet found for location %s", machine.Location)
		return nil, nil, nil, fmt.Errorf("no subnet found for location %s", machine.Location)
	}
	l.Debugf("Found subnet for machine %s in location %s", machine.ID, machine.Location)

	nsg := deployment.NetworkSecurityGroups[machine.Location]
	if nsg == nil {
		l.Errorf("No network security group found for location %s", machine.Location)
		return nil, nil, nil, fmt.Errorf("no network security group found for location %s", machine.Location)
	}
	l.Debugf("Found network security group for machine %s in location %s", machine.ID, machine.Location)

	// Create NIC
	nic, err := p.Client.CreateNetworkInterface(
		ctx,
		deployment.ResourceGroupName,
		machine.Location,
		machine.ID,
		deployment.Tags,
		subnet[0],
		&publicIP,
		nsg,
	)
	if err != nil {
		l.Errorf("Failed to create network interface for machine %s: %v", machine.ID, err)
		return nil, nil, nil, fmt.Errorf("failed to create network interface for machine %s: %w", machine.ID, err)
	}
	l.Debugf("Created network interface for machine %s", machine.ID)

	publicIPAddress := ""
	if publicIP.Properties != nil && publicIP.Properties.IPAddress != nil {
		publicIPAddress = *publicIP.Properties.IPAddress
	}
	privateIPAddress := ""
	if nic.Properties != nil && nic.Properties.IPConfigurations != nil &&
		len(nic.Properties.IPConfigurations) > 0 {
		if nic.Properties.IPConfigurations[0].Properties != nil &&
			nic.Properties.IPConfigurations[0].Properties.PrivateIPAddress != nil {
			privateIPAddress = *nic.Properties.IPConfigurations[0].Properties.PrivateIPAddress
		}
	}

	logMessage := fmt.Sprintf(
		"Created network resources for machine %s: Public IP: %s, Private IP: %s",
		machine.ID,
		publicIPAddress,
		privateIPAddress,
	)
	l.Info(logMessage)
	disp.Log(logMessage)

	return &publicIP, &nic, nsg, nil
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
	l.Debugf("Deployment Summary for Resource Group: %s\n", deployment.ResourceGroupName)
	l.Debugf("Location: %s\n", deployment.ResourceGroupLocation)
	if deployment.OrchestratorNode != nil {
		l.Debugf("Orchestrator Node: %+v\n", deployment.OrchestratorNode)
	}
	l.Debugf("Machines: %d\n", len(deployment.Machines))
	for i, machine := range deployment.Machines {
		l.Debugf("  Machine %d: %+v\n", i+1, machine)
	}

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

	return nil
}

// prepareResourceGroup prepares or creates a resource group for the deployment
func (p *AzureProvider) PrepareResourceGroup(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display) (string, string, error) {
	// Check if the resource group name already contains a timestamp
	resourceGroupName := deployment.ResourceGroupName + "-" + time.Now().Format("20060102150405")
	resourceGroupLocation := deployment.ResourceGroupLocation

	for _, machine := range deployment.Machines {
		disp.UpdateStatus(&models.Status{
			ID:     machine.ID,
			Type:   "VM",
			Status: fmt.Sprintf("Creating Resource Group - %s", resourceGroupName),
		})
	}

	_, err := p.Client.GetOrCreateResourceGroup(
		ctx,
		resourceGroupName,
		resourceGroupLocation,
		deployment.Tags,
	)
	if err != nil {
		for _, machine := range deployment.Machines {
			disp.UpdateStatus(&models.Status{
				ID:     machine.ID,
				Type:   "VM",
				Status: fmt.Sprintf("Failed to create Resource Group - %s", resourceGroupName),
			})
		}
		return "", "", fmt.Errorf("failed to create resource group: %w", err)
	}

	for _, machine := range deployment.Machines {
		disp.UpdateStatus(&models.Status{
			ID:     machine.ID,
			Type:   "VM",
			Status: fmt.Sprintf("Created Resource Group - %s", resourceGroupName),
		})
	}

	return resourceGroupName, resourceGroupLocation, nil
}
