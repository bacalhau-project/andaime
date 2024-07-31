package azure

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/pulumi/pulumi-azure-native-sdk/compute"
	"github.com/pulumi/pulumi-azure-native-sdk/network"
	"github.com/pulumi/pulumi-azure-native-sdk/resources"
	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/optup"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

var progressStream io.Writer
var globalDeployment *models.Deployment
var globalPulumiCtx *pulumi.Context

const eventStack = "pulumi:pulumi:Stack"
const eventResourceGroup = "azure-native:resources:ResourceGroup"
const eventVirtualNetwork = "azure-native:network:VirtualNetwork"
const eventNetworkSecurityGroup = "azure-native:network:NetworkSecurityGroup"
const eventNetworkInterface = "azure-native:network:NetworkInterface"
const eventPublicIPAddress = "azure-native:network:PublicIPAddress"
const eventVirtualMachine = "azure-native:compute:VirtualMachine"

var validResourceSlice = []string{
	eventStack,
	eventResourceGroup,
	eventVirtualNetwork,
	eventNetworkSecurityGroup,
	eventNetworkInterface,
	eventPublicIPAddress,
	eventVirtualMachine,
}

const progressFilePath = "/tmp/progress.log"

const WaitForIPAddressesTimeout = 20 * time.Second
const WaitForResourcesTimeout = 2 * time.Minute
const WaitForResourcesTicker = 5 * time.Second

const DefaultDiskSize = 30

const StatusCreating = "creating"
const StatusCreated = "created"

type progressWriter struct {
	ctx      context.Context
	contexts map[string]func()
}

func NewProgressWriter(ctx context.Context) *progressWriter {
	return &progressWriter{
		ctx:      ctx,
		contexts: map[string]func(){},
	}
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	UpdateMachineStatuses(&pw.ctx, string(p))
	return len(p), nil
}

