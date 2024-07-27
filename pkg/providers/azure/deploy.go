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
	resourceGroupName, resourceGroupLocation, err := p.prepareResourceGroup(ctx, deployment, disp)
	if err != nil {
		return err
	}

	deployment.ResourceGroupName = resourceGroupName
	deployment.ResourceGroupLocation = resourceGroupLocation

	err = p.createNetworkInfrastructure(ctx, deployment, disp)
	if err != nil {
		return err
	}

	err = p.processOrchestratorNode(ctx, deployment, disp)
	if err != nil {
		return err
	}

	err = p.processNonOrchestratorMachines(ctx, deployment, disp)
	if err != nil {
		return err
	}

	return p.finalizeDeployment(ctx, deployment, disp)
}

// createNetworkInfrastructure sets up the network infrastructure for the deployment
func (p *AzureProvider) createNetworkInfrastructure(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	l := logger.Get()

	l.Debugf("Creating network infrastructure for deployment: %v", deployment)
	locations := make([]string, 0)
	for _, machine := range deployment.NonOrchestratorMachines {
		locations = append(locations, machine.Location)
	}
	if deployment.OrchestratorNode != nil {
		locations = append(locations, deployment.OrchestratorNode.Location)
	}

	err := p.createNetworkResources(ctx, deployment, locations, disp)
	if err != nil {
		return err
	}

	l.Debugf("Created network infrastructure for deployment: %v", deployment)
	return deployment.UpdateViperConfig()
}

