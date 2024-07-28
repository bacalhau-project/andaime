package azure

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/compute"
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/network"
	"github.com/pulumi/pulumi-azure-native/sdk/go/azure/resources"
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

	// Create virtual networks
	vnets, err := createVNets(pulumiCtx, deployment, rg.Name, tags)
	if err != nil {
		return fmt.Errorf("failed to create virtual networks: %w", err)
	}

	// Create network security groups and rules
	l.Info("Creating network security groups")
	nsgs, err := createNSGs(pulumiCtx, deployment, rg.Name, tags)
	if err != nil {
		return fmt.Errorf("failed to create network security groups: %w", err)
	}
	l.Info("Network security groups created successfully")

	// Ensure NSGs are created before proceeding
	var nsgResources []pulumi.Resource
	for _, nsg := range nsgs {
		nsgResources = append(nsgResources, nsg)
	}

	// Create virtual machines, depending on NSGs
	l.Info("Creating virtual machines")
	err = createVMs(pulumiCtx, deployment, rg.Name, vnets, nsgs, tags, nsgResources)
	if err != nil {
		return fmt.Errorf("failed to create virtual machines: %w", err)
	}

	return nil
}

// func createVMs(
// 	pulumiCtx *pulumi.Context,
// 	deployment *models.Deployment,
// 	resourceGroupName pulumi.StringInput,
// 	vnets map[string]*network.VirtualNetwork,
// 	nsg *network.NetworkSecurityGroup,
// 	tags pulumi.StringMap,
// ) error {
// 	l := logger.Get()
// 	for _, machine := range deployment.Machines {
// 		for _, param := range machine.Parameters {
// 			for i := 0; i < param.Count; i++ {
// 				if err := pulumiCtx.Context().Err(); err != nil {
// 					return fmt.Errorf("deployment cancelled while creating VMs: %w", err)
// 				}

// 				vmName := machine.Name + "-" + fmt.Sprint(i)
// 				l.Debugf("Creating VM: %s", vmName)

// 				// Create public IP
// 				publicIP, err := createPublicIP(
// 					pulumiCtx,
// 					vmName,
// 					resourceGroupName,
// 					machine.Location,
// 					tags,
// 				)
// 				if err != nil {
// 					return fmt.Errorf("failed to create public IP for VM %s: %w", vmName, err)
// 				}

// 				// Create network interface
// 				nic, err := createNetworkInterface(
// 					pulumiCtx,
// 					vmName,
// 					resourceGroupName,
// 					machine.Location,
// 					vnets[machine.Location],
// 					publicIP,
// 					nsg,
// 					tags,
// 				)
// 				if err != nil {
// 					return fmt.Errorf(
// 						"failed to create network interface for VM %s: %w",
// 						vmName,
// 						err,
// 					)
// 				}

// 				// Create VM
// 				err = createVirtualMachine(
// 					pulumiCtx,
// 					vmName,
// 					resourceGroupName,
// 					machine,
// 					param,
// 					nic,
// 					deployment.SSHPublicKeyData,
// 					tags,
// 				)
// 				if err != nil {
// 					return fmt.Errorf("failed to create VM %s: %w", vmName, err)
// 				}

// 				l.Infof("VM created successfully: %s", vmName)
// 			}
// 		}
// 	}
// 	return nil
// }

// func createPublicIP(
// 	pulumiCtx *pulumi.Context,
// 	name string,
// 	resourceGroupName pulumi.StringInput,
// 	location string,
// 	tags pulumi.StringMap,
// ) (*network.PublicIPAddress, error) {
// 	return network.NewPublicIPAddress(
// 		pulumiCtx,
// 		name+"-ip",
// 		&network.PublicIPAddressArgs{
// 			ResourceGroupName: resourceGroupName,
// 			Location:          pulumi.String(location),
// 			Tags:              tags,
// 		},
// 	)
// }

// func createNetworkInterface(
// 	pulumiCtx *pulumi.Context,
// 	name string,
// 	resourceGroupName pulumi.StringInput,
// 	location string,
// 	vnet *network.VirtualNetwork,
// 	publicIP *network.PublicIPAddress,
// 	nsg *network.NetworkSecurityGroup,
// 	tags pulumi.StringMap,
// ) (*network.NetworkInterface, error) {
// 	return network.NewNetworkInterface(
// 		pulumiCtx,
// 		name+"-nic",
// 		&network.NetworkInterfaceArgs{
// 			ResourceGroupName: resourceGroupName,
// 			Location:          pulumi.String(location),
// 			IpConfigurations: network.NetworkInterfaceIPConfigurationArray{
// 				&network.NetworkInterfaceIPConfigurationArgs{
// 					Name: pulumi.String("ipconfig"),
// 					Subnet: &network.SubnetTypeArgs{
// 						Id: vnet.Subnets.Index(pulumi.Int(0)).Id(),
// 					},
// 					PublicIPAddress: &network.PublicIPAddressTypeArgs{
// 						Id: publicIP.ID(),
// 					},
// 				},
// 			},
// 			NetworkSecurityGroup: &network.NetworkSecurityGroupTypeArgs{
// 				Id: nsg.ID(),
// 			},
// 			Tags: tags,
// 		},
// 	)
// }

