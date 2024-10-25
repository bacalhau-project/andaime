package gcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	internal_gcp "github.com/bacalhau-project/andaime/internal/clouds/gcp"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/cenkalti/backoff/v4"
	"github.com/spf13/viper"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/proto"
)

func (c *LiveGCPClient) CreateVPCNetwork(ctx context.Context, networkName string) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	projectID := m.Deployment.GetProjectID()
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 5 * time.Minute

	operation := func() error {
		network := &computepb.Network{
			Name:                  &networkName,
			AutoCreateSubnetworks: to.Ptr(true),
			RoutingConfig: &computepb.NetworkRoutingConfig{
				RoutingMode: to.Ptr("GLOBAL"),
			},
		}

		op, err := c.networksClient.Insert(ctx, &computepb.InsertNetworkRequest{
			Project:         projectID,
			NetworkResource: network,
		})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				l.Debugf("Network %s already exists, skipping creation", networkName)
				return backoff.Permanent(nil)
			}
			return fmt.Errorf("failed to create network: %v", err)
		}

		// Wait for the operation to complete
		err = op.Wait(ctx)
		if err != nil {
			return fmt.Errorf("failed to wait for network creation: %v", err)
		}

		return nil
	}

	err := backoff.Retry(operation, b)
	if err != nil {
		return fmt.Errorf("failed to create VPC network after retries: %v", err)
	}

	l.Infof("VPC network %s created successfully", networkName)
	return nil
}

func (c *LiveGCPClient) CreateFirewallRules(ctx context.Context, networkName string) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	projectID := m.Deployment.GetProjectID()
	l.Debugf("Creating firewall rules in project: %s", projectID)

	allowedPorts := viper.GetIntSlice("gcp.allowed_ports")
	if len(allowedPorts) == 0 {
		return fmt.Errorf("no allowed ports specified in the configuration")
	}

	networkName = "default"

	for _, port := range allowedPorts {
		for _, machine := range m.Deployment.GetMachines() {
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.GetName(),
				models.GCPResourceTypeFirewall,
				models.ResourceStatePending,
				fmt.Sprintf("Creating FW for port %d", port),
			))
		}

		ruleName := fmt.Sprintf("default-allow-%d", port)

		b := backoff.NewExponentialBackOff()
		b.MaxElapsedTime = 10 * time.Second

		operation := func() error {
			firewallRule := &computepb.Firewall{
				Name: &ruleName,
				Network: to.Ptr(
					fmt.Sprintf("projects/%s/global/networks/%s", projectID, networkName),
				),
				Allowed: []*computepb.Allowed{
					{
						IPProtocol: to.Ptr("tcp"),
						Ports:      []string{strconv.Itoa(port)},
					},
				},
				SourceRanges: []string{"0.0.0.0/0"},
				Direction:    to.Ptr("INGRESS"),
			}

			_, err := c.firewallsClient.Insert(ctx, &computepb.InsertFirewallRequest{
				Project:          projectID,
				FirewallResource: firewallRule,
			})
			if err != nil {
				if strings.Contains(err.Error(), "already exists") {
					l.Debugf("Firewall rule %s already exists, skipping creation", ruleName)
					return nil
				}
				if strings.Contains(err.Error(), "Compute Engine API has not been used") {
					l.Debugf("Compute Engine API is not yet active. Retrying... (FW Rules)")
					return err
				}
				return backoff.Permanent(fmt.Errorf("failed to create firewall rule: %v", err))
			}

			return nil
		}

		err := backoff.Retry(operation, b)
		if err != nil {
			l.Errorf("Failed to create firewall rule for port %d after retries: %v", port, err)
			return fmt.Errorf(
				"failed to create firewall rule for port %d after retries: %v",
				port,
				err,
			)
		}
		l.Infof("Firewall rule created or already exists for port %d", port)

		for _, machine := range m.Deployment.GetMachines() {
			m.UpdateStatus(models.NewDisplayStatusWithText(
				machine.GetName(),
				models.GCPResourceTypeFirewall,
				models.ResourceStateRunning,
				fmt.Sprintf("Created or verified FW Rule for port %d", port),
			))
		}
	}

	return nil
}

