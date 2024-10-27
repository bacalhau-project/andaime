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

func (c *LiveGCPClient) CreateVPCNetwork(
	ctx context.Context,
	projectID string,
	networkName string,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	if projectID == "" {
		return fmt.Errorf("project ID is not set")
	}

	b := backoff.NewExponentialBackOff()
	// Allow up to 5 minutes for network/firewall operations
	b.MaxElapsedTime = 5 * time.Minute
	// Start retrying after 10 seconds
	b.InitialInterval = 10 * time.Second
	// Cap retry interval at 30 seconds
	b.MaxInterval = 30 * time.Second

	operation := func() error {
		// Verify network exists and is ready before attempting firewall changes
		network, err := c.networksClient.Get(ctx, &computepb.GetNetworkRequest{
			Project: projectID,
			Network: networkName,
		})
		if err != nil {
			l.Debugf("Network %s not found: %v", networkName, err)
			return fmt.Errorf("network not found: %w", err)
		}
		if network.SelfLink == nil || *network.SelfLink == "" {
			l.Debugf("Network %s not fully provisioned yet", networkName)
			return fmt.Errorf("network %s not fully provisioned yet", networkName)
		}
		// First check if network exists
		_, err := c.networksClient.Get(ctx, &computepb.GetNetworkRequest{
			Project: projectID,
			Network: networkName,
		})
		if err == nil {
			l.Debugf("Network %s already exists, skipping creation", networkName)
			return nil
		}
		if !isNotFoundError(err) {
			return fmt.Errorf("failed to check network existence: %v", err)
		}

		// Create the network if it doesn't exist
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
			return fmt.Errorf("failed to create network: %v", err)
		}

		// Wait for the operation to complete
		err = op.Wait(ctx)
		if err != nil {
			return fmt.Errorf("failed to wait for network creation: %v", err)
		}

		l.Infof(
			"VPC network %s created successfully. Waiting 10 seconds for network propagation...",
			networkName,
		)
		time.Sleep(10 * time.Second) //nolint:mnd
		l.Infof("Network propagation wait complete for %s", networkName)
		return nil
	}

	err := backoff.Retry(operation, b)
	if err != nil {
		return fmt.Errorf("failed to create VPC network after retries: %v", err)
	}

	// Mark firewall resource as complete for all VMs
	model := display.GetGlobalModelFunc()
	if model != nil && model.Deployment != nil {
		for _, machine := range model.Deployment.GetMachines() {
			machine.SetMachineResourceState(
				"compute.googleapis.com/Firewall",
				models.ResourceStateSucceeded,
			)
			model.UpdateStatus(models.NewDisplayStatusWithText(
				machine.GetName(),
				models.GCPResourceTypeFirewall,
				models.ResourceStateSucceeded,
				"Firewall rules configured",
			))
		}
	}

	return nil
}

// Add this function to first cleanup default rules
func (c *LiveGCPClient) CleanupFirewallRules(
	ctx context.Context,
	projectID string,
	networkName string,
) error {
	l := logger.Get()
	l.Infof(
		"Cleaning up default firewall rules in project: %s for network: %s",
		projectID,
		networkName,
	)

	// List existing firewall rules
	req := &computepb.ListFirewallsRequest{
		Project: projectID,
		Filter: to.Ptr(
			fmt.Sprintf(`network="projects/%s/global/networks/%s"`, projectID, networkName),
		),
	}

	it := c.firewallsClient.List(ctx, req)
	for {
		rule, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to list firewall rules: %v", err)
		}

		// Skip SSH rule (port 22)
		if rule.Allowed != nil && len(rule.Allowed) > 0 {
			for _, allowed := range rule.Allowed {
				if allowed.Ports != nil && len(allowed.Ports) > 0 {
					if allowed.Ports[0] == "22" {
						l.Infof("Keeping SSH firewall rule: %s", *rule.Name)
						continue
					}
				}
			}
		}

		// Delete non-SSH rule
		l.Infof("Deleting firewall rule: %s", *rule.Name)
		_, err = c.firewallsClient.Delete(ctx, &computepb.DeleteFirewallRequest{
			Project:  projectID,
			Firewall: *rule.Name,
		})
		if err != nil {
			return fmt.Errorf("failed to delete firewall rule %s: %v", *rule.Name, err)
		}
	}

	return nil
}