// func createVirtualMachine(
// 	pulumiCtx *pulumi.Context,
// 	name string,
// 	resourceGroupName pulumi.StringInput,
// 	machine models.Machine,
// 	param models.Parameters,
// 	nic *network.NetworkInterface,
// 	sshPublicKeyData []byte,
// 	tags pulumi.StringMap,
// ) error {
// 	_, err := compute.NewVirtualMachine(
// 		pulumiCtx,
// 		name,
// 		&compute.VirtualMachineArgs{
// 			ResourceGroupName: resourceGroupName,
// 			Location:          pulumi.String(machine.Location),
// 			NetworkProfile: &compute.NetworkProfileArgs{
// 				NetworkInterfaces: compute.NetworkInterfaceReferenceArray{
// 					&compute.NetworkInterfaceReferenceArgs{
// 						Id: nic.ID(),
// 					},
// 				},
// 			},
// 			HardwareProfile: &compute.HardwareProfileArgs{
// 				VmSize: pulumi.String(param.Type),
// 			},
// 			OsProfile: &compute.OSProfileArgs{
// 				ComputerName:  pulumi.String(machine.ComputerName),
// 				AdminUsername: pulumi.String("azureuser"),
// 				LinuxConfiguration: &compute.LinuxConfigurationArgs{
// 					Ssh: &compute.SshConfigurationArgs{
// 						PublicKeys: compute.SshPublicKeyTypeArray{
// 							&compute.SshPublicKeyTypeArgs{
// 								KeyData: pulumi.String(string(sshPublicKeyData)),
// 								Path:    pulumi.String("/home/azureuser/.ssh/authorized_keys"),
// 							},
// 						},
// 					},
// 				},
// 			},
// 			StorageProfile: &compute.StorageProfileArgs{
// 				OsDisk: &compute.OSDiskArgs{
// 					CreateOption: pulumi.String("FromImage"),
// 					ManagedDisk: &compute.ManagedDiskParametersArgs{
// 						StorageAccountType: pulumi.String("Premium_LRS"),
// 					},
// 					DiskSizeGB: pulumi.Int(machine.DiskSizeGB),
// 				},
// 				ImageReference: &compute.ImageReferenceArgs{
// 					Publisher: pulumi.String("Canonical"),
// 					Offer:     pulumi.String("UbuntuServer"),
// 					Sku:       pulumi.String("18.04-LTS"),
// 					Version:   pulumi.String("latest"),
// 				},
// 			},
// 			Tags: tags,
// 		},
// 	)
// 	return err
// }

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
	resourceGroupName pulumi.StringInput,
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
				publicIP, err := createPublicIP(
					pulumiCtx,
					vmName,
					resourceGroupName,
					machine.Location,
					tags,
				)
				if err != nil {
					return fmt.Errorf("failed to create public IP for VM %s: %w", vmName, err)
				}

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
				)
				if err != nil {
					return fmt.Errorf(
						"failed to create network interface for VM %s: %w",
						vmName,
						err,
					)
				}

				// Create VM
				err = createVirtualMachine(
					pulumiCtx,
					vmName,
					resourceGroupName,
					machine,
					param,
					nic,
					deployment.SSHPublicKeyData,
					tags,
				)
				if err != nil {
					return fmt.Errorf("failed to create VM %s: %w", vmName, err)
				}

				l.Infof("VM created successfully: %s", vmName)
			}
		}
	}
	return nil
}

func createPublicIP(
	pulumiCtx *pulumi.Context,
	name string,
	resourceGroupName pulumi.StringInput,
	location string,
	tags pulumi.StringMap,
) (*network.PublicIPAddress, error) {
	return network.NewPublicIPAddress(
		pulumiCtx,
		name+"-ip",
		&network.PublicIPAddressArgs{
			ResourceGroupName: resourceGroupName,
			Location:          pulumi.String(location),
			Tags:              tags,
		},
	)
}

func createNetworkInterface(
	pulumiCtx *pulumi.Context,
	name string,
	resourceGroupName pulumi.StringInput,
	location string,
	vnet *network.VirtualNetwork,
	publicIP *network.PublicIPAddress,
	nsg *network.NetworkSecurityGroup,
	tags pulumi.StringMap,
) (*network.NetworkInterface, error) {
	return network.NewNetworkInterface(
		pulumiCtx,
		name+"-nic",
		&network.NetworkInterfaceArgs{
			ResourceGroupName: resourceGroupName,
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
	)
}

func createVirtualMachine(
	pulumiCtx *pulumi.Context,
	name string,
	resourceGroupName pulumi.StringInput,
	machine models.Machine,
	param models.Parameters,
	nic *network.NetworkInterface,
	sshPublicKeyData []byte,
	tags pulumi.StringMap,
) error {
	_, err := compute.NewVirtualMachine(
		pulumiCtx,
		name,
		&compute.VirtualMachineArgs{
			ResourceGroupName: resourceGroupName,
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
	)
	return err
}

func createNSGs(
	pulumiCtx *pulumi.Context,
	deployment *models.Deployment,
	resourceGroupName pulumi.StringInput,
	tags pulumi.StringMap,
) (map[string]*network.NetworkSecurityGroup, error) {
	nsgs := make(map[string]*network.NetworkSecurityGroup)
	for _, location := range deployment.Locations {
		nsgRules := network.SecurityRuleTypeArray{}
		for i, port := range deployment.AllowedPorts {
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
				ResourceGroupName: resourceGroupName,
				Location:          pulumi.String(location),
				SecurityRules:     nsgRules,
				Tags:              tags,
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to create network security group in %s: %w",
				location,
				err,
			)
		}
		nsgs[location] = nsg
	}
	return nsgs, nil
}

func createVNets(
	pulumiCtx *pulumi.Context,
	deployment *models.Deployment,
	resourceGroupName pulumi.StringInput,
	tags pulumi.StringMap,
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
				ResourceGroupName: resourceGroupName,
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
