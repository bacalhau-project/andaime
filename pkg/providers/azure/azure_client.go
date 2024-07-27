//nolint:lll
package azure

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/bacalhau-project/andaime/pkg/logger"
)

type AzureClient interface {
	// Resource Group API
	GetOrCreateResourceGroup(
		ctx context.Context,
		location, name string,
		tags map[string]*string,
	) (*armresources.ResourceGroup, error)
	DestroyResourceGroup(ctx context.Context, resourceGroupName string) error

	CreateVirtualNetwork(
		ctx context.Context,
		resourceGroupName string,
		location string,
		tags map[string]*string,
	) (armnetwork.VirtualNetwork, error)
	GetVirtualNetwork(
		ctx context.Context,
		resourceGroupName string,
		vnetName string,
		location string,
	) (armnetwork.VirtualNetwork, error)
	CreatePublicIP(
		ctx context.Context,
		resourceGroupName string,
		location string,
		machineID string,
		tags map[string]*string,
	) (armnetwork.PublicIPAddress, error)
	GetPublicIP(
		ctx context.Context,
		resourceGroupName string,
		ipName string,
		location string,
	) (armnetwork.PublicIPAddress, error)
	CreateVirtualMachine(
		ctx context.Context,
		resourceGroupName string,
		location string,
		machineID string,
		parameters armcompute.VirtualMachine,
		tags map[string]*string,
	) (armcompute.VirtualMachine, error)
	GetVirtualMachine(
		ctx context.Context,
		resourceGroupName string,
		location string,
		vmName string,
	) (armcompute.VirtualMachine, error)
	CreateNetworkInterface(
		ctx context.Context,
		resourceGroupName string,
		location string,
		machineID string,
		tags map[string]*string,
		subnet *armnetwork.Subnet,
		publicIP *armnetwork.PublicIPAddress,
		nsg *armnetwork.SecurityGroup,
	) (armnetwork.Interface, error)
	GetNetworkInterface(
		ctx context.Context,
		resourceGroupName string,
		nicName string,
	) (armnetwork.Interface, error)
	CreateNetworkSecurityGroup(
		ctx context.Context,
		resourceGroupName string,
		location string,
		ports []int,
		tags map[string]*string,
	) (armnetwork.SecurityGroup, error)
	GetNetworkSecurityGroup(
		ctx context.Context,
		resourceGroupName string,
		nsgName string,
	) (armnetwork.SecurityGroup, error)

	// ResourceGraphClientAPI
	SearchResources(
		ctx context.Context,
		resourceGroup string,
		subscriptionID string,
		tags map[string]*string,
	) (armresourcegraph.ClientResourcesResponse, error)

	// Subscriptions API
	NewSubscriptionListPager(
		ctx context.Context,
		options *armsubscription.SubscriptionsClientListOptions,
	) *runtime.Pager[armsubscription.SubscriptionsClientListResponse]
}

// LiveAzureClient wraps all Azure SDK calls
type LiveAzureClient struct {
	virtualNetworksClient   *armnetwork.VirtualNetworksClient
	publicIPAddressesClient *armnetwork.PublicIPAddressesClient
	interfacesClient        *armnetwork.InterfacesClient
	virtualMachinesClient   *armcompute.VirtualMachinesClient
	securityGroupsClient    *armnetwork.SecurityGroupsClient
	resourceGroupsClient    *armresources.ResourceGroupsClient
	resourceGraphClient     *armresourcegraph.Client
	subscriptionsClient     *armsubscription.SubscriptionsClient
	nsgCache                sync.Map
}

type nsgCacheEntry struct {
	wg     sync.WaitGroup
	result armnetwork.SecurityGroup
	err    error
}

var NewAzureClientFunc = NewAzureClient

// NewAzureClient creates a new AzureClient
func NewAzureClient(subscriptionID string) (AzureClient, error) {
	// Get credential from CLI
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	// Create Azure clients
	virtualNetworksClient, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	publicIPAddressesClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	interfacesClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	virtualMachinesClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	securityGroupsClient, err := armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	resourceGroupsClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	subscriptionsClient, err := armsubscription.NewSubscriptionsClient(cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}
	resourceGraphClient, err := armresourcegraph.NewClient(cred, nil)
	if err != nil {
		return &LiveAzureClient{}, err
	}

	return &LiveAzureClient{
		virtualNetworksClient:   virtualNetworksClient,
		publicIPAddressesClient: publicIPAddressesClient,
		interfacesClient:        interfacesClient,
		virtualMachinesClient:   virtualMachinesClient,
		securityGroupsClient:    securityGroupsClient,
		resourceGroupsClient:    resourceGroupsClient,
		resourceGraphClient:     resourceGraphClient,
		subscriptionsClient:     subscriptionsClient,
	}, nil
}

