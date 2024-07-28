package azure

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"os"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/pulumi/pulumi-azure-native-sdk/compute"
	"github.com/pulumi/pulumi-azure-native-sdk/network"
	"github.com/pulumi/pulumi-azure-native-sdk/resources"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// DeployResources deploys Azure resources based on the provided configuration.
// Config should be the Azure subsection of the viper config.
func (p *AzureProvider) DeployResources(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	l := logger.Get()
	l.Info("Starting Azure resource deployment")

	// Set the start time for the deployment
	deployment.StartTime = time.Now()

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Create a done channel to signal completion
	done := utils.NewSafeChannel[struct{}](1)

	// Start a goroutine to handle cancellation and completion
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			l.Info("Deployment cancelled, closing all channels")
			utils.CloseAllChannels()
		case <-done.Ch:
			l.Info("Deployment completed, closing display channel")
			disp.Close()
		}
	}()

	// Ensure all goroutines finish before returning
	defer func() {
		done.Close()
		wg.Wait()
		l.Info("All goroutines finished, exiting")
	}()

	// Wrap the entire function in a defer/recover block
	defer func() {
		if r := recover(); r != nil {
			l.Errorf("Panic occurred during Azure deployment: %v\n%s", r, debug.Stack())
			disp.UpdateStatus(&models.Status{
				ID:     deployment.UniqueID,
				Type:   "Deployment",
				Status: "Failed - Panic occurred",
			})
		}
	}()

	stackName := deployment.UniqueID
	projectName := "andaime"
	runProgram := func(ctx *pulumi.Context) error {
		return deploymentProgram(ctx, deployment)
	}

	// Create a stack (this will create a project if it doesn't exist)
	stack, err := auto.UpsertStackInlineSource(ctx, stackName, projectName, runProgram)
	if err != nil {
		return fmt.Errorf("failed to create/update stack: %w", err)
	}

	// Configure the stack (if needed)
	// stack.SetConfig(ctx, "azure:location", auto.ConfigValue{Value: deployment.ResourceGroupLocation})

	// Run the update
	_, err = stack.Up(ctx, optup.Message("Updating Azure resources"))
	if err != nil {
		return fmt.Errorf("failed to update stack: %w", err)
	}

	deployment.EndTime = time.Now()
	l.Infof(
		"Azure deployment completed successfully in %v",
		deployment.EndTime.Sub(deployment.StartTime),
	)
	disp.UpdateStatus(&models.Status{
		ID:     deployment.UniqueID,
		Type:   "Deployment",
		Status: "Completed",
	})

	// Print the table of machines and IPs
	printMachineIPTable(deployment)

	return nil
}

// deploymentProgram defines the Pulumi program for Azure resource deployment
func deploymentProgram(pulumiCtx *pulumi.Context, deployment *models.Deployment) error {
	l := logger.Get()
	l.Debug("Starting deployment program")

	tags := pulumi.StringMap{}
	for k, v := range deployment.Tags {
		tags[k] = pulumi.String(*v)
	}

	// Create resource group
	rg, err := createResourceGroup(pulumiCtx, deployment, tags)
	if err != nil {
		return fmt.Errorf("failed to create resource group: %w", err)
	}

	dependencies := []pulumi.Resource{rg}

	// Create virtual networks
	vnets, err := createVNets(pulumiCtx, deployment, tags, dependencies)
	if err != nil {
		return fmt.Errorf("failed to create virtual networks: %w", err)
	}

	dependencies = append(dependencies, mapToResourceSlice(vnets)...)

	// Create network security groups and rules
	l.Info("Creating network security groups")
	nsgs, err := createNSGs(pulumiCtx, deployment, tags, dependencies)
	if err != nil {
		return fmt.Errorf("failed to create network security groups: %w", err)
	}
	l.Info("Network security groups created successfully")

	// Ensure NSGs are created before proceeding
	var nsgResources []pulumi.Resource
	for _, nsg := range nsgs {
		nsgResources = append(nsgResources, nsg)
	}

	dependencies = append(dependencies, nsgResources...)

	// Create virtual machines, depending on NSGs and VNets
	l.Info("Creating virtual machines")
	err = createVMs(
		pulumiCtx,
		deployment,
		deployment.ResourceGroupName,
		vnets,
		nsgs,
		tags,
		dependencies,
	)
	if err != nil {
		return fmt.Errorf("failed to create virtual machines: %w", err)
	}

	// Create a pulumi.All to wait for all resources to be created
	interfaceResources := make([]interface{}, len(dependencies))
	for i, r := range dependencies {
		interfaceResources[i] = r
	}
	pulumi.All(interfaceResources...).ApplyT(func(args []interface{}) error {
		l.Info("All resources created successfully")
		return nil
	})

	return nil
}

