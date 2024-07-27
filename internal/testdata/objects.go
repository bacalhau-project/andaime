package testdata

import (
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

var TestSubnet = armnetwork.Subnet{
	Name: to.Ptr("testSubnet"),
	Properties: &armnetwork.SubnetPropertiesFormat{
		AddressPrefix: to.Ptr("10.0.0.0/24"),
	},
}

//nolint:lll
var TestVirtualNetwork = armnetwork.VirtualNetwork{
	ID: to.Ptr(
		"/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testRG/providers/Microsoft.Network/virtualNetworks/testVNet",
	),
	Name:     to.Ptr("testVNet"),
	Location: to.Ptr("eastus"),
	Properties: &armnetwork.VirtualNetworkPropertiesFormat{
		AddressSpace: &armnetwork.AddressSpace{
			AddressPrefixes: []*string{to.Ptr("10.0.0.0/16")},
		},
		Subnets: []*armnetwork.Subnet{&TestSubnet},
	},
}

var TestPublicIPAddress = armnetwork.PublicIPAddress{
	Name: to.Ptr("testIP"),
	Properties: &armnetwork.PublicIPAddressPropertiesFormat{
		IPAddress: to.Ptr("256.256.256.256"),
	},
}

//nolint:lll
var TestInterface = armnetwork.Interface{
	ID: to.Ptr(
		"/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testRG/providers/Microsoft.Network/networkInterfaces/testNIC",
	),
	Name: to.Ptr("testNIC"),
	Properties: &armnetwork.InterfacePropertiesFormat{
		IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
			{
				Name:       to.Ptr("testIPConfig"),
				Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{},
			},
		},
	},
}

var TestVirtualMachine = armcompute.VirtualMachine{
	Name: to.Ptr("testVM"),
	Properties: &armcompute.VirtualMachineProperties{
		StorageProfile: &armcompute.StorageProfile{
			ImageReference: &armcompute.ImageReference{
				Publisher: to.Ptr("Canonical"),
				Offer:     to.Ptr("UbuntuServer"),
				Version:   to.Ptr("latest"),
			},
		},
		OSProfile: &armcompute.OSProfile{
			ComputerName:  to.Ptr("testVM"),
			AdminUsername: to.Ptr("testuser"),
			AdminPassword: to.Ptr("testpassword"),
			LinuxConfiguration: &armcompute.LinuxConfiguration{
				SSH: &armcompute.SSHConfiguration{
					PublicKeys: []*armcompute.SSHPublicKey{
						{Path: to.Ptr("/home/testuser/.ssh/authorized_keys"),
							KeyData: to.Ptr(TestPublicSSHKeyMaterial),
						},
					},
				},
			},
		},
		HardwareProfile: &armcompute.HardwareProfile{
			VMSize: &armcompute.PossibleVirtualMachineSizeTypesValues()[0],
		},
	},
}

var TestNSG = armnetwork.SecurityGroup{
	ID: to.Ptr(
		"/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/testRG/providers/Microsoft.Network/networkSecurityGroups/testNSG",
	),
	Name: to.Ptr("testNSG"),
	Properties: &armnetwork.SecurityGroupPropertiesFormat{
		SecurityRules: []*armnetwork.SecurityRule{
			{
				Name: to.Ptr("testRule"),
				Properties: &armnetwork.SecurityRulePropertiesFormat{
					Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
					Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
					DestinationAddressPrefix: to.Ptr("*"),
					DestinationPortRange:     to.Ptr("22"),
					Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
					SourceAddressPrefix:      to.Ptr("*"),
					SourcePortRange:          to.Ptr("*"),
				},
			},
		},
	},
}