func (c *LiveGCPClient) CreateIP(
	ctx context.Context,
	projectID, region string,
	address *computepb.Address,
) (*computepb.Address, error) {
	if region == "" {
		return nil, fmt.Errorf("region is not set")
	}

	if projectID == "" {
		return nil, fmt.Errorf("projectID is not set in CreateIP")
	}

	if address.Name == nil {
		return nil, fmt.Errorf("addressName is not set")
	}
	addressName := *address.Name

	if address.AddressType == nil {
		return nil, fmt.Errorf("addressType is not set")
	}

	// Insert the address
	op, err := c.addressesClient.Insert(ctx, &computepb.InsertAddressRequest{
		Project: projectID,
		Region:  region,
		AddressResource: &computepb.Address{
			Name:        to.Ptr(addressName),
			AddressType: address.AddressType,
			Region:      to.Ptr(region),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start reserving IP address: %v", err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("operation to reserve IP address failed: %v", err)
	}

	return c.addressesClient.Get(ctx, &computepb.GetAddressRequest{
		Project: projectID,
		Region:  region,
		Address: addressName,
	})
}

func (c *LiveGCPClient) DeleteIP(
	ctx context.Context,
	projectID, region, addressName string,
) error {
	op, err := c.addressesClient.Delete(ctx, &computepb.DeleteAddressRequest{
		Project: projectID,
		Region:  region,
		Address: addressName,
	})
	if err != nil {
		return fmt.Errorf("failed to start deleting IP address: %v", err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed to wait for IP address deletion: %v", err)
	}

	return nil
}

func (c *LiveGCPClient) ListAddresses(
	ctx context.Context,
	projectID, region string,
) ([]*computepb.Address, error) {
	ipAddressIterator := c.addressesClient.List(ctx, &computepb.ListAddressesRequest{
		Project: projectID,
		Region:  region,
	})

	var addresses []*computepb.Address
	var err error
	for {
		var address *computepb.Address
		address, err = ipAddressIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list IP addresses: %v", err)
		}
		addresses = append(addresses, address)
	}

	return addresses, err
}

func (c *LiveGCPClient) CreateVM(
	ctx context.Context,
	projectID string,
	machine models.Machiner,
	ip *computepb.Address,
) (*computepb.Instance, error) {
	// Validate input and prerequisites
	if err := c.validateCreateVMInput(ctx, projectID, machine); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Prepare VM configuration
	instance, err := c.prepareVMInstance(ctx, projectID, machine, ip)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare VM instance: %w", err)
	}

	// Create the instance
	op, err := c.computeClient.Insert(ctx, &computepb.InsertInstanceRequest{
		Project:          projectID,
		Zone:             machine.GetLocation(),
		InstanceResource: instance,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for instance creation: %w", err)
	}

	// Retrieve and return the created instance
	return c.computeClient.Get(ctx, &computepb.GetInstanceRequest{
		Project:  projectID,
		Zone:     machine.GetZone(),
		Instance: machine.GetName(),
	})
}

func (c *LiveGCPClient) validateCreateVMInput(
	ctx context.Context,
	projectID string,
	machine models.Machiner,
) error {
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	if projectID == "" {
		return fmt.Errorf("projectID is not set in validateCreateVMInput")
	}

	if err := c.validateZone(projectID, machine.GetLocation()); err != nil {
		return fmt.Errorf("invalid zone: %w", err)
	}

	if machine.GetSSHUser() == "" {
		return fmt.Errorf("SSH user is not set in the deployment model")
	}

	if m.Deployment.SSHPublicKeyMaterial == "" {
		return fmt.Errorf("public key material is not set in the deployment model")
	}

	if machine.GetVMSize() == "" {
		return fmt.Errorf("vm size is not set on this machine")
	}

	if machine.GetDiskSizeGB() == 0 {
		return fmt.Errorf("disk size is not set on this machine")
	}

	return nil
}

func (c *LiveGCPClient) prepareVMInstance(
	ctx context.Context,
	projectID string,
	machine models.Machiner,
	ip *computepb.Address,
) (*computepb.Instance, error) {
	l := logger.Get()
	l.Debugf("Preparing VM instance %s in project %s", machine.GetName(), projectID)

	if projectID == "" {
		return nil, fmt.Errorf("projectID is not set in prepareVMInstance")
	}

	networkName := "default"
	network, err := c.getOrCreateNetwork(ctx, projectID, networkName)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create network: %w", err)
	}

	// Get the SSH public and private keys
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return nil, fmt.Errorf("global model or deployment is nil")
	}

	startupScript := c.generateStartupScript(
		machine.GetSSHUser(),
		strings.TrimSpace(m.Deployment.SSHPublicKeyMaterial),
	)

	// Format the SSH key for GCP
	sshKeyEntry := fmt.Sprintf(
		"%s:%s",
		machine.GetSSHUser(),
		strings.TrimSpace(m.Deployment.SSHPublicKeyMaterial),
	)

	instance := &computepb.Instance{
		Name: proto.String(machine.GetName()),
		MachineType: proto.String(fmt.Sprintf(
			"zones/%s/machineTypes/%s",
			machine.GetLocation(),
			machine.GetVMSize(),
		)),
		Disks: []*computepb.AttachedDisk{
			{
				AutoDelete: to.Ptr(true),
				Boot:       to.Ptr(true),
				Type:       to.Ptr("PERSISTENT"),
				InitializeParams: &computepb.AttachedDiskInitializeParams{
					DiskSizeGb:  to.Ptr(int64(machine.GetDiskSizeGB())),
					SourceImage: to.Ptr(machine.GetDiskImageURL()),
				},
			},
		},
		NetworkInterfaces: []*computepb.NetworkInterface{
			{
				Network: network.SelfLink,
				AccessConfigs: []*computepb.AccessConfig{
					{
						Type:  to.Ptr("ONE_TO_ONE_NAT"),
						Name:  to.Ptr("External NAT"),
						NatIP: ip.Address,
					},
				},
			},
		},
		ServiceAccounts: []*computepb.ServiceAccount{
			{
				Email: to.Ptr("default"),
				Scopes: []string{
					"https://www.googleapis.com/auth/compute",
				},
			},
		},
		Metadata: &computepb.Metadata{
			Items: []*computepb.Items{
				{
					Key:   to.Ptr("startup-script"),
					Value: to.Ptr(startupScript),
				},
				{
					Key:   to.Ptr("ssh-keys"),
					Value: to.Ptr(machine.GetSSHUser() + ":" + sshKeyEntry),
				},
			},
		},
	}

	return instance, nil
}
func (c *LiveGCPClient) getOrCreateNetwork(
	ctx context.Context,
	projectID, networkName string,
) (*computepb.Network, error) {
	l := logger.Get()
	l.Debugf("Getting or creating network %s in project %s", networkName, projectID)

	if projectID == "" {
		return nil, fmt.Errorf("projectID is not set in getOrCreateNetwork")
	}

	// Always try to get the network first
	network, err := c.networksClient.Get(ctx, &computepb.GetNetworkRequest{
		Project: projectID,
		Network: networkName,
	})
	if err == nil {
		return network, nil
	}

	// If it's the default network and we couldn't get it, there's a problem
	if networkName == "default" {
		return nil, fmt.Errorf("default network not found in project %s: %v", projectID, err)
	}

	// For non-default networks, create if not found
	if !isNotFoundError(err) {
		return nil, fmt.Errorf("failed to get network: %v", err)
	}

	network = &computepb.Network{
		Name:                  &networkName,
		AutoCreateSubnetworks: to.Ptr(true),
	}

	req := &computepb.InsertNetworkRequest{
		Project:         projectID,
		NetworkResource: network,
	}

	op, err := c.networksClient.Insert(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %v", err)
	}

	// Wait for the operation to complete
	err = op.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for network creation: %v", err)
	}

	return c.networksClient.Get(ctx, &computepb.GetNetworkRequest{
		Project: projectID,
		Network: networkName,
	})
}