func getVnetName(resourceGroupName, location string) string {
	return resourceGroupName + "-" + location + "-vnet"
}

func getSubnetName(resourceGroupName, location string) string {
	return resourceGroupName + "-" + location + "-subnet"
}

// CreateVirtualNetwork creates a new virtual network
func (c *LiveAzureClient) CreateVirtualNetwork(ctx context.Context,
	resourceGroupName, location string,
	tags map[string]*string) (armnetwork.VirtualNetwork, error) {
	vnetName := getVnetName(resourceGroupName, location)
	subnetName := getSubnetName(resourceGroupName, location)

	l := logger.Get()
	l.Debugf("CreateVirtualNetwork: %s", vnetName)
	subnet := armnetwork.Subnet{
		Name: to.Ptr(subnetName),
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr("10.0.0.0/24"),
		},
	}

	vnet := armnetwork.VirtualNetwork{
		Name:     to.Ptr(vnetName),
		Location: to.Ptr(location),
		Tags:     tags,
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{to.Ptr("10.0.0.0/16")},
			},
			Subnets: []*armnetwork.Subnet{&subnet},
		},
	}

	poller, err := c.virtualNetworksClient.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		vnetName,
		vnet,
		nil,
	)
	if err != nil {
		logger.LogAzureAPIEnd("CreateVirtualNetwork", err)
		return armnetwork.VirtualNetwork{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	logger.LogAzureAPIEnd("CreateVirtualNetwork", err)
	if err != nil {
		return armnetwork.VirtualNetwork{}, err
	}

	l.Debugf("CreateVirtualNetwork: %s", *resp.VirtualNetwork.Name)

	return resp.VirtualNetwork, nil
}

// GetVirtualNetwork retrieves a virtual network
func (c *LiveAzureClient) GetVirtualNetwork(ctx context.Context,
	resourceGroupName, location, vnetName string) (armnetwork.VirtualNetwork, error) {
	l := logger.Get()
	l.Debugf("GetVirtualNetwork: %s", vnetName)
	resp, err := c.virtualNetworksClient.Get(ctx, resourceGroupName, vnetName, nil)
	if err != nil {
		return armnetwork.VirtualNetwork{}, err
	}

	return resp.VirtualNetwork, nil
}

func getPublicIPName(machineID string) string {
	return machineID + "-ip"
}

// CreatePublicIP creates a new public IP address
func (c *LiveAzureClient) CreatePublicIP(ctx context.Context,
	resourceGroupName string,
	location string,
	machineID string,
	tags map[string]*string) (armnetwork.PublicIPAddress, error) {
	l := logger.Get()
	ipName := getPublicIPName(machineID)
	l.Debugf("CreatePublicIP: %s", ipName)

	parameters := armnetwork.PublicIPAddress{
		Name:     to.Ptr(ipName),
		Location: to.Ptr(location),
		Tags:     tags,
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}

	poller, err := c.publicIPAddressesClient.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		ipName,
		parameters,
		nil,
	)
	if err != nil {
		l.Errorf("CreatePublicIP: %s", err)
		return armnetwork.PublicIPAddress{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	l.Debugf("CreatePublicIP: %s", *resp.PublicIPAddress.Name)
	if err != nil {
		return armnetwork.PublicIPAddress{}, err
	}

	return resp.PublicIPAddress, nil
}

func (c *LiveAzureClient) GetPublicIP(ctx context.Context,
	resourceGroupName, location, ipName string) (armnetwork.PublicIPAddress, error) {
	l := logger.Get()
	l.Debugf("GetPublicIP: %s", ipName)
	resp, err := c.publicIPAddressesClient.Get(ctx, resourceGroupName, ipName, nil)
	if err != nil {
		return armnetwork.PublicIPAddress{}, err
	}
	return resp.PublicIPAddress, nil
}

func getVMName(machineID string) string {
	return machineID + "-vm"
}

func (c *LiveAzureClient) CreateVirtualMachine(ctx context.Context,
	resourceGroupName string,
	location string,
	machineID string,
	parameters armcompute.VirtualMachine,
	tags map[string]*string) (armcompute.VirtualMachine, error) {
	vmName := getVMName(machineID)

	l := logger.Get()
	l.Debugf("CreateVirtualMachine: %s", vmName)

	poller, err := c.virtualMachinesClient.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		vmName,
		parameters,
		nil,
	)
	if err != nil {
		l.Errorf("CreateVirtualMachine: %s", err)
		return armcompute.VirtualMachine{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	l.Debugf("CreateVirtualMachine: %s", vmName)
	if err != nil {
		l.Errorf("CreateVirtualMachine: %s", err)
		return armcompute.VirtualMachine{}, err
	}

	return resp.VirtualMachine, nil
}

func (c *LiveAzureClient) GetVirtualMachine(
	ctx context.Context,
	resourceGroupName, location, vmName string,
) (armcompute.VirtualMachine, error) {
	l := logger.Get()
	l.Debugf("GetVirtualMachine: %s", vmName)
	resp, err := c.virtualMachinesClient.Get(ctx, resourceGroupName, vmName, nil)
	if err != nil {
		return armcompute.VirtualMachine{}, err
	}

	return resp.VirtualMachine, nil
}

func getNetworkInterfaceName(machineID string) string {
	return machineID + "-nic"
}

func (c *LiveAzureClient) CreateNetworkInterface(ctx context.Context,
	resourceGroupName, location string,
	machineID string,
	tags map[string]*string,
	subnet *armnetwork.Subnet,
	publicIP *armnetwork.PublicIPAddress,
	nsg *armnetwork.SecurityGroup,
) (armnetwork.Interface, error) {
	nicName := getNetworkInterfaceName(machineID)

	l := logger.Get()
	l.Debugf("CreateNetworkInterface: %s", nicName)

	parameters := armnetwork.Interface{
		Location: to.Ptr(location),
		Tags:     tags,
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet:                    subnet,
						PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
						PublicIPAddress:           publicIP,
					},
				},
			},
			NetworkSecurityGroup: nsg,
		},
	}
	poller, err := c.interfacesClient.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		nicName,
		parameters,
		nil,
	)
	if err != nil {
		logger.LogAzureAPIEnd("CreateNetworkInterface", err)
		return armnetwork.Interface{}, err
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	logger.LogAzureAPIEnd("CreateNetworkInterface", err)
	if err != nil {
		return armnetwork.Interface{}, err
	}

	return resp.Interface, nil
}