// createNetworkResources creates the network resources for each location
func (p *AzureProvider) createNetworkResources(ctx context.Context,
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
				subnet, err := p.createNetworkResourcesForLocation(
					ctx,
					deployment,
					loc,
					disp,
				)
				if err != nil {
					errChan <- err
					return
				}
				deployment.Subnets[loc] = subnet
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
func (p *AzureProvider) createNetworkResourcesForLocation(
	ctx context.Context,
	deployment *models.Deployment,
	location string,
	disp *display.Display,
) ([]*armnetwork.Subnet, error) {
	disp.UpdateStatus(&models.Status{
		ID:     fmt.Sprintf("vnet-%s", location),
		Type:   "VNET",
		Status: "Creating",
	})
	vnet, err := p.Client.CreateVirtualNetwork(
		ctx,
		deployment.ResourceGroupName,
		location+"-vnet",
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

	disp.UpdateStatus(&models.Status{
		ID:     fmt.Sprintf("nsg-%s", location),
		Type:   "NSG",
		Status: "Creating",
	})
	_, err = p.Client.CreateNetworkSecurityGroup(
		ctx,
		deployment.ResourceGroupName,
		location+"-nsg",
		location,
		deployment.AllowedPorts,
		deployment.Tags,
	)
	if err != nil {
		disp.UpdateStatus(&models.Status{
			ID:     fmt.Sprintf("nsg-%s", location),
			Type:   "NSG",
			Status: "Failed",
		})
		return nil, fmt.Errorf("failed to create network security group in %s: %v", location, err)
	}
	disp.UpdateStatus(&models.Status{
		ID:     fmt.Sprintf("nsg-%s", location),
		Type:   "NSG",
		Status: "Created",
	})

	return []*armnetwork.Subnet{vnet.Properties.Subnets[0]}, nil
}

// createPublicIPs creates public IPs for all machines in the deployment
func (p *AzureProvider) createPublicIPs(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) (*models.Deployment, error) {
	l := logger.Get()

	l.Debugf("Creating public IPs for deployment: %v", deployment)
	publicIPs := make(map[string]string)
	var allMachines []models.Machine

	if deployment.OrchestratorNode != nil {
		allMachines = append(allMachines, *deployment.OrchestratorNode)
	}
	allMachines = append(allMachines, deployment.NonOrchestratorMachines...)

	for _, machine := range allMachines {
		internalMachine := machine
		for i := 0; i < machine.Parameters[0].Count; i++ {
			vmName := fmt.Sprintf("%s-%d", machine.Location, i)
			publicIP, err := p.createPublicIPForVM(ctx, deployment, &internalMachine, disp)
			if err != nil {
				return nil, err
			}
			publicIPs[vmName] = *publicIP.Properties.IPAddress
		}
	}

	l.Debugf("Created public IPs for deployment: %v", deployment)
	return deployment, nil
}

// createPublicIPForVM creates a public IP for a specific VM
func (p *AzureProvider) createPublicIPForVM(
	ctx context.Context,
	deployment *models.Deployment,
	machine *models.Machine,
	disp *display.Display,
) (*armnetwork.PublicIPAddress, error) {
	disp.UpdateStatus(&models.Status{
		ID:     fmt.Sprintf("public-ip-%s", machine.Name),
		Type:   "PUBLIC-IP",
		Status: "Creating",
	})
	publicIP, err := p.Client.CreatePublicIP(
		ctx,
		deployment.ResourceGroupName,
		machine.Name+"-ip",
		machine.Location,
		deployment.Tags,
	)
	if err != nil {
		disp.UpdateStatus(&models.Status{
			ID:     fmt.Sprintf("public-ip-%s", machine.Name),
			Type:   "PUBLIC-IP",
			Status: "Failed",
		})
		return nil, fmt.Errorf("failed to create public IP for VM %s: %v", machine.Name, err)
	}
	disp.UpdateStatus(&models.Status{
		ID:     fmt.Sprintf("public-ip-%s", machine.Name),
		Type:   "PUBLIC-IP",
		Status: "Created",
	})
	return &publicIP, nil
}

// processOrchestratorNode processes the orchestrator node if it exists
func (p *AzureProvider) processOrchestratorNode(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	if deployment.OrchestratorNode == nil {
		return nil
	}

	err := p.processMachines(
		ctx,
		[]models.Machine{*deployment.OrchestratorNode},
		deployment,
		disp,
	)
	if err != nil {
		return fmt.Errorf("failed to process orchestrator node: %v", err)
	}

	return deployment.UpdateViperConfig()
}

// processNonOrchestratorMachines processes non-orchestrator machines
func (p *AzureProvider) processNonOrchestratorMachines(ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display) error {
	err := p.processMachines(
		ctx,
		deployment.NonOrchestratorMachines,
		deployment,
		disp,
	)
	if err != nil {
		return fmt.Errorf("failed to process non-orchestrator nodes: %v", err)
	}

	return deployment.UpdateViperConfig()
}

// processMachines processes a list of machines
func (p *AzureProvider) processMachines(ctx context.Context,
	machines []models.Machine,
	deployment *models.Deployment,
	disp *display.Display) error {
	l := logger.Get()

	defaultCount := viper.GetInt("azure.default_count_per_zone")
	defaultType := viper.GetString("azure.default_machine_type")

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("processMachines cancelled before starting: %w", err)
	}

	for _, machine := range machines {
		internalMachine := machine
		count := defaultCount
		machineType := defaultType
		isOrchestrator := false

		if len(internalMachine.Parameters) > 0 {
			// Check if Count is a valid positive integer
			if internalMachine.Parameters[0].Count > 0 {
				count = internalMachine.Parameters[0].Count
			} else {
				count = defaultCount
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
						ID:     vmName,
						Type:   "Azure",
						Status: "Creating",
					})
					_, err := DeployVM(ctx,
						p.Client,
						deployment,
						&internalMachine,
						disp,
					)
					if err != nil {
						errChan <- fmt.Errorf("error deploying VM %s in %s: %v", vmName, internalMachine.Location, err)
						disp.UpdateStatus(&models.Status{
							ID:     vmName,
							Type:   "Azure",
							Status: "Failed",
						})
						return
					}
					disp.UpdateStatus(&models.Status{
						ID:        vmName,
						Type:      "Azure",
						Status:    "Created",
						PublicIP:  "",
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
	return nil
}

// finalizeDeployment performs any necessary cleanup and final steps
func (p *AzureProvider) finalizeDeployment(
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
	fmt.Printf("Deployment Summary for Resource Group: %s\n", deployment.ResourceGroupName)
	fmt.Printf("Location: %s\n", deployment.ResourceGroupLocation)
	if deployment.OrchestratorNode != nil {
		fmt.Printf("Orchestrator Node: %+v\n", deployment.OrchestratorNode)
	}
	fmt.Printf("Non-Orchestrator Machines: %d\n", len(deployment.NonOrchestratorMachines))
	for i, machine := range deployment.NonOrchestratorMachines {
		fmt.Printf("  Machine %d: %+v\n", i+1, machine)
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
func (p *AzureProvider) prepareResourceGroup(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display) (string, string, error) {
	resourceGroupName := deployment.ResourceGroupName + "-rg-" + time.Now().Format("20060102150405")
	resourceGroupLocation := deployment.ResourceGroupLocation

	disp.UpdateStatus(&models.Status{
		ID:     resourceGroupName,
		Type:   "Resource Group",
		Status: "Creating",
	})

	_, err := p.Client.GetOrCreateResourceGroup(
		ctx,
		resourceGroupName,
		resourceGroupLocation,
		deployment.Tags,
	)
	if err != nil {
		disp.UpdateStatus(&models.Status{
			ID:     resourceGroupName,
			Type:   "Resource Group",
			Status: "Failed",
		})
		return "", "", fmt.Errorf("failed to create resource group: %w", err)
	}

	disp.UpdateStatus(&models.Status{
		ID:     resourceGroupName,
		Type:   "Resource Group",
		Status: "Created",
	})

	return resourceGroupName, resourceGroupLocation, nil
}
