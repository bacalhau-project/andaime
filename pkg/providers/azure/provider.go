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

	for {
		select {
		case <-statusTicker.C:
			p.updateStatus(disp)
		case <-resourceTicker.C:
			err := p.PollAndUpdateResources(ctx)
			if err != nil {
				l.Errorf("Failed to poll and update resources: %v", err)
			}
		case <-ctx.Done():
			l.Debug("Context done, exiting resource polling")
			close(done)
			return
		}
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

	// Parse specific properties based on the type
	switch propType {
	case "NSG":
		if securityRules, ok := properties["securityRules"].([]interface{}); ok {
			props["securityRules"] = parseSecurityRules(securityRules)
		}
	case "PIP":
		if ipAddress, ok := properties["ipAddress"].(string); ok {
			props["ipAddress"] = ipAddress
		}
	case "VNET":
		if addressSpace, ok := properties["addressSpace"].(map[string]interface{}); ok {
			if addressPrefixes, ok := addressSpace["addressPrefixes"].([]interface{}); ok {
				props["addressSpace"] = map[string]interface{}{
					"addressPrefixes": parseStringSlice(addressPrefixes),
				}
			}
		}
	case "NIC":
		if ipConfigurations, ok := properties["ipConfigurations"].([]interface{}); ok {
			props["ipConfigurations"] = parseIPConfigurations(ipConfigurations)
		}
	}

	// If no specific properties were parsed, return all properties
	if len(props) == 1 && props["provisioningState"] != nil {
		return properties
	}

	return props
}

func parseSecurityRules(rules []interface{}) []*armnetwork.SecurityRule {
	securityRules := make([]*armnetwork.SecurityRule, 0, len(rules))
	for _, rule := range rules {
		if ruleMap, ok := rule.(map[string]interface{}); ok {
			props, ok := ruleMap["properties"].(map[string]interface{})
			if !ok {
				continue
			}
			securityRules = append(securityRules, &armnetwork.SecurityRule{
				Name: utils.ToPtr(ruleMap["name"].(string)),
				Properties: &armnetwork.SecurityRulePropertiesFormat{
					Protocol: (*armnetwork.SecurityRuleProtocol)(
						utils.ToPtr(props["protocol"].(string)),
					),
					SourcePortRange:      utils.ToPtr(props["sourcePortRange"].(string)),
					DestinationPortRange: utils.ToPtr(props["destinationPortRange"].(string)),
					SourceAddressPrefix:  utils.ToPtr(props["sourceAddressPrefix"].(string)),
					DestinationAddressPrefix: utils.ToPtr(
						props["destinationAddressPrefix"].(string),
					),
					Access: (*armnetwork.SecurityRuleAccess)(
						utils.ToPtr(props["access"].(string)),
					),
					Priority: utils.ToPtr(int32(props["priority"].(float64))),
					Direction: (*armnetwork.SecurityRuleDirection)(
						utils.ToPtr(props["direction"].(string)),
					),
				},
			})
		}
	}
	return securityRules
}

func parseIPConfigurations(configs []interface{}) []*armnetwork.InterfaceIPConfiguration {
	ipConfigurations := make([]*armnetwork.InterfaceIPConfiguration, 0, len(configs))
	for _, config := range configs {
		if configMap, ok := config.(map[string]interface{}); ok {
			props, ok := configMap["properties"].(map[string]interface{})
			if !ok {
				continue
			}

			ipConfigurations = append(ipConfigurations, &armnetwork.InterfaceIPConfiguration{
				Name: utils.ToPtr(configMap["name"].(string)),
				Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
					PrivateIPAddress: utils.ToPtr(props["privateIPAddress"].(string)),
				},
			})
		}
	}
	return ipConfigurations
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
