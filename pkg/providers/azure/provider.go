package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/globals"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/spf13/viper"
)

// AzureProvider wraps the Azure deployment functionality
type AzureProviderer interface {
	GetClient() AzureClient
	SetClient(client AzureClient)
	GetConfig() *viper.Viper
	SetConfig(config *viper.Viper)

	StartResourcePolling(ctx context.Context, done chan<- struct{})
	DeployResources(ctx context.Context) error
	FinalizeDeployment(ctx context.Context) error
	DestroyResources(ctx context.Context, resourceGroupName string) error
}

type AzureProvider struct {
	Client AzureClient
	Config *viper.Viper
}

var AzureProviderFunc = NewAzureProvider

// NewAzureProvider creates a new AzureProvider instance
func NewAzureProvider() (AzureProviderer, error) {
	config := viper.GetViper()
	if !config.IsSet("azure") {
		return nil, fmt.Errorf("azure configuration is required")
	}

	if !config.IsSet("azure.subscription_id") {
		return nil, fmt.Errorf("azure.subscription_id is required")
	}

	subscriptionID := config.GetString("azure.subscription_id")
	client, err := NewAzureClientFunc(subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}

	return &AzureProvider{
		Client: client,
		Config: config,
	}, nil
}

func (p *AzureProvider) GetClient() AzureClient {
	return p.Client
}

func (p *AzureProvider) SetClient(client AzureClient) {
	p.Client = client
}

func (p *AzureProvider) GetConfig() *viper.Viper {
	return p.Config
}

func (p *AzureProvider) SetConfig(config *viper.Viper) {
	p.Config = config
}

func (p *AzureProvider) DestroyResources(ctx context.Context, resourceGroupName string) error {
	return p.Client.DestroyResourceGroup(ctx, resourceGroupName)
}

// Updates the state machine with the latest resource state
func (p *AzureProvider) ListAllResourcesInSubscription(ctx context.Context,
	subscriptionID string,
	tags map[string]*string) error {
	l := logger.Get()

	err := p.Client.ListAllResourcesInSubscription(ctx,
		subscriptionID,
		tags)
	if err != nil {
		l.Errorf("Failed to query Azure resources: %v", err)
		return fmt.Errorf("failed to query resources: %v", err)
	}

	l.Debugf("Azure Resource Graph response - done listing resources.")

	return nil
}

func (p *AzureProvider) StartResourcePolling(ctx context.Context, done chan<- struct{}) {
	l := logger.Get()
	disp := display.GetGlobalDisplay()

	l.Debug("Starting StartResourcePolling")

	statusTicker := time.NewTicker(globals.MillisecondsBetweenUpdates * time.Millisecond)
	resourceTicker := time.NewTicker(globals.NumberOfSecondsToProbeResourceGroup * time.Second)
	defer statusTicker.Stop()
	defer resourceTicker.Stop()

	resourceStates := make(map[string]string)

	for {
		select {
		case <-statusTicker.C:
			p.updateStatus(disp)
		case <-resourceTicker.C:
			err := p.PollAndUpdateResources(ctx)
			if err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
			}
			
			// Update and display resource states
			deployment := GetGlobalDeployment()
			for _, resource := range deployment.Resources {
				currentState := resourceStates[resource.ID]
				if currentState != resource.Status {
					resourceStates[resource.ID] = resource.Status
					resourceType := getResourceTypeAbbreviation(resource.Type)
					disp.UpdateStatus(&models.Status{
						ID:     resource.ID,
						Type:   resourceType,
						Status: fmt.Sprintf("%s %s", resourceType, resource.Status),
					})
				}
			}
		case <-ctx.Done():
			l.Debug("Context done, exiting resource polling")
			close(done)
			return
		}
	}
}

func getResourceTypeAbbreviation(resourceType string) string {
	switch {
	case strings.Contains(resourceType, "networkSecurityGroups"):
		return "NSG"
	case strings.Contains(resourceType, "publicIPAddresses"):
		return "PIP"
	case strings.Contains(resourceType, "virtualNetworks"):
		return "VNET"
	case strings.Contains(resourceType, "subnets"):
		return "SNET"
	case strings.Contains(resourceType, "networkInterfaces"):
		return "NIC"
	case strings.Contains(resourceType, "virtualMachines"):
		return "VM"
	default:
		return "UNK"
	}
}