// DeployResources deploys Azure resources based on the provided configuration.
// Config should be the Azure subsection of the viper config.
func (p *AzureProvider) DeployResources(
	ctx context.Context,
	deployment *models.Deployment,
	disp *display.Display,
) error {
	l := logger.Get()

	defer func() {
		if r := recover(); r != nil {
			l.Errorf("Panic occurred during Azure deployment: %v\n%s", r, debug.Stack())
		}
	}()
	deploymentStartTime := time.Now()
	l.Info("Starting Azure resource deployment")

	defer func() {
		if r := recover(); r != nil {
			l.Errorf("Panic occurred during Azure deployment: %v\n%s", r, debug.Stack())
		}
	}()

	// Wrap the entire function in a defer/recover block
	defer func() {
		if r := recover(); r != nil {
			l.Errorf("Panic occurred during Azure deployment: %v\n%s", r, debug.Stack())
		}
	}()

	stackName := deployment.UniqueID
	projectName := "andaime"
	runProgram := func(ctx *pulumi.Context) error {
		return deploymentProgram(ctx, deployment)
	}

	//nolint:gomnd // this is a temp file
	eventStream := utils.CreateEventChannel("azure_event_stream", 100)
	l.Debugf("Channel created: azure_event_stream")

	// Open progressStream
	progressStream, err := os.Create(progressFilePath)
	if err != nil {
		return fmt.Errorf("failed to create progress file: %w", err)
	}
	defer progressStream.Close()

	progressWriter := NewProgressWriter(ctx)

	opts := []optup.Option{
		optup.EventStreams(eventStream),
		optup.ProgressStreams(progressStream, progressWriter),
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

	var provisionedVMs []*compute.VirtualMachine
	pollVMChan := utils.CreateVMChannel("poll_vm_chan", len(deployment.Machines))

	// Create a go func event loop to watch eventStream
	go func() {
		defer func() {
			l.Debugf("Closing channel: azure_event_stream")
			utils.CloseChannel(eventStream)
		}()
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

			// Check if the event is a ResourcePreEvent and the step is "create"
			if d.ResourcePreEvent != nil &&
				d.ResourcePreEvent.Metadata.Op == apitype.OpCreate {
				// Check if the resource is a VirtualMachine
				if d.ResourcePreEvent.Metadata.Type == "azure:compute/virtualMachine:VirtualMachine" {
					name := d.ResourcePreEvent.Metadata.URN
					virtualMachine := &compute.VirtualMachine{}
					provisionedVMs = append(provisionedVMs, virtualMachine)
					l.Debugf("Added VM to provisionedVMs: %s", name)
					pollVMChan <- virtualMachine
				}
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
				l.Debugf("Total provisioned VMs: %d", len(provisionedVMs))
				break
			}
		}
	}()

	// Run the update
	globalDeployment = deployment
	_, err = stack.Up(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to update stack: %w", err)
	}

	deployment.EndTime = time.Now()
	l.Infof(
		"Azure deployment completed successfully in %v",
		deployment.EndTime.Sub(deployment.StartTime),
	)
	printMachineIPTable(deployment)

	// Close all channels
	l.Debugf("Closing all channels")
	utils.CloseAllChannels()

	deploymentDuration := time.Since(deploymentStartTime)
	l.Infof("Deployment completed successfully in %v", deploymentDuration)
	return nil
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
	vnetResources := mapToResourceSlice(vnets)

	dependencies = append(dependencies, vnetResources...)

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
	resourcesToReturn, err := createVMs(
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

	// Add a single pulumi.All at the end to process events and update status
	allResources := []pulumi.Resource{}
	allResources = append(allResources, rg)
	allResources = append(allResources, vnetResources...)
	allResources = append(allResources, nsgResources...)
	allResources = append(allResources, resourcesToReturn...)

	pulumi.All(sliceToInterfaceSlice(allResources)...).ApplyT(func(args []interface{}) error {
		for _, arg := range args {
			// If ProgressStream is nil, create it
			if progressStream == nil {
				progressStream, err = os.Create(progressFilePath)
				if err != nil {
					l.Errorf("Failed to create progress file: %v", err)
				}
			}
			argType := reflect.TypeOf(arg)
			l.Debugf("%s: Arg received - %v", time.Now().Format(time.RFC3339), argType)
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

	for _, machine := range deployment.Machines {
		disp.UpdateStatus(&models.Status{
			ID:     machine.ID,
			Status: "RG created - " + deployment.ResourceGroupName,
		})
	}
	return rg, nil
}

// Returns a list of all resources to watch
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
		if err := pulumiCtx.Context().Err(); err != nil {
			return nil, fmt.Errorf("deployment cancelled while creating VMs: %w", err)
		}

		vmName := machine.Name
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
	nic *network.NetworkInterface,
	sshPublicKeyData []byte,
	tags pulumi.StringMap,
	dependsOn []pulumi.Resource,
) (*compute.VirtualMachine, error) {
	l := logger.Get()
	l.Debugf("Creating virtual machine in %s for machine %s", machine.Location, machine.Name)

	vm, _ := compute.NewVirtualMachine(
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
				VmSize: pulumi.String(machine.VMSize),
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
						return DefaultDiskSize
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
				Priority:                 pulumi.Int(basePriority + i),
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

func UpdateMachineStatuses(ctx *context.Context, event string) {
	l := logger.Get()
	disp := display.GetGlobalDisplay()

	pattern := `(?m)^.*\s([a-zA-Z0-9-:]+)\s+([a-zA-Z0-9-:]+)+\s+(creating|created).*`
	re := regexp.MustCompile(pattern)

	deployment := globalDeployment

	var resourceType, resourceName, status string
	if re.MatchString(event) {
		matches := re.FindStringSubmatch(event)
		resourceType = matches[1]
		resourceName = matches[2]
		status = matches[3]
	} else {
		return
	}

	if !slices.Contains(validResourceSlice, resourceType) {
		// l.Debugf("Invalid resource type: %s", resourceType)
		return
	}

	var machineID, location string
	if strings.HasPrefix(resourceName, "vm-") {
		vmNameParts := strings.Split(resourceName, "-")
		machineID = vmNameParts[1]
	} else {
		locationParts := strings.Split(resourceName, "-")
		location = locationParts[len(locationParts)-1]
	}

	switch resourceType {
	case "pulumi:pulumi:Stack":
		// If the resource type is a Stack, we need to update the deployment status
		l.Debugf("Deployment started, put it on all machines")
		var msg string
		if status == StatusCreating {
			msg = fmt.Sprintf("Creating Stack %s deployment ...", resourceName)
		} else {
			msg = fmt.Sprint("Completed.", resourceName)
		}

		for _, machine := range deployment.Machines {
			disp.UpdateStatus(&models.Status{
				ID:     machine.ID,
				Status: msg,
			})
		}
	case "azure-native:resources:ResourceGroup":
		l.Debugf("RG started, put it on all machines")
		var msg string
		if status == StatusCreating {
			msg = fmt.Sprintf("Creating RG %s ...", resourceName)
		} else {
			msg = fmt.Sprintf("Creating RG %s ... Done ✅", resourceName)
		}

		for _, machine := range deployment.Machines {
			disp.UpdateStatus(&models.Status{
				ID:     machine.ID,
				Status: msg,
			})
		}
	case "azure-native:network:VirtualNetwork":
		l.Debugf("VNet started, put it on all machines in a location: %s", location)
		var msg string
		if status == StatusCreating {
			msg = fmt.Sprintf("Creating VNet %s ...", resourceName)
		} else {
			msg = fmt.Sprintf("Creating VNet %s ... Done ✅", resourceName)
		}

		for _, machine := range deployment.Machines {
			if machine.Location == location {
				disp.UpdateStatus(&models.Status{
					ID:     machine.ID,
					Status: msg,
				})
			}
		}
	case "azure-native:network:NetworkSecurityGroup":
		l.Debugf("NSG started, put it on all machines in a location: %s", location)
		var msg string
		if status == StatusCreating {
			msg = fmt.Sprintf("Creating NSG %s ...", resourceName)
		} else {
			msg = fmt.Sprintf("Creating NSG %s ... Done ✅", resourceName)
		}

		for _, machine := range deployment.Machines {
			if machine.Location == location {
				disp.UpdateStatus(&models.Status{
					ID:     machine.ID,
					Status: msg,
				})
			}
		}
	case "azure-native:network:PublicIPAddress":
		l.Debugf("PublicIP started: %s", resourceName)

		var msg string
		if status == StatusCreating {
			msg = fmt.Sprintf("Creating PublicIP %s ...", resourceName)
		} else {
			msg = fmt.Sprintf("Creating PublicIP %s ... Done ✅", resourceName)
		}

		disp.UpdateStatus(&models.Status{
			ID:     machineID,
			Status: msg,
		})

	case "azure-native:network:NetworkInterface":
		l.Debugf("NIC started: %s", resourceName)

		isDone := false

		var msg string
		if status == StatusCreating {
			msg = fmt.Sprintf("Creating NIC %s ...", resourceName)
		} else {
			msg = fmt.Sprintf("Creating NIC %s ... Done ✅", resourceName)
			isDone = true
		}

		disp.UpdateStatus(&models.Status{
			ID:     machineID,
			Status: msg,
		})

		if isDone {
			// Update the NIC on the Machine
			for i, machine := range deployment.Machines {
				if machine.ID == machineID {
					deployment.Machines[i].NIC = resourceName
				}
			}
		}

	case "azure-native:compute:VirtualMachine":
		// If the resource type is a VirtualMachine, we need to update machine statuses
		l.Debugf("VM started: %s", resourceName)

		isDone := false

		var msg string
		if status == StatusCreating {
			msg = fmt.Sprintf("Creating VM %s ...", resourceName)
		} else {
			msg = fmt.Sprintf("Creating VM %s ... Done ✅", resourceName)
			isDone = true
		}

		disp.UpdateStatus(&models.Status{
			ID:     machineID,
			Status: msg,
		})
		if isDone {
			pulumi.Run(func(pulumiCtx *pulumi.Context) error {
				for i, machine := range deployment.Machines {
					if machine.ID == machineID {
						// Use the NIC to get the IP address
						nic, err := network.LookupNetworkInterface(
							pulumiCtx,
							&network.LookupNetworkInterfaceArgs{
								ResourceGroupName:    deployment.ResourceGroupName,
								NetworkInterfaceName: machine.NIC,
							},
						)
						if err != nil {
							l.Debugf("Failed to get NIC: %v", err)
							continue
						}

						if len(nic.IpConfigurations) > 0 &&
							nic.IpConfigurations[0].PublicIPAddress != nil {
							machine.PublicIP = *nic.IpConfigurations[0].PublicIPAddress.IpAddress
							machine.PrivateIP = *nic.IpConfigurations[0].PrivateIPAddress
						}

						l.Debugf(
							"Public IP: %s, Private IP: %s",
							machine.PublicIP,
							machine.PrivateIP,
						)
						go func() {
							l.Debugf(
								"Starting SSH check for machine %s with IP %s",
								machine.ID,
								machine.PublicIP,
							)
							sshKeyMaterial, err := os.ReadFile(deployment.SSHPrivateKeyPath)
							if err != nil {
								l.Errorf(
									"Failed to read SSH private key for machine %s: %v",
									machine.ID,
									err,
								)
								return
							}
							l.Debugf("SSH key read successfully for machine %s", machine.ID)

							err = sshutils.WaitForSSHToBeLive(&sshutils.SSHConfig{
								Host:               machine.PublicIP,
								Port:               deployment.SSHPort,
								PrivateKeyMaterial: string(sshKeyMaterial),
							}, sshutils.SSHRetryAttempts, sshutils.SSHRetryDelay)

							if err != nil {
								l.Errorf(
									"SSH connection failed for machine %s: %v",
									machine.ID,
									err,
								)
								disp.UpdateStatus(&models.Status{
									ID: machine.ID,
									Status: fmt.Sprintf(
										"SSH connection failed after %d seconds",
										sshutils.SSHRetryAttempts*int(
											sshutils.SSHRetryDelay.Seconds(),
										),
									),
									PublicIP:  "---",
									PrivateIP: "---",
								})
								deployment.Machines[i].Status = models.MachineStatusFailed
							} else {
								l.Infof("SSH connection successful for machine %s", machine.ID)

								disp.UpdateStatus(&models.Status{
									ID:        machine.ID,
									Status:    "SSH connection successful",
									PublicIP:  machine.PublicIP,
									PrivateIP: machine.PrivateIP,
								})
								deployment.Machines[i].Status = models.MachineStatusComplete
							}
							l.Debugf("SSH check completed for machine %s", machine.ID)
						}()
					}
				}
				return nil
			})
		}
	}
}
