//nolint:lll
package azure

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	azureutils "github.com/bacalhau-project/andaime/internal/clouds/azure"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/spf13/viper"
)

const (
	basePriority = 1000
)

func IsValidVMSize(vmSize string) bool {
	validSizes := viper.GetStringSlice("azure.valid_vm_sizes")
	for _, size := range validSizes {
		if size == vmSize {
			return true
		}
	}
	return false
}

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
		parameters *armcompute.VirtualMachine,
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
		sgName string,
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

	// New methods for ARM template deployment
	DeployTemplate(
		ctx context.Context,
		resourceGroupName string,
		deploymentName string,
		template map[string]interface{},
		parameters map[string]interface{},
		tags map[string]*string,
	) (*runtime.Poller[armresources.DeploymentsClientCreateOrUpdateResponse], error)
	GetDeploymentsClient() *armresources.DeploymentsClient
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
	deploymentsClient       *armresources.DeploymentsClient
	nsgCache                sync.Map
}

func (c *LiveAzureClient) GetDeploymentsClient() *armresources.DeploymentsClient {
	return c.deploymentsClient
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
	deploymentsClient, err := armresources.NewDeploymentsClient(subscriptionID, cred, nil)
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
		deploymentsClient:       deploymentsClient,
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
	return fmt.Sprintf("ip-%s", machineID)
}

// CreatePublicIP creates a new public IP address or returns an existing one
func (c *LiveAzureClient) CreatePublicIP(ctx context.Context,
	resourceGroupName string,
	location string,
	machineID string,
	tags map[string]*string) (armnetwork.PublicIPAddress, error) {
	l := logger.Get()
	ipName := getPublicIPName(machineID)
	l.Debugf("CreatePublicIP - starting: %s", ipName)

	// First, try to get the existing public IP
	existingIP, err := c.GetPublicIP(ctx, resourceGroupName, location, ipName)
	if err == nil {
		l.Infof("Public IP %s already exists, returning existing IP", ipName)
		return existingIP, nil
	}

	parameters := armnetwork.PublicIPAddress{
		Name:     to.Ptr(ipName),
		Location: to.Ptr(location),
		Tags:     tags,
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}

	maxRetries := 10
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		l.Debugf("CreatePublicIP: Attempt %d of %d", attempt+1, maxRetries)

		poller, err := c.publicIPAddressesClient.BeginCreateOrUpdate(
			ctx,
			resourceGroupName,
			ipName,
			parameters,
			nil,
		)
		if err != nil {
			l.Errorf("CreatePublicIP BeginCreateOrUpdate failed: %s", err)
			lastErr = err
			backoffDuration := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			time.Sleep(backoffDuration)
			continue
		}

		resp, err := poller.PollUntilDone(ctx, nil)
		if err != nil {
			if strings.Contains(err.Error(), "Canceled") ||
				strings.Contains(err.Error(), "Conflict") {
				l.Warnf("CreatePublicIP polling encountered an error, retrying: %s", err)
				lastErr = err
				backoffDuration := time.Duration(math.Pow(2, float64(attempt))) * time.Second
				time.Sleep(backoffDuration)
				continue
			}
			l.Errorf("CreatePublicIP polling failed: %s", err)
			return armnetwork.PublicIPAddress{}, err
		}

		l.Infof("CreatePublicIP: Successfully created %s", *resp.PublicIPAddress.Name)
		return resp.PublicIPAddress, nil
	}

	l.Errorf("CreatePublicIP: Failed after %d attempts", maxRetries)
	return armnetwork.PublicIPAddress{}, lastErr
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

func (c *LiveAzureClient) DeployTemplate(
	ctx context.Context,
	resourceGroupName string,
	deploymentName string,
	template map[string]interface{},
	parameters map[string]interface{},
	tags map[string]*string,
) (*runtime.Poller[armresources.DeploymentsClientCreateOrUpdateResponse], error) {
	l := logger.Get()
	l.Debugf("DeployTemplate: Beginning - %s", deploymentName)

	deployment := armresources.Deployment{
		Properties: &armresources.DeploymentProperties{
			Template:   template,
			Parameters: &parameters,
			Mode:       to.Ptr(armresources.DeploymentModeIncremental),
		},
		Tags: tags,
	}

	future, err := c.GetDeploymentsClient().BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		deploymentName,
		deployment,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to begin deployment: %w", err)
	}

	return future, nil
}