func (p *AzureProvider) updateStatus(disp *display.Display) {
	l := logger.Get()
	allMachinesComplete := true
	dep := GetGlobalDeployment()
	for _, machine := range dep.Machines {
		if machine.Status != models.MachineStatusComplete {
			allMachinesComplete = false
		}
		if machine.Status == models.MachineStatusComplete {
			continue
		}
		disp.UpdateStatus(&models.Status{
			ID: machine.Name,
			ElapsedTime: time.Duration(
				time.Since(machine.StartTime).
					Milliseconds() /
					1000, //nolint:gomnd // Divide by 1000 to convert milliseconds to seconds
			),
		})
	}
	if allMachinesComplete {
		l.Debug("All machines complete, resource polling will stop")
	}
}

func parseNetworkProperties(properties map[string]interface{}, propType string) map[string]interface{} {
	props := make(map[string]interface{})

	// Always include provisioningState if available
	if provisioningState, ok := properties["provisioningState"].(string); ok {
		props["provisioningState"] = provisioningState
	}

	// Define a map of property types to their parsing functions
	parsers := map[string]func(interface{}) interface{}{
		"NSG":  func(i interface{}) interface{} { return parseSecurityRules(i) },
		"PIP":  func(i interface{}) interface{} { return i },
		"VNET": func(i interface{}) interface{} { return parseAddressSpace(i) },
		"NIC":  func(i interface{}) interface{} { return parseIPConfigurations(i) },
	}

	// Parse specific properties based on the type
	if parser, ok := parsers[propType]; ok {
		key := map[string]string{
			"NSG":  "securityRules",
			"PIP":  "ipAddress",
			"VNET": "addressSpace",
			"NIC":  "ipConfigurations",
		}[propType]
		props[key] = parser(properties[key])
	}

	// If no specific properties were parsed, return all properties
	if len(props) == 1 && props["provisioningState"] != nil {
		return properties
	}

	return props
}

func parseNetworkRules[T any](items interface{}, parseItem func(map[string]interface{}) T) []T {
	itemSlice, ok := items.([]interface{})
	if !ok {
		return nil
	}

	parsedItems := make([]T, 0, len(itemSlice))
	for _, item := range itemSlice {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		parsedItems = append(parsedItems, parseItem(itemMap))
	}
	return parsedItems
}

func parseSecurityRules(rules interface{}) []*armnetwork.SecurityRule {
	return parseNetworkRules(rules, func(ruleMap map[string]interface{}) *armnetwork.SecurityRule {
		ruleProps, ok := ruleMap["properties"].(map[string]interface{})
		if !ok {
			return nil
		}
		return &armnetwork.SecurityRule{
			Name: utils.ToPtr(ruleMap["name"].(string)),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 (*armnetwork.SecurityRuleProtocol)(utils.ToPtr(ruleProps["protocol"].(string))),
				SourcePortRange:          utils.ToPtr(ruleProps["sourcePortRange"].(string)),
				DestinationPortRange:     utils.ToPtr(ruleProps["destinationPortRange"].(string)),
				SourceAddressPrefix:      utils.ToPtr(ruleProps["sourceAddressPrefix"].(string)),
				DestinationAddressPrefix: utils.ToPtr(ruleProps["destinationAddressPrefix"].(string)),
				Access:                   (*armnetwork.SecurityRuleAccess)(utils.ToPtr(ruleProps["access"].(string))),
				Priority:                 utils.ToPtr(int32(ruleProps["priority"].(float64))),
				Direction:                (*armnetwork.SecurityRuleDirection)(utils.ToPtr(ruleProps["direction"].(string))),
			},
		}
	})
}

func parseAddressSpace(addressSpace interface{}) map[string]interface{} {
	space, ok := addressSpace.(map[string]interface{})
	if !ok {
		return nil
	}

	addressPrefixes, ok := space["addressPrefixes"].([]interface{})
	if !ok {
		return nil
	}

	return map[string]interface{}{
		"addressPrefixes": parseStringSlice(addressPrefixes),
	}
}

func parseIPConfigurations(configs interface{}) []*armnetwork.InterfaceIPConfiguration {
	return parseNetworkRules(configs, func(configMap map[string]interface{}) *armnetwork.InterfaceIPConfiguration {
		props, ok := configMap["properties"].(map[string]interface{})
		if !ok {
			return nil
		}
		return &armnetwork.InterfaceIPConfiguration{
			Name: utils.ToPtr(configMap["name"].(string)),
			Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
				PrivateIPAddress: utils.ToPtr(props["privateIPAddress"].(string)),
			},
		}
	})
}

func parseStringSlice(slice []interface{}) []*string {
	result := make([]*string, 0, len(slice))
	for _, item := range slice {
		if str, ok := item.(string); ok {
			result = append(result, &str)
		}
	}
	return result
}

var _ AzureProviderer = &AzureProvider{}