// Helper function to convert map of VNets to a slice of pulumi.Resource
func mapToResourceSlice(m map[string]*network.VirtualNetwork) []pulumi.Resource {
	result := make([]pulumi.Resource, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}

func createResourceGroup(
	pulumiCtx *pulumi.Context,
	deployment *models.Deployment,
	tags pulumi.StringMap,
) (*resources.ResourceGroup, error) {
	l := logger.Get()
	l.Info("Creating resource group")

	rg, err := resources.NewResourceGroup(
		pulumiCtx,
		deployment.ResourceGroupName,
		&resources.ResourceGroupArgs{
			ResourceGroupName: pulumi.String(deployment.ResourceGroupName),
			Location:          pulumi.String(deployment.ResourceGroupLocation),
			Tags:              tags,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource group: %w", err)
	}

	l.Infof("Resource group created: %s", deployment.ResourceGroupName)
	return rg, nil
}

func createVMs(
	pulumiCtx *pulumi.Context,
	deployment *models.Deployment,
	resourceGroupName string,
	vnets map[string]*network.VirtualNetwork,
	nsgs map[string]*network.NetworkSecurityGroup,
	tags pulumi.StringMap,
	dependsOn []pulumi.Resource,
) error {
	l := logger.Get()
	for _, machine := range deployment.Machines {
		for _, param := range machine.Parameters {
			for i := 0; i < param.Count; i++ {
				if err := pulumiCtx.Context().Err(); err != nil {
					return fmt.Errorf("deployment cancelled while creating VMs: %w", err)
				}

				vmName := machine.Name + "-" + fmt.Sprint(i)
				l.Debugf("Creating VM: %s", vmName)

				// Create public IP
				publicIPAddressName := vmName + "-ip"
				publicIP, err := createPublicIP(
					pulumiCtx,
					publicIPAddressName,
					resourceGroupName,
					machine.Location,
					tags,
					dependsOn,
				)
				if err != nil {
					return fmt.Errorf("failed to create public IP for VM %s: %w", vmName, err)
				}

				dependsOn = append(dependsOn, publicIP)

				// Create network interface
				nic, err := createNetworkInterface(
					pulumiCtx,
					vmName,
					resourceGroupName,
					machine.Location,
					vnets[machine.Location],
					publicIP,
					nsgs[machine.Location],
					tags,
					dependsOn,
				)
				if err != nil {
					return fmt.Errorf(
						"failed to create network interface for VM %s: %w",
						vmName,
						err,
					)
				}

				dependsOn = append(dependsOn, nic)

				// Create VM
				vm, err := createVirtualMachine(
					pulumiCtx,
					vmName,
					resourceGroupName,
					machine,
					param,
					nic,
					deployment.SSHPublicKeyData,
					tags,
					dependsOn,
				)
				if err != nil {
					return fmt.Errorf("failed to create VM %s: %w", vmName, err)
				}

				// Use Output.All to wait for the VM to be created before looking up its public IP
				pulumi.All(vm.ID(), publicIP.Name).ApplyT(func(args []interface{}) error {
					vmID := fmt.Sprintf("%v", args[0])
					publicIPName := fmt.Sprintf("%v", args[1])

					publicIPResult, err := network.LookupPublicIPAddress(
						pulumiCtx,
						&network.LookupPublicIPAddressArgs{
							PublicIpAddressName: publicIPName,
							ResourceGroupName:   resourceGroupName,
						},
						pulumi.Parent(vm),
					)
					if err != nil {
						return fmt.Errorf("failed to lookup public IP address: %w", err)
					}

					machine.PublicIP = *publicIPResult.IpAddress
					machine.InstanceID = vmID

					return nil
				})

				l.Infof("VM created successfully: %s", vmName)
			}
		}
	}
	return nil
}

func createPublicIP(
	pulumiCtx *pulumi.Context,
	publicIPAddressName string,
	resourceGroupName string,
	location string,
	tags pulumi.StringMap,
	dependsOn []pulumi.Resource,
) (*network.PublicIPAddress, error) {
	return network.NewPublicIPAddress(
		pulumiCtx,
		publicIPAddressName,
		&network.PublicIPAddressArgs{
			ResourceGroupName: pulumi.String(resourceGroupName),
			Location:          pulumi.String(location),
			Tags:              tags,
		},
		pulumi.DependsOn(dependsOn),
	)
}

func createNetworkInterface(
	pulumiCtx *pulumi.Context,
	name string,
	resourceGroupName string,
	location string,
	vnet *network.VirtualNetwork,
	publicIP *network.PublicIPAddress,
	nsg *network.NetworkSecurityGroup,
	tags pulumi.StringMap,
	dependsOn []pulumi.Resource,
) (*network.NetworkInterface, error) {
	return network.NewNetworkInterface(
		pulumiCtx,
		name+"-nic",
		&network.NetworkInterfaceArgs{
			ResourceGroupName: pulumi.String(resourceGroupName),
			Location:          pulumi.String(location),
			IpConfigurations: network.NetworkInterfaceIPConfigurationArray{
				&network.NetworkInterfaceIPConfigurationArgs{
					Name: pulumi.String("ipconfig"),
					Subnet: &network.SubnetTypeArgs{
						Id: vnet.Subnets.Index(pulumi.Int(0)).Id(),
					},
					PublicIPAddress: &network.PublicIPAddressTypeArgs{
						Id: publicIP.ID(),
					},
				},
			},
			NetworkSecurityGroup: &network.NetworkSecurityGroupTypeArgs{
				Id: nsg.ID(),
			},
			Tags: tags,
		},
		pulumi.DependsOn(dependsOn),
	)
}

func createVirtualMachine(
	pulumiCtx *pulumi.Context,
	name string,
	resourceGroupName string,
	machine models.Machine,
	param models.Parameters,
	nic *network.NetworkInterface,
	sshPublicKeyData []byte,
	tags pulumi.StringMap,
	dependsOn []pulumi.Resource,
) (*compute.VirtualMachine, error) {
	vm, err := compute.NewVirtualMachine(
		pulumiCtx,
		name,
		&compute.VirtualMachineArgs{
			ResourceGroupName: pulumi.String(resourceGroupName),
			Location:          pulumi.String(machine.Location),
			NetworkProfile: &compute.NetworkProfileArgs{
				NetworkInterfaces: compute.NetworkInterfaceReferenceArray{
					&compute.NetworkInterfaceReferenceArgs{
						Id: nic.ID(),
					},
				},
			},
			HardwareProfile: &compute.HardwareProfileArgs{
				VmSize: pulumi.String(param.Type),
			},
			OsProfile: &compute.OSProfileArgs{
				ComputerName:  pulumi.String(machine.ComputerName),
				AdminUsername: pulumi.String("azureuser"),
				LinuxConfiguration: &compute.LinuxConfigurationArgs{
					Ssh: &compute.SshConfigurationArgs{
						PublicKeys: compute.SshPublicKeyTypeArray{
							&compute.SshPublicKeyTypeArgs{
								KeyData: pulumi.String(string(sshPublicKeyData)),
								Path:    pulumi.String("/home/azureuser/.ssh/authorized_keys"),
							},
						},
					},
				},
			},
			StorageProfile: &compute.StorageProfileArgs{
				OsDisk: &compute.OSDiskArgs{
					CreateOption: pulumi.String("FromImage"),
					ManagedDisk: &compute.ManagedDiskParametersArgs{
						StorageAccountType: pulumi.String("Premium_LRS"),
					},
					DiskSizeGB: pulumi.Int(func() int {
						if machine.DiskSizeGB > 0 {
							return int(machine.DiskSizeGB)
						}
						return 30 // Default disk size in GB
					}()),
				},
				ImageReference: &compute.ImageReferenceArgs{
					Publisher: pulumi.String("Canonical"),
					Offer:     pulumi.String("UbuntuServer"),
					Sku:       pulumi.String("18.04-LTS"),
					Version:   pulumi.String("latest"),
				},
			},
			Tags: tags,
		},
		pulumi.DependsOn(dependsOn),
	)
	if err != nil {
		return nil, HandleAzureError(err)
	}
	return vm, nil
}

func createNSGs(
	pulumiCtx *pulumi.Context,
	deployment *models.Deployment,
	tags pulumi.StringMap,
	dependencies []pulumi.Resource,
) (map[string]*network.NetworkSecurityGroup, error) {
	l := logger.Get()
	l.Debug("Creating network security groups")
	nsgs := make(map[string]*network.NetworkSecurityGroup)
	for _, location := range deployment.Locations {
		l.Debugf("Creating network security group for location: %s", location)
		nsgRules := network.SecurityRuleTypeArray{}
		for i, port := range deployment.AllowedPorts {
			l.Debugf("Creating NSG rule for location - %s - port: %d", location, port)
			nsgRules = append(nsgRules, &network.SecurityRuleTypeArgs{
				Name:                     pulumi.Sprintf("Port%d", port),
				Priority:                 pulumi.Int(1000 + i),
				Direction:                pulumi.String("Inbound"),
				Access:                   pulumi.String("Allow"),
				Protocol:                 pulumi.String("Tcp"),
				SourceAddressPrefix:      pulumi.String("*"),
				SourcePortRange:          pulumi.String("*"),
				DestinationAddressPrefix: pulumi.String("*"),
				DestinationPortRange:     pulumi.Sprintf("%d", port),
			})
		}

		nsg, err := network.NewNetworkSecurityGroup(
			pulumiCtx,
			fmt.Sprintf("nsg-%s-%s", deployment.ResourceGroupName, location),
			&network.NetworkSecurityGroupArgs{
				ResourceGroupName: pulumi.String(deployment.ResourceGroupName),
				Location:          pulumi.String(location),
				SecurityRules:     nsgRules,
				Tags:              tags,
			},
			pulumi.DependsOn(dependencies),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to create network security group in %s: %w",
				location,
				err,
			)
		}
		nsgs[location] = nsg
		l.Debugf("Network security group created in %s", location)
	}
	return nsgs, nil
}

func createVNets(
	pulumiCtx *pulumi.Context,
	deployment *models.Deployment,
	tags pulumi.StringMap,
	dependencies []pulumi.Resource,
) (map[string]*network.VirtualNetwork, error) {
	l := logger.Get()
	l.Info("Creating virtual networks")
	vnets := make(map[string]*network.VirtualNetwork)
	for _, location := range deployment.Locations {
		if err := pulumiCtx.Context().Err(); err != nil {
			return nil, fmt.Errorf(
				"deployment cancelled while creating virtual networks: %w",
				err,
			)
		}

		l.Debugf("Creating virtual network in location: %s", location)
		vnet, err := network.NewVirtualNetwork(
			pulumiCtx,
			"vnet-"+location,
			&network.VirtualNetworkArgs{
				ResourceGroupName: pulumi.String(deployment.ResourceGroupName),
				Location:          pulumi.String(location),
				AddressSpace: &network.AddressSpaceArgs{
					AddressPrefixes: pulumi.StringArray{pulumi.String("10.0.0.0/16")},
				},
				Subnets: network.SubnetTypeArray{
					&network.SubnetTypeArgs{
						Name:          pulumi.String("default"),
						AddressPrefix: pulumi.String("10.0.1.0/24"),
					},
				},
				Tags: tags,
			},
			pulumi.DependsOn(dependencies),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to create virtual network in %s: %w",
				location,
				err,
			)
		}
		vnets[location] = vnet
		l.Infof("Virtual network created in %s", location)
	}
	return vnets, nil
}
func printMachineIPTable(deployment *models.Deployment) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Machine Name", "Public IP"})

	for _, machine := range deployment.Machines {
		if machine.PublicIP != "" {
			ipAddress := machine.PublicIP
			if ipAddress != "" {
				table.Append([]string{machine.Name, ipAddress})
			}
		}
	}

	table.Render()
}