func (c *LiveAzureClient) CreateVirtualMachine(ctx context.Context,
	resourceGroupName string,
	location string,
	machineID string,
	parameters *armcompute.VirtualMachine,
	tags map[string]*string) (armcompute.VirtualMachine, error) {
	vmName := getVMName(machineID)

	l := logger.Get()
	l.Debugf("CreateVirtualMachine: Beginning - %s", vmName)

	poller, err := c.virtualMachinesClient.BeginCreateOrUpdate(
		ctx,
		resourceGroupName,
		vmName,
		*parameters,
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
	return machineID + "nic"
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

	// Validate location
	if !IsValidLocation(location) {
		l.Errorf("Invalid location: %s", location)
		return armnetwork.Interface{}, fmt.Errorf("invalid location: %s", location)
	}

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
	sgName string,
	ports []int,
	tags map[string]*string,
) (armnetwork.SecurityGroup, error) {
	l := logger.Get()

	l.Debugf("CreateNetworkSecurityGroup: Starting for %s in %s", sgName, location)

	// Use LoadOrStore to ensure only one goroutine creates the cache entry
	l.Debugf("CreateNetworkSecurityGroup: Loading or storing cache entry for %s", sgName)
	entry, loaded := c.nsgCache.LoadOrStore(sgName, &nsgCacheEntry{})
	if entry == nil {
		err := fmt.Errorf("failed to create cache entry")
		l.Errorf("CreateNetworkSecurityGroup: %v", err)
		return armnetwork.SecurityGroup{}, err
	}
	cacheEntry := entry.(*nsgCacheEntry)
	l.Debugf("CreateNetworkSecurityGroup: Cache entry loaded: %t", loaded)

	if loaded {
		// If the entry was already in the cache, wait for the result
		l.Debugf("CreateNetworkSecurityGroup: Waiting for existing operation for %s", sgName)
		cacheEntry.wg.Wait()
		l.Debugf("CreateNetworkSecurityGroup: Existing operation completed for %s", sgName)
		return cacheEntry.result, cacheEntry.err
	}

	l.Debugf("CreateNetworkSecurityGroup: Starting creation of NSG %s", sgName)
	// This goroutine is responsible for creating the NSG
	cacheEntry.wg.Add(1)
	defer cacheEntry.wg.Done()

	l.Debugf("CreateNetworkSecurityGroup: Added to cache entry for %s", sgName)

	// Check if the NSG already exists
	l.Debugf("CreateNetworkSecurityGroup: Checking if NSG %s already exists", sgName)
	existingNSG, err := c.GetNetworkSecurityGroup(ctx, resourceGroupName, sgName)
	if err != nil {
		l.Errorf("CreateNetworkSecurityGroup: Error getting NSG %s: %v", sgName, err)
		cacheEntry.err = err
		return armnetwork.SecurityGroup{}, err
	} else if existingNSG.Name != nil {
		// err will be nil, need to check to see if name is nil to see if NSG already exists
		l.Debugf("CreateNetworkSecurityGroup: NSG %s already exists, returning existing", sgName)
		cacheEntry.result = existingNSG
		return existingNSG, nil
	}

	// No error getting NSG, but it doesn't exist, so proceed with creation
	l.Debugf(
		"CreateNetworkSecurityGroup: NSG %s does not exist or is still being created, proceeding with creation",
		sgName,
	)

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
		return armnetwork.SecurityGroup{}, err
	}

	l.Debugf("CreateNetworkSecurityGroup: Waiting for creation of NSG %s to complete", sgName)
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		l.Errorf("CreateNetworkSecurityGroup: Error during creation of NSG %s: %v", sgName, err)
		cacheEntry.err = err
		return armnetwork.SecurityGroup{}, err
	}

	if resp.SecurityGroup.Name == nil {
		err := fmt.Errorf("created NSG has nil Name")
		l.Errorf("CreateNetworkSecurityGroup: %v", err)
		cacheEntry.err = err
		return armnetwork.SecurityGroup{}, err
	}

	l.Debugf("CreateNetworkSecurityGroup: Successfully created NSG %s", *resp.SecurityGroup.Name)
	cacheEntry.result = resp.SecurityGroup
	l.Infof("CreateNetworkSecurityGroup: Completed creation of NSG %s", *resp.SecurityGroup.Name)
	return resp.SecurityGroup, nil
}

func (c *LiveAzureClient) GetNetworkSecurityGroup(ctx context.Context,
	resourceGroupName, sgName string) (armnetwork.SecurityGroup, error) {
	l := logger.Get()
	l.Debugf("GetNetworkSecurityGroup: Starting for %s", sgName)
	resp, err := c.securityGroupsClient.Get(ctx, resourceGroupName, sgName, nil)
	if err != nil {
		// If it's just ResourceNotFound, that's ok, we'll create it
		if strings.Contains(err.Error(), "ResourceNotFound") {
			l.Debugf("GetNetworkSecurityGroup: NSG %s not found, will be created", sgName)
			return armnetwork.SecurityGroup{}, nil
		} else {
			l.Errorf("GetNetworkSecurityGroup: Error getting NSG %s: %v", sgName, err)
			return armnetwork.SecurityGroup{}, err
		}
	}

	if resp.SecurityGroup.Name == nil {
		l.Errorf("GetNetworkSecurityGroup: Retrieved NSG %s has nil Name", sgName)
		return armnetwork.SecurityGroup{}, fmt.Errorf("retrieved NSG has nil Name")
	}

	l.Debugf("GetNetworkSecurityGroup: Successfully retrieved NSG %s", *resp.SecurityGroup.Name)
	return resp.SecurityGroup, nil
}

func IsValidLocation(location string) bool {
	locations, err := azureutils.GetLocations()
	if err != nil {
		logger.Get().Errorf("Failed to get Azure locations: %v", err)
		return false
	}
	for _, validLocation := range locations {
		if strings.EqualFold(location, validLocation) {
			return true
		}
	}
	return false
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
