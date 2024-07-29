package azure

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/davecgh/go-spew/spew"
	"github.com/olekukonko/tablewriter"
	"github.com/pulumi/pulumi-azure-native-sdk/compute"
	"github.com/pulumi/pulumi-azure-native-sdk/network"
	"github.com/pulumi/pulumi-azure-native-sdk/resources"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/sanity-io/litter"
)

var progressStream io.Writer

const progressFilePath = "/tmp/progress.log"

// DeployResources deploys Azure resources based on the provided configuration.
// Config should be the Azure subsection of the viper config.
func (p *AzureProvider) DeployResources(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	l := logger.Get()
	l.Info("Starting Azure resource deployment")

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Create a done channel to signal completion
	done := utils.CreateStructChannel(1)

	// Start a goroutine to handle cancellation and completion
	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			l.Info("Deployment cancelled, closing all channels")
			disp.Close()
		case <-done:
			l.Info("Deployment completed, closing display channel")
			if disp != nil {
				disp.Close()
			}
		}
	}()

	// Ensure all goroutines finish before returning
	defer func() {
		utils.CloseChannel(done)
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

	// err := os.Truncate(progressFilePath, 0)
	// if err != nil {
	// 	l.Errorf("Failed to truncate /tmp/process.log: %v", err)
	// }

	//nolint:gomnd // this is a temp file
	eventStream := utils.CreateEventChannel(
		100,
	)

	// Open progressStream
	progressStream, err := os.Create(progressFilePath)
	if err != nil {
		return fmt.Errorf("failed to create progress file: %w", err)
	}
	defer progressStream.Close()

	opts := []optup.Option{
		optup.EventStreams(eventStream),
		optup.ProgressStreams(progressStream),
	}

	// Create a stack (this will create a project if it doesn't exist)
	stack, err := auto.UpsertStackInlineSource(
		ctx,
		stackName,
		projectName,
		runProgram,
	)
	if err != nil {
		return fmt.Errorf("failed to create/update stack: %w", err)
	}

	// Create a go func event loop to watch eventStream
	go func() {
		collect := []events.EngineEvent{}
		eventTypes := []string{}
		eventFilePath := "/tmp/event.log"
		f, err := os.Create(eventFilePath)
		if err != nil {
			fmt.Println(err)
			return
		}
		for d := range eventStream {
			_, err = fmt.Fprintln(f, d)
			collect = append(collect, d)
			eventTypes = append(eventTypes, typeOfEvent(d))
			if err != nil {
				l.Errorf("Failed to write to event file: %v", err)
				f.Close()
				return
			}
		}

		err = f.Close()
		if err != nil {
			l.Errorf("Failed to close event file: %v", err)
			return
		}

		for _, event := range collect {
			if event.SummaryEvent != nil {
				l.Debugf("Summary event: %v", event.SummaryEvent)
				l.Debugf("Open channels: %v", utils.GlobalChannels)
			}
		}
	}()

	// Run the update
	_, err = stack.Up(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to update stack: %w", err)
	}

	deployment.EndTime = time.Now()
	l.Infof(
		"Azure deployment completed successfully in %v",
		deployment.EndTime.Sub(deployment.StartTime),
	)
	// Print all open channels
	for _, channel := range utils.GlobalChannels {
		l.Debugf("Open channel: %v", channel)
	}

	disp.UpdateStatus(&models.Status{
		ID:     deployment.UniqueID,
		Type:   "Deployment",
		Status: "Completed",
	})

	// Wait for IP addresses to be populated or timeout
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			allIPsValid := true
			for _, machine := range deployment.Machines {
				if machine.PublicIP == "" {
					allIPsValid = false
					break
				}
			}
			if allIPsValid {
				// Print the table of machines and IPs
				printMachineIPTable(deployment)
				return nil
			}
		case <-timeout:
			l.Warn("Timeout waiting for IP addresses to be populated")
			printMachineIPTable(deployment)
			return nil
		case <-ctx.Done():
			l.Info("Deployment cancelled while waiting for IP addresses")
			return ctx.Err()
		}
	}
}