func (c *LiveGCPClient) validateZone(projectID, zone string) error {
	l := logger.Get()
	l.Debugf("Validating zone: %s in project: %s", zone, projectID)

	isValid := internal_gcp.IsValidGCPLocation(zone)
	if !isValid {
		return fmt.Errorf("zone %s is not valid", zone)
	}

	l.Debugf("Successfully validated zone %s", zone)
	return nil
}

func (c *LiveGCPClient) CheckFirewallRuleExists(
	ctx context.Context,
	projectID, ruleName string,
) error {
	l := logger.Get()
	l.Debugf("Checking if firewall rule %s exists in project %s", ruleName, projectID)

	req := &computepb.GetFirewallRequest{
		Project:  projectID,
		Firewall: ruleName,
	}

	_, err := c.firewallsClient.Get(ctx, req)
	if err != nil {
		if isNotFoundError(err) {
			return fmt.Errorf("firewall rule %s does not exist", ruleName)
		}
		return fmt.Errorf("failed to check firewall rule existence: %v", err)
	}

	return nil
}

func (c *LiveGCPClient) ValidateMachineType(
	ctx context.Context,
	machineType, location string,
) (bool, error) {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return false, fmt.Errorf("global model or deployment is nil")
	}
	projectID := m.Deployment.GetProjectID()
	l.Debugf("Validating machine type %s in location %s", machineType, location)

	req := &computepb.GetMachineTypeRequest{
		Project:     projectID,
		Zone:        location,
		MachineType: machineType,
	}

	_, err := c.machineTypesClient.Get(ctx, req)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to validate machine type: %v", err)
	}

	return true, nil
}

func (c *LiveGCPClient) GetVMExternalIP(
	ctx context.Context,
	vmName string,
	locationData map[string]string,
) (string, error) {
	projectID := locationData["projectID"]
	zone := locationData["zone"]

	if projectID == "" || zone == "" {
		return "", fmt.Errorf("projectID or zone is not set")
	}

	l := logger.Get()
	l.Infof(
		"Getting external IP address for VM %s in project %s and zone %s",
		vmName,
		projectID,
		zone,
	)

	req := &computepb.GetInstanceRequest{
		Project:  projectID,
		Zone:     zone,
		Instance: vmName,
	}

	instance, err := c.computeClient.Get(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get VM: %v", err)
	}

	return *instance.NetworkInterfaces[0].AccessConfigs[0].NatIP, nil
}

func (c *LiveGCPClient) GetVMZone(ctx context.Context, projectID, vmName string) (string, error) {
	l := logger.Get()
	l.Debugf("Getting zone for VM %s in project %s", vmName, projectID)

	instance, err := c.computeClient.Get(ctx, &computepb.GetInstanceRequest{
		Project:  projectID,
		Instance: vmName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get VM instance: %v", err)
	}

	zone := instance.Zone
	if zone == nil {
		return "", fmt.Errorf("zone not found for VM instance %s", vmName)
	}

	zoneStr := strings.TrimPrefix(*zone, "zones/")

	l.Debugf("Found zone %s for VM %s", zoneStr, vmName)
	return zoneStr, nil
}