func (c *LiveAzureClient) GetNetworkInterface(
	ctx context.Context,
	resourceGroupName, nicName string,
) (armnetwork.Interface, error) {
	logger.LogAzureAPIStart("GetNetworkInterface")

	resp, err := c.interfacesClient.Get(
		ctx,
		resourceGroupName,
		nicName,
		nil,
	)
	logger.LogAzureAPIEnd("GetNetworkInterface", err)
	if err != nil {
		return armnetwork.Interface{}, err
	}

	return resp.Interface, nil
}

func getNetworkSecurityGroupName(resourceGroupName string) string {
	return resourceGroupName + "-nsg"
}

func (c *LiveAzureClient) CreateNetworkSecurityGroup(ctx context.Context,
	resourceGroupName string,
	location string,
	ports []int,
	tags map[string]*string,
) (armnetwork.SecurityGroup, error) {
	sgName := getNetworkSecurityGroupName(resourceGroupName)

	l := logger.Get()
	l.Debugf("CreateNetworkSecurityGroup: Starting for %s", resourceGroupName)
	logger.LogAzureAPIStart("CreateNetworkSecurityGroup")

	defer func() {
		if r := recover(); r != nil {
			l.Errorf("CreateNetworkSecurityGroup: Panic recovered: %v", r)
			debug.PrintStack()
			logger.WriteToDebugLog(fmt.Sprintf("Panic in CreateNetworkSecurityGroup: %v\n%s", r, debug.Stack()))
		}
	}()

	// Use LoadOrStore to ensure only one goroutine creates the cache entry
	entry, loaded := c.nsgCache.LoadOrStore(sgName, &nsgCacheEntry{})
	cacheEntry := entry.(*nsgCacheEntry)

	if loaded {
		// If the entry was already in the cache, wait for the result
		l.Debugf("CreateNetworkSecurityGroup: Waiting for existing operation for %s", sgName)
		cacheEntry.wg.Wait()
		l.Debugf("CreateNetworkSecurityGroup: Existing operation completed for %s", sgName)
		return cacheEntry.result, cacheEntry.err
	}

	// This goroutine is responsible for creating the NSG
	cacheEntry.wg.Add(1)
	defer cacheEntry.wg.Done()

	// Check if the NSG already exists
	l.Debugf("CreateNetworkSecurityGroup: Checking if NSG %s already exists", sgName)
	existingNSG, err := c.GetNetworkSecurityGroup(ctx, resourceGroupName, sgName)
	if err == nil {
		// NSG already exists, return it
		l.Debugf("CreateNetworkSecurityGroup: NSG %s already exists, returning existing", sgName)
		cacheEntry.result = existingNSG
		return existingNSG, nil
	}
	
	// Log the error, but continue with creation if it's a "not found" error
	if err != nil {
		l.Debugf("CreateNetworkSecurityGroup: Error checking for existing NSG %s: %v", sgName, err)
		logger.WriteToDebugLog(fmt.Sprintf("Error checking for existing NSG %s: %v", sgName, err))
		// Only proceed if it's a "not found" error
		if !strings.Contains(err.Error(), "ResourceNotFound") {
			cacheEntry.err = err
			return armnetwork.SecurityGroup{}, err
		}
	}

	l.Debugf("CreateNetworkSecurityGroup: NSG %s does not exist, creating new", sgName)

	securityRules := []*armnetwork.SecurityRule{}
	for i, port := range ports {
		ruleName := fmt.Sprintf("Allow-%d", port)
		securityRules = append(securityRules, &armnetwork.SecurityRule{
			Name: to.Ptr(ruleName),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
				SourceAddressPrefix:      to.Ptr("*"),
				SourcePortRange:          to.Ptr("*"),
				DestinationAddressPrefix: to.Ptr("*"),
				DestinationPortRange:     to.Ptr(fmt.Sprintf("%d", port)),
				Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
				Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
				Priority:                 to.Ptr(int32(basePriority + i)),
			},
		})
	}

	parameters := armnetwork.SecurityGroup{
		Name:     to.Ptr(sgName),
		Location: to.Ptr(location),
		Tags:     tags,
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: securityRules,
		},
	}

	l.Debugf("CreateNetworkSecurityGroup: Beginning creation of NSG %s", sgName)
	poller, err := c.securityGroupsClient.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		sgName,
		parameters,
		nil,
	)
	if err != nil {
		l.Errorf("CreateNetworkSecurityGroup: Error starting creation of NSG %s: %v", sgName, err)
		cacheEntry.err = err
		logger.WriteToDebugLog(fmt.Sprintf("Error in CreateNetworkSecurityGroup (BeginCreateOrUpdate): %v", err))
		return armnetwork.SecurityGroup{}, err
	}

	l.Debugf("CreateNetworkSecurityGroup: Waiting for creation of NSG %s to complete", sgName)
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		l.Errorf("CreateNetworkSecurityGroup: Error during creation of NSG %s: %v", sgName, err)
		cacheEntry.err = err
		logger.WriteToDebugLog(fmt.Sprintf("Error in CreateNetworkSecurityGroup (PollUntilDone): %v", err))
		return armnetwork.SecurityGroup{}, err
	}

	l.Debugf("CreateNetworkSecurityGroup: Successfully created NSG %s", sgName)
	cacheEntry.result = resp.SecurityGroup
	return resp.SecurityGroup, nil
}

