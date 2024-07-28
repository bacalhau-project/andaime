package azure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
)

var (
	basePriority = 100
)

// createPublicIP creates a public IP address with retry logic
func (p *AzureProvider) CreatePublicIP(
	ctx context.Context,
	deployment *models.Deployment,
	machine *models.Machine,
	disp *display.Display,
) (*armnetwork.PublicIPAddress, error) {
	l := logger.Get()
	maxRetries := 5
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		l.Debugf(
			"CreatePublicIP: Attempt %d of %d for machine %s",
			attempt+1,
			maxRetries,
			machine.ID,
		)

		createdIP, err := p.Client.CreatePublicIP(
			ctx,
			deployment.ResourceGroupName,
			machine.Location,
			machine.ID,
			deployment.Tags,
		)

		if err == nil {
			l.Infof(
				"CreatePublicIP: Successfully created public IP for machine %s on attempt %d",
				machine.ID,
				attempt+1,
			)
			return &createdIP, nil
		}

		lastErr = err
		if strings.Contains(err.Error(), "Canceled") {
			l.Warnf(
				"CreatePublicIP: Operation was canceled, retrying for machine %s: %v",
				machine.ID,
				err,
			)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		l.Errorf("CreatePublicIP: Failed to create public IP for machine %s: %v", machine.ID, err)
		return nil, fmt.Errorf("failed to create public IP after %d attempts: %v", maxRetries, err)
	}

	return nil, fmt.Errorf("failed to create public IP after %d attempts: %v", maxRetries, lastErr)
}

// createNIC creates a network interface with both public and private IP
func (p *AzureProvider) CreateNIC(
	ctx context.Context,
	deployment *models.Deployment,
	machine *models.Machine,
	disp *display.Display,
) (*armnetwork.Interface, error) {
	if deployment.Subnets == nil {
		return nil, fmt.Errorf("subnets not found")
	}

	if machine.Location == "" {
		return nil, fmt.Errorf("location not found")
	}

	if len(deployment.Subnets[machine.Location]) == 0 {
		return nil, fmt.Errorf("no subnets found for location %s", machine.Location)
	}

	if machine.NetworkSecurityGroup == nil {
		return nil, fmt.Errorf("network security group not found")
	}

	if machine.NetworkSecurityGroup.ID == nil {
		return nil, fmt.Errorf("network security group ID not found")
	}

	if len(deployment.Subnets[machine.Location]) == 0 {
		return nil, fmt.Errorf("no subnets found for location %s", machine.Location)
	}

	createdNIC, err := p.Client.CreateNetworkInterface(
		ctx,
		deployment.ResourceGroupName,
		machine.Location,
		getNetworkInterfaceName(machine.ID),
		deployment.Tags,
		deployment.Subnets[machine.Location][0],
		machine.PublicIPAddress,
		machine.NetworkSecurityGroup,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network interface: %v", err)
	}

	return &createdNIC, nil
}

// createVM creates the virtual machine using Bicep template
func (p *AzureProvider) CreateVirtualMachine(
	ctx context.Context,
	deployment *models.Deployment,
	machine models.Machine,
	disp *display.Display,
) (*armcompute.VirtualMachine, error) {
	if machine.Interface == nil {
		return nil, fmt.Errorf("network interface not created for machine %s", machine.ID)
	}

	// Prepare parameters for Bicep template
	params := map[string]interface{}{
		"vmName": map[string]interface{}{
			"value": machine.ComputerName,
		},
		"vmSize": map[string]interface{}{
			"value": machine.VMSize,
		},
		"adminUsername": map[string]interface{}{
			"value": "azureuser",
		},
		"networkInterfaceId": map[string]interface{}{
			"value": *machine.Interface.ID,
		},
		"location": map[string]interface{}{
			"value": machine.Location,
		},
		"osDiskSizeGB": map[string]interface{}{
			"value": getDiskSizeGB(machine.DiskSizeGB),
		},
	}

	// Create the ARM template
	template := map[string]interface{}{
		"$schema":        "https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#",
		"contentVersion": "1.0.0.0",
		"parameters":     params,
		"resources": []interface{}{
			map[string]interface{}{
				"type":       "Microsoft.Compute/virtualMachines",
				"apiVersion": "2021-03-01",
				"name":       "[parameters('vmName')]",
				"location":   "[parameters('location')]",
				"properties": map[string]interface{}{
					"hardwareProfile": map[string]interface{}{
						"vmSize": "[parameters('vmSize')]",
					},
					"osProfile": map[string]interface{}{
						"computerName":  "[parameters('vmName')]",
						"adminUsername": "[parameters('adminUsername')]",
						"linuxConfiguration": map[string]interface{}{
							"disablePasswordAuthentication": true,
							"ssh": map[string]interface{}{
								"publicKeys": []interface{}{
									map[string]interface{}{
										"path":    "[concat('/home/', parameters('adminUsername'), '/.ssh/authorized_keys')]",
										"keyData": string(deployment.SSHPublicKeyData),
									},
								},
							},
						},
					},
					"storageProfile": map[string]interface{}{
						"imageReference": map[string]interface{}{
							"publisher": "Canonical",
							"offer":     "UbuntuServer",
							"sku":       "18.04-LTS",
							"version":   "latest",
						},
						"osDisk": map[string]interface{}{
							"createOption": "FromImage",
							"managedDisk": map[string]interface{}{
								"storageAccountType": "Premium_LRS",
							},
							"diskSizeGB": "[parameters('osDiskSizeGB')]",
						},
					},
					"networkProfile": map[string]interface{}{
						"networkInterfaces": []interface{}{
							map[string]interface{}{
								"id": "[parameters('networkInterfaceId')]",
								"properties": map[string]interface{}{
									"primary": true,
								},
							},
						},
					},
				},
			},
		},
	}

	// Deploy the ARM template
	future, err := p.Client.DeployTemplate(
		ctx,
		deployment.ResourceGroupName,
		machine.ComputerName+"-deployment",
		template,
		params,
		deployment.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy VM template: %v", err)
	}

	_, err = future.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{
		Frequency: time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to poll for VM deployment completion: %v", err)
	}

	// Get the created VM
	createdVM, err := p.Client.GetVirtualMachine(
		ctx,
		deployment.ResourceGroupName,
		machine.Location,
		machine.ComputerName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get created virtual machine: %v", err)
	}

	return &createdVM, nil
}

// createNSG creates a network security group with the specified open ports
func (p *AzureProvider) CreateNSG(
	ctx context.Context,
	deployment *models.Deployment,
	location string,
	disp *display.Display,
) (*armnetwork.SecurityGroup, error) {
	l := logger.Get()
	l.Debugf("CreateNSG: %s", location)

	for _, machine := range deployment.Machines {
		if machine.Location == location {
			disp.UpdateStatus(&models.Status{
				ID:     machine.ID,
				Type:   "VM",
				Status: "Creating NSG for " + location,
			})
		}
	}

	nsgName := fmt.Sprintf("%s-%s-nsg", deployment.ResourceGroupName, location)
	createdNSG, err := p.Client.CreateNetworkSecurityGroup(
		ctx,
		deployment.ResourceGroupName,
		location,
		nsgName,
		deployment.AllowedPorts,
		deployment.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create network security group: %v", err)
	}

	return &createdNSG, nil
}

// getDiskSizeGB returns the disk size in GB, using a default value if not set
func getDiskSizeGB(diskSize int32) int32 {
	if diskSize <= 0 {
		return 30 // Default disk size if not specified
	}
	return diskSize
}
