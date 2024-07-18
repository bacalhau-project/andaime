package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

// generateTags creates a map of tags for Azure resources
func generateTags(projectID, uniqueID string) map[string]*string {
	return map[string]*string{
		"andaime":                   to.Ptr("true"),
		"andaime-id":                to.Ptr(uniqueID),
		"andaime-project":           to.Ptr(fmt.Sprintf("%s-%s", uniqueID, projectID)),
		"unique-id":                 to.Ptr(uniqueID),
		"project-id":                to.Ptr(projectID),
		"deployed-by":               to.Ptr("andaime"),
		"andaime-resource-tracking": to.Ptr("true"),
	}
}

type DefaultSSHWaiter struct{}

func (w *DefaultSSHWaiter) WaitForSSH(publicIP, username string, privateKey []byte) error {
	return waitForSSH(publicIP, username, privateKey)
}

func searchResources(ctx context.Context,
	clients ClientInterfaces,
	resourceGroup string,
	tags map[string]*string,
	subscriptionID string) (armresourcegraph.ClientResourcesResponse, error) {
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

	res, err := clients.ResourceGraphClient.Resources(ctx, request, nil)
	if err != nil {
		return armresourcegraph.ClientResourcesResponse{}, fmt.Errorf("failed to query resources: %v", err)
	}

	if res.Data == nil {
		return armresourcegraph.ClientResourcesResponse{}, nil
	}

	return res, nil
}

// NewClientInterfaces creates a new ClientInterfaces struct with initialized clients
func NewClientInterfaces(subscriptionID string) (*ClientInterfaces, error) {
	// Load credentials from CLI
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return &ClientInterfaces{}, fmt.Errorf("failed to obtain a credential: %v", err)
	}

	// Initialize Resource Graph client
	resourceGraphClient, err := armresourcegraph.NewClient(cred, nil)
	if err != nil {
		return &ClientInterfaces{}, fmt.Errorf("failed to create resource graph client: %v", err)
	}

	// Initialize Virtual Networks client
	virtualNetworksClient, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	if err != nil {
		return &ClientInterfaces{}, fmt.Errorf("failed to create virtual networks client: %v", err)
	}

	// Initialize Public IP Addresses client
	publicIPAddressesClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return &ClientInterfaces{}, fmt.Errorf("failed to create public IP addresses client: %v", err)
	}

	// Initialize Network Interfaces client
	networkInterfacesClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return &ClientInterfaces{}, fmt.Errorf("failed to create network interfaces client: %v", err)
	}

	// Initialize Virtual Machines client
	virtualMachinesClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return &ClientInterfaces{}, fmt.Errorf("failed to create virtual machines client: %v", err)
	}

	// Initialize Network Security Groups client
	securityGroupsClient, err := armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return &ClientInterfaces{}, fmt.Errorf("failed to create network security groups client: %v", err)
	}

	return &ClientInterfaces{
		ResourceGraphClient:     resourceGraphClient,
		VirtualNetworksClient:   virtualNetworksClient,
		PublicIPAddressesClient: publicIPAddressesClient,
		NetworkInterfacesClient: networkInterfacesClient,
		VirtualMachinesClient:   virtualMachinesClient,
		SecurityGroupsClient:    securityGroupsClient,
	}, nil
}

func ensureTags(tags map[string]*string, projectID, uniqueID string) {
	if tags == nil {
		tags = map[string]*string{}
	}
	if tags["andaime"] == nil {
		tags["andaime"] = to.Ptr("true")
	}
	if tags["deployed-by"] == nil {
		tags["deployed-by"] = to.Ptr("andaime")
	}
	if tags["andaime-resource-tracking"] == nil {
		tags["andaime-resource-tracking"] = to.Ptr("true")
	}
	if tags["unique-id"] == nil {
		tags["unique-id"] = to.Ptr(uniqueID)
	}
	if tags["project-id"] == nil {
		tags["project-id"] = to.Ptr(projectID)
	}
	if tags["andaime-project"] == nil {
		tags["andaime-project"] = to.Ptr(fmt.Sprintf("%s-%s", uniqueID, projectID))
	}
}