func (c *LiveAzureClient) GetNetworkSecurityGroup(ctx context.Context,
	resourceGroupName, sgName string) (armnetwork.SecurityGroup, error) {
	logger.LogAzureAPIStart("GetNetworkSecurityGroup")
	resp, err := c.securityGroupsClient.Get(ctx, resourceGroupName, sgName, nil)
	logger.LogAzureAPIEnd("GetNetworkSecurityGroup", err)
	if err != nil {
		return armnetwork.SecurityGroup{}, err
	}

	return resp.SecurityGroup, nil
}

func (c *LiveAzureClient) SearchResources(ctx context.Context,
	resourceGroup string,
	subscriptionID string,
	tags map[string]*string) (armresourcegraph.ClientResourcesResponse, error) {
	logger.LogAzureAPIStart("SearchResources")
	query := "Resources | project id, name, type, location, tags"
	if resourceGroup != "" {
		query += fmt.Sprintf(" | where resourceGroup == '%s'", resourceGroup)
	}
	for key, value := range tags {
		if value != nil {
			query += fmt.Sprintf(" | where tags['%s'] == '%s'", key, *value)
		}
	}

	request := armresourcegraph.QueryRequest{
		Query:         to.Ptr(query),
		Subscriptions: []*string{to.Ptr(subscriptionID)},
	}

	res, err := c.resourceGraphClient.Resources(ctx, request, nil)
	logger.LogAzureAPIEnd("SearchResources", err)
	if err != nil {
		return armresourcegraph.ClientResourcesResponse{}, fmt.Errorf(
			"failed to query resources: %v",
			err,
		)
	}

	if res.Data == nil || len(res.Data.([]interface{})) == 0 {
		fmt.Println("No resources found")
		return armresourcegraph.ClientResourcesResponse{}, nil
	}

	return res, nil
}

func (c *LiveAzureClient) NewSubscriptionListPager(
	ctx context.Context,
	options *armsubscription.SubscriptionsClientListOptions,
) *runtime.Pager[armsubscription.SubscriptionsClientListResponse] {
	return c.subscriptionsClient.NewListPager(options)
}

func (c *LiveAzureClient) DestroyResourceGroup(
	ctx context.Context,
	resourceGroupName string,
) error {
	logger.LogAzureAPIStart("DestroyResourceGroup")
	_, err := c.resourceGroupsClient.BeginDelete(ctx, resourceGroupName, nil)
	logger.LogAzureAPIEnd("DestroyResourceGroup", err)
	if err != nil {
		return err
	}

	return nil
}
