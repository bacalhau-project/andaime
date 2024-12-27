package testdata

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

// FakeVirtualMachine returns a fake Azure Virtual Machine for testing
func FakeVirtualMachine() *armcompute.VirtualMachine {
	nicID := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkInterfaces/nic1"
	return &armcompute.VirtualMachine{
		Properties: &armcompute.VirtualMachineProperties{
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{ID: &nicID},
				},
			},
		},
	}
}

// FakeNetworkInterface returns a fake Azure Network Interface for testing
func FakeNetworkInterface() *armnetwork.Interface {
	privateIPAddress := "10.0.0.4"
	publicIPAddressID := "/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/publicIPAddresses/pip1"
	return &armnetwork.Interface{
		ID: to.Ptr("/subscriptions/sub1/resourceGroups/rg1/providers/Microsoft.Network/networkInterfaces/nic1"),
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{{
				Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
					PrivateIPAddress: &privateIPAddress,
					PublicIPAddress: &armnetwork.PublicIPAddress{
						Properties: &armnetwork.PublicIPAddressPropertiesFormat{
							IPAddress: to.Ptr("1.2.3.4"),
						},
						ID: &publicIPAddressID,
					},
				},
			}},
		},
	}
}

// FakePublicIPAddress returns a fake Azure Public IP Address for testing
func FakePublicIPAddress(ip string) *armnetwork.PublicIPAddress {
	return &armnetwork.PublicIPAddress{
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			IPAddress: &ip,
		},
	}
}

// FakeResourceGroup returns a fake Azure Resource Group for testing
func FakeResourceGroup() *armresources.ResourceGroup {
	return &armresources.ResourceGroup{
		Name:     to.Ptr("test-rg"),
		Location: to.Ptr("eastus"),
		Tags:     map[string]*string{"test": to.Ptr("value")},
	}
}

// FakeDeployment returns a fake Azure Deployment for testing
func FakeDeployment() armresources.DeploymentExtended {
	successState := armresources.ProvisioningStateSucceeded
	return armresources.DeploymentExtended{
		Properties: &armresources.DeploymentPropertiesExtended{
			ProvisioningState: &successState,
		},
	}
}