// Update the required ports to include our specific needs
var requiredPorts = []struct {
	Port        int
	Protocol    string
	Description string
	Priority    int32
}{
	{22, "tcp", "SSH", 1000},
	{1234, "tcp", "Custom Port 1234", 1001},
	{1235, "tcp", "Custom Port 1235", 1002},
	{4222, "tcp", "NATS", 1003},
}

func (c *LiveGCPClient) CreateFirewallRules(
	ctx context.Context,
	projectID string,
	networkName string,
) error {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return fmt.Errorf("global model or deployment is nil")
	}

	l.Infof(
		"Creating firewall rules in project: %s for network: %s",
		projectID,
		networkName,
	)

	// First cleanup existing rules
	if err := c.CleanupFirewallRules(ctx, projectID, networkName); err != nil {
		return fmt.Errorf("failed to cleanup existing firewall rules: %v", err)
	}
	// Add any extra ports from config
	ports := make([]struct {
		Port        int
		Protocol    string
		Description string
		Priority    int32
	}, len(requiredPorts))
	copy(ports, requiredPorts)

	if extraPorts := viper.GetIntSlice("gcp.allowed_ports"); len(extraPorts) > 0 {
		for _, port := range extraPorts {
			ports = append(ports, struct {
				Port        int
				Protocol    string
				Description string
				Priority    int32
			}{port, "tcp", "Custom", 1004})
		}
	}

	// Create both ingress and egress rules for each port
	for _, portInfo := range ports {
		for _, direction := range []string{"INGRESS", "EGRESS"} {
			// Create allow rule for required port
			ruleName := fmt.Sprintf(
				"allow-%s-%d-%s",
				strings.ToLower(direction),
				portInfo.Port,
				portInfo.Protocol,
			)
			l.Infof(
				"Creating firewall rule: %s for port %d (%s)",
				ruleName,
				portInfo.Port,
				portInfo.Description,
			)

			for _, machine := range m.Deployment.GetMachines() {
				m.UpdateStatus(models.NewDisplayStatusWithText(
					machine.GetName(),
					models.GCPResourceTypeFirewall,
					models.ResourceStatePending,
					fmt.Sprintf(
						"Creating %s FW rule for %s port %d",
						direction,
						portInfo.Description,
						portInfo.Port,
					),
				))
			}

			b := backoff.NewExponentialBackOff()
			b.MaxElapsedTime = 5 * time.Minute
			b.InitialInterval = 10 * time.Second
			b.MaxInterval = 30 * time.Second

			operation := func() error {
				// Check if network is ready first
				network, err := c.networksClient.Get(ctx, &computepb.GetNetworkRequest{
					Project: projectID,
					Network: networkName,
				})
				if err != nil {
					return fmt.Errorf("network not found: %w", err)
				}
				if network.SelfLink == nil || *network.SelfLink == "" {
					return fmt.Errorf("network not fully provisioned yet")
				}
				firewallRule := &computepb.Firewall{
					Name: &ruleName,
					Network: to.Ptr(
						fmt.Sprintf("projects/%s/global/networks/%s", projectID, networkName),
					),
					Allowed: []*computepb.Allowed{
						{
							IPProtocol: to.Ptr(portInfo.Protocol),
							Ports:      []string{strconv.Itoa(portInfo.Port)},
						},
					},
					Direction: to.Ptr(direction),
					Priority:  to.Ptr(portInfo.Priority),
					Description: to.Ptr(
						fmt.Sprintf(
							"%s port %d (%s)",
							direction,
							portInfo.Port,
							portInfo.Description,
						),
					),
					TargetTags: []string{"andaime-node"},
				}

				// Set appropriate ranges based on direction
				if direction == "INGRESS" {
					firewallRule.SourceRanges = []string{"0.0.0.0/0"}
				} else {
					firewallRule.DestinationRanges = []string{"0.0.0.0/0"}
				}

				// Attempt to create the firewall rule
				op, err := c.firewallsClient.Insert(ctx, &computepb.InsertFirewallRequest{
					Project:          projectID,
					FirewallResource: firewallRule,
				})
				if err != nil {
					if strings.Contains(err.Error(), "The resource 'projects") && strings.Contains(err.Error(), "is not ready") {
						l.Debugf("Network %s is not ready yet, will retry...", networkName)
						return err
					} else if strings.Contains(err.Error(), "already exists") {
						l.Infof(
							"Firewall rule %s already exists, verifying configuration...",
							ruleName,
						)
						// Verify existing rule
						existingRule, getErr := c.firewallsClient.Get(
							ctx,
							&computepb.GetFirewallRequest{
								Project:  projectID,
								Firewall: ruleName,
							},
						)
						if getErr != nil {
							l.Warnf(
								"Failed to verify existing firewall rule %s: %v",
								ruleName,
								getErr,
							)
						} else {
							l.Infof("Verified existing firewall rule %s - Direction: %s, Port: %d",
								ruleName, *existingRule.Direction, portInfo.Port)
						}
						return nil
					}
					if strings.Contains(err.Error(), "Compute Engine API has not been used") {
						l.Debugf("Compute Engine API is not yet active. Retrying... (FW Rules)")
						return err
					}
					return backoff.Permanent(fmt.Errorf("failed to create firewall rule: %v", err))
				}

				// Wait for the operation to complete
				if err := op.Wait(ctx); err != nil {
					if strings.Contains(err.Error(), "not ready") {
						l.Infof("Network %s is still being configured, retrying...", networkName)
						return err
					}
					return fmt.Errorf("failed waiting for firewall rule creation: %w", err)
				}
				// Wait for the firewall rule operation to complete
				if err := op.Wait(ctx); err != nil {
					if strings.Contains(err.Error(), "not ready") {
						l.Debugf("Network %s still being configured, will retry...", networkName)
						return err
					}
					return fmt.Errorf("failed waiting for firewall rule creation: %w", err)
				}
				l.Debugf("Successfully created firewall rule")
				return nil
			}

			err := backoff.Retry(operation, b)
			if err != nil {
				l.Errorf("Failed to create firewall rule %s after retries: %v", ruleName, err)
				return fmt.Errorf(
					"failed to create firewall rule %s after retries: %v",
					ruleName,
					err,
				)
			}
			l.Infof("Firewall rule %s created or verified", ruleName)

			for _, machine := range m.Deployment.GetMachines() {
				m.UpdateStatus(models.NewDisplayStatusWithText(
					machine.GetName(),
					models.GCPResourceTypeFirewall,
					models.ResourceStateRunning,
					fmt.Sprintf(
						"Created or verified %s FW rule for %s port %d",
						direction,
						portInfo.Description,
						portInfo.Port,
					),
				))
			}
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

	// Insert the address with network configuration
	op, err := c.addressesClient.Insert(ctx, &computepb.InsertAddressRequest{
		Project: projectID,
		Region:  region,
		AddressResource: &computepb.Address{
			Name:        to.Ptr(addressName),
			AddressType: address.AddressType,
			Region:      to.Ptr(region),
			NetworkTier: to.Ptr("PREMIUM"),
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
	networkName string,
) (*computepb.Instance, error) {
	// Validate input and prerequisites
	if err := c.validateCreateVMInput(ctx, projectID, machine); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Prepare VM configuration
	instance, err := c.prepareVMInstance(ctx, projectID, machine, ip, networkName)
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
	_ context.Context,
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
	_ context.Context,
	projectID string,
	machine models.Machiner,
	ip *computepb.Address,
	networkName string,
) (*computepb.Instance, error) {
	l := logger.Get()
	l.Debugf("Preparing VM instance %s in project %s", machine.GetName(), projectID)

	if projectID == "" {
		return nil, fmt.Errorf("projectID is not set in prepareVMInstance")
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
		Tags: &computepb.Tags{
			Items: []string{"andaime-node"},
		},
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
				Network: proto.String(
					fmt.Sprintf("projects/%s/global/networks/%s", projectID, networkName),
				),
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
	projectID string,
	machineType, location string,
) (bool, error) {
	l := logger.Get()
	m := display.GetGlobalModelFunc()
	if m == nil || m.Deployment == nil {
		return false, fmt.Errorf("global model or deployment is nil")
	}
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