func typeOfEvent(event events.EngineEvent) string {
	return reflect.TypeOf(event).String()
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
	vms, err := createVMs(
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

	// Create/truncate progressFilePath
	// err = os.Truncate(progressFilePath, 0)
	// if err != nil {
	// 	l.Errorf("Failed to truncate /tmp/progress.log: %v", err)
	// }

	// Add a single pulumi.All at the end to process events and update status
	pulumi.All(sliceToInterfaceSlice(vms)...).ApplyT(func(args []interface{}) error {
		for _, arg := range args {
			// If ProgressStream is nil, create it
			if progressStream == nil {
				progressStream, err = os.Create(progressFilePath)
				if err != nil {
					l.Errorf("Failed to create progress file: %v", err)
				}
			}
			// Print out information about the resource, to the progress file. Format the resource as an interface, and print
			// it out nicely, with keys and values, with indentation
			l.Debugf(spew.Sprint(arg))
			_, err := progressStream.Write([]byte(litter.Sdump(arg)))
			if err != nil {
				l.Errorf("Failed to write to progress file: %v", err)
			}
		}
		return nil
	})

	return nil
}

// Helper function to convert map of VNets to a slice of pulumi.Resource
func mapToResourceSlice[T pulumi.Resource](m map[string]T) []pulumi.Resource {
	result := make([]pulumi.Resource, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}

func sliceToInterfaceSlice(slice []pulumi.Resource) []interface{} {
	result := make([]interface{}, len(slice))
	for i, v := range slice {
		result[i] = v
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

	disp := display.GetGlobalDisplay()
	for _, machine := range deployment.Machines {
		disp.UpdateStatus(&models.Status{
			ID:     machine.ID,
			Status: "Creating resource group",
		})
	}

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

	// Wait for the resource group to be created
	pulumi.All(rg.ID()).ApplyT(func(args []interface{}) error {
		l.Debugf("Resource group created: %s", deployment.ResourceGroupName)
		return nil
	})

	l.Infof("Resource group created: %s", deployment.ResourceGroupName)
	for _, machine := range deployment.Machines {
		disp.UpdateStatus(&models.Status{
			ID:     machine.ID,
			Status: "RG created - " + deployment.ResourceGroupName,
		})
	}
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
) ([]pulumi.Resource, error) {
	l := logger.Get()
	resourcesToReturn := []pulumi.Resource{}
	for _, machine := range deployment.Machines {
		for _, param := range machine.Parameters {
			for i := 0; i < param.Count; i++ {
				if err := pulumiCtx.Context().Err(); err != nil {
					return nil, fmt.Errorf("deployment cancelled while creating VMs: %w", err)
				}

				vmName := machine.Name + "-" + fmt.Sprint(i)
				l.Debugf("Starting Public IP creation for VM: %s", vmName)

				// Create public IP
				publicIPAddressName := vmName + "-ip"
				publicIP, err := createPublicIP(
					pulumiCtx,
					machine,
					publicIPAddressName,
					resourceGroupName,
					machine.Location,
					tags,
					dependsOn,
				)
				if err != nil {
					return nil, fmt.Errorf("failed to create public IP for VM %s: %w", vmName, err)
				}

				dependsOn = append(dependsOn, publicIP)

				l.Debugf("Starting Network Interface creation for VM: %s", vmName)

				resourcesToReturn = append(resourcesToReturn, publicIP)

				// Create network interface
				nic, err := createNetworkInterface(
					pulumiCtx,
					machine,
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
					return nil, fmt.Errorf(
						"failed to create network interface for VM %s: %w",
						vmName,
						err,
					)
				}

				dependsOn = append(dependsOn, nic)
				resourcesToReturn = append(resourcesToReturn, nic)

				l.Debugf("Starting Virtual Machine creation for VM: %s", vmName)

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
					return nil, fmt.Errorf("failed to start creating VM %s: %w", vmName, err)
				}
				resourcesToReturn = append(resourcesToReturn, vm)
			}
		}
	}
	return resourcesToReturn, nil
}

func createPublicIP(
	pulumiCtx *pulumi.Context,
	machine models.Machine,
	publicIPAddressName string,
	resourceGroupName string,
	location string,
	tags pulumi.StringMap,
	dependsOn []pulumi.Resource,
) (*network.PublicIPAddress, error) {
	l := logger.Get()

	disp := display.GetGlobalDisplay()
	disp.UpdateStatus(&models.Status{
		ID:     machine.ID,
		Status: "Creating public IP in " + location,
	})

	publicIP, err := network.NewPublicIPAddress(
		pulumiCtx,
		publicIPAddressName,
		&network.PublicIPAddressArgs{
			ResourceGroupName: pulumi.String(resourceGroupName),
			Location:          pulumi.String(location),
			Tags:              tags,
		},
		pulumi.DependsOn(dependsOn),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create public IP: %w", err)
	}

	pulumi.All(publicIP.ID()).ApplyT(func(args []interface{}) error {
		l.Infof("Public IP created successfully: %s", publicIPAddressName)
		return nil
	})

	disp.UpdateStatus(&models.Status{
		ID:     machine.ID,
		Status: "Public IP created in " + location,
	})

	return publicIP, nil
}

func createNetworkInterface(
	pulumiCtx *pulumi.Context,
	machine models.Machine,
	name string,
	resourceGroupName string,
	location string,
	vnet *network.VirtualNetwork,
	publicIP *network.PublicIPAddress,
	nsg *network.NetworkSecurityGroup,
	tags pulumi.StringMap,
	dependsOn []pulumi.Resource,
) (*network.NetworkInterface, error) {
	l := logger.Get()
	l.Debugf("Creating network interface in %s for machine %s", location, machine.Name)

	nic, err := network.NewNetworkInterface(
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
	if err != nil {
		return nil, fmt.Errorf("failed to create network interface: %w", err)
	}

	return nic, nil

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
	l := logger.Get()
	l.Debugf("Creating virtual machine in %s for machine %s", machine.Location, machine.Name)

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
	disp := display.GetGlobalDisplay()
	for _, machine := range deployment.Machines {
		disp.UpdateStatus(&models.Status{
			ID:     machine.ID,
			Status: "Creating network security groups",
		})
	}

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
	pulumi.All(sliceToInterfaceSlice(mapToResourceSlice(nsgs))...).
		ApplyT(func(args []interface{}) error {
			l.Info("All network security groups created successfully")
			return nil
		})

	return nsgs, nil
}

func createVNets(
	pulumiCtx *pulumi.Context,
	deployment *models.Deployment,
	tags pulumi.StringMap,
	dependencies []pulumi.Resource,
) (map[string]*network.VirtualNetwork, error) {
	l := logger.Get()
	l.Debugf("Creating virtual networks for deployment: %+v", deployment)
	vnets := make(map[string]*network.VirtualNetwork)
	disp := display.GetGlobalDisplay()
	for _, location := range deployment.Locations {
		for _, machine := range deployment.Machines {
			if machine.Location == location {
				disp.UpdateStatus(&models.Status{
					ID:     machine.ID,
					Status: "Creating VNet - " + location,
				})
			}
		}

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
		for _, machine := range deployment.Machines {
			if machine.Location == location {
				disp.UpdateStatus(&models.Status{
					ID:     machine.ID,
					Status: "VNet created - " + location,
				})
			}
		}
	}

	pulumi.All(sliceToInterfaceSlice(mapToResourceSlice(vnets))...).
		ApplyT(func(args []interface{}) error {
			l.Info("All virtual networks created successfully")
			return nil
		})

	return vnets, nil
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
